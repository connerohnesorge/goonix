# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9** (`internal/nixversion`, CI via `cachix/install-nix-action` + `install_url` for 2.24.9). nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./... -count=1
```

**CI:** ubuntu/macos `go test ./...` (CGO off). **ladder** job: Nix 2.24.9 + `./internal/oracle/...`, `./ladder/...`, `./eval/...`.

## Phase map

| Phase | Scope | Tests / evidence | Left |
|-------|--------|------------------|------|
| **0** | Repo, CI, CGO gate, goldens | `scripts/check-cgo.sh`, `.github/workflows/ci.yml`, `ladder/*`, `internal/oracle` | — |
| **1** | Parser / lang | `parser/*`, `lang/*` (`parse-okay-*`) | CRLF / subversion parse cases skipped in lang harness (upstream corpus gaps; documented in test) |
| **2** | Eval core | `eval/*`, fixpoint, merge, `with` | — |
| **2b** | Builtins, Nix files | `eval/builtins*.go`, oracle repros | — |
| **3** | Spike / decision | README, go-nix | — |
| **4a** | Store paths, hashes, ATerm | `store/*`, `store/drvshow/*`, modulo oracles | — |
| **4b** | Local SQLite store | `store/local/*` (800 LOC, 4 tests) | GC roots stub; no daemon |
| **5** | Builder | `builder/*` (693 LOC, 3 tests), `cmd/goonix build` | Linux user-namespace sandbox (flag; default off) |
| **6** | Substituter | `substituter/*` (338 LOC, 5 tests) | Live cache test skips offline (`TestSubstituteNetToolsFromCache`) |
| **7** | CLI | `cmd/goonix` (`build`, `eval`, …) | `TestGoonixBuildEchoHiInTempStore` |
| **#11** | go-nix integration | `store`, `eval` serialization | — |
| **#13** | Bootstrap ladder | `TestBootstrapStage2StdenvLinuxATermMatchesNix`, closure key-set probe | — |
| **#14** | drvPath ladder | `ladder.TestHelloDrvPathMatchesGolden`, stdenvNoCC, curl | `TestDrvDiffHelloGoldenClosure` diagnostic only |

### Package inventory (lines / tests)

| Package | Lines (.go) | Test functions |
|---------|-------------|----------------|
| `store/local` | 800 | `TestNarHashMatchesGoNixDump`, `TestAddToStoreTextAndReferences`, `TestNarHashMatchesNixStoreDump`, `TestGCRootStub` |
| `builder` | 693 | `TestTopoSortOrder`, `TestBuildTrivialShAndToFileDep`, `TestBuildEchoHiMatchesNixBuild` |
| `substituter` | 338 | `TestParseFixtureNarinfo`, `TestFixtureNarinfoSignatureVerifies`, `TestFixtureNarinfoSignatureRejectedWhenTampered`, `TestSubstituteNetToolsFromCache`, `TestDecompressNonePassthrough` |

Legacy stub packages `build/` and `substitute/` removed (replaced by `builder/` and `substituter/`).

## Skips (explained)

| Skip | Reason |
|------|--------|
| `lang` CRLF / subversion | Parser corpus cases not in scope for v0 |
| `TestBuild*` / CLI when no `/bin/sh` | Host has no POSIX shell |
| `TestBuildEchoHiMatchesNixBuild` when no `nix-build` | Nix CLI not installed |
| `TestPkgConfigFetchurl*` when no `/nix/store` | Fetchurl oracle needs host store path materialisation |
| `TestSubstituteNetToolsFromCache` / `testing.Short` | Network or cache unreachable |
| Oracle tests when `nix-instantiate` missing | CI ladder job provides Nix; unit job skips |

## Oracle / tooling

- **`internal/oracle`**: hermetic `nix-instantiate` (2.24.9); temp-store probes via `--store` + `ChrootIOEffects`.
- **`store/drvshow`**: Nix 2.24 flat map + v4; **exact map-key pairing** for duplicate logical drvs (stage2 stdenv).
- **`drvdiff`**: closure compare; BFS pairing with exact-key preference.

## Semantics (recent)

- **`derivationStrict`**: `drvPath` + outputs only on strict value; strict attr access throws on missing/non-attrs (Nix-aligned).
- **`__structuredAttrs`**: `TestStructuredAttrsStrictATermMatchesNix` (derivationStrict rung oracle).
- **Rung08 `toFile`**: drvPath oracle in isolated temp store (`ChrootIOEffects` + nix `--store`).
- **`makeScopeWithSplicing`**: `TestMakeScopeWithSplicingCallPackageWithAutoMergeMatchesNix` (small lib probe; invalid xorg abort probes removed).

## CGO

`bash scripts/check-cgo.sh` **green** (`CGO_ENABLED=0`).
