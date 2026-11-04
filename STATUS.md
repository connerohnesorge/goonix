# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... -run TestBuiltinLazinessAudit -count=1
go test ./eval/... ./lang/... -count=1
go test ./eval/... -run 'TestBootStagesTakeBisectMatchesNix|TestImportNixpkgsOnly' -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — lazy `map`/`genList` split; fixpoint overlay path fixed |
| #13 | Ladder / goldens | **partial** — hello evaluates; drv hash ≠ Nix (derivationStrict env/inputs) |

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

**Import:** `TestImportNixpkgsOnly` **green** (evaluates pinned nixpkgs import without forcing full `pkgs`).

## Blocker: hello golden / ladder drvPath hash

**Symptom:** `pkgs.hello` evaluates; `drvPath` thunk forces to a path, but **store hash** differs from Nix / committed golden (`gciipqhq…` vs goonix `0kn0nr4…`).

**Cause:** `attrsToDerivation` does not implement Nix `derivationStrict` env flattening or string-context `inputDrvs`/`inputSrcs`. Full flattening was attempted but forces boot attrs (e.g. stdenv overrides / `perl.override`) and hits infinite recursion at `pkgs/stdenv/linux/default.nix:347`.

**Next:** Nix-compatible `derivationStrict` env + input collection without over-forcing (string context / selective coercion).

## Fixpoint / xorg overlay (727)

**Root cause:** `lib.extends` overlay `// mapNamesToAttrs (setLicense …)` + eager `map` forced fixpoint `self` during WHNF.

**Fix:** Lazy **`builtins.map`** / **`concatMap`** via `deferredApplyCall`. **`genList`** stays eager for boot `imap1`/`dfold`.

## Blocker: `stdenv.shell` missing — **fixed**

**Fix:** `builtins.derivation` matches Nix `primops/derivation.nix` (top-level drv attrs + lazy outputs).

## Top-down boot bisect (historical)

`testdata/oracle/boot_chain_bisect_probe.nix` — goonix failed at depth 0 (same 6681 stack). Bottom-up bisect isolated the earlier bootstrap **glibc** override bug (k=2).

## Tests

- **Green:** CGO, `./lang/...`, `TestBuiltinLazinessAudit`, boot dfold oracles, `TestWithPkgsFixpointProbeMatchesNix`, `TestBootStagesTakeBisectMatchesNix`, `TestImportNixpkgsOnly`, derivation shell/oracle tests
- **Red:** `TestHelloDrvPathMatchesGolden` (drv hash), some xorg/scope tests (`allowAliases`, paths)

Debug: `GOONIX_TRACE_FORCE=1` on infinite recursion.
