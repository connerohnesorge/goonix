# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestImportNixpkgsNoStoreCopy|TestStringInterp|TestBootstrapStage0' -count=1
go test ./ladder/... -count=1
```

## Path contexts / import laziness (green)

- No **`importDepth`** gate: interpolation copies whenever the string is forced.
- **`inStringInterpolation`** only while coercing `"${…}"` parts (not whole literal bodies) — fixes eager NAR on import without hiding copies in imported modules.
- **`TestImportNixpkgsNoStoreCopy`**, **`TestStringInterpInsideImportedModuleMatchesNix`**, path oracle tests **green**.

## Bootstrap / stdenv probe (red)

- **Stage0** ATerm (glibc files, binutils-wrapper, stdenv-linux): **green**.
- **Stage1** stdenv-linux ATerm: **green**.
- **Stage2+** diverge: **`bootstrap-stage2-gcc-wrapper-13.3.0`** env references wrong **`xgcc`** / **`binutils-wrapper`** store paths vs Nix → propagates to **`TestStdenvNoCCMkDerivationOracleMatchesNix`**, **`TestHelloDrvPathMatchesGolden`**, curl ladder.
- Regression tests: **`TestBootstrapStage2StdenvLinuxATermMatchesNix`**, **`TestBootstrapStage2GCCWrapperATermMatchesNix`**.

## Ladder

| Test | Status |
|------|--------|
| input_drv / toFile | **green** |
| hello / stdenvNoCC / curl drvPath | **red** (bootstrap chain above) |

## Other

- Derivation **`==`** by **`drvPath`**: **green** (curl check-meta python).
- **CGO**: `bash scripts/check-cgo.sh` **green**.
