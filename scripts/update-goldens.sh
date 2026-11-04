#!/usr/bin/env bash
# Regenerate committed goldens using hermetic Nix 2.24.9 + nixpkgs 50ab7937.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
source "${NIX_PROFILE:-/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh}" 2>/dev/null || true

NIXPKGS="${NIXPKGS:-$ROOT/testdata/nixpkgs}"
PINNED_REV="50ab793786d9de88ee30ec4e4c24fb4236fc2674"
if [[ ! -d "$NIXPKGS" ]]; then
  echo "Fetching nixpkgs ${PINNED_REV}..." >&2
  mkdir -p "$ROOT/testdata"
  curl -fsSL -o /tmp/nixpkgs.tar.gz "https://github.com/NixOS/nixpkgs/archive/${PINNED_REV}.tar.gz"
  rm -rf "$NIXPKGS"
  mkdir -p "$NIXPKGS"
  tar -xzf /tmp/nixpkgs.tar.gz -C "$NIXPKGS" --strip-components=1
fi
echo "$PINNED_REV" > "$ROOT/testdata/pinned-nixpkgs-rev"
echo "$PINNED_REV" > "$NIXPKGS/.pinned-rev" 2>/dev/null || true

NIX="${NIX:-nix-instantiate}"
NIX_FLAGS=(
  --option substituters ''
  --option restrict-eval false
  -I "nixpkgs=$NIXPKGS"
  -I "nix=$ROOT/testdata/nix"
)

LIB_VER=$("$NIX" "${NIX_FLAGS[@]}" --eval --raw -E '(import <nixpkgs/lib>).version' | tr -d '"')
if [[ "$LIB_VER" != 24.11* ]]; then
  echo "refusing to update goldens: lib.version=$LIB_VER (expected 24.11*)" >&2
  exit 1
fi

HELLO_DRV=$("$NIX" "${NIX_FLAGS[@]}" --eval --raw -E '
  with import <nixpkgs> { system = "x86_64-linux"; config = {}; overlays = []; };
  hello.drvPath
' | tr -d '"')

mkdir -p "$ROOT/testdata/golden"
OLD=$(cat "$ROOT/testdata/golden/hello.drvPath" 2>/dev/null || true)
printf '%s\n' "$HELLO_DRV" > "$ROOT/testdata/golden/hello.drvPath"
echo "Wrote testdata/golden/hello.drvPath = $HELLO_DRV"
if [[ -n "$OLD" && "$OLD" != "$HELLO_DRV" ]]; then
  echo "Note: hello.drvPath changed from $OLD" >&2
fi
