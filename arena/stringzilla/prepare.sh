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
archive="$out/stringzilla-5.1.2.tar.gz"
source="$out/stringzilla-5.1.2"
root="$out/root"
sha256='7c2a952d6305df23bd4e592c28c27786e0d77982949233df20029370cd0096ad'
mkdir -p "$out" "$root/usr/include" "$root/usr/lib/x86_64-linux-gnu/pkgconfig"
if [ ! -f "$archive" ]; then
	python3 -m pip download --no-deps --no-binary=:all: --dest "$out" 'stringzilla==5.1.2'
	# pip names the source archive deterministically for this pinned release.
	if [ ! -f "$archive" ]; then
		printf 'StringZilla source archive was not written as %s\n' "$archive" >&2
		exit 1
	fi
fi
printf '%s  %s\n' "$sha256" "$archive" | sha256sum --check --status
rm -rf "$source"
tar -xzf "$archive" -C "$out"

# Mirror StringZilla's documented Linux dynamic-dispatch build: compile every
# current single-CPU translation unit, include all x86 backends, and let the
# runtime select the Ice Lake implementation. O3 retains the arena's previous
# full-strength optimization level.
sources=(
	"$source/c/stringzilla/runtime.c"
	"$source/c/stringzilla/compare.c"
	"$source/c/stringzilla/memory.c"
	"$source/c/stringzilla/hash.c"
	"$source/c/stringzilla/cipher.c"
	"$source/c/stringzilla/find.c"
	"$source/c/stringzilla/sort.c"
	"$source/c/stringzilla/intersect.c"
	"$source/c/stringzilla/utf8_norm.c"
	"$source/c/stringzilla/utf8_runes.c"
	"$source/c/stringzilla/utf8_tokens.c"
	"$source/c/stringzilla/utf8_wordbreaks.c"
	"$source/c/stringzilla/utf8_graphemes.c"
	"$source/c/stringzilla/utf8_sentences.c"
	"$source/c/stringzilla/utf8_linebreaks.c"
	"$source/c/stringzilla/utf8_uncased_fold.c"
	"$source/c/stringzilla/utf8_uncased.c"
)
cc -std=c99 -O3 -fPIC -shared -D_GNU_SOURCE \
	-Wno-incompatible-pointer-types -Wno-discarded-qualifiers \
	-DSZ_DYNAMIC_DISPATCH=1 -DSZ_IS_BIG_ENDIAN_=0 -DSZ_IS_64BIT_X86_=1 -DSZ_IS_64BIT_ARM_=0 \
	-DSZ_USE_WESTMERE=1 -DSZ_USE_GOLDMONT=1 -DSZ_USE_HASWELL=1 -DSZ_USE_SKYLAKE=1 -DSZ_USE_ICELAKE=1 \
	-DSZ_USE_NEON=0 -DSZ_USE_NEONAES=0 -DSZ_USE_NEONSHA=0 -DSZ_USE_SVE=0 -DSZ_USE_SVE2=0 -DSZ_USE_SVE2AES=0 \
	-I"$source/include" "${sources[@]}" -o "$root/usr/lib/x86_64-linux-gnu/libstringzilla.so"
rm -rf "$root/usr/include/stringzilla"
cp -a "$source/include/stringzilla" "$root/usr/include/"
cat > "$root/usr/lib/x86_64-linux-gnu/pkgconfig/stringzilla.pc" <<'EOF'
prefix=/usr
exec_prefix=${prefix}
libdir=${prefix}/lib/x86_64-linux-gnu
includedir=${prefix}/include

Name: stringzilla
Description: StringZilla UTF-8 case-insensitive search
Version: 5.1.2
Libs: -L${libdir} -lstringzilla
Cflags: -I${includedir} -DSZ_DYNAMIC_DISPATCH=1
EOF
