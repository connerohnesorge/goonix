# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — fixpoint pool + pending `//` `with`; hello at `callPackageWith` auto-args |
| #13 | Ladder / goldens | **partial** — past xorg `undefined callPackage`; `fetchurl` in scoped `callPackageWith` auto pool |

## Fixpoint sibling pool — **done**

Locked `lib.fix` thunks publish attribute names incrementally; `intersectAttrs` / `callPackageWith` see auto-args (e.g. `fetchurl`) via `fixPublish` memo without forcing every value.

## Pending `//` RHS visible in `with` — **done**

- **`with` lookup**: deferred merge thunks with `pendingMergeRhs` and **not** `fixPublish` merge LHS+RHS read-only (`mergeAttrBindMaps`).
- **`let self = lhs // rhs`**: `selfHideSym` on `//` bindings; merge-left hide returns `Deferred(currentForce)`.
- **`evalVar`**: defer locked `fixPublish` / `pendingMergeRhs` thunks.
- **`markFixPublishArg`**: do **not** set `fixPublish` on merge thunks (`selfHideSym` or `pendingMergeRhs`) — otherwise `f self` inside `makeScopeWithSplicing'` poisons `with self` lookup and generated xorg default loses `callPackage` (Nix 2.24.9 keeps merge and fixpoint paths separate).

Regression: `eval/pending_merge_regression_test.go`, `eval/make_scope_callpackage_test.go`, `eval/xorg_real_scope_test.go`.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** past xorg generated `default.nix:374` **`undefined variable "callPackage"`**; fails at **`lib.customisation.callPackageWith`** missing **`fetchurl`** in auto-args when real `newScope`/`__splicedPackages` applies package functions.

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **green** except full-`pkgs` import / xorg integration (expect `fetchurl` until auto pool fix)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (`callPackageWith` / `fetchurl`)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (`callPackageWith` auto-args / `fetchurl`) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
