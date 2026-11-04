# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... -run TestBuiltinLazinessAudit -count=1
go test ./eval/... ./lang/... -count=1
go test ./eval/... -run 'TestBootStagesTakeBisectMatchesNix|TestImportNixpkgsOnly|TestDerivationStrictRung|TestDerivationLazyStrictUntilDrvPath' -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — lazy `builtins.derivation` / `derivationStrict` until `drvPath`; lazy `map`/`genList` split |
| #13 | Ladder / goldens | **partial** — hello evaluates; drv hash ≠ Nix (full stdenv graph / input-drv modulo) |

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
| 1 minimal, 2 args, 3 env, 4 multi-output, 5 __ignoreNulls, 6 coercions, 7 path→inputSrcs, 8 toFile, 10 fixed-output (SRI in env) | **match** drvPath (+ ATerm when `.drv` on disk) |
| 9 inputDrvs | **skip** — Nix `pathDerivationModulo` substitutes **masked** input hashes; go-nix `CalculateDrvReplacement` hashes unmasked ATerm (needs export or fork) |

**Lazy `builtins.derivation`:** `eval/derivation_builtin.go` — shared `lazyStrictDerivation` thunk; `drvPath` / outputs select from `derivationStrict` without forcing flatten until needed (`TestDerivationLazyStrictUntilDrvPath`).

**Env flatten:** `eval/derivation_env.go` (`fillDerivationFromAttrs`); `derivationValueToGoNix` ensures every output name has an empty env slot before `CalculateOutputPaths` (multi-out modulo).

**Store text paths:** `store/content.go` — `text:sha256:<hex>:<storeDir>:<name>` fingerprint (Nix `toFile`).

**String context:** derivation outputs from `derivationStrict` carry `Built` context on strings for `inputDrvs` / `inputSrcs` when coerced into env.

**Strict cache:** `Evaluator.drvStrictByPath` for chained `derivationStrict` in `let` bindings (input replacement precomputation).

## Hello / stdenvNoCC

- **Hello golden:** goonix `pkgs.hello.drvPath` still differs from Nix (`TestHelloDrvPathMatchesGolden`); golden file matches Nix oracle. Remaining gap is full dependency graph / input-drv modulo on real `mkDerivation`, not import-only regressions.
- **stdenvNoCC:** covered indirectly by boot bisect + import; dedicated `mkDerivation` oracle test not added yet.

## Fixpoint / xorg overlay (727)

**Root cause:** `lib.extends` overlay `// mapNamesToAttrs (setLicense …)` + eager `map` forced fixpoint `self` during WHNF.

**Fix:** Lazy **`builtins.map`** / **`concatMap`** via `deferredApplyCall`. **`genList`** stays eager for boot `imap1`/`dfold`.

## Blocker: `stdenv.shell` missing — **fixed**

**Fix:** `builtins.derivation` matches Nix `primops/derivation.nix` (top-level drv attrs + lazy outputs).

## Top-down boot bisect (historical)

`testdata/oracle/boot_chain_bisect_probe.nix` — goonix failed at depth 0 (same 6681 stack). Bottom-up bisect isolated the earlier bootstrap **glibc** override bug (k=2).

## Tests

- **Green:** CGO, `./lang/...`, `TestBuiltinLazinessAudit`, boot dfold oracles, `TestWithPkgsFixpointProbeMatchesNix`, `TestBootStagesTakeBisectMatchesNix`, `TestImportNixpkgsOnly`, derivation strict rungs (except 9), lazy strict tests
- **Red / skip:** `TestHelloDrvPathMatchesGolden`, rung 9 inputDrvs, some xorg/scope tests (`allowAliases`, paths)

Debug: `GOONIX_TRACE_FORCE=1` logs `derivationStrict` when flatten runs.
