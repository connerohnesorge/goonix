# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestPathPlusOracle|TestBootstrapXgcc|TestBootstrapStage2Binutils|TestBootstrapStage2GCCWrapper|TestBootstrapStage2Stdenv|TestImportNixpkgsLibVersion' -count=1
go test ./ladder -count=1
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
| bootstrap-stage2-stdenv-linux | **green** (byte ATerm + `stage2_stdenv_oracle.nix` list attrs) |
| stdenvNoCC probe / hello ladder drvPath | **green** (see below) |
| curl ladder drvPath | **green** (`TestCurlLadderMatchesGolden`; full run, not `-short`) |

**Hello “green”** means `ladder.TestHelloDrvPathMatchesGolden`: goonix `drvPath` is checked against Nix via `evalLadderDrvPath`, then against the pinned golden file `testdata/golden/hello.drvPath` (`…/gciipqhqkdlqqn803zd4a389v86ran45-hello-2.12.1.drv`). That is **not** a full hello-closure byte ATerm match (`eval/drvdiff_test.go` closure test remains separate/skipped in short runs).

## CGO

`bash scripts/check-cgo.sh` **green**.
