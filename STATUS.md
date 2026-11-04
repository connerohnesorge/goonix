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
| #13 | Ladder / goldens | **blocked** on `stdenv.shell` during `mkDerivation` |

## Fixpoint / xorg overlay (727)

**Root cause:** `lib.extends` applies an overlay whose body is `{ … } // (mapNamesToAttrs (setLicense …) names)`. WHNF of that `//` runs `map f names`; eager `builtins.map` applied `setLicense` for every name immediately. Each `setLicense` forced `super.${name}` (e.g. `callPackage` at xorg `default.nix:374`), which forced the fixpoint `self` at `customisation.nix:727` while it was still blackholed.

**Fix:** **`builtins.map` (and `concatMap` list elements) defer function application** via `deferredApplyCall` — same laziness as Nix. `genList` stays eager so stdenv `booter.nix` / `imap1` bootstrapping is unchanged.

**Reproducer:** `testdata/oracle/extends_overlay_map_super.nix` (PASS on Nix 2.24.9). Tests: `TestExtendsOverlayMapNamesDoesNotForceAllSuperDuringFixpoint`, `TestLetSelfRecursiveBindsOneThunk`, laziness audit case `extends overlay mapNames super`.

No special cases for `self = f self // rhs` or blackhole peeking.

## Blocker: `stdenv.shell` missing — **fixed**

**Root cause:** Nix’s `derivation` is **not** `builtins.derivationStrict`. It is `primops/derivation.nix`, which returns `drvAttrs // … // { drvPath, outPath, type, … }` so argument attributes (`shell`, `initialPath`, …) stay on the stdenv attrset. goonix had `builtins.derivation` identical to `derivationStrict`, whose result only has `type`, `name`, `system`, `builder`, `drvPath`, `outPath`, `args`, and `env`. After `derivation { inherit shell; … } // { mkDerivation; … }`, `shell` was absent, so `make-derivation.nix:426` (`attrs.realBuilder or stdenv.shell`, parsed as **SelectOr**, not cursed `or`) failed on the default branch.

**Fix (eval/):** `builtinDerivation` calls `derivationStrict` then `mergeAttrBindMaps(drvAttrs, strict)` (rhs wins on `drvPath` / `type` like Nix). Test: `TestDerivationExposesEnvAttrsAtTopLevel`.

Not caused by lazy `map`, `genList` / `imap1` booter, or `evalOrFallback`.

Import now proceeds past `mkDerivation`; next failure (separate): `all-packages.nix:6681` (`stdenv.cc.isGNU` — WHNF / override typing).

## Reverted (wrong semantics)

- **`//` empty-LHS fallback** on infinite recursion — reverted.
- **Lazy `applyLambda` (deferred body)** — broke WHNF at call sites; reverted.

## Laziness (current)

- Lazy list elements, lazy call arguments, lazy `zipAttrsWith`, lazy **`map`** / **`concatMap`** elements; **eager `genList`**.
- **`with`**: deferred attribute-set thunk; `lookupEnv` propagates `forceWHNF` errors.
- **`TestBuiltinLazinessAudit`**: builtins + forms + xorg-shaped probes + extends/mapNames overlay.

Debug: `GOONIX_TRACE_FORCE=1` on infinite recursion.

## Tests

- **Green**: CGO, `./lang/...`, `TestEvalModulesTopLevelConfigNix`, `TestBuiltinLazinessAudit`, `TestExtendsOverlayMapNamesDoesNotForceAllSuperDuringFixpoint`, `TestLetSelfRecursiveBindsOneThunk`, `TestPendingMergeWithSelfAndFixpointIntersectAttrs`
- **Red**: full `import <nixpkgs>`, hello golden, ladder (`stdenv.shell` in `make-derivation`)
