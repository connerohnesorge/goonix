#!/usr/bin/env bash
# Regenerate committed goldens using pinned Nix 2.24.9 + nixpkgs 50ab7937.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
source "${NIX_PROFILE:-/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh}" 2>/dev/null || true

NIXPKGS="${NIXPKGS:-$ROOT/testdata/nixpkgs}"
if [[ ! -d "$NIXPKGS" ]]; then
  echo "Fetching nixpkgs ${NIXPKGS_REV:-50ab793786d9de88ee30ec4e4c24fb4236fc2674}..." >&2
  mkdir -p "$ROOT/testdata"
  curl -fsSL -o /tmp/nixpkgs.tar.gz "https://github.com/NixOS/nixpkgs/archive/${NIXPKGS_REV:-50ab793786d9de88ee30ec4e4c24fb4236fc2674}.tar.gz"
  rm -rf "$NIXPKGS"
  mkdir -p "$NIXPKGS"
  tar -xzf /tmp/nixpkgs.tar.gz -C "$NIXPKGS" --strip-components=1
fi

NIX="${NIX:-nix-instantiate}"
HELLO_DRV=$($NIX -I nixpkgs="$NIXPKGS" --eval -E '
  with import <nixpkgs> { system = "x86_64-linux"; config = {}; overlays = []; };
  hello.drvPath
' | tr -d '"')

mkdir -p "$ROOT/testdata/golden"
printf '%s\n' "$HELLO_DRV" > "$ROOT/testdata/golden/hello.drvPath"
echo "Wrote testdata/golden/hello.drvPath = $HELLO_DRV"
