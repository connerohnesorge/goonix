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
- **Reverted derivation env “fingerprint normalization”** (no `strings.Fields` / prepending `/nix/store/`); env bytes must match Nix coercion exactly (stage0/stage1 invariant tests).
- **`coerceToString` on lists** keeps empty elements (fixes e.g. texinfo `buildInputs` trailing space vs Nix).
- **Topo pairing** for duplicate `bootstrap-stageN-stdenv-linux` nodes prefers the instance whose `defaultNativeBuildInputs` includes the stage gcc-wrapper.
- **Stage0/1 stdenv env invariants** vs Nix `.drv` bytes; stage1 head-hook `outPath` oracle.
- **`outPath`-only** attrset coercion (no `out` fallback).

## Bootstrap ATerm ladder

| Node | Status |
|------|--------|
| bootstrap-tools, stage0/1 stdenv env bytes | **green** (invariant tests; disambiguate duplicate stage1 stdenv) |
| stage2 cc / ERP stdenv oracles | **green** |
| texinfo (stage1), list→env trailing space | **green** |
| xgcc byte ATerm | **red** (go env has extra empty `libcCross` / `targetConfig` keys Nix omits with `__ignoreNulls`) |
| binutils-wrapper, gcc-wrapper, stage2 stdenv | **red** (downstream of xgcc / pairing) |
| stdenvNoCC, hello, curl | **red** |

## Import / contexts

- **`TestImportNixpkgsLibVersionOnly`**, **`TestImportNixpkgsNoStoreCopy`**, path interp oracles **green**.

## CGO

`bash scripts/check-cgo.sh` **green**.
