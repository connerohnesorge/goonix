#!/usr/bin/env bash
# Windows end-to-end check for goonix builds, run by CI on windows-latest
# from Git Bash: bash scripts/windows-e2e.sh path/to/goonix.exe
#
# 1. Substitute pkgs/windows hello from NovaNix's cache into a fresh store and
#    run it: NAR restore, the nova.exec stream, and symlink-free trees.
# 2. Build hello from source in C:\nix\store: builtin:fetchurl,
#    builtin:unpack of the MSYS2 and MinGW-w64 seeds, and a native MSYS2 bash
#    builder in a job object. Then run it.
# 3. Compare the local NAR hashes with NovaNix's narinfo. msys-seed must
#    match. hello is reported: it matches only when the build directory and
#    toolchain are the ones NovaNix's cache was built with.
set -euo pipefail

goonix=$1
CACHE_URL=https://cache.novavero.ai
CACHE_PUBLIC_KEY=cache.novavero.ai-1:9gQ7tLWMM+2tdC9H5sKMJltDIPfD7X2GWlZe8Aa8hHQ=

check_hello() {
  local out=${1#out=}
  local exe got
  exe="$(cygpath -u "$out")/bin/hello.exe"
  got=$("$exe" | tr -d '\r')
  if [ "$got" != "Hello, world!" ]; then
    echo "hello printed: $got" >&2
    exit 1
  fi
  echo "ran $exe: $got"
}

cached_narhash() {
  curl -fsS "$CACHE_URL/$1.narinfo" | sed -n 's/^NarHash: //p' | tr -d '\r'
}

store_path() {
  local dir=$1 name=$2
  ls "$dir" | grep -E "^[0-9a-z]{32}-$name\$" | head -n1
}

echo "== substitute hello from $CACHE_URL"
subst_root=$(cygpath -w "$RUNNER_TEMP/subst")
line=$("$goonix" build --store "$subst_root" --substituter "$CACHE_URL" --trusted-key "$CACHE_PUBLIC_KEY" pkgs/windows/hello.nix | tail -n1 | tr -d '\r')
echo "$line"
check_hello "$line"

echo "== build hello from source in C:\\nix\\store"
line=$("$goonix" build pkgs/windows/hello.nix | tail -n1 | tr -d '\r')
echo "$line"
check_hello "$line"

echo "== compare with $CACHE_URL"
status=0
for name in msys-seed mingw-w64-seed hello; do
  base=$(store_path /c/nix/store "$name")
  hash=${base%%-*}
  local_hash=$("$goonix" hash-path "C:\\nix\\store\\$base" | tr -d '\r')
  cache_hash=$(cached_narhash "$hash")
  if [ "$local_hash" = "$cache_hash" ]; then
    echo "$base: same NAR as the cache ($local_hash)"
  else
    echo "$base: local $local_hash, cache $cache_hash"
    if [ "$name" = msys-seed ]; then
      status=1
    fi
  fi
done
exit $status
