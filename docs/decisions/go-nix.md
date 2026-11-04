# go-nix as a computation library

## Context

Phase 4a needs pure functions for Nix store paths, content hashes, NAR serialization, and derivation parsing. The [go-nix](https://github.com/nix-community/go-nix) module already implements these concerns in Go.

## Decision

Depend on **github.com/nix-community/go-nix** and wrap:

- `pkg/nixbase32` and `pkg/nixhash` for hash parsing and store-path digests
- `pkg/storepath` for path structure and validation
- `pkg/nar` for NAR output and recursive path hashing
- `pkg/derivation` for `.drv` parsing and path/output computation

Do **not** import `pkg/sqlite` or other persistence layers from go-nix. Goonix will introduce its own store backend when we need mutation and querying.

## Consequences

- Faster bootstrap: we inherit battle-tested parsing and hashing instead of reimplementing ATerm and NAR details.
- CGO-free builds remain possible when only these packages are linked (verified with `CGO_ENABLED=0 go test ./...`).
- Store directory customization is handled in goonix `store.Config` by rewriting prefixes; go-nix itself hardcodes `/nix/store` internally when fingerprinting derivations.

## Alternatives considered

- **Reimplement from scratch** — high correctness risk for subtle Nix compatibility details.
- **Use go-nix sqlite store wholesale** — pulls CGO/sqlite into the default build and couples us to an experimental persistence layout we do not need yet.
