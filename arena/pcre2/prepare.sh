#!/usr/bin/env bash
# Download a pinned PCRE2 development package into a caller-owned directory.
# This is for unprivileged arena builds; system installations work too.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	printf 'usage: %s DESTINATION\n' "$0" >&2
	exit 2
fi

case "$(uname -m)" in
x86_64)
	package='libpcre2-dev_10.46-1~deb13u1_amd64.deb'
	url="https://deb.debian.org/debian/pool/main/p/pcre2/$package"
	sha256='b5cb023bfaad356053583af384f7b75caec3ac127a197f889164ef4d96397cef'
	;;
*)
	printf 'PCRE2 arena setup currently supports x86_64 hosts, got %s\n' "$(uname -m)" >&2
	exit 1
	;;
esac

out=$1
archive="$out/$package"
root="$out/root"
# Native arena packages share this root, so preparation can happen in either
# order without deleting another baseline's headers or runtime libraries.
mkdir -p "$out" "$root"
if [ ! -f "$archive" ]; then
	curl --fail --location --retry 3 --silent --show-error "$url" --output "$archive"
fi
printf '%s  %s\n' "$sha256" "$archive" | sha256sum --check --status
dpkg-deb --extract "$archive" "$root"
