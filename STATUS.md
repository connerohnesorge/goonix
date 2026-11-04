# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

**Hermetic Nix oracle (`internal/oracle`):** every `nix-instantiate` uses `--option substituters '' --option restrict-eval false -I nixpkgs=$PWD/testdata/nixpkgs -I nix=$PWD/testdata/nix` with `NIX_PATH` stripped from the environment. Regenerate goldens with `scripts/update-goldens.sh` or `nix-instantiate --eval` on `testdata/ladder/*.nix`.

```bash
bash scripts/check-cgo.sh
go test ./store/... -run 'TestParseFixedOutput|TestStorePathForFilesystemPath' -count=1
go test ./eval/... -run 'TestGlibcBootstrap|TestBootstrapStage0Binutils|TestCurlPlatform|TestSelectOr' -count=1
go test ./ladder/... -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** |
| #13 | Ladder / goldens | **partial** — `input_drv`, `toFile` **green**; hello / stdenvNoCC / curl still red |

## Bootstrap ATerm chain (green)

- **`''` strings:** parse-time `stripIndentation` only (Nix 2.24.9).
- **Derivation env paths:** `coerceEnvStrings` + single `coerceToString` with `CopyPaths` for `''`/`""` and concat parts; `store.CopyPathToStore` uses `0755` while building trees (fixes empty `-by-name` store objects); `RealiseStorePath` skips only non-empty store paths.
- **`__structuredAttrs`:** JSON env via `fillStructuredDerivationFromAttrs` (Nix `printValueAsJSON` shape).
- **`builtins.fromJSON`:** attr keys use `ev.Intern` (was a fresh interner — broke `sdkVersions.${…}` and apple-sdk on linux).
- **Oracles:** `TestGlibcBootstrapFilesATermMatchesNix`, `TestBootstrapStage0StdenvLinuxATermMatchesNix`, `TestBootstrapStage0BinutilsWrapperATermMatchesNix`, `TestCurlPlatformProbeMatchesNix`, `TestSelectOrMissingAttrMatchesNix` (**green**). No `lib.throw` injection; global `throw` via `evalVar` → `Builtins`.

## Ladder goldens

| Rung | Test | Status |
|------|------|--------|
| inputDrvs | `TestInputDrvLadderMatchesGolden` | **green** |
| toFile | `TestToFileLadderMatchesGolden` | **green** |
| hello | `TestHelloDrvPathMatchesGolden` | **red** |
| stdenvNoCC | `TestStdenvNoCCLadderMatchesGolden` | **red** |
| curl | `TestCurlLadderMatchesGolden` | **red** — check-meta python propagation assert during closure (past apple-sdk / `fromJSON`) |

## Still red / next

- Ladder hello / stdenvNoCC / curl `drvPath` goldens vs oracle
- `TestStdenvNoCCMkDerivationOracleMatchesNix`, `TestDrvDiffStdenvNoCCProbeReportsDeepest` (if not covered by ATerm chain)

## derivationStrict rungs 1–10

**Green** drvPath + ATerm oracles (`eval/derivation_strict_rung_test.go`).

## CGO

`bash scripts/check-cgo.sh` — **green** (no CGO in module graph).
