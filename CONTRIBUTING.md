# Contributing

- Pure Go only: `CGO_ENABLED=0 go build ./...` and `bash scripts/check-cgo.sh`.
- Goldens: `go test ./... -update` (via `internal/golden`) after `scripts/update-goldens.sh` when Nix is available.
- Oracle bumps: edit `internal/oracle/pins.go` and regenerate goldens deliberately.
