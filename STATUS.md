# goonix v0 status (issues #2–#14)

Oracle: **Nix 2.24.9**, nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`**. Branch: `cursor/phase-0-scaffolding-7255`, PR #15.

```bash
bash scripts/check-cgo.sh
go test ./eval -run 'TestImportNixpkgsLibVersion|TestNullDynamic|TestXgccNoEmpty|TestBootstrapTools|TestBootstrapStage2GCCWrapperEnvContext|TestStdenvDrvPath' -count=1
```

## Fixes (recent)

- **Null dynamic attr names** (`${if … then "CC" else null} = …`) are dropped like Nix (fixes xgcc **`env[""] = "gcc -m64"`** from gcc’s solaris `CC` stub).
- **`derivationStrict` env context invariant** in eval: string-context drv outputs must appear in env text and in `inputDrvs`; structured attrs checked via `__json`.
- **`derivationStrict` memo** by `drvAttrs` attr-set identity; **`WithVerifyStrictMemo`** recomputes on cache hit in tests.
- **`builtins.placeholder`** returns `"/" + nixBase32(sha256("nix-output:" + name))` at eval time (no string context, no strict rewrite); oracles in `placeholder_oracle_test.go`.
- **Derivation env** store paths in space-separated lists get `/nix/store/` when coerced as bare fingerprints.
- **Stage1 stdenv oracle** (`stage1_stdenv_oracle.nix`): `cc == null`, `cc ? outPath`, `hasCC`, `dnbiLen`, `dnbiNames`, `preHook` metrics.
- **`outPath`-only** attrset coercion (no `out` fallback).

## Bootstrap ATerm ladder

| Node | Status |
|------|--------|
| bootstrap-tools, stage0/1 | **green** |
| stage2 cc / ERP stdenv oracles | **green** |
| perl (stage1) placeholder env | **fixed** (configureFlags); downstream hashes still diverge until stdenv list/env bugs fixed |
| stage1 stdenv `defaultNativeBuildInputs` | **red** (path set / store prefix vs Nix; topo culprit) |
| xgcc, binutils-wrapper, gcc-wrapper, stage2 stdenv | **red** (byte ATerm; pair by gcc-wrapper → xgcc) |
| stdenvNoCC, hello, curl | **red** |

## Import / contexts

- **`TestImportNixpkgsLibVersionOnly`**, **`TestImportNixpkgsNoStoreCopy`**, path interp oracles **green**.

## CGO

`bash scripts/check-cgo.sh` **green**.
