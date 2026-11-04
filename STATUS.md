# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9** (`internal/nixversion`, CI via `cachix/install-nix-action` + `install_url` for 2.24.9). nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./... -count=1
```

CI: **ubuntu/macos** run `go test ./...` (CGO off). **ladder** job installs Nix 2.24.9 and runs `./internal/oracle/...`, `./ladder/...`, `./eval/...`.

## Phase map

| Phase | Scope | Tests / evidence | Left |
|-------|--------|------------------|------|
| **0** | Repo, CI, CGO gate, goldens | `scripts/check-cgo.sh`, `.github/workflows/ci.yml`, `ladder/*`, `internal/oracle` | — |
| **1** | Parser / lang | `parser/*`, `lang/*` (`parse-okay-*`) | CRLF / subversion parse cases still skipped in lang harness |
| **2** | Eval core | `eval/*` unit tests, fixpoint, merge, `with` | Full xorg `makeScopeWithSplicing` on real `default.nix` (skipped; Nix aborts on some self-`//` probes too) |
| **2b** | Builtins, Nix files | `eval/builtins*.go`, oracle repros under `eval/testdata/` | `__structuredAttrs` byte oracle (not started) |
| **3** | Spike / decision | docs in README, go-nix usage | — |
| **4a** | Store paths, hashes, ATerm | `store/*`, `store/drvshow/*`, modulo oracles | — |
| **4b** | Local SQLite store | `store/local/*`, `TestNarHash*` / registration tests | GC roots stub only; no daemon |
| **5** | Builder | `builder/*`, trivial `sh` + toFile acceptance (Nix hash oracle when `nix-store` present) | Linux user-namespace sandbox (flag; default off, unsandboxed) |
| **6** | Substituter | `substituter/*`, fixture narinfo unit tests | Live cache integration optional/offline skip |
| **7** | CLI / integration | `cmd/goonix` | — |
| **#11** | go-nix integration | `store`, `eval` serialization | — |
| **#13** | Bootstrap ladder | ATerm oracles, `TestBootstrapStage2StdenvOracle*` | **stage2 stdenv byte ATerm** via stdenvNoCC closure probe: extra gcc-wrapper path in `env.defaultNativeBuildInputs` (skipped test; list oracle green) |
| **#14** | drvPath ladder | `ladder.TestHelloDrvPathMatchesGolden`, stdenvNoCC, curl | Full hello-closure byte diff (`TestDrvDiffHelloGoldenClosure`) is diagnostic only |

## Oracle / tooling

- **`internal/oracle`**: `nix-instantiate --eval --json` (2.24.9-safe); `EvalFileRawBytes` preserves string newlines.
- **`store/drvshow`**: parses Nix 2.24 `derivation show` (flat map) and v4 `{derivations,version}`.
- **`drvdiff`**: closure byte compare uses **NameStdenv BFS pairing** for duplicate logical drvs.

## Hello “green”

`ladder.TestHelloDrvPathMatchesGolden`: Nix-oracle `drvPath` then pinned golden `testdata/golden/hello.drvPath` (`…/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`). Not a full closure ATerm match.

## Semantics (recent)

- **`derivationStrict`**: returns `drvPath` + output paths only; `type`/`name`/… on `builtins.derivation` wrapper (`derivation_builtin.go`). Oracle: `TestDerivationStrictAttrNamesMatchNix`.
- **`genList`**: lazy elements (`deferredApplyCall`).
- **`builtins.nixVersion`**: `internal/nixversion` (no `testing` import in eval).

## CGO

`bash scripts/check-cgo.sh` **green** (`CGO_ENABLED=0`).
