# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestImportNixpkgsLibVersion|TestNullDynamic|TestBootstrapXgcc|TestBootstrapTools|TestBootstrapStage2GCCWrapper|TestStdenvDrvPath' -count=1
```

## Fixes (recent)

- **`builtins.toString`** keeps string context (gcc `env = mapAttrs (_: v: toString v)` for `CPATH` / `LIBRARY_PATH`).
- **`builtins.concatStringsSep`** merges per-element string context (Nix `__concatStringsSep`).
- **`derivationStrict` + `__ignoreNulls`**: force attr values before dropping nulls (no empty `libcCross` / `targetConfig` env keys).
- **xgcc drvAttrs oracles** (`xgcc_drvattrs_oracle.nix`, strict env / inputDrvs / `getContext` on `LIBRARY_PATH`).
- **`equalValues`**: `drvPath` shortcut only when **both** sides are derivations (`type == "derivation"`).
- **Duplicate stdenv pairing** in topo byte diff: **BFS order from closure root**, not longest `defaultNativeBuildInputs`.
- **Path/string `+`**: string result with merged context (no silent `Path` concat drop).
- **Null dynamic attr names**, list empty elements, stage0/1 env invariants, placeholder, strict memo (unchanged from prior notes).

## Bootstrap ATerm ladder

| Node | Status |
|------|--------|
| bootstrap-tools, stage0/1 stdenv env bytes | **green** |
| stage2 cc / ERP stdenv oracles | **green** |
| texinfo, xgcc byte ATerm (topo name+stdenv) | **green** |
| gcc-wrapper (stage2) | **green** |
| binutils-wrapper, stage2 stdenv | **red** (closure pairing / upstream drv) |
| stdenvNoCC, hello, curl | **red** |

## Import / contexts

- **`TestImportNixpkgsLibVersionOnly`**, path interp oracles, **`TestStringContextAuditMatchesNix`** (incl. `toString`) **green**.

## CGO

`bash scripts/check-cgo.sh` **green**.
