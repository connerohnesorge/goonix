# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — Nix WHNF select/`?`; top-level `checked` chain; stdenv cross still blocked |
| #13 | Ladder / goldens | **partial** — same stdenv cross blocker |

## Eval laziness (Nix 2.24.9)

- **`//`**: `evalOpUpdate` + `mergeAttrBindMaps` (no deep force of attr values).
- **Select / `?`**: `forceWHNF` (thunk peel only, stop on locked fixpoint thunks); **final path segment is not WHNF-forced** (lazy attr binding).
- **`isList` / `forceList`**: two-step `forceValue` to WHNF list (not select-style deep walk).
- **Calls**: `evalCall` uses `forceValueDeep` on the function; **`applyCallable` does a second `forceValue` when the head is still deferred** so curried `throwIfNot … lib.foldr …` chains work with lazy select results.
- **Fixpoint**: blackhole + **`shouldPrimeUpdateLHS` RHS-first `//` priming** remains **load-bearing** (`TestFixpointIntersectAttrsSeesPublishedBinds`).
- **Removed (oracle-green without them)**: `primeUpdateRhsMemo`, `publishAttrSetIfForcing`.

Regression tests: `eval/whnf_regression_test.go` (`TestLibInitBootStagesCrossShape`), `eval/whnf_internal_test.go`, `eval/checked_expr_test.go`, fixpoint/with/callPackage oracles.

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** evaluates through lib, modules, and `checked` overlays; fails at `pkgs/stdenv/cross/default.nix:23` (`value is not an attribute set` on `lib.init bootStages` path).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: unit + oracle **green**; full/minimal `pkgs` import and xorg/scope integration **red** at stdenv cross (same site)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (stdenv cross)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (stdenv cross / `lib.init bootStages`) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
