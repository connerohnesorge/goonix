# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — strict blackhole; Nix `//` order; lazy `or` / `addErrorContext`; modules `config` still recurses |
| #13 | Ladder / goldens | **blocked** on modules `config` |

## Strict blackhole (no partial memo)

Removed: `currentForce`, `publishAttrSetIfForcing`, `publishBindingIfForcing`, `shouldPrimeUpdateLHS`, locked-thunk memo peek during force.

- **`//`**: RHS `evalAttrs` then LHS (Nix `evalOpUpdate` order).
- **`forceValue` / `forceThunk`**: locked without completed memo → infinite recursion (no partial attrset).
- **Fixpoint tests**: bare `final // { … }` is **invalid in real Nix** (`testdata/oracle/fixpoint_bare_update.nix`); tests use **`lib.extends` + `lib.fix`** (`fixpoint_extends.nix`).

## Laziness fixes (Nix-aligned)

- **`e1 or e2`**: cursed-invoke / `SelectOr` — do not `mustWHNF`/`forceValue` the fixpoint when choosing the left side.
- **`builtins.addErrorContext`**: lazy wrapper (context applied when the inner value is forced).
- **`equalValues`**: WHNF peel; `null` on either side without deep attr forcing.
- **`builtins.is*`**: `mustWHNF` on the scrutinee (not full `forceValue`).
- **`with` lookups**: `forceWHNF` on the attrset (not `forceAttrs`).

## Oracle reproducers

See `testdata/oracle/` and `eval/oracle_repro_test.go` (runs when `nix-instantiate` is on PATH).

## Blocker: `lib/modules.nix:270` (`config` fixpoint)

Minimal repro: `eval/modules_config_repro_test.go` (`configEval.config` with `config.nix` + `{ config = { }; }`).

Nix evaluates this; goonix still recurses while evaluating option `_module.freeformType` and **module argument `config`** (`applyModuleArgs` / `addErrorContext` path). Next trace: who forces the `config` formal during option merge before the merged `config` let completes.

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- Lib WHNF oracles, extends fixpoint, pending-merge `with self`, `evalModules` empty: **green**
- Full `import <nixpkgs>` / hello golden: **red** (modules `config`)
