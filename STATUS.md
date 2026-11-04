# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — `recurseIntoAttrs`/xorg line fixed; hello now fails later on `callPackage` scoping / `let self = lhs // rhs` |
| #13 | Ladder / goldens | **partial** — import progresses past xorg; blocked ~`all-packages.nix:10294` |

## Scoping fix (parse.nix) — **done**

Root cause for `lib/systems/parse.nix:570` (`inherit darwin`) and `:915` (`parsed`): **`forceHideSym` hid every lookup of the attribute name while forcing a same-named binding** (e.g. forcing `darwin = kernels.macos` hid the let-bound `darwin` used inside nested `{ inherit darwin; }`).

Fix: hide only on **`//` left operand** when the binding is the **current force thunk** (`inMergeLeft` + `currentForce` + `selfHideSym`).

Additional lexical scoping: **non-recursive attrsets** no longer register `bindEnv.bindThunk` for each name (fixes `{ module = module }` infinite recursion without shadowing let bindings).

Regression tests: `eval/parse_kernels_test.go`, `eval/merge_alias_test.go`, `eval/parsed_self_test.go`, `eval/inherit_scope_test.go`.

## Callable / pkgs import fixes — **partial**

- **`builtins.intersectAttrs`**: required for real `lib.callPackageWith` / `makeOverridable` (two-stage xorg `default.nix` imports).
- **`inherit (lib) …` in non-recursive attrsets**: use `populateEnv` for `inherit (from)` thunks so `lib` resolves to the outer formal, not `pkgs.lib` via `with pkgs`.
- **`lookupEnv`**: walk lexical `Vars` up the parent chain before `with` on each frame (formals beat `with` shadowing).
- **`let self = lhs // rhs`**: stash RHS attr shells in `pendingMergeRhs`; while `self` is forcing, expose `lhs // rhs` keys (e.g. `callPackage` from `makeScopeWithSplicing'`) to re-entrant lookups; force `with` attrs to a value.

**Fixed:** `all-packages.nix:13754` (`recurseIntoAttrs xorgPackages` — callee was blackhole `{ … }` from wrong `lib` lookup).

**Next blocker:** full `pkgs` import / hello golden — `undefined variable "callPackage"` (e.g. `all-packages.nix:10294`, xorg `default.nix:374` in isolated tests). Likely remaining gap: `self` passed into `{ … }: self: with self` package functions during `let self = f self // { callPackage = … }` still missing merged RHS in some paths.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** past xorg `recurseIntoAttrs`; fails on `callPackage` while evaluating nixpkgs fixpoint / xorg generated package set.

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **mostly green**; red: full `pkgs` import / xorg integration tests (`TestImportNixpkgsOnly`, `TestPkgsXorgIsAttrset`, `TestHello…` via ladder)
- Parse / platform / callPackage unit tests (`TestMkSystemFromStringX8664Linux`, kernel alias, `TestCallPackageWithTwoStageImport`, merge/inherit scope): **green**

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (`callPackage` / scope merge) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
