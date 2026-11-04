# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... -run TestBuiltinLazinessAudit -count=1
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — lazy `map`/`genList` split; fixpoint overlay path fixed |
| #13 | Ladder / goldens | **blocked** on `stdenv.cc.isGNU` in `all-packages` |

## Fixpoint / xorg overlay (727)

**Root cause:** `lib.extends` applies an overlay whose body is `{ … } // (mapNamesToAttrs (setLicense …) names)`. WHNF of that `//` runs `map f names`; eager `builtins.map` applied `setLicense` for every name immediately. Each `setLicense` forced `super.${name}` (e.g. `callPackage` at xorg `default.nix:374`), which forced the fixpoint `self` at `customisation.nix:727` while it was still blackholed.

**Fix:** **`builtins.map` (and `concatMap` list elements) defer function application** via `deferredApplyCall` — same laziness as Nix. `genList` stays eager so stdenv `booter.nix` / `imap1` bootstrapping is unchanged.

**Reproducer:** `testdata/oracle/extends_overlay_map_super.nix` (PASS on Nix 2.24.9). Tests: `TestExtendsOverlayMapNamesDoesNotForceAllSuperDuringFixpoint`, `TestLetSelfRecursiveBindsOneThunk`, laziness audit case `extends overlay mapNames super`.

No special cases for `self = f self // rhs` or blackhole peeking.

## Blocker: `stdenv.shell` missing — **fixed**

**Root cause:** Nix’s `derivation` is **not** `builtins.derivationStrict`. It is `primops/derivation.nix`, which returns `drvAttrs // … // { drvPath, outPath, type, … }` so argument attributes (`shell`, `initialPath`, …) stay on the stdenv attrset. goonix had `builtins.derivation` identical to `derivationStrict`, whose result only has `type`, `name`, `system`, `builder`, `drvPath`, `outPath`, `args`, and `env`. After `derivation { inherit shell; … } // { mkDerivation; … }`, `shell` was absent, so `make-derivation.nix:426` (`attrs.realBuilder or stdenv.shell`, parsed as **SelectOr**, not cursed `or`) failed on the default branch.

**Fix (eval/):** `builtins.derivation` follows Nix `primops/derivation.nix`: lazy `drvAttrs`, per-output thunks, `all`, head `//` overlay (`derivation_builtin.go` / `buildNixDerivation`). Tests: `TestDerivationMatchesNixOutStructure`, `TestDerivationExposesEnvAttrsAtTopLevel` (shell).

Import now proceeds past `mkDerivation`; next failure (separate): `all-packages.nix:6681` (`stdenv.cc.isGNU`).

## Blocker: `all-packages.nix:6681` (`stdenv.cc.isGNU`)

**Symptom:** `(builtins.tryEval pkgs).success` is **false** in goonix (error at `6681:8`, `stdenv.cc` null in `gccStdenv`); Nix 2.24.9 returns **true**.

**Oracle:** `builtins.typeOf (import nixpkgs {…}).stdenv.cc` → `"set"` (Nix 2.24.9).

**Force stack (import / `tryEval pkgs`):** Linux boot `dfold` → stage `assert isBuiltByNixpkgsCompiler prevStage.glibc` → forces `pkgs.glibc` → `callPackage … { stdenv = gccStdenv; }` (`all-packages.nix:10294`) → `makeOverridable` (`customisation.nix:165`) → **`gccStdenv` at `:6681`** (`if stdenv.cc.isGNU`) with **`stdenv.cc` null** in goonix.

**Boot / dfold audit (goonix vs Nix 2.24.9):**

| Oracle | Result |
|--------|--------|
| `testdata/oracle/dfold_stage_order.nix` (3 stages) | **match** |
| `testdata/oracle/dfold_nine_stage_order.nix` (9 stages, linux count) | **match** |
| `testdata/oracle/dfold_succ_binding.nix` (`succ = go cur (n+1)`) | **match** |
| `testdata/oracle/list_boot_primitives.nix` (`imap1`, `reverseList`, `genList`, `foldl'`, …) | **match** |
| `testdata/oracle/linux_boot_stage_dfold.nix` (real stageFuns + stub merge) | Nix **cannot** strict-eval chain (asserts); goonix logs `chainByDfoldIndex` in test |

**Eval change:** `builtins.genList` is **eager** again (map/`concatMap` stay lazy via `deferredApplyCall`) so boot `imap1`/`dfold` list cells match Nix bootstrapping expectations.

**Still to fix:** why real boot + `allPackages` forces `gccStdenv` with null `stdenv.cc` under `tryEval pkgs` while Nix does not (stage `prevStage` / cc-wrapper / laziness on the glibc assert path).

**Probe (Nix 2.24.9 vs goonix):** `testdata/oracle/import_head_probe.nix` — `{ tryPkgs, hasOverrides, ccType, tryStdenv }`. Nix: all true / `"set"`. Goonix: `tryPkgs=false` @6681.

**Boot-chain bisect (`testdata/oracle/boot_chain_bisect_probe.nix`, `TestBootChainBisectMatchesNix`):** Walk `pkgs.stdenv.__bootPackages…` with `pick = d` per depth. Nix: depth 0–7 `stageTryOk`, final has `overrides`, `ccType=set`. Goonix: **depth 0** fails forcing `pkgs.stdenv` (same 6681 stack); depth ≥1 `stageTryOk=false` (cannot walk `__bootPackages` until depth 0 passes). **First divergence = depth 0 (final import stdenv head), not a deeper `linux/default.nix` stage index** until go can WHNF final `pkgs.stdenv`.

**Root (unchanged):** `tryEval pkgs` WHNF runs boot `dfold` → stage `assert isBuiltByNixpkgsCompiler prevStage.glibc` → `callPackage { stdenv = gccStdenv }` → `with pkgs` `stdenv.cc.isGNU` with **`stdenv.cc` null** in goonix. Synthetic `lib.fix` + `with pkgs` + `stdenvOverrides` matches Nix (`TestWithPkgsFixpointProbeMatchesNix`); failure is real boot + full `all-packages` on the assert/glibc path, not abstract `extends`/`//` folder merge (`TestStdenvFolderMergePreservesOverrides`).

## Reverted (wrong semantics)

- **`//` empty-LHS fallback** on infinite recursion — reverted.
- **Lazy `applyLambda` (deferred body)** — broke WHNF at call sites; reverted.

## Laziness (current)

- Lazy list elements, lazy call arguments, lazy `zipAttrsWith`, lazy **`map`** / **`concatMap`** / **`genList`** elements (`deferredApplyCall`); **`tryEval`** WHNF-only (not `deepSeq`).
- **`with`**: deferred attribute-set thunk; `lookupEnv` propagates `forceWHNF` errors.
- **`TestBuiltinLazinessAudit`**: builtins + forms + xorg-shaped probes + extends/mapNames overlay.

Debug: `GOONIX_TRACE_FORCE=1` on infinite recursion.

## Tests

- **Green**: CGO, `./lang/...`, `TestEvalModulesTopLevelConfigNix`, `TestBuiltinLazinessAudit`, `TestExtendsOverlayMapNamesDoesNotForceAllSuperDuringFixpoint`, `TestLetSelfRecursiveBindsOneThunk`, `TestDerivationExposesEnvAttrsAtTopLevel`, `TestPendingMergeWithSelfAndFixpointIntersectAttrs`
- **Red**: full `import <nixpkgs>`, hello golden, ladder (`all-packages.nix:6681` / `stdenv.cc.isGNU`)
