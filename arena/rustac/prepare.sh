#!/usr/bin/env bash
# Build the pinned direct Rust aho-corasick DFA entrant into the caller-owned
# native root. The runtime assigns Cargo's target directory per source arm;
# this script asks Cargo for that location instead of overriding it.
set -euo pipefail

if [ "$(uname -m)" != "x86_64" ]; then
  echo "Rust Aho-Corasick field entrant requires x86_64" >&2
  exit 1
fi
if ! command -v cargo >/dev/null; then
  echo "Rust Aho-Corasick field entrant requires cargo" >&2
  exit 1
fi
if ! command -v python3 >/dev/null; then
  echo "Rust Aho-Corasick field entrant requires python3 for Cargo metadata" >&2
  exit 1
fi
if ! command -v patch >/dev/null; then
  echo "Rust Aho-Corasick field entrant requires patch for its pinned audit" >&2
  exit 1
fi

out=${1:?usage: prepare.sh OUTPUT_DIR}
mkdir -p "$out"
out=$(cd "$out" && pwd)
source_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root="$out/root"
libdir="$root/usr/lib/x86_64-linux-gnu"
pcdir="$libdir/pkgconfig"
build_source="$out/rustac-source"
memchr_source="$out/rustac-memchr-source"

# Fetch from a manifest without the local patch first, so a cold Cargo cache
# can still stage the exact registry source that the locked build audits.
cargo fetch --manifest-path "$source_dir/fetch/Cargo.toml" --locked
cargo_home=${CARGO_HOME:-"$HOME/.cargo"}
registry_source=$(find "$cargo_home/registry/src" -type d -name 'memchr-2.8.3' -print -quit)
if [ -z "$registry_source" ]; then
  echo "Cargo did not provide the pinned memchr 2.8.3 source" >&2
  exit 1
fi

# Keep the patched dependency and its manifest in the caller-owned output
# tree. This avoids changing the checkout or sharing native artifacts between
# comparison arms.
rm -rf "$build_source" "$memchr_source"
mkdir -p "$build_source"
cp "$source_dir/Cargo.toml" "$source_dir/Cargo.lock" "$build_source/"
cp -a "$source_dir/src" "$build_source/src"
cp -a "$registry_source" "$memchr_source"
patch --batch --fuzz=0 -d "$memchr_source" -p1 < "$source_dir/../rure/patches/memchr-dispatch-audit.patch"

cargo build --manifest-path "$build_source/Cargo.toml" --release --locked
target=$(cargo metadata --manifest-path "$build_source/Cargo.toml" --format-version=1 --no-deps |
  python3 -c 'import json, sys; print(json.load(sys.stdin)["target_directory"])')
if [ ! -f "$target/release/libcasei_rustac.so" ]; then
  printf 'Rust Aho-Corasick shared library was not produced in Cargo target directory %s\n' "$target" >&2
  exit 1
fi
mkdir -p "$libdir" "$pcdir"
install -m 0755 "$target/release/libcasei_rustac.so" "$libdir/libcasei_rustac.so"
cat >"$pcdir/casei-rustac.pc" <<'EOF'
prefix=/usr
libdir=${prefix}/lib/x86_64-linux-gnu

Name: casei-rustac
Description: pinned direct Rust aho-corasick DFA arena entrant with audited prefilter
Version: 0.1.0
Libs: -L${libdir} -lcasei_rustac
Cflags:
EOF
