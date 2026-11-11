#!/usr/bin/env bash
set -euo pipefail
# Packages with .c files in the module graph must be empty when building pure Go.
# CGO_ENABLED=0 still lists CgoFiles on cgo-only deps (e.g. mattn/go-sqlite3).
export CGO_ENABLED=0
pkgs=$(go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' ./...)
if [[ -n "$pkgs" ]]; then
  echo "CGO packages found (forbidden):" >&2
  echo "$pkgs" >&2
  exit 1
fi
echo "OK: no CGO packages in module graph"
