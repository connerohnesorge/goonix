# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — `with self` + pending `//` merge fixed; hello at `callPackageWith` auto-arg pool |
| #13 | Ladder / goldens | **partial** — import reaches `lib.customisation.callPackageWith` (missing auto args) |

## Callable / `with self` + `let self = lhs // rhs` — **done (this turn)**

While `self` is forcing `f self // { callPackage = … }`, locked thunks now expose **`lhs // rhs` keys** via `pendingMergeRhs` (including when `memo` is only the LHS). `with self` on a deferred self-merge argument materializes that merged view before body lookup.

Regression tests: `eval/with_self_callpackage_test.go`, `eval/makeoverridable_self_test.go`.

Also: **`builtins.sort`**, **`unsafeGetAttrPos`** returns null for bool `functionArgs` maps (error paths use `fn` location); lazy **`intersectAttrs`** binding maps.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** past xorg / `recurseIntoAttrs`; fails importing `pkgs` with  
  `lib.customisation.callPackageWith: Function called without required argument "fetchurl"`  
  (auto-args from `__splicedPackages` / fixpoint `self` while locked — next: fixpoint sibling visibility for `intersectAttrs` / `callPackageWith`).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **green** except full-`pkgs` import / xorg integration (4 tests)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (same `callPackageWith` abort)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (`callPackageWith` auto pool / fixpoint) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
