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

LIB_VER=$("$NIX" "${NIX_FLAGS[@]}" --eval -E '(import <nixpkgs/lib>).version' | tr -d '"')
if [[ "$LIB_VER" != 24.11* ]]; then
  echo "refusing to update goldens: lib.version=$LIB_VER (expected 24.11*)" >&2
  exit 1
fi

HELLO_DRV=$("$NIX" "${NIX_FLAGS[@]}" --eval -E '
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

# Closure goldens for the issue #9 targets: sha256sum of every .drv that
# nix-instantiate writes for the attribute, sorted by file name.
mkdir -p "$ROOT/testdata/golden/closure"
for attr in hello stdenv bash coreutils; do
  TMP=$(mktemp -d)
  env -u HOME NIX_PATH= "$NIX" --store "$TMP" "$NIXPKGS" -A "$attr" \
    --argstr system x86_64-linux --arg config '{}' --arg overlays '[]' >/dev/null
  (cd "$TMP/nix/store" && sha256sum -- *.drv) > "$ROOT/testdata/golden/closure/$attr.sha256"
  chmod -R u+w "$TMP" && rm -rf "$TMP"
  echo "Wrote testdata/golden/closure/$attr.sha256 ($(wc -l < "$ROOT/testdata/golden/closure/$attr.sha256") drvs)"
done
"$NIX" "${NIX_FLAGS[@]}" --eval -E '(import <nixpkgs> { system = "x86_64-linux"; config = {}; overlays = []; }).lib.version' | tr -d '"' > "$ROOT/testdata/golden/lib.version"

# drvPath goldens for testdata/parity/cases.txt.
: > "$ROOT/testdata/golden/parity.txt"
grep -vE '^#|^$' "$ROOT/testdata/parity/cases.txt" | while read -r label expr; do
  TMP=$(mktemp -d)
  printf 'let pkgs = import <nixpkgs> { system = "x86_64-linux"; config = { }; overlays = [ ]; }; in %s\n' "$expr" > "$TMP/case.nix"
  drv=$(env -u HOME NIX_PATH= "$NIX" --store "$TMP/store" -I "nixpkgs=$NIXPKGS" "$TMP/case.nix" | tail -1)
  printf '%s %s\n' "$label" "$drv" >> "$ROOT/testdata/golden/parity.txt"
  chmod -R u+w "$TMP" && rm -rf "$TMP"
done
echo "Wrote testdata/golden/parity.txt"
