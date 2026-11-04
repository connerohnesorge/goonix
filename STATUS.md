# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — fixpoint sibling pool for `intersectAttrs` **done**; hello at xorg `with self` / `callPackage` |
| #13 | Ladder / goldens | **partial** — import past `fetchurl` auto-args; xorg generated `default.nix` |

## Fixpoint sibling visibility (this turn) — **done**

While a **`lib.fix` / `let x = f x`** thunk is locked (`fixPublish`), attribute names are **published incrementally** into `memo` as bindings are inserted (not only when the attrset finishes). `attrsBindingMap` / `intersectAttrs` read that pool for deferred fixpoint `self` / `__splicedPackages` without forcing every value. `forceValue` on a locked fixpoint deferred returns the in-progress `memo` instead of blackhole or re-entering the force.

Regression tests: `eval/fixpoint_sibling_test.go`, `eval/fixpoint_callpackage_test.go`.

Also unchanged: **`with self` + `let self = lhs // rhs`** (`pendingMergeRhs`, `mergeAttrBindMaps`), **`builtins.sort`**, **`unsafeGetAttrPos`**.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** past **`callPackageWith` / `fetchurl` auto-args**; fails in xorg generated `default.nix` with **`undefined variable "callPackage"`** (`self: with self;` inside `makeScopeWithSplicing'` while `self = f self // { callPackage = … }` is still forcing — next: RHS merge siblings visible in `with` without polluting fixpoint `memo`).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **green** except full-`pkgs` import / xorg integration (4 tests)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (same xorg `callPackage` abort)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (xorg scope `callPackage` in `with self`) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
