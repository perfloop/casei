#!/usr/bin/env bash
# Build the pinned rure C API with Cargo and stage its header and shared library
# into a caller-owned shared native-dependency root.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	printf 'usage: %s DESTINATION\n' "$0" >&2
	exit 2
fi

out=$1
root="$out/root"
source_copy="$out/rure-source"
memchr_copy="$out/memchr-source"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cargo_home=${CARGO_HOME:-"$HOME/.cargo"}
mkdir -p "$root/usr/include" "$root/usr/lib/x86_64-linux-gnu/pkgconfig"

# Fetch the exact registry sources through Cargo so its checksums remain the
# source-integrity boundary. The checked fetch lock fixes the complete Rust
# dependency graph; the audited build lock below replaces memchr with the local
# post-verification source copy.
cargo fetch --manifest-path "$script_dir/fetch/Cargo.toml" --locked

source=$(find "$cargo_home/registry/src" -type f -path '*/rure-0.2.5/Cargo.toml' -printf '%h\n' -quit)
memchr=$(find "$cargo_home/registry/src" -type f -path '*/memchr-2.8.3/Cargo.toml' -printf '%h\n' -quit)
if [ -z "$source" ] || [ -z "$memchr" ]; then
	printf 'cargo did not expose pinned rure 0.2.5 and memchr 2.8.3 source trees under %s\n' "$cargo_home" >&2
	exit 1
fi
rm -rf "$source_copy" "$memchr_copy"
cp -a "$source" "$source_copy"
cp -a "$memchr" "$memchr_copy"
patch --batch --fuzz=0 -p1 -d "$memchr_copy" < "$script_dir/patches/memchr-dispatch-audit.patch"
patch --batch --fuzz=0 -p1 -d "$source_copy" < "$script_dir/patches/rure-dispatch-audit.patch"
cp "$script_dir/Cargo.lock" "$source_copy/Cargo.lock"

# Do not override Cargo's target-directory selection: the runtime assigns each
# measured arm an isolated output lane. Ask Cargo where this invocation placed
# its artifacts, then stage only the C ABI files under the shared native root.
cargo build --manifest-path "$source_copy/Cargo.toml" --release --locked
target=$(cargo metadata --manifest-path "$source_copy/Cargo.toml" --format-version=1 --no-deps |
	sed -n 's/.*"target_directory":"\([^"]*\)".*/\1/p')
if [ -z "$target" ] || [ ! -f "$target/release/librure.so" ]; then
	printf 'rure shared library was not produced in Cargo target directory %s\n' "$target" >&2
	exit 1
fi
install -m 0644 "$source_copy/include/rure.h" "$root/usr/include/rure.h"
install -m 0755 "$target/release/librure.so" "$root/usr/lib/x86_64-linux-gnu/librure.so"
cat > "$root/usr/lib/x86_64-linux-gnu/pkgconfig/rure.pc" <<'EOF'
prefix=/usr
exec_prefix=${prefix}
libdir=${prefix}/lib/x86_64-linux-gnu
includedir=${prefix}/include

Name: rure
Description: Rust regex C API
Version: 0.2.5
Libs: -L${libdir} -lrure
Cflags: -I${includedir}
EOF
