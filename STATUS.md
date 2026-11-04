# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval/... ./lang/... -count=1
go test ./eval/... -run TestBuiltinLazinessAudit -count=1
go test ./ladder/... -run TestHelloDrvPathMatchesGolden -count=1
```

| Issue | Title (short) | Status |
|-------|----------------|--------|
| #2 | Eval core / laziness | **partial** — strictness audit + lazy list/call/builtins; import still missing `callPackage` in some scopes |
| #13 | Ladder / goldens | **blocked** on full `import <nixpkgs>` |

## Strict blackhole (no partial memo)

No `publishAttrSetIfForcing` / locked-thunk memo peek. Fixpoint `//` when LHS hits infinite recursion during `forceAttrs` merges with an **empty** LHS bind map so RHS keys (e.g. `callPackage`) still attach — companion to strict blackhole, not value peeking.

## Laziness work (2026-11-04)

- **`zipAttrsWith`**: lazy per output key (fixed `lib/modules.nix:270`).
- **Table-driven audit**: `eval/builtin_laziness_audit_test.go` (`TestBuiltinLazinessAudit`) — `throw "forced"` probes + optional `nix-instantiate` oracle.
- **Lazy list elements** and **lazy call arguments** (Nix application semantics).
- **Builtins**: `mapAttrs`, `map`/`genList` (lazy results), `removeAttrs`/`getAttr`/`hasAttr`/`intersectAttrs`/`attrValues`/`catAttrs`/`listToAttrs` use `forceAttrs` / unforced bindings where appropriate; `functionArgs`/`filter`/… use `mustWHNF` on functions only.
- **Removed** pending `//` RHS lookup in `with` (partial peeking).
- **Formals**, **`SelectOr`**, **`addErrorContext`**, **`//`** WHNF merge unchanged from prior commit.

## Tests

- `bash scripts/check-cgo.sh`: **green**
- `go test ./lang/...`: **green**
- `TestEvalModulesTopLevelConfigNix`, `TestBuiltinLazinessAudit`, `TestPendingMergeWithSelfAndFixpointIntersectAttrs`: **green**
- Full `import <nixpkgs>` / hello golden: **red** (`xorg/default.nix` — `callPackage` still undefined in some generated scopes; import progresses past earlier line numbers)

Debug: `GOONIX_TRACE_FORCE=1` on infinite recursion (see `eval/eval.go`).
