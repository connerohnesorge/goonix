# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — Nix-aligned `//` / fixpoint / `with`; past callPackage ping-pong; hello reaches `stdenv/cross` |
| #13 | Ladder / goldens | **partial** — same blocker as full `pkgs` import |

## Eval laziness refactor (Nix 2.24.9 model)

Removed stacked flags (`fixPublish`, `pendingMergeRhs`, `selfHideSym`, `publishUnderConstruction`, merge-left hide, partial-memo special cases).

Core behaviour now:

- **`//`**: `evalAttrs` both sides, merge binding maps without forcing values (`evalOpUpdate` + `mergeAttrBindMaps`).
- **Fixpoint force**: blackhole on re-entrant thunk force; `primeUpdateRhsMemo` for top-level `f x // rhs`; optional RHS-first priming when LHS is the fixpoint var; publish attr names into fixpoint memo while forcing overlay attrset literals.
- **`with`**: force `with` attrs only on fallback lookup (lexical bindings win).
- **Select / WHNF**: `forceValueDeep` (bounded) for attr navigation; self-referential defer detection in `forceThunk`.

Regression oracles: `eval/pending_merge_regression_test.go`, `eval/with_self_callpackage_test.go`, `eval/fixpoint_sibling_test.go`, `eval/formal_lib_test.go`, makeScope/xorg tests (unit-level **green** where they do not import full `pkgs`).

## Hello golden (gate)

- **Target:** `/nix/store/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`
- **goonix:** evaluates `<nixpkgs>` through lib, modules, and top-level `checked` overlays; fails at `pkgs/stdenv/cross/default.nix:23` (`value is not an attribute set` on `lib.init bootStages` path).

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `go test ./eval/...`: **green** except 8 integration tests that import full/minimal `pkgs` / xorg (stdenv cross / scope splicing)
- `go test ./ladder/... -run TestHelloDrvPathMatchesGolden`: **red** (same stdenv cross site)

## Ladder rungs

| Rung | Status |
|------|--------|
| hello `.drvPath` | **blocked** (stdenv cross / full pkgs boot) |
| dependent / toFile / stdenvNoCC / curl | **not started** |
| network `nix-build` rungs | **skipped** |
