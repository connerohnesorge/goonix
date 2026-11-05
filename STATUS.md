# goonix v0 status (issues #2–#13)

Oracle: **Nix 2.24.9** (`internal/nixversion`, CI **ladder** job via `cachix/install-nix-action` + `install_url` for 2.24.9). Optional **Nix 2.35.1** job (`nix-2-35`, `continue-on-error: true`) runs ATerm / strict-rung / FOD smoke. nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15. The code this note describes is `074441da0`. Its green run is https://github.com/connerohnesorge/goonix/actions/runs/42078109252 (ubuntu, macOS, the Nix 2.24.9 ladder, and the optional Nix 2.35 job). This file is the documentation commit on top of that SHA.

```bash
bash scripts/check-cgo.sh
go test ./... -count=1
```

**CI:** ubuntu/macos `go test ./... -short` (CGO off). **ladder** job: sysctl userns for Nix sandbox on Ubuntu 24.04, Nix 2.24.9, full `./internal/oracle`, `./ladder`, `./eval`, `./store`, `./cmd`, `./builder`, substituter hello test. The store/cmd `go test` step fails the job when tests fail (no `|| true` on that command). The ladder sets `GOONIX_REQUIRE_NIX_224=1`, so an oracle test that skips because Nix is not 2.24.9 fails the job, and a JSON scan fails the job if a test whose name contains `Oracle`, `Nix224`, `MatchesNix`, `LikeNix`, or `MatchNix` skipped. Oracle `nix-instantiate` calls set `--option experimental-features 'nix-command flakes'` themselves, so they do not depend on the machine `nix.conf`.

GitHub **#14** is "Post-v0: __structuredAttrs support". It is not the drvPath ladder, and it is not done.

Green run for `074441da0` (ubuntu, macOS, Nix 2.24.9 ladder, and the optional Nix 2.35 job all passed): https://github.com/connerohnesorge/goonix/actions/runs/42078109252

There is no Nix daemon in this tree. A daemon is out of scope for this map. `goonix gc` talks to the local SQLite store directly.

**Paths in tests:** prefer `testutil.ModuleRoot(t)` and `testutil.NixpkgsPath(t)`; legacy `modRoot(t)` in `eval/*_test.go` delegates to `testutil.ModuleRoot`.

## Phase map

| Phase | Scope | Tests / evidence | Left |
|-------|--------|------------------|------|
| **0** | Repo, CI, CGO gate, goldens | `scripts/check-cgo.sh`, `.github/workflows/ci.yml`, `ladder/*`, `internal/oracle` | — |
| **1** | Parser / lang | `parser/*`, `lang/*` (`parse-okay-*`) | CRLF / subversion parse cases skipped in lang harness |
| **2** | Eval core | `eval/*`, fixpoint, merge, `with` | Lexical builtins before `with`; POSIX regex leftmost-longest |
| **2b** | Builtins, Nix files | `eval/builtins*.go`, `builtins_impl_oracle_test.go`, `builtins_review_oracle_test.go` | `getFlake`, `parseFlakeRef`, and `flakeRefToString` require the `flakes` feature; `fetchTree` requires `fetch-tree` (enabling `flakes` turns that on too). `fromTOML` is TOML 1.0 (tables, inline tables, arrays of tables, hex/oct/bin). `inf` and `nan` stay floats (`typeOf` is `"float"`). A second `[t]` header errors with `table ("t") already exists`. Datetimes only with `parse-toml-timestamps`, otherwise Nix's "Dates and times are not supported". `parseFlakeRef` of `github:NixOS/nixpkgs/refs/heads/main` keeps ref `refs/heads/main`. Query params `ref`, `rev`, `host`, `narHash`, and `dir` are kept; unknown query keys are dropped. `gitlab:a/b/v1` is a gitlab ref. `sourcehut:` is not a parsed flake URL. `toJSON` of an attrset calls `__toString` or, if that is absent, emits `outPath`; a path value is copied into the store; a function errors `cannot convert a function to JSON`. `toXML` covers lists, null, functions, paths, and floats. `convertHash` takes `hashAlgo` and base16/base32/base64. `readFileType` returns `symlink`. `readDir` on a missing path says `reading directory …: No such file or directory`. `path` `recursive=false` is a flat hash, rejects directories, and a wrong SRI says `store path mismatch`. `filterSource` / `path` keep symlinks in the NAR. `fetchurl` accepts a URL string. `fetchTarball` and `fetchTree` of a tarball keep symlinks and share the store path and narHash; a missing archive says `Failed to open archive`. `fetchTree` covers file, tarball, github, gitlab, sourcehut, and indirect (or Nix's error). `getFlake` on a missing path reports `path '…' does not exist` and does not `git clone` a `github:` ref. `unsafeGetAttrPos` returns the attribute's source position. Embedded `<nix/fetchurl.nix>` |
| **3** | Spike / decision | README, go-nix | — |
| **4a** | Store paths, hashes, ATerm | `store/*`, `store/drvshow/*`, modulo oracles | — |
| **4b** | Local SQLite store, GC | `store/local/gc.go`, `cmd/goonix gc` | Direct roots under `gcroots/`, indirect roots under `gcroots/auto/<nix32(sha1(path))>`, and `profiles/` are real symlinks. `CollectGarbage` keeps the reference closure of those roots and deletes the rest. No daemon |
| **5** | Builder | `builder/*`, `cmd/goonix build` | No `$out` pre-creation. On Linux the default build is `unshare -rmp --fork`, plus `-n` when the derivation is not fixed-output. Fixed-output builds keep the network. The host build directory is bind-mounted at `/build`, and `HOME` is `/homeless-shelter`, which is not created. pivot_root into a root with that build dir, the input closure, `/bin/sh` (and its loader), `/dev/null`, `/dev/zero`, `/dev/random`, `/dev/urandom`, and `/proc`. The builder is uid 1000 and gid 1000 inside a nested user namespace. Host paths such as `/usr/bin/id` are not visible. The mount marker is removed after the build. A failed build does not leave a partial output. A fixed-output hash mismatch says `hash mismatch in fixed-output derivation` with the specified and got SRI. Setup failure still falls back only when `AllowRelocated` is set. There is no sandbox on/off flag. `outputHashMode` accepts `flat`, `nar`, `recursive`, `text` (needs `dynamic-derivations`), and `git` (needs `git-hashing` and SHA-1) |
| **6** | Substituter | `substituter/*` | Live cache test skips on `-short`/offline |
| **7** | CLI | `cmd/goonix` | `parse`, `eval`, `instantiate`, `build`, `derivation-show`, `drv-diff`, `gc`. Instantiate writes every dependency `.drv` into the store, not only the top one. No daemon |
| **#13** | Bootstrap ladder | stage stdenv ATerm oracles | — |
| **drvPath** | drvPath ladder | `ladder.TestHelloDrvPathMatchesGolden`, … | large closure drvdiff diagnostic skips. This is not GitHub #14 |

## Oracle coverage (recent)

| Area | Test(s) | Notes |
|------|---------|--------|
| **ATerm escaping → drvPath** | `eval/aterm_escape_drvpath_oracle_test.go`, `aterm_escape_reject_oracle_test.go` | Nix `(derivation attrs).drvPath` vs goonix `derivationStrict`; temp-store `.drv` byte compare |
| **Rung08 + structured attrs** | `eval/derivation_strict_rung_test.go`, structured attrs oracle | Full ATerm in temp store |
| **Linux boot chain** | `eval/boot_stage_diff_test.go` + `testdata/oracle/linux_boot_stage_chain.nix` | Stage count + dfold order vs Nix (not full production boot walk — needs bootstrap store paths) |
| **xorg + splicing** | `eval/xorg_drvpath_oracle_test.go`, `eval/uri_query_oracle_test.go`, `eval/builtin_regex_oracle_test.go`, `eval/with_scoping_oracle_test.go` | POSIX `match`/`split`; `lib.strings.sanitizeDerivationName`; smoke oracles include **`pkgs.git.out.drvPath`** |
| **FOD** | `builder/fod_oracle_test.go` | sha1/sha256/sha512 flat, sha256 recursive, HTTP + `file://`; out paths vs Nix strict |
| **Build chain** | `builder/build_chain_oracle_test.go` | `nix-build` chain; args include quote, backslash, newline, tab; out paths and NAR hashes |
| **Builtin bodies** | `eval/builtins_impl_oracle_test.go`, `eval/builtins_review_oracle_test.go`, `eval/no_stub_builtin_test.go` | Nix 2.24.9 oracles. The stub scan covers `eval/`, `builder/`, `store/`, `cmd/`, and `parser/` (non-test files) and fails on `not implemented` or `unsupported`. The exemptions are Nix's own `cannot show unsupported input` and the parser fallthrough, which keeps `unsupported expression` with an explicit `stub-scan: exempt` comment |
| **outputHashMode** | `eval/output_hash_mode_oracle_test.go`, `store/local/git_hash_test.go` | `flat`, `nar`/`recursive`, `text`, and `git` out paths vs Nix 2.24.9. Git object hashes vs `git hash-object` / `git write-tree`. Unknown mode is `invalid value '…' for 'outputHashMode' attribute` |
| **Sandbox** | `builder/sandbox_test.go` | Builder uid from `/proc/self/status` is 1000, `/usr/bin/id` is absent, a store path outside the closure is absent, and a failing builder leaves no output |
| **GC** | `store/local/gc_test.go`, `cmd/goonix/gc_cmd_test.go` | Indirect auto-root name vs `nix-hash --type sha1 --base32`. GC keeps a rooted path and its references and deletes the rest |

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
| `eval/or_invoke_test.go` | `null or 7` not Nix strict semantics (intentional) |
| `eval/pkg_config_debug_test.go`, wrapper debug tests | Require `GOONIX_DEBUG_*` env |
| `store/local/store_test.go` | `nix-store` not on PATH |
| `store/aterm_roundtrip_test.go` | Nix or on-disk drv missing |
| `ladder/*` | Missing testdata nixpkgs tarball |
| `eval/systems_parse_test.go`, `modules_cycle_test.go`, `rec_nested_test.go` | Missing nixpkgs pin |
| `internal/oracle/version_test.go` | `t.Skip` with a reason when `nix-instantiate --version` is not 2.24.9. The ladder sets `GOONIX_REQUIRE_NIX_224=1`, which turns that skip into a failure |
| `eval/boot_*_bisect`, `stdenv_overrides_probe`, `oracle_repro` | No `nix-instantiate` |
| `parser/parser_test.go` | Corpus file missing |
| `substituter` net-tools | Superseded by hello cache test in CI |

## Oracle / tooling

- **`internal/oracle`**: hermetic `nix-instantiate` (2.24.9); temp-store via `--store` + `ChrootIOEffects`; `EvalDerivationDrvPath*`, `EvalStrictOutPath`.
- **`store/drvshow`**: Nix 2.24 flat map + v4; duplicate logical drv key pairing.
- **`drvdiff`**: closure compare with exact-key preference.

## Still not Nix 2.24.9

- No daemon, and no daemon protocol. That is out of scope for this map.
- GitHub #14, `__structuredAttrs`, is not implemented.
- `parseFlakeRef` does not accept `sourcehut:` URLs.
- Flake registries are not consulted. A missing indirect flake still reports `cannot find flake 'flake:…' in the flake registries` without reading a registry.
- Successful `github`, `gitlab`, and `sourcehut` fetches are not byte-compared to Nix's git cache. A missing sourcehut ref is fetched with the `curl` binary (HTTP/2, IPv4, no proxy, four-minute wait) because Go's HTTP client does not receive that host's response. Without `curl` on `PATH`, that error is not Nix's `HTTP error 404`.
- The sandbox always provides `/bin/sh` and a fresh `/proc`. It does not read Nix's `sandbox-paths`, and it is not a multi-user `nixbld` pool. A `text` hash that is not SHA-256 returns an error; Nix 2.24.9 aborts on that assertion.
- `fetchTree` of a `file` input hashes the NAR of the file. Field comparison against Nix is gated on `nix-instantiate --version` containing `2.24.9`.

## CGO

`bash scripts/check-cgo.sh` **green** (`CGO_ENABLED=0`).
