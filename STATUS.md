# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

**Hermetic Nix oracle (`internal/oracle`):** every `nix-instantiate` uses `--option substituters '' --option restrict-eval false -I nixpkgs=$PWD/testdata/nixpkgs -I nix=$PWD/testdata/nix` with `NIX_PATH` stripped from the environment. Regenerate goldens with `scripts/update-goldens.sh` or `nix-instantiate --eval` on `testdata/ladder/*.nix`.

```bash
bash scripts/check-cgo.sh
go test ./store/... -run 'TestParseFixedOutput|TestStorePathForFilesystemPath' -count=1
go test ./eval/... -run 'TestFetchurlSRI|TestPkgConfigFetchurl|TestDrvDiffStdenvNoCC|TestStdenvNoCCMkDerivation' -count=1
go test ./ladder/... -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** |
| #13 | Ladder / goldens | **partial** — `input_drv`, `toFile` **green**; hello / stdenvNoCC / curl still red (bootstrap closure drv hashes) |

## FOD / fetchurl (fixed)

- **`ParseFixedOutputHash`:** legacy **nix-base32** `outputHash` with explicit `outputHashAlgo` (nixpkgs `fetchurl`, e.g. pkg-config) via `nixhash.ParseAny(hash, &algo)`; SRI still works with empty algo.
- **Do not clear** explicit `outputHashAlgo` when `outputHash` is SRI — nixpkgs fetchurl keeps both; only `<nix/fetchurl.nix>` sets `outputHashAlgo = ""` for SRI via the `.nix` default.
- **Oracle:** `store/parse_fixed_output_test.go`, `eval/pkg_config_fetchurl_oracle_test.go`, `eval/fetchurl_sri_oracle_test.go` (**green**).

## Filesystem path → store (fixed)

- **`StorePathForFilesystemPath`:** Nix `hashStringsModulo("source", "sha256", base16NarHash, storeDir, name)` + `CompressHash` (was wrongly using raw NAR digest as path hash).
- **String interpolation:** `evalString` / `evalIndentedString` use `CopyPaths` with `Stat` guard before copying.
- **Oracle:** `store/path_store_oracle_test.go` vs `builtins.path` (**green**).

## drv-diff (extended)

- **Leaf-first ATerm byte compare:** `CompareBytesBottomUp` + `FormatByteCulprit` (first paired node whose raw `.drv` bytes differ while input `.drv` bytes all match). Use `AnalyseWithATerm` / `goonix drv-diff`.
- Normalized field diff retained (`FormatMismatchDetail`) for debugging only.
- **Byte culprit (stdenvNoCC inst):** `bootstrap-stage0-binutils-wrapper-` (glibc-bootstrapFiles + bootstrap-stage0-stdenv-linux ATerm oracles **green**).

## Fixes (bootstrap chain)

- **`''` strings:** deferred stripIndent; unescape; interpolation padding only when `${` starts the line; `evalConcatStrings` merges IndentedRaw before one strip; `joinIndentedRawBodies` for `+`/`++` concat.
- **inputSrcs / inputDrvs:** only from string **context** at coercion time (`mergeValueContext` on `Ctx` only); no scanning env text or path fallback.
- **Derivation env newlines:** `coerceDerivationEnvString` trailing newline rules for multi-line `.drv` env (e.g. preHook, buildCommand).
- **Oracles:** `eval/derivation_env_inputsrc_test.go`, `eval/indented_string_oracle_test.go`, `eval/glibc_bootstrap_files_aterm_test.go`, `eval/bootstrap_stdenv_linux_aterm_test.go`.

## Ladder goldens

| Rung | Test | Status |
|------|------|--------|
| inputDrvs | `TestInputDrvLadderMatchesGolden` | **green** |
| toFile | `TestToFileLadderMatchesGolden` | **green** |
| hello | `TestHelloDrvPathMatchesGolden` | **red** (full hello closure) |
| stdenvNoCC | `TestStdenvNoCCLadderMatchesGolden` | **red** |
| curl | `TestCurlLadderMatchesGolden` | **red** (large; skip `-short`) |

## Still red

- `TestStdenvNoCCMkDerivationOracleMatchesNix`, `TestDrvDiffStdenvNoCCProbeReportsDeepest` (next: `bootstrap-stage0-binutils-wrapper-`)
- `TestHelloDrvPathMatchesGolden`, stdenv/curl ladder (downstream)

## derivationStrict rungs 1–10

**Green** drvPath + ATerm oracles (`eval/derivation_strict_rung_test.go`).
