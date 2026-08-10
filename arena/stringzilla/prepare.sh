#!/usr/bin/env bash
# Download and build the pinned StringZilla shared C library into a caller-owned
# shared native-dependency root.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	printf 'usage: %s DESTINATION\n' "$0" >&2
	exit 2
fi

case "$(uname -m)" in
x86_64)
	;;
*)
	printf 'StringZilla arena setup currently supports x86_64 hosts, got %s\n' "$(uname -m)" >&2
	exit 1
	;;
esac

out=$1
archive="$out/stringzilla-4.5.0.tar.gz"
source="$out/stringzilla-4.5.0"
root="$out/root"
sha256='30497ebfa857c5f0dcf68e2bb7a7f33396ee7a2ef07406d1c2134d57b1ba38f9'
mkdir -p "$out" "$root/usr/include" "$root/usr/lib/x86_64-linux-gnu/pkgconfig"
if [ ! -f "$archive" ]; then
	python3 -m pip download --no-deps --no-binary=:all: --dest "$out" 'stringzilla==4.5.0'
	# pip names the source archive deterministically for this pinned release.
	if [ ! -f "$archive" ]; then
		printf 'StringZilla source archive was not written as %s\n' "$archive" >&2
		exit 1
	fi
fi
printf '%s  %s\n' "$sha256" "$archive" | sha256sum --check --status
rm -rf "$source"
tar -xzf "$archive" -C "$out"

# This is StringZilla's documented dynamic-dispatch configuration, mirrored
# from its source distribution's Linux build settings. It compiles all x86
# backends and selects Ice Lake AVX-512 at runtime.
cc -std=c99 -O3 -fPIC -shared -D_GNU_SOURCE \
	-DSZ_DYNAMIC_DISPATCH=1 -DSZ_IS_BIG_ENDIAN_=0 -DSZ_IS_64BIT_X86_=1 -DSZ_IS_64BIT_ARM_=0 \
	-DSZ_USE_WESTMERE=1 -DSZ_USE_GOLDMONT=1 -DSZ_USE_HASWELL=1 -DSZ_USE_SKYLAKE=1 -DSZ_USE_ICE=1 \
	-DSZ_USE_NEON=0 -DSZ_USE_NEON_AES=0 -DSZ_USE_NEON_SHA=0 -DSZ_USE_SVE=0 -DSZ_USE_SVE2=0 -DSZ_USE_SVE2_AES=0 \
	-I"$source/include" "$source/c/stringzilla.c" -o "$root/usr/lib/x86_64-linux-gnu/libstringzilla.so"
rm -rf "$root/usr/include/stringzilla"
cp -a "$source/include/stringzilla" "$root/usr/include/"
cat > "$root/usr/lib/x86_64-linux-gnu/pkgconfig/stringzilla.pc" <<'EOF'
prefix=/usr
exec_prefix=${prefix}
libdir=${prefix}/lib/x86_64-linux-gnu
includedir=${prefix}/include

Name: stringzilla
Description: StringZilla UTF-8 case-insensitive search
Version: 4.5.0
Libs: -L${libdir} -lstringzilla
Cflags: -I${includedir} -DSZ_DYNAMIC_DISPATCH=1
EOF
