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

Pinned versions live in `internal/oracle/pins.go`.

## Layout

| Package | Phase | Role |
|---------|-------|------|
| `parser/` | 1 | Hand-written lexer/parser + `--parse` printer |
| `data/` | 3 | Value representation (plain Go pointers; see `data/DECISION.md`) |
| `store/` | 4a | Store-path hashing, NAR, `.drv` (wraps [go-nix](https://github.com/nix-community/go-nix) pure packages) |
| `store/local/` | 4b | On-disk store + `modernc.org/sqlite` metadata |
| `eval/` | 2 / 2b | Lazy evaluator + builtins / NIX_PATH |
| `substitute/`, `build/` | 6 / 5 | HTTP binary cache + builder (initial stubs) |
| `cmd/goonix` | — | `parse`, `eval`, `instantiate` |
| `ladder/` | 7 / 13 | Integration tests vs committed goldens |

## Oracle pair

- **Nix:** 2.24.9  
- **nixpkgs:** `50ab793786d9de88ee30ec4e4c24fb4236fc2674` (nixos-24.11)

CI downloads nixpkgs for ladder jobs; it is not vendored in git (see `.gitignore`).

## Prior art & license

Apache-2.0. Design references: NovaNix (Apache-2.0), nix-community/go-nix (Apache-2.0). **Do not copy** Profpatsch/gonix (LGPL-2.1) sources.

NovaNix parity facts (for docs): **22** matching packages on nixpkgs 24.11 / Nix 2.24.9; flat post-order AST walk (**not** a VM); **21** node types / **25** opcodes / **105** builtins.

## Status (v0)

Hello `.drvPath` golden is still in progress; see [STATUS.md](STATUS.md) for issues #2–#14 and test evidence. Open PR #15 for the phase checklist.
