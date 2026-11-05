# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9** (`internal/nixversion`, CI **ladder** job via `cachix/install-nix-action` + `install_url` for 2.24.9). Optional **Nix 2.35.1** job (`nix-2-35`, `continue-on-error: true`) runs ATerm / strict-rung / FOD smoke. nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15. Latest green CI: https://github.com/connerohnesorge/goonix/actions/runs/41959199295 (commit `f2a3055c0`).

```bash
bash scripts/check-cgo.sh
go test ./... -count=1
```

**CI:** ubuntu/macos `go test ./... -short` (CGO off). **ladder** job: sysctl userns for Nix sandbox on Ubuntu 24.04, Nix 2.24.9, full `./internal/oracle`, `./ladder`, `./eval`, `./builder`, substituter hello test.

**Paths in tests:** prefer `testutil.ModuleRoot(t)` and `testutil.NixpkgsPath(t)`; legacy `modRoot(t)` in `eval/*_test.go` delegates to `testutil.ModuleRoot`.

## Phase map

| Phase | Scope | Tests / evidence | Left |
|-------|--------|------------------|------|
| **0** | Repo, CI, CGO gate, goldens | `scripts/check-cgo.sh`, `.github/workflows/ci.yml`, `ladder/*`, `internal/oracle` | — |
| **1** | Parser / lang | `parser/*`, `lang/*` (`parse-okay-*`) | CRLF / subversion parse cases skipped in lang harness |
| **2** | Eval core | `eval/*`, fixpoint, merge, `with` | — |
| **2b** | Builtins, Nix files | `eval/builtins*.go`, oracle repros | — |
| **3** | Spike / decision | README, go-nix | — |
| **4a** | Store paths, hashes, ATerm | `store/*`, `store/drvshow/*`, modulo oracles | — |
| **4b** | Local SQLite store | `store/local/*` | GC roots stub; no daemon |
| **5** | Builder | `builder/*`, `cmd/goonix build` | FOD matrix oracle; goonix **sandbox off by default**; `TestBuildEchoHiMatchesNixBuild` uses `nix-build --store local?root=…` with **sandbox on** + hermetic env (no host `NIX_STORE`) so CI can write outputs |
| **6** | Substituter | `substituter/*` | Live cache test skips on `-short`/offline |
| **7** | CLI | `cmd/goonix` | — |
| **#13** | Bootstrap ladder | stage stdenv ATerm oracles | — |
| **#14** | drvPath ladder | `ladder.TestHelloDrvPathMatchesGolden`, … | large closure drvdiff diagnostic skips |

## Oracle coverage (recent)

| Area | Test(s) | Notes |
|------|---------|--------|
| **ATerm escaping → drvPath** | `eval/aterm_escape_drvpath_oracle_test.go`, `aterm_escape_reject_oracle_test.go` | Nix `(derivation attrs).drvPath` vs goonix `derivationStrict`; temp-store `.drv` byte compare |
| **Rung08 + structured attrs** | `eval/derivation_strict_rung_test.go`, structured attrs oracle | Full ATerm in temp store |
| **Linux boot chain** | `eval/boot_stage_diff_test.go` + `testdata/oracle/linux_boot_stage_chain.nix` | Stage count + dfold order vs Nix (not full production boot walk — needs bootstrap store paths) |
| **xorg + splicing** | `eval/xorg_drvpath_oracle_test.go`, `eval/uri_query_oracle_test.go` | `pkgs.xorg.libX11.drvPath`, xcbproto, smoke oracles (python3, openssl, libXext); fetchurl names with `?`/`&` sanitized via `store.SanitizeStorePathName` |
| **FOD** | `builder/fod_oracle_test.go` | sha1/sha256/sha512 flat, sha256 recursive, HTTP + `file://`; out paths vs Nix strict |

## Post-hack test audit (item 5)

After removing eval shortcuts (`lookupEnv` / `systemPredicates` / cross `fn` hacks), expectations must come from Nix where oracles exist:

| Test | Change |
|------|--------|
| `TestStdenvCrossLine23Subexprs` | **Was:** hand-coded `want` map for `typeOf` strings. **Now:** `oracle.EvalImportJSON` on the same expression as goonix (live Nix). |
| `TestPlainSystemAttrsetIsLinuxThrowsLikeNix` | Already used `oracle.EvalExprAllowError` for Nix side (unchanged). |
| Boot / xorg drvPath oracles | Nix `EvalRaw` / import JSON only (no edited drvPath constants). |

## Skips (complete)

| Location | Reason |
|----------|--------|
| `lang/*` CRLF / subversion | Parser corpus gaps |
| `eval/aterm_escape_drvpath_oracle_test.go` env-key escape subcase | Nix rejects illegal env **names** (goonix matches); case skipped when Nix errors |
| `TestBuild*` / `cmd/goonix` when no `/bin/sh` | Host has no POSIX shell |
| `TestBuildEchoHiMatchesNixBuild` when no `nix-build` | Nix CLI not installed (ladder job has Nix) |
| `TestPkgConfigFetchurl*` / many bootstrap oracles when no `/nix/store` | Need host store materialisation |
| `TestSubstituteNetToolsFromCache`, `testing.Short`, network errors | Offline or `-short` unit job |
| `TestSubstituteHelloFromCache` network / cache miss | Same (ladder runs this test) |
| `TestDrvDiffHelloGoldenClosure` | Large hello closure |
| `TestImportHeadProbe` | Known red probe until import head fixed |
| `TestPkgsDrvPathSmokeMatchesNixOracle/git` | `pkgs.git` eager-evaluates `all-packages` (infinite recursion); Nix oracle still verified |
| `eval/or_invoke_test.go` | `null or 7` not Nix strict semantics (intentional) |
| `eval/pkg_config_debug_test.go`, wrapper debug tests | Require `GOONIX_DEBUG_*` env |
| `store/local/store_test.go` | `nix-store` not on PATH |
| `store/aterm_roundtrip_test.go` | Nix or on-disk drv missing |
| `ladder/*` | Missing testdata nixpkgs tarball |
| `eval/systems_parse_test.go`, `modules_cycle_test.go`, `rec_nested_test.go` | Missing nixpkgs pin |
| `internal/oracle/version_test.go` | Wrong Nix version on PATH |
| `eval/boot_*_bisect`, `stdenv_overrides_probe`, `oracle_repro` | No `nix-instantiate` |
| `parser/parser_test.go` | Corpus file missing |
| `substituter` net-tools | Superseded by hello cache test in CI |

## Oracle / tooling

- **`internal/oracle`**: hermetic `nix-instantiate` (2.24.9); temp-store via `--store` + `ChrootIOEffects`; `EvalDerivationDrvPath*`, `EvalStrictOutPath`.
- **`store/drvshow`**: Nix 2.24 flat map + v4; duplicate logical drv key pairing.
- **`drvdiff`**: closure compare with exact-key preference.

## CGO

`bash scripts/check-cgo.sh` **green** (`CGO_ENABLED=0`).
