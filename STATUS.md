# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — strict blackhole; lazy `zipAttrsWith`, formals, `or`, `addErrorContext`; import `pkgs //` still recurses |
| #13 | Ladder / goldens | **blocked** on full `import <nixpkgs>` (`splice.nix` / `callPackage`) |

## Strict blackhole (no partial memo)

Removed: `currentForce`, `publishAttrSetIfForcing`, `publishBindingIfForcing`, `shouldPrimeUpdateLHS`, locked-thunk memo peek during force.

- **`forceValue` / `forceThunk`**: locked without completed memo → infinite recursion (no partial attrset).
- **Fixpoint tests**: bare `final // { … }` is **invalid in real Nix** (`testdata/oracle/fixpoint_bare_update.nix`); tests use **`lib.extends` + `lib.fix`** (`fixpoint_extends.nix`).

## Laziness fixes (Nix-aligned)

- **Formals** (`applyFormals`): `forceAttrs` on the single argument (WHNF only); bind declared formals as lazy refs from the arg set; defaults lazy; do not copy the whole arg attrset into the env.
- **`a.${n} or d` (`SelectOr`)**: force base to WHNF only; if present return binding unforced; else default. Cursed `e or e2` stays in `evalCall` / `evalOrFallback`.
- **`mapAttrs` / `functionArgs` / `addErrorContext`**: `mapAttrs` forces function + set WHNF; values are lazy `f name attr`; `functionArgs` forces only the function; `addErrorContext` returns the value and attaches context on force failure (`Thunk.errCtx`).
- **`//`**: both operands `evalAttrs` (WHNF heads); merge binding maps without forcing values.
- **`zipAttrsWith`**: **lazy per output key** (was eagerly forcing every name in the union — root cause of `lib/modules.nix:270` via `pushedDownDefinitionsByName`).
- **`with` + pending `//`**: RHS attribute lookup on unevaluated `lhs // rhs` for missing names (e.g. `callPackage` on fixpoint self).
- **`equalValues`**: WHNF peel; `null` short-circuit.
- **`builtins.is*`**: `mustWHNF` on scrutinee.
- **`with` lookups**: `forceWHNF` on the attrset (plus pending `//` RHS).

## Oracle reproducers

See `testdata/oracle/` and `eval/oracle_repro_test.go` (runs when `nix-instantiate` is on PATH).

## Blocker: full `import <nixpkgs>`

- **`TestEvalModulesTopLevelConfigNix`**: **green** (fixed via lazy `zipAttrsWith`).
- **Import / hello golden**: **red** — e.g. `pkgs/top-level/splice.nix:152` (`pkgs // …` while `pkgs` fixpoint still forcing); xorg `callPackage` scope.

Debug: `GOONIX_TRACE_FORCE=1 go test ./eval/... -run TestEvalModulesTopLevelConfigNix -v` prints a force stack on infinite recursion.

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- Lib WHNF oracles, extends fixpoint, pending-merge `with self`, `evalModules` config repro: **green**
- Full `import <nixpkgs>` / hello golden: **red**
