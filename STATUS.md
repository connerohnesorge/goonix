# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

**Hermetic Nix oracle (`internal/oracle`):** every `nix-instantiate` uses `--option substituters '' --option restrict-eval false -I nixpkgs=$PWD/testdata/nixpkgs -I nix=$PWD/testdata/nix` with `NIX_PATH` stripped from the environment. Regenerate goldens with `scripts/update-goldens.sh` or `nix-instantiate --eval` on `testdata/ladder/*.nix`.

```bash
bash scripts/check-cgo.sh
go test ./store/... -run 'TestParseFixedOutput|TestStorePathForFilesystemPath' -count=1
go test ./eval/... -run 'TestGlibcBootstrap|TestBootstrapStage0Binutils|TestStringInterp|TestPythonModuleSelfEquality' -count=1
go test ./ladder/... -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** |
| #13 | Ladder / goldens | **partial** — `input_drv`, `toFile` **green**; hello / stdenvNoCC / curl drvPath still red |

## Bootstrap ATerm chain (green)

- **`''` strings:** parse-time `stripIndentation` only (Nix 2.24.9).
- **String path contexts:** `"${./p}"` and `"s" + p` copy to the store (via `inStringInterpolation` / `concatCoercedStrings`); `importDepth` on `builtins.import` keeps `TestImportNixpkgsOnly` lazy; `evalConcatStrings` folds through `concatCoercedStrings`.
- **Derivation env paths:** `materializeStorePaths` during env coercion only.
- **`__structuredAttrs`:** JSON env via `fillStructuredDerivationFromAttrs`.
- **`builtins.fromJSON`:** attr keys use `ev.Intern`.
- **Derivation `==`:** attrsets with `type = "derivation"` compare by forced `drvPath` (fixes python `pythonModule != python` check-meta false positive).
- **Oracles:** bootstrap ATerm trio, `TestStringInterpPathGetContextMatchesNix`, `TestStringPlusPathCopiesMatchesNix`, `TestPythonModuleSelfEqualityMatchesNix`, `TestCurlPlatformProbeMatchesNix`, `TestSelectOrMissingAttrMatchesNix` (**green**).

## Ladder goldens

| Rung | Test | Status |
|------|------|--------|
| inputDrvs | `TestInputDrvLadderMatchesGolden` | **green** |
| toFile | `TestToFileLadderMatchesGolden` | **green** |
| hello | `TestHelloDrvPathMatchesGolden` | **red** |
| stdenvNoCC | `TestStdenvNoCCLadderMatchesGolden` | **red** |
| curl | `TestCurlLadderMatchesGolden` | **red** — past check-meta; **drvPath** mismatch vs oracle |

## Still red / next

- `TestStdenvNoCCMkDerivationOracleMatchesNix` / `TestDrvDiffStdenvNoCCProbeReportsDeepest` — byte diff deepest logical **libffi-3.4.6** (env store paths e.g. bash/stdenv-linux differ from Nix on probe)
- Ladder hello / stdenvNoCC / curl **drvPath** goldens

## derivationStrict rungs 1–10

**Green** drvPath + ATerm oracles (`eval/derivation_strict_rung_test.go`).

## CGO

`bash scripts/check-cgo.sh` — **green** (no CGO in module graph).
