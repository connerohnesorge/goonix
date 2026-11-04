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
| #2 | Eval core / laziness | **partial** | Lazy `mapAttrs`, lazy `with`, `//` self-merge, blackhole `or` defaults; hello still blocked on xorg scope |
| #3 | Value / data model | **done** | `eval/value.go`, `data/` used across eval + store |
| #4 | Parser | **partial** | Hand-written parser; 3 vendored `parse-okay-*` cases skipped in `lang/` (CRLF, indented strings, subversion) |
| #5 | Builtins | **partial** | Large builtin set; `readDir` fixed to attrset; `zipAttrsWith`; `__functor` calls |
| #6 | NIX_PATH / import | **partial** | `eval/nixpath.go`, `import` cache, `testdata/nix/fetchurl.nix` |
| #7 | Builder | **stub** | `build/doc.go` — not required for drvPath eval |
| #8 | Substituter | **stub** | `substitute/doc.go` — not required for drvPath eval |
| #9 | CLI / integration | **partial** | `cmd/goonix` parse/eval/instantiate; ladder drives hello |
| #10 | CI | **done** | `.github/workflows/ci.yml`, `scripts/check-cgo.sh` green |
| #11 | go-nix decision | **done** | `docs/decisions/go-nix.md`, `store/` wraps pure go-nix |
| #12 | Store 4a | **partial** | Path/hash/NAR in `store/`; used for derivation text |
| #13 | Ladder / goldens | **partial** | `TestHelloDrvPathMatchesGolden` **fails** at `all-packages.nix:13754` (`makeScopeWithSplicing'` / xorg); golden path unchanged |
| #14 | Docs / tracking | **partial** | README updated; this file replaces missing GitHub issue comments |

## Hello golden (primary v0 gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **Nix reference:** `nix-instantiate --eval -E 'with import ./testdata/nixpkgs { ... }; hello.drvPath'` matches golden.
- **goonix:** fails during `pkgs.xorg` (`makeScopeWithSplicing'` / `lib.extends` / `callPackageWith` chain). Prior fixes: lazy aliases (`mapAttrs`), `readDir` attrset, `__functor`, lazy `with` for fixpoint `callPackage`.

## Ladder rungs (no network build)

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (see above) |
| dependent / toFile / stdenvNoCC / curl | **not started** — depends on hello path |
| rungs needing `nix-build` / substituters | **skipped** (documented) |

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./eval/...`: **green** (incl. lazy `mapAttrs`, `readDir`)
- `go test ./lang/...`: **green** (in-scope parse-okay files only)
- `go test ./ladder/...`: **red** (hello golden)
