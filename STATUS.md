# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

Debug type mismatches (thunk kind, memo, thunk source pos):

```bash
GOONIX_DEBUG_TYPE=1 go test ./eval/... -run TestImportNixpkgsOnly -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — unified `forceValue` peel; lib WHNF oracles green; full import blocked at modules |
| #13 | Ladder / goldens | **partial** — same import blocker |

## WHNF (Nix 2.24.9)

- **`forceValue`**: loops on deferred heads — memo peel, then `forceThunk`; locked without memo → infinite-recursion error; no 512-step cap.
- **`forceWHNF`**: alias of `forceValue` (may still return a thunk in soft contexts).
- **`mustWHNF`**: `forceValue` + error if still `VKDeferred` — calls, `//` / forceAttrs, lists, `if`/bool/arithmetic/strings, `typeOf`.
- **Select / `?`**: `mustWHNF` on base and non-final segments; **final segment stays lazy**. HasAttr uses soft `forceWHNF` on base (Nix `?` navigation).
- **`applyCallable`**: single `mustWHNF` on function head (no double `forceValue`).

## Fixpoint (load-bearing)

- **`shouldPrimeUpdateLHS` RHS-first `//` priming** when LHS is the fixpoint var bound to `currentForce` (`TestFixpointIntersectAttrsSeesPublishedBinds`).
- **`publishAttrSetIfForcing`** + **`publishBindingIfForcing`**: merge in-progress attr names into the fixpoint blackhole memo while a force is active.
- **`with` lookups**: `forceWHNF` on the `with` attrset (not full `forceAttrs`).

## Lib WHNF oracles (green)

- `builtins.isAttrs (import lib)`
- `builtins.typeOf (import lib) == "set"`
- `(import lib).trivial.id 1 == 1`
- minimal `fix` + `inherit (self.t) a` (`eval/lib_oracle_test.go`)

## Full `import <nixpkgs>` blocker

`TestImportNixpkgsOnly` fails at **`lib/modules.nix:270`** while evaluating option `_module.freeformType` / module argument `config` (infinite recursion during `forceValue` on the module `config` fixpoint). Past the previous **`stdenv/cross/default.nix:23`** “not an attribute set” failure when `lib` stayed a thunk.

Regression: `eval/stdenv_cross_oracle_test.go` (`isLinux` / bootStages subexpr types).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- Lib WHNF / fixpoint / with / callPackage / checked / WHNF unit tests: **green**
- Full `pkgs` import / hello golden: **red** at modules `config` (above)

## Ladder

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (modules `config`) |
| later rungs | **not started** |
