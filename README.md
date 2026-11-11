# goonix

Pure-Go Nix evaluator and store implementation (`CGO_ENABLED=0`), checked against **Nix 2.24.9** and nixpkgs **`50ab793786d9de88ee30ec4e4c24fb4236fc2674`** (same pair as [NovaNix parity CI](https://github.com/Novavero-AI/nova-nix)).

## Build & test

Requires Go **1.23+**.

```bash
export CGO_ENABLED=0
go build ./...
go test ./...
bash scripts/check-cgo.sh   # must print OK (no cgo packages in graph)
```

Regenerate oracle goldens (needs `nix-instantiate` on PATH):

```bash
bash scripts/update-goldens.sh
```

Pinned Nix version: `internal/nixversion` (also re-exported from `internal/oracle/pins.go`). CI ladder job installs **Nix 2.24.9** explicitly.

## Layout

| Package | Phase | Role |
|---------|-------|------|
| `parser/` | 1 | Hand-written lexer/parser + `--parse` printer |
| `data/` | 3 | Value representation (plain Go pointers; see `data/DECISION.md`) |
| `store/` | 4a | Store-path hashing, NAR, `.drv` (wraps [go-nix](https://github.com/nix-community/go-nix) pure packages) |
| `store/local/` | 4b | On-disk store + `modernc.org/sqlite` metadata |
| `eval/` | 2 / 2b | Lazy evaluator + builtins / NIX_PATH |
| `substitute/`, `build/` | 6 / 5 | HTTP binary cache + builder (initial stubs) |
| `cmd/goonix` | — | `parse`, `eval`, `instantiate` (with `-I`, `-A`, `--arg`, `--argstr`), `build`, `gc` |
| `ladder/` | 7 / 13 | Integration tests vs committed goldens |

## Oracle pair

- **Nix:** 2.24.9  
- **nixpkgs:** `50ab793786d9de88ee30ec4e4c24fb4236fc2674` (nixos-24.11)

CI downloads nixpkgs for ladder jobs; it is not vendored in git (see `.gitignore`).

## Prior art & license

Apache-2.0. Design references: NovaNix (Apache-2.0), nix-community/go-nix (Apache-2.0). **Do not copy** Profpatsch/gonix (LGPL-2.1) sources.

NovaNix parity facts (for docs): **22** matching packages on nixpkgs 24.11 / Nix 2.24.9; flat post-order AST walk (**not** a VM); **21** node types / **25** opcodes / **105** builtins.

## Status (v0)

`goonix instantiate testdata/nixpkgs -A hello --argstr system x86_64-linux --arg config '{}' --arg overlays '[]'` writes the same `.drv` closure as Nix 2.24.9, byte for byte. The same holds for `stdenv`, `bash`, and `coreutils`. `ladder.TestInstantiateClosureMatchesGolden` checks this against `testdata/golden/closure/*.sha256` without Nix installed. `ladder.TestParityCasesMatchGolden` checks all 18 of NovaNix's nixpkgs drvPath cases (testdata/parity/cases.txt), from hello through clang, qtbase, systemd, neovim, and a NixOS initrd and toplevel, against Nix 2.24.9's drvPaths. See [STATUS.md](STATUS.md) for issues #2–#14 and test evidence.
