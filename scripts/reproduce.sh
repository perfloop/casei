#!/usr/bin/env bash
# Reproduce casei's benchmark: build the entire competitor field from source,
# then run the scoreboard. This is what CI runs on every push.
#
# Requirements: x86-64 Linux with AVX-512 (Intel Ice Lake or newer). casei's
# benchmarked result is AVX-512-specific; on other hardware it runs a correct
# but uncompetitive scalar fallback, so the script refuses rather than print a
# misleading number.
set -euo pipefail

arch="$(uname -m)"
if [ "$arch" != "x86_64" ]; then
  echo "casei's result is x86-64 AVX-512 only; this host is '$arch'." >&2
  echo "casei still runs correctly here via a scalar fallback, but it is not competitive and the field will not build." >&2
  exit 1
fi
if ! grep -qw avx512f /proc/cpuinfo 2>/dev/null; then
  echo "This host has no AVX-512 (need Intel Ice Lake or newer, e.g. a GCP n2/c3)." >&2
  echo "The field build and the measured result both require it." >&2
  exit 1
fi
if ! grep -qw avx512vbmi /proc/cpuinfo 2>/dev/null; then
  echo "note: this host lacks AVX-512 VBMI, so Vectorscan will not dispatch its strongest path." >&2
  echo "      the result still holds, but the headline is the equal-width VBMI comparison (Ice Lake / Sapphire Rapids)." >&2
fi

echo "==> Installing build dependencies (cargo, cmake, boost, pkg-config)"
sudo apt-get update -qq
sudo apt-get install -y -qq cargo cmake curl libboost-dev pkg-config python3-pip build-essential

root="$(cd "$(dirname "$0")/.." && pwd)"
native="$(mktemp -d)"
cd "$root/arena"
for dep in pcre2 vectorscan rure rustac stringzilla; do
  echo "==> Building competitor from source: $dep"
  "./$dep/prepare.sh" "$native"
done

export PKG_CONFIG_PATH="$native/root/usr/lib/x86_64-linux-gnu/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="$native/root"
export LD_LIBRARY_PATH="$native/root/usr/lib/x86_64-linux-gnu"

echo "==> Running the scoreboard (BenchmarkBar: x_vs_best per row, with per-entrant dispatched width)"
go test -run '^$' -bench '^BenchmarkBar$' -benchtime 30x -count 3

echo
echo "==> Per-competitor throughput (BenchmarkIndexFold / BenchmarkMatcher, MB/s per engine)"
go test -run '^$' -bench '^BenchmarkIndexFold$' -benchtime 100ms -count 3
go test -run '^$' -bench '^BenchmarkMatcher$'   -benchtime 100ms -count 3
