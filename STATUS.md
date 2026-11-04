# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

**Hermetic Nix oracle (`internal/oracle`):** every `nix-instantiate` uses `--option substituters '' --option restrict-eval false -I nixpkgs=$PWD/testdata/nixpkgs -I nix=$PWD/testdata/nix` with `NIX_PATH` stripped from the environment. `testdata/nixpkgs/.pinned-rev` must match `oracle.NixpkgsRev`; `lib.version` must be `24.11*`. Regenerate goldens with `scripts/update-goldens.sh` (same flags). **Note:** `hello.drvPath` golden tracks hermetic `--eval` of `pkgs.hello.drvPath` (unchanged at `gciipqhq…` unless pin or hello changes); goonix hello drvPath still diverges until bootstrap/FOD leaves match.

```bash
bash scripts/check-cgo.sh
go test ./eval/... -run TestBuiltinLazinessAudit -count=1
go test ./eval/... ./lang/... -count=1
go test ./eval/... -run 'TestBootStagesTakeBisectMatchesNix|TestImportNixpkgsOnly|TestDerivationStrictRung|TestDerivationLazyStrictUntilDrvPath|TestModulo|TestStdenvNoCCMkDerivation' -count=1
go test ./store/... -run 'TestHashModuloOracle|TestWriteDerivationForHash' -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — lazy `builtins.derivation` / `derivationStrict` until `drvPath`; lazy `map`/`genList` split |
| #13 | Ladder / goldens | **partial** — hello evaluates; drv hash ≠ Nix (remaining mkDerivation / stdenv graph gaps) |

## Bottom-up boot bisect (6681) — **fixed at k=2**

**Method:** `stdenvStages = args: lib.take k (import <nixpkgs/pkgs/stdenv> args)` for `k = 1..11`; compare goonix vs Nix 2.24.9 via `testdata/oracle/boot_stages_take_bisect_probe.nix`.

| k | Stage (linux list) | First divergence |
|---|-------------------|------------------|
| 1 | `__raw` only | None (degenerate row; no stdenv probes by design) |
| 2 | bootstrap-stage0 | **`glibcDrvTryOk`** — goonix forced `pkgs.glibc` via all-packages `callPackage` instead of bootstrap dummy |
| 3+ | bootstrap-stage1… | Was **`tryPkgs`** @6681 until k=2 fixed; now **match** through k=11 |

**Root cause (k=2):** Dynamic attribute names in attrsets (`${localSystem.libc} = …` in stdenv overrides) were dropped because `insertBinding` ignored `bind.Path[0].Expr != nil`.

**Fix:** `eval/attrset.go` — resolve dynamic names at bind time; propagate errors from `populateAttrSet`. **Follow-up:** `builtins.splitVersion` (lib `versions.nix`) for k≥5.

**Tests kept:** `TestBootStagesTakeBisectMatchesNix`, `TestBootStagesTakeFirstDivergence`, `TestBootTake2GlibcPassthruMatchesNix`, `TestDynamicAttrNameFromSetMatchesNix`.

**Import:** `TestImportNixpkgsOnly` **green** (evaluates pinned nixpkgs import without forcing full `pkgs` WHNF).

## derivationStrict ladder (vs Nix 2.24.9)

Oracle per rung: `nix-instantiate --eval -E '(builtins.derivationStrict …).drvPath'` and compare to goonix `AttrsToDerivation` + `SerializeDerivation` (ATerm round-trip via `store.DrvPath`).

| Rung | Status |
|------|--------|
| 1 minimal, 2 args, 3 env, 4 multi-output, 5 __ignoreNulls, 6 coercions, 7 path→inputSrcs, 8 toFile, **9 inputDrvs**, 10 fixed-output (SRI in env) | **match** drvPath (+ ATerm when `.drv` on disk) |

**`hashDerivationModulo` (Nix 2.24.9):** implemented in `store/hash_modulo.go` + ATerm writer `store/hash_aterm.go` (no LGPL copy). Memoized by `.drv` path in `ModuloCache`. FOD branch uses `fixed:out:` + `ContentAddress::printMethodAlgo()` (e.g. flat sha256 → `"sha256"`, not `"flat"`) + Base16 hash + output path. **FOD store paths:** `store/fixed_output_path.go` implements Nix 2.24.9 `makeFixedOutputPath` (recursive `r:sha256` → `hashStrings("source", …)`; flat/other → `sha256("fixed:out:" + methodAlgo + ":" + base16 + ":")` then `output:out`). `ParseFixedOutputHash` decodes SRI when `outputHashAlgo` is empty (`Algo().String()`). Oracle: `store/fixed_output_path_oracle_test.go`, `eval/fetchurl_sri_oracle_test.go` (**green** drvPath).

**Input collection fix (rung 9):** Derivation output strings (`!<drv>!out` context) go to **`inputDrvs` only**, not **`inputSrcs`** (`eval/derivation_env.go` `mergeValueContext`).

**Lazy `builtins.derivation`:** `eval/derivation_builtin.go` — shared `lazyStrictDerivation` thunk; `drvPath` / outputs select from `derivationStrict` without forcing flatten until needed (`TestDerivationLazyStrictUntilDrvPath`).

**Env flatten:** `eval/derivation_env.go` (`fillDerivationFromAttrs`); `derivationValueToGoNix` ensures every output name has an empty env slot before output-path modulo (multi-out).

**Store text paths:** `store/content.go` — `text:sha256:<hex>:<storeDir>:<name>` fingerprint (Nix `toFile`).

**String context:** derivation outputs from `derivationStrict` carry `Built` context on strings for `inputDrvs` when coerced into env.

**Strict cache:** `Evaluator.moduloCache` + `drvStrictByPath` register strict derivations for chained `let` bindings and modulo recursion.

## Recursive drv diff (new)

`store/drvshow` emits the same JSON shape as `nix derivation show --recursive`. `internal/drvdiff` walks paired input derivations by **logical name** (suffix after store hash) and reports the deepest node whose own fields differ while matched inputs agree, plus **missing go closure nodes**.

```bash
NIXPATH=nixpkgs=$PWD/testdata/nixpkgs go run ./cmd/goonix drv-diff \
  --nix-instantiate eval/testdata/stdenv_nocc_inst.nix \
  --go-eval testdata/oracle/stdenv_nocc_mkderivation_probe.nix \
  -I nixpkgs=$PWD/testdata/nixpkgs
```

**Closure export:** `strictDrvs` records every `derivationStrict`; `DerivationShowClosure` walks `inputDrvs` through that map. Hermetic oracle fixed channel bleed (`bash-5.3p20` ghost); drv-diff **missing** count **0** (225=225). **Wrapper fix:** `builtins.substring` with negative length (Nix `removePrefix` uses `substring preLen (-1) str`); empty prefix no longer zeroes `ccName`. Oracle: `TestStdenvCcWrapperNameMatchesNix`, `eval/getname_substring_oracle_test.go`. **Remaining deepest diff:** fetchurl FOD leaves (e.g. `pkg-config-0.29.2.tar.gz` path/hash); hello / stdenv probe drvPaths still differ from Nix.


- **Hello golden:** `TestHelloDrvPathMatchesGolden` still **red** — hello evaluates end-to-end; remaining gap is **`hashDerivationModulo` / closure inputs** vs Nix (not env `{ ... }` coercion).
- **stdenvNoCC:** `TestStdenvNoCCMkDerivationOracleMatchesNix` — **red** (drvPath hash only; same logical derivation).

## Fixpoint / xorg overlay (727)

**Root cause:** `lib.extends` overlay `// mapNamesToAttrs (setLicense …)` + eager `map` forced fixpoint `self` during WHNF.

**Fix:** Lazy **`builtins.map`** / **`concatMap`** via `deferredApplyCall`. **`genList`** stays eager for boot `imap1`/`dfold`.

## Blocker: `stdenv.shell` missing — **fixed**

**Fix:** `builtins.derivation` matches Nix `primops/derivation.nix` (top-level drv attrs + lazy outputs).

## Top-down boot bisect (historical)

`testdata/oracle/boot_chain_bisect_probe.nix` — goonix failed at depth 0 (same 6681 stack). Bottom-up bisect isolated the earlier bootstrap **glibc** override bug (k=2).

## Tests

- **Green:** CGO, `./lang/...`, `TestBuiltinLazinessAudit`, boot dfold oracles, `TestWithPkgsFixpointProbeMatchesNix`, `TestBootStagesTakeBisectMatchesNix`, `TestImportNixpkgsOnly`, derivation strict rungs **1–10**, lazy strict tests, `store` hash-modulo oracles, `TestModuloTwoLevelOutPathVsNix`
- **Red / skip:** `TestHelloDrvPathMatchesGolden`, `TestStdenvNoCCMkDerivationOracleMatchesNix`, some xorg/scope tests (`allowAliases`, paths)

Debug: `GOONIX_TRACE_FORCE=1` logs `derivationStrict` when flatten runs.
