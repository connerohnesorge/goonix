# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestImportNixpkgsNoStoreCopy|TestBootstrapTools|TestStage2CCExpand|TestBootstrapStage2CCAttr' -count=1
go test ./ladder/... -count=1
```

## Path contexts / import laziness (green)

- No **`importDepth`** gate; **`inStringInterpolation`** only while coercing `"${…}"` parts.
- Derivation env coercion: **`outPath` only** (no `out` fallback) — Nix `coerceToString` on attrsets.
- **`TestImportNixpkgsNoStoreCopy`**, path/interp oracle tests **green**.

## Bootstrap ATerm ladder (bottom-up)

| Node | Status |
|------|--------|
| **bootstrap-tools** | **green** (`TestBootstrapToolsATermMatchesNix`) |
| **stage0** glibc files, binutils-wrapper, stdenv-linux | **green** |
| **stage1** stdenv-linux | **green** |
| **stage2 cc attr oracle** (names / `? outPath` / DNBI shape) | **green** |
| **stage2 cc → expand-response-params stdenv** (via `.drv`) | **green** |
| **stage2 xgcc / binutils-wrapper / gcc-wrapper / stdenv** (byte ATerm) | **red** — paired via gcc-wrapper **`inputDrvs`**; env store paths differ from upstream drv graph |
| stdenvNoCC probe, hello, curl | **red** (propagated) |

Regression tests: `TestBootstrapStage2XgccATermMatchesNix`, `TestBootstrapStage2BinutilsWrapperATermMatchesNix`, `TestBootstrapStage2GCCWrapperATermMatchesNix`, `TestBootstrapStage2StdenvLinuxATermMatchesNix`.

## Ladder

| Test | Status |
|------|--------|
| input_drv / toFile | **green** |
| hello / stdenvNoCC / curl drvPath | **red** |

## Other

- Derivation **`==`** by **`drvPath`**: **green**.
- **CGO**: `bash scripts/check-cgo.sh` **green**.
