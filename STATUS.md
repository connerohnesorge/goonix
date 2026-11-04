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
| #2 | Eval core / laziness | **partial** — `forceWHNF` / `mustWHNF` split; cross line 23 still blocked |
| #13 | Ladder / goldens | **partial** — same stdenv cross blocker |

## WHNF (Nix 2.24.9)

- **`forceWHNF`**: peel memo chains then `forceValue`; locked without memo → infinite-recursion error (no pass-through); soft cap 512 steps then may still return a thunk.
- **`mustWHNF`**: `forceWHNF` + error if still `VKDeferred` — used for **calls**, **`//` / forceAttrs**, **lists**, **`if`/bool/arithmetic/strings**, **`typeOf`**.
- **Select / `?`**: **soft `forceWHNF` only** (never `mustWHNF` on select — re-enters module `equalValues` and hangs); final path segment stays lazy.
- **`applyCallable` / `evalCall`**: `mustWHNF` on function head (removed double `forceValue` hack).

## Fixpoint (load-bearing)

- **`shouldPrimeUpdateLHS` RHS-first `//` priming**: required (`TestFixpointIntersectAttrsSeesPublishedBinds`).
- Removed: `primeUpdateRhsMemo`, `publishAttrSetIfForcing`.

## stdenv cross line 23 (`lib.init bootStages`)

With `GOONIX_DEBUG_TYPE=1`, import fails with:

`want attribute set; got kind=thunk locked=false memo=true thunkPos=.../pkgs/top-level/default.nix:58:17`

(pristineLib import) when select needs `lib` as attrset at `cross/default.nix:23`. Eager `mustWHNF` on whole `lib` at applyFormals hits `lib/default.nix:11` fixpoint blackhole.

Regression: `eval/stdenv_cross_oracle_test.go` (subexpr types vs Nix when `nix-instantiate` on PATH).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- Fixpoint / with / callPackage / checked / WHNF unit tests: **green**
- Full `pkgs` import / hello golden: **red** at stdenv cross (above)

## Ladder

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (stdenv cross) |
| later rungs | **not started** |
