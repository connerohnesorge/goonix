# Value representation (Phase 3 spike)

## Decision

Use **plain Go pointers** (`*data.Value`) for the runtime value model, matching the gonix-style approach used in `github.com/nix-community/go-nix` command tooling rather than a custom bump allocator with handles.

## Alternatives considered

1. **Plain pointers** — allocate `Value` structs on the heap and link them with `*Value`, `map[Symbol]*Value`, and slices.
2. **Simple arena handles** — store values in a contiguous `[]arenaValue` slice and refer to them with `uint32` handles (no GC tracing through pointers, but extra indirection and manual lifetime management).

## Benchmarks

Run on the project dev host with:

```bash
CGO_ENABLED=0 go test ./data -bench=. -benchmem -count=5
```

Representative results (linux/amd64, Go 1.23, Intel Xeon):

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkValuePointerBuild` | ~1.9k | ~5.8k | 35 |
| `BenchmarkValueArenaBuild` | ~1.1k | ~2.3k | 8 |
| `BenchmarkValuePointerWalk` | ~141 | 0 | 0 |
| `BenchmarkValueArenaWalk` | ~145 | 0 | 0 |

Arena handles win on small attrset construction (fewer allocations) because the spike stores attrs inline in a slice. Walk cost is a wash. We still pick pointers because the evaluator will share substructures, attach source locations, and interoperate with the Go GC without a parallel lifetime model.

## Rationale

- Idiomatic Go: the evaluator, parser, and future GC hooks can use normal pointer graphs.
- Simpler debugging and stack traces compared to handle tables.
- No upfront commitment to a custom allocator before we have real workload profiles from evaluation and the Nix store interface.

Revisit if profiling shows allocation pressure from large fixed-output closures or deeply nested attrsets.
