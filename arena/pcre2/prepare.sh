#!/usr/bin/env bash
# Build pinned PCRE2 source with Unicode and JIT into a caller-owned directory.
# This is for unprivileged arena builds; system installations work too.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	printf 'usage: %s DESTINATION\n' "$0" >&2
	exit 2
fi

if [ "$(uname -m)" != "x86_64" ]; then
	printf 'PCRE2 arena setup currently supports x86_64 hosts, got %s\n' "$(uname -m)" >&2
	exit 1
fi

out=$1
archive="$out/pcre2-10.47.tar.bz2"
source="$out/pcre2-10.47"
build="$out/pcre2-build"
root="$out/root"
url='https://github.com/PCRE2Project/pcre2/releases/download/pcre2-10.47/pcre2-10.47.tar.bz2'
sha256='47fe8c99461250d42f89e6e8fdaeba9da057855d06eb7fc08d9ca03fd08d7bc7'
# Native arena packages share this root, so preparation can happen in either
# order without deleting another baseline's headers or runtime libraries.
mkdir -p "$out" "$root"
if [ ! -f "$archive" ]; then
	curl --fail --location --retry 3 --silent --show-error "$url" --output "$archive"
fi
printf '%s  %s\n' "$sha256" "$archive" | sha256sum --check --status
rm -rf "$source" "$build"
tar -xjf "$archive" -C "$out"

cmake -S "$source" -B "$build" \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_INSTALL_PREFIX=/usr \
	-DCMAKE_INSTALL_LIBDIR=lib/x86_64-linux-gnu \
	-DBUILD_SHARED_LIBS=ON \
	-DPCRE2_BUILD_PCRE2_8=ON \
	-DPCRE2_BUILD_PCRE2_16=OFF \
	-DPCRE2_BUILD_PCRE2_32=OFF \
	-DPCRE2_BUILD_PCRE2GREP=OFF \
	-DPCRE2_BUILD_TESTS=OFF \
	-DPCRE2_SUPPORT_JIT=ON \
	-DPCRE2_SUPPORT_UNICODE=ON
cmake --build "$build" --parallel "${CMAKE_BUILD_PARALLEL_LEVEL:-2}"
DESTDIR="$root" cmake --install "$build"

grep -Ex 'PCRE2_SUPPORT_JIT:BOOL=ON' "$build/CMakeCache.txt" >/dev/null
grep -Ex 'PCRE2_SUPPORT_UNICODE:BOOL=ON' "$build/CMakeCache.txt" >/dev/null
test -f "$root/usr/lib/x86_64-linux-gnu/libpcre2-8.so"
test -f "$root/usr/lib/x86_64-linux-gnu/pkgconfig/libpcre2-8.pc"
