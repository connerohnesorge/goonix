# goonix v0 status (issues #2–#13)

Oracle: **Nix 2.24.9** (`internal/nixversion`, CI **ladder** job via `cachix/install-nix-action` + `install_url` for 2.24.9). Optional **Nix 2.35.1** job (`nix-2-35`, `continue-on-error: true`) runs ATerm / strict-rung / FOD smoke. nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15. The code this note describes is `c5dd86dee`. GitHub Actions is not running that commit: see CI below.

```bash
bash scripts/check-cgo.sh
go test ./... -count=1
```

**CI:** ubuntu/macos `go test ./... -short` (CGO off). **ladder** job: sysctl userns for Nix sandbox on Ubuntu 24.04, Nix 2.24.9, full `./internal/oracle`, `./ladder`, `./eval`, `./store`, `./cmd`, `./builder`, substituter hello test. The store/cmd `go test` step fails the job when tests fail. There is no `|| true` on that command. The ladder sets `GOONIX_REQUIRE_NIX_224=1`, so an oracle test that skips because Nix is not 2.24.9 fails the job. A JSON scan of the eval, ladder, builder, store, and cmd logs fails the job on any other skip except five intentional non-oracle tests (`TestOrFallbackNullUsesRight` and the four `GOONIX_DEBUG_*` probes). Oracle `nix-instantiate` calls set `--option experimental-features 'nix-command flakes'` themselves, so they do not depend on the machine `nix.conf`.

GitHub **#14** is "Post-v0: __structuredAttrs support". It is not the drvPath ladder. It is done. The `.drv` keeps user attributes in `__json` (plus the real output paths). The builder writes `.attrs.json` and `.attrs.sh`. `outputChecks`, `unsafeDiscardReferences`, `exportReferencesGraph`, and the `outputs` object match Nix 2.24.9. An output scan records the output's own store path and its sibling outputs, so `allowedReferences = []` on a self-referencing output fails the way Nix fails, and garbage collection keeps a sibling a rooted output refers to. `TestOutputScanRecordsSelfAndSiblings` is that check. `TestStructuredAttrsStrictATermMatchesNix` compares drvPath and out path. `TestStructuredAttrsBuildMatchesNix` compares the built store path and NAR, including a discarded reference and a `maxSize` failure. The top-level structuredAttrs warning is printed once, at evaluation, with Nix's message.

**CI is blocked by GitHub billing.** Jobs fail in about two seconds, before any step starts, with "The job was not started because recent account payments have failed or your spending limit needs to be increased." The push of `d3b7c11dc` is https://github.com/connerohnesorge/goonix/actions/runs/42734610311 and the matching PR run is https://github.com/connerohnesorge/goonix/actions/runs/42734615877. The push of `2c51cc555` is https://github.com/connerohnesorge/goonix/actions/runs/42737662960 and fails the same way. Ubuntu, the ladder, and the Nix 2.35 job never start, and macOS is cancelled because the Ubuntu job did not start. The last green run is https://github.com/connerohnesorge/goonix/actions/runs/42087047591 for `78ddd0add`. It does not cover any commit after that one. Until billing is fixed, the check is a local `go test ./...` under Nix 2.24.9.

There is no Nix daemon in this tree. A daemon is out of scope for this map. `goonix gc` talks to the local SQLite store directly.

**Paths in tests:** prefer `testutil.ModuleRoot(t)` and `testutil.NixpkgsPath(t)`; legacy `modRoot(t)` in `eval/*_test.go` delegates to `testutil.ModuleRoot`.

## Phase map

| Phase | Scope | Tests / evidence | Left |
|-------|--------|------------------|------|
| **0** | Repo, CI, CGO gate, goldens | `scripts/check-cgo.sh`, `.github/workflows/ci.yml`, `ladder/*`, `internal/oracle` | — |
| **1** | Parser / lang | `parser/*`, `lang/*` (`parse-okay-*`) | CRLF / subversion parse cases skipped in lang harness |
| **2** | Eval core | `eval/*`, fixpoint, merge, `with` | Lexical builtins before `with`; POSIX regex leftmost-longest |
| **2b** | Builtins, Nix files | `eval/builtins*.go`, `builtins_impl_oracle_test.go`, `builtins_review_oracle_test.go` | `getFlake`, `parseFlakeRef`, and `flakeRefToString` require the `flakes` feature; `fetchTree` requires `fetch-tree` (enabling `flakes` turns that on too). `fromTOML` is TOML 1.0 (tables, inline tables, arrays of tables, hex/oct/bin). `inf` and `nan` stay floats (`typeOf` is `"float"`). A second `[t]` header errors with `table ("t") already exists`. Datetimes only with `parse-toml-timestamps`, otherwise Nix's "Dates and times are not supported". `parseFlakeRef` of `github:NixOS/nixpkgs/refs/heads/main` keeps ref `refs/heads/main`. Query params `ref`, `rev`, `host`, `narHash`, and `dir` are kept; unknown query keys are dropped. `gitlab:a/b/v1` is a gitlab ref. `sourcehut:~edolstra/nix-serve` is a sourcehut ref, including a `~` in the owner. A bare id such as `nixpkgs` is an indirect flake ref (`flake:nixpkgs`). `toJSON` of an attrset calls `__toString` or, if that is absent, emits `outPath`; a path value is copied into the store; a function errors `cannot convert a function to JSON`. `toXML` covers lists, null, functions, paths, and floats. `convertHash` takes `hashAlgo` and base16/base32/base64. `readFileType` returns `symlink`. `readDir` on a missing path says `reading directory …: No such file or directory`. `path` `recursive=false` is a flat hash, rejects directories, and a wrong SRI says `store path mismatch`. `filterSource` / `path` keep symlinks in the NAR. `fetchurl` accepts a URL string. `fetchTarball` and `fetchTree` of a tarball keep symlinks and share the store path and narHash; a missing archive says `Failed to open archive`. `fetchTree` covers file, tarball, github, gitlab, sourcehut, and indirect. Sourcehut resolves HEAD and tags through `git.sr.ht/<owner>/<repo>/HEAD` and `/info/refs`, then downloads `archive/<sha>.tar.gz`, so HEAD and tags include `rev` and `shortRev`. `builtins.toFile` returns a string with path context (`typeOf` is `"string"`). A `drvPath` string carries all-outputs context. `fromJSON` of a decimal or exponent, such as `"100.0"`, is a float. `parseFlakeRef` accepts `git+ssh`, `git+file`, `hg+https`, and `hg+file` as well as the http and https forms. `fromTOML` accepts `+nan` and `-nan`, and a dotted key into an inline table errors because inline tables are immutable. A float literal below the smallest normal value, including `5.0e-324`, is `invalid float`. Unary minus leaves the sign of NaN alone, so `toString (-nan)` stays `nan` and negating `-nan` stays `-nan`. An attrset `fetchTree` of github, gitlab, or sourcehut honours `ref` and `rev`. A git flake ref keeps `submodules=1` and `dir` in `parseFlakeRef`, `flakeRefToString`, `fetchTree`, and `getFlake`. `fromTOML` of `a = [1]` followed by `[[a]]` errors with `array of table ("a") collides with existing value`, and `[a.b]` after an inline table `a` errors because inline tables are immutable. Indirect ids are resolved through the user registry (`$XDG_CONFIG_HOME/nix/registry.json` or `$HOME/.config/nix/registry.json`), the system registry (`$NIX_CONF_DIR/registry.json`, default `/etc/nix/registry.json`), then the global registry (`flake-registry`, default `https://channels.nixos.org/flake-registry.json`). `getFlake` on a missing path reports `path '…' does not exist` and does not `git clone` a `github:` ref. `unsafeGetAttrPos` returns the attribute's source position. Embedded `<nix/fetchurl.nix>` |
| **3** | Spike / decision | README, go-nix | — |
| **4a** | Store paths, hashes, ATerm | `store/*`, `store/drvshow/*`, modulo oracles | — |
| **4b** | Local SQLite store, GC | `store/local/gc.go`, `cmd/goonix gc` | Direct roots under `gcroots/`, indirect roots under `gcroots/auto/<nix32(sha1(path))>`, and `profiles/` are real symlinks. `CollectGarbage` keeps the reference closure of those roots and deletes the rest. No daemon |
| **5** | Builder | `builder/*`, `cmd/goonix build` | No `$out` pre-creation. On Linux the default build is `unshare -rmp --fork`, plus `-n` when the derivation is not fixed-output. Fixed-output builds keep the network. The host build directory is bind-mounted at `/build`, and `HOME` is `/homeless-shelter`, which is not created. pivot_root into a root with that build dir, the input closure, `/bin/sh` (and its loader), `/dev/null`, `/dev/zero`, `/dev/random`, `/dev/urandom`, and `/proc`. The builder is uid 1000 and gid 1000 inside a nested user namespace. Host paths such as `/usr/bin/id` are not visible. The mount marker is removed after the build. A failed build does not leave a partial output. A fixed-output hash mismatch says `hash mismatch in fixed-output derivation` with the specified and got SRI. Setup failure still falls back only when `AllowRelocated` is set. There is no sandbox on/off flag. `outputHashMode` accepts `flat`, `nar`, `recursive`, `text` (needs `dynamic-derivations`), and `git` (needs `git-hashing` and SHA-1). `__structuredAttrs` writes `/build/.attrs.json` and `/build/.attrs.sh` and does not export the derivation env |
| **6** | Substituter | `substituter/*` | Live cache test skips on `-short`/offline |
| **7** | CLI | `cmd/goonix` | `parse`, `eval`, `instantiate`, `build`, `derivation-show`, `drv-diff`, `gc`. `instantiate` accepts `derivation { … }`. `eval` and `instantiate` take `-I`, `-A`, `--arg`, and `--argstr`, and auto-call a function result like `nix-instantiate`. A directory argument means its `default.nix`. `--store DIR` is the same physical root for `instantiate` and `build`: logical paths stay under `/nix/store`, and files land in `DIR/store/`. Both commands write every dependency `.drv`, escaped the same way the hash is computed, so the file on disk hashes to its own store path. No daemon |
| **#13** | Bootstrap ladder | stage stdenv ATerm oracles | — |
| **drvPath** | drvPath ladder | `ladder.TestHelloDrvPathMatchesGolden`, … | large closure drvdiff diagnostic skips. This is not GitHub #14 |
| **#9** | Integration milestone | `ladder.TestInstantiateClosureMatchesGolden`, `ladder.TestEvalLibVersionMatchesGolden` | — . `hello` (253 drvs), `stdenv` (224), `bash` (166), `coreutils` (170): every `.drv` matches Nix 2.24.9 by sha256, no Nix needed. Goldens from `scripts/update-goldens.sh` |

## Oracle coverage (recent)

| Area | Test(s) | Notes |
|------|---------|--------|
| **ATerm escaping → drvPath** | `eval/aterm_escape_drvpath_oracle_test.go`, `aterm_escape_reject_oracle_test.go` | Nix `(derivation attrs).drvPath` vs goonix `derivationStrict`; temp-store `.drv` byte compare |
| **Rung08 + structured attrs** | `eval/derivation_strict_rung_test.go`, structured attrs oracle | Full ATerm in temp store |
| **Linux boot chain** | `eval/boot_stage_diff_test.go` + `testdata/oracle/linux_boot_stage_chain.nix` | Stage count + dfold order vs Nix (not full production boot walk — needs bootstrap store paths) |
| **xorg + splicing** | `eval/xorg_drvpath_oracle_test.go`, `eval/uri_query_oracle_test.go`, `eval/builtin_regex_oracle_test.go`, `eval/with_scoping_oracle_test.go` | POSIX `match`/`split`; `lib.strings.sanitizeDerivationName`; smoke oracles include **`pkgs.git.out.drvPath`** |
| **FOD** | `builder/fod_oracle_test.go` | sha1/sha256/sha512 flat, sha256 recursive, HTTP + `file://`; out paths vs Nix strict |
| **Build chain** | `builder/build_chain_oracle_test.go` | `nix-build` chain; args include quote, backslash, newline, tab; out paths and NAR hashes |
| **__structuredAttrs** | `eval/structured_attrs_aterm_oracle_test.go`, `builder/structured_attrs_oracle_test.go` | drvPath, out path, built NAR, `exportReferencesGraph`, `unsafeDiscardReferences`, and `maxSize` vs Nix |
| **Flake registries** | `eval/flake_registry_oracle_test.go` | Local user, system, and global registry files. User beats system beats global. A chain applies a ref override. A missing id is `cannot find flake 'flake:…'` |
| **github fetchTree** | `eval/fetch_tree_github_oracle_test.go` | `edolstra/flake-compat` at `5edf11c44bc78a0d334f6334cdaf7d60d732daab`. `outPath` and `narHash` match Nix. Runs in the ladder job |
| **sourcehut fetchTree** | `eval/fetch_tree_sourcehut_oracle_test.go` | `~sircmpwn/scdoc` at commit `24be30ef17d5ca0162f11b35258c8fa2413d556a`, tag `1.11.2`, and HEAD. `outPath`, `narHash`, `rev`, and `shortRev` match Nix. HEAD is resolved from `/HEAD` and `/info/refs`, then the archive is `archive/<sha>.tar.gz`. A missing rev is `HTTP error 404`. Runs in the ladder job |
| **Builtin bodies** | `eval/builtins_impl_oracle_test.go`, `eval/builtins_review_oracle_test.go`, `eval/no_stub_builtin_test.go` | Nix 2.24.9 oracles. The stub scan covers `eval/`, `builder/`, `store/`, `cmd/`, and `parser/` (non-test files) and fails on `not implemented` or `unsupported`. The exemptions are Nix's own `cannot show unsupported input` and the parser fallthrough, which keeps `unsupported expression` with an explicit `stub-scan: exempt` comment |
| **outputHashMode** | `eval/output_hash_mode_oracle_test.go`, `store/local/git_hash_test.go` | `flat`, `nar`/`recursive`, `text`, and `git` out paths vs Nix 2.24.9. Git object hashes vs `git hash-object` / `git write-tree`. Unknown mode is `invalid value '…' for 'outputHashMode' attribute`. SHA-256 `text` matches. A non-SHA-256 `text` hash makes Nix 2.24.9 abort (`Assertion … SHA256` in `store-api.cc`); goonix returns `text output hash must be sha256` and does not abort the process |
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
- The sandbox brings up `lo` with `127.0.0.1` when the build has its own network namespace. `TERM` is `xterm-256color`. `NIX_BUILD_CORES` comes from the derivation when that sets it, otherwise from the `cores` setting (`0` is passed through), otherwise from the CPU count. The sandbox does not read Nix's `sandbox-paths`, it is not a multi-user `nixbld` pool, and there is no sandbox on/off flag.
- A `text` hash that is not SHA-256 returns `text output hash must be sha256`. Nix 2.24.9 aborts the process on `Assertion … ti.hash.algo == HashAlgorithm::SHA256` in `store-api.cc`. goonix does not abort the process. SHA-256 text mode matches and is tested.
- `fetchTree` of a `file` input hashes the NAR of the file. Field comparison against Nix is gated on `nix-instantiate --version` containing `2.24.9`.
- The global nixpkgs registry entry is a `tarball` (`nixexprs.tar.zst`). `getFlake` and `fetchTree` of an indirect id resolve that kind of target, including zstd, and a local `.tar.zst` oracle matches Nix. The test does not download the live channel tarball. That archive changes and is large.
- Parser corpus cases for CRLF and subversion are still skipped.
- POSIX `match` and `split` are not leftmost-longest.
- Lexical builtins are not in scope before a `with` the way Nix places them.
- The large hello-closure drvdiff (`TestDrvDiffHelloGoldenClosure`) is skipped.
- Mercurial `fetchTree` shells out to `hg` and is not byte-compared with Nix. Git fetches shell out to `git`. A local git repo is compared with Nix, including `submodules=1` and `dir`. Remote git URLs are not pinned.

## CGO

`bash scripts/check-cgo.sh` **green** (`CGO_ENABLED=0`).
