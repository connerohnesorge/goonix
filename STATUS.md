# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

Evidence commands (from repo root, `CGO_ENABLED=0`, `testdata/nixpkgs` present):

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status | Evidence |
|-------|----------------|--------|----------|
| #2 | Eval core / laziness | **partial** | Lazy `mapAttrs`, lazy `with`, `//` self-merge through `let`, blackhole `or`; hello blocked on `lib/systems/parse.nix` |
| #3 | Value / data model | **done** | `eval/value.go`, `data/` used across eval + store |
| #4 | Parser | **partial** | Hand-written parser; attrset `Members` source order; 3 vendored `parse-okay-*` skipped in `lang/` |
| #5 | Builtins | **partial** | Large builtin set; `readDir` attrset; `__functor` / `applyCallableN` |
| #6 | NIX_PATH / import | **partial** | `eval/nixpath.go`, `import` cache |
| #7 | Builder | **stub** | `build/doc.go` |
| #8 | Substituter | **stub** | `substitute/doc.go` |
| #9 | CLI / integration | **partial** | `cmd/goonix`; ladder drives hello |
| #10 | CI | **done** | `.github/workflows/ci.yml`, `scripts/check-cgo.sh` green |
| #11 | go-nix decision | **done** | `docs/decisions/go-nix.md` |
| #12 | Store 4a | **partial** | `store/` path/hash/NAR |
| #13 | Ladder / goldens | **partial** | Hello golden **fails** in `parse.nix` (not xorg call stack) |
| #14 | Docs / tracking | **partial** | README; this file |

## Hello golden (primary v0 gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **Nix reference:** `nix-instantiate --eval` on pinned nixpkgs matches golden.
- **goonix:** `TestHelloDrvPathMatchesGolden` fails at `testdata/nixpkgs/lib/systems/parse.nix:915` (`undefined variable "parsed"` in `mkSystemFromSkeleton` let body). Same module fails earlier on `mkSystemFromString` / `kernels.darwin` at **line 570** (`undefined variable "darwin"` in `families = { inherit darwin; }`) when `types.kernel` / `attrValues kernels` forces alias merge paths.

### Eval fixes landed this iteration

- `evalCall` uses `applyCallableN` / `__functor` (unblocks `pkgs.xorg` attrset shape vs “attempt to call non-function”).
- Parser attrset **`Members`** preserves `inherit` before binds (nixpkgs `all-packages.nix` ordering).
- **`//` self-merge:** `selfMergeThunk` through `let` bodies; `self.memo` after left operand is forced; attrset `self.memo` after populate.
- Plain **`inherit`** in attrsets: lookup env = populate env; `inherit (from)` still binds into inner `bindEnv`.
- **`forceHideSym`** on attr bindings for `name = name // …` fixpoint patterns.
- Lazy **`builtins.mapAttrs`**, **`readDir`** attrset, lazy **`with`**.

### Remaining parse.nix scoping (blocker)

- `parse.kernels.macos` / `.ios` evaluate; `parse.kernels.darwin` (merge alias `darwin = kernels.macos`) and `builtins.attrValues parse.kernels` fail at inherit `darwin` (570).
- `parse.mkSystemFromSkeleton` / hello path fails at `parsed` in let body (915) once platform checks run.

Regression tests: `eval/merge_alias_test.go`, `eval/parse_kernels_test.go`, `eval/inherit_scope_test.go`, `eval/parsed_self_test.go`.

## Ladder rungs (no network build)

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (`parse.nix` above) |
| dependent / toFile / stdenvNoCC / curl | **not started** — blocked on hello eval |
| rungs needing `nix-build` / substituters | **skipped** (no network build in CI) |

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./eval/...`: **red** — nixpkgs integration tests (`TestHelloDrvPathMatchesGolden` path, `TestPkgsXorgIsAttrset`, `TestMkSystemFromStringX8664Linux`, kernel alias tests)
- `go test ./lang/...`: **green** (in-scope parse-okay files)
- `go test ./ladder/...`: **red** (hello golden)
