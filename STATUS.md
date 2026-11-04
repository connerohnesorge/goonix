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

- Field diffs: env (path-normalized), outputs `.path` / `.hashAlgo` / `.hash`, args/srcs normalized.
- **`FormatMismatchDetail`:** full env/outputs dump + raw ATerm both sides (`internal/drvdiff/detail.go`); wired in `eval/drvdiff_test.go` and `goonix drv-diff`.
- **Closure:** 225=225, missing **0**. Deepest mismatch for stdenvNoCC inst is now the **probe mkDerivation** itself (bash/stdenv-linux input drv hashes still differ upstream) — bootstrap hook / FOD leaves no longer deepest.

## Ladder goldens

| Rung | Test | Status |
|------|------|--------|
| inputDrvs | `TestInputDrvLadderMatchesGolden` | **green** |
| toFile | `TestToFileLadderMatchesGolden` | **green** |
| hello | `TestHelloDrvPathMatchesGolden` | **red** (full hello closure) |
| stdenvNoCC | `TestStdenvNoCCLadderMatchesGolden` | **red** |
| curl | `TestCurlLadderMatchesGolden` | **red** (large; skip `-short`) |

## Still red

- `TestStdenvNoCCMkDerivationOracleMatchesNix`, `TestDrvDiffStdenvNoCCProbeReportsDeepest` (depth 0 until bash/stdenv-linux `.drv` bytes match Nix)
- `TestHelloDrvPathMatchesGolden` (same bootstrap graph)

## derivationStrict rungs 1–10

**Green** drvPath + ATerm oracles (`eval/derivation_strict_rung_test.go`).
