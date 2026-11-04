# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — parse.nix scoping fixed; hello blocked on xorg/callPackage path |
| #13 | Ladder / goldens | **partial** — hello fails at `all-packages.nix:13754` |

## Scoping fix (parse.nix) — **done**

Root cause for `lib/systems/parse.nix:570` (`inherit darwin`) and `:915` (`parsed`): **`forceHideSym` hid every lookup of the attribute name while forcing a same-named binding** (e.g. forcing `darwin = kernels.macos` hid the let-bound `darwin` used inside nested `{ inherit darwin; }`).

Fix: hide only on **`//` left operand** when the binding is the **current force thunk** (`inMergeLeft` + `currentForce` + `selfHideSym`).

Additional lexical scoping: **non-recursive attrsets** no longer register `bindEnv.bindThunk` for each name (fixes `{ module = module }` infinite recursion without shadowing let bindings).

Regression tests: `eval/parse_kernels_test.go`, `eval/merge_alias_test.go`, `eval/parsed_self_test.go`, `eval/inherit_scope_test.go`.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** fails at `all-packages.nix:13754` (`recurseIntoAttrs xorgPackages` — `attempt to call non-function`). Same failure on pre-scoping commit when importing full `pkgs`; next focus: `callPackageWith` / `makeOverridable` / `builtins.functionArgs` for `self:` package functions (xorg `default.nix`).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **red** — `TestPkgsXorgIsAttrset`, `TestRecurseIntoAttrsTypeInPkgs` (full `pkgs` import)
- `go test ./ladder/...`: **red** — hello golden (xorg path above)
- Parse / platform tests (`TestMkSystemFromStringX8664Linux`, kernel alias tests): **green**

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (xorg / callPackage) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
