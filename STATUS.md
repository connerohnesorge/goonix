# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestPathPlusOracle|TestBootstrapXgcc|TestBootstrapStage2Binutils|TestBootstrapStage2GCCWrapper|TestImportNixpkgsLibVersion' -count=1
go test ./ladder -run Hello -count=1
```

## Fixes (recent)

- **Path `+`**: path-left stays `path` (no context); path + store-context string errors; string-left copies path to store (`eval/testdata/path_plus_oracle.nix`).
- **`getContext`** rejects path values like Nix.
- **`builtins.toString` / `concatStringsSep`**: preserve string context (xgcc `LIBRARY_PATH`, zlib `inputDrvs`).
- **Closure pairing**: `NameStdenvPairID` when matching logical drv names (binutils-wrapper duplicate instances).
- **xgcc**, **stage2 gcc-wrapper**, **binutils-wrapper** byte ATerm oracles **green** (with pairing).
- **Duplicate stdenv** topo pairing: BFS from closure root.

## Bootstrap ATerm ladder

| Node | Status |
|------|--------|
| bootstrap-tools, stage0/1 env bytes | **green** |
| xgcc, gcc-wrapper, binutils-wrapper | **green** |
| bootstrap-stage2-stdenv-linux | **red** (go `defaultNativeBuildInputs` env lists extra `bootstrap-stage2-gcc-wrapper` path vs Nix) |
| stdenvNoCC probe / hello ladder drvPath | **green** (golden) |
| curl | **red** (large closure; not re-run this turn) |

## CGO

`bash scripts/check-cgo.sh` **green**.
