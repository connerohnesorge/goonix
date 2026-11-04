# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — fixpoint pool + pending `//` `with` lookup; hello at xorg generated `default.nix` |
| #13 | Ladder / goldens | **partial** — import past `fetchurl` auto-args; xorg `callPackage` in generated default |

## Fixpoint sibling pool — **done** (prior commit + `attrsBindingMap` order fix)

Locked `lib.fix` thunks publish attribute names incrementally; `intersectAttrs` / `callPackageWith` see auto-args (e.g. `fetchurl`) via `fixPublish` memo without forcing every value.

## Pending `//` RHS visible in `with` (this turn) — **done (unit / makeScope)**

- **`with` lookup**: if `env.With` is a deferred thunk with `pendingMergeRhs` and **not** `fixPublish`, merge LHS+RHS **read-only** (`mergeAttrBindMaps`) — never publishes into the pkgs fixpoint memo.
- **`let self = lhs // rhs`**: `selfHideSym` on `let` bindings whose value is `//` (same as attrset inserts).
- **Merge-left `f self`**: hide returns `Deferred(currentForce)` so the fixpoint argument stays the in-flight merge thunk.
- **`evalVar`**: deferred for locked `fixPublish` / `pendingMergeRhs` thunks (avoids passing partial `memo` without RHS keys).

Regression: `eval/pending_merge_regression_test.go`, `eval/make_scope_callpackage_test.go`.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** past **`fetchurl` / `callPackageWith` auto-args**; fails at xorg generated `default.nix:374` — **`undefined variable "callPackage"`** under `self: with self;` (full `callPackageWith` + real `makeScopeWithSplicing'` path; stub makeScope tests **green**).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **green** except full-`pkgs` import / xorg integration (4 tests)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (xorg `callPackage`)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (xorg generated default `callPackage`) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
