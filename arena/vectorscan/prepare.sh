#!/usr/bin/env bash
# Build the pinned Vectorscan source with every dispatch target the arena
# audits.  A distro libhs is not sufficient here: its database compiler and
# runtime target set are distribution-dependent.
set -euo pipefail

if [ "$(uname -m)" != "x86_64" ]; then
  echo "Vectorscan field entrant requires x86_64" >&2
  exit 1
fi

out=${1:?usage: prepare.sh OUTPUT_DIR}
root="$out/root"
archive="$out/vectorscan_5.4.11.orig.tar.gz"
ragel_deb="$out/ragel_6.10-4_amd64.deb"
source="$out/vectorscan-5.4.11"
build="$out/vectorscan-build"
ragel_root="$out/ragel"

mkdir -p "$out" "$root"

fetch() {
  local url=$1 file=$2 digest=$3
  if [ ! -f "$file" ]; then
    curl --fail --location --retry 3 --silent --show-error "$url" -o "$file"
  fi
  printf '%s  %s\n' "$digest" "$file" | sha256sum --check --status
}

# Debian's source archive is a stable, independently checksummed release
# artifact.  Ragel is unpacked rather than installed so preparation remains
# unprivileged and leaves the host toolchain untouched.
fetch \
  https://deb.debian.org/debian/pool/main/v/vectorscan/vectorscan_5.4.11.orig.tar.gz \
  "$archive" \
  163835d9a73d2705c6030ea551e866f36b5feb4a4800a94266589351928eca0d
fetch \
  https://deb.debian.org/debian/pool/main/r/ragel/ragel_6.10-4_amd64.deb \
  "$ragel_deb" \
  5fbad0c37aa630d32650440a41b59e712ee44b3ea6be49454eda36062573bb1a

rm -rf "$source" "$build" "$ragel_root"
tar -xzf "$archive" -C "$out"
dpkg-deb --extract "$ragel_deb" "$ragel_root"

# The library does not need Vectorscan's optional command-line tools. Removing
# that subtree avoids their unrelated optional dependencies without changing
# hs_shared or hs_runtime_shared.
rm -rf "$source/tools"

cmake -S "$source" -B "$build" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX=/usr \
  -DCMAKE_INSTALL_LIBDIR=lib/x86_64-linux-gnu \
  -DRAGEL="$ragel_root/usr/bin/ragel" \
  -DBUILD_SHARED_LIBS=ON \
  -DBUILD_STATIC_LIBS=OFF \
  -DFAT_RUNTIME=ON \
  -DBUILD_AVX2=ON \
  -DBUILD_AVX512=ON \
  -DBUILD_AVX512VBMI=ON
cmake --build "$build" --parallel "${CMAKE_BUILD_PARALLEL_LEVEL:-2}" \
  --target hs_shared hs_runtime_shared
DESTDIR="$root" cmake --install "$build"

# Keep a source-visible assertion that the requested fat runtime was actually
# configured. Avoid a grep pipeline under pipefail: grep may otherwise report
# a harmless SIGPIPE as a setup failure.
grep -Ex 'CMAKE_INSTALL_PREFIX:PATH=/usr' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'CMAKE_INSTALL_LIBDIR:PATH=lib/x86_64-linux-gnu' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'BUILD_SHARED_LIBS:[^=]+=ON' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'BUILD_STATIC_LIBS:[^=]+=OFF' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'FAT_RUNTIME:[^=]+=ON' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'BUILD_AVX2:[^=]+=ON' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'BUILD_AVX512:[^=]+=ON' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'BUILD_AVX512VBMI:[^=]+=ON' "$build/CMakeCache.txt" >/dev/null
test -f "$root/usr/lib/x86_64-linux-gnu/libhs.so"
test -f "$root/usr/lib/x86_64-linux-gnu/pkgconfig/libhs.pc"
