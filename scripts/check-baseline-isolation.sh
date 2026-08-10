#!/usr/bin/env bash
# The candidate must not be able to reach a competitor.
#
# An engine that calls veloz cannot beat veloz; it can only add overhead to it.
# The scoreboard cannot see the difference -- it reports a ratio either way --
# so this is enforced structurally instead. The field lives in ./arena, its own
# module, and nothing in the candidate module's import graph may reach it.
set -euo pipefail

# Go implementations the scoreboard measures against. Native PCRE2, rure,
# Vectorscan, StringZilla, and the direct Rust DFA are also field entrants;
# their cgo bindings have no Go import path, so their library names are checked
# below.
FIELD='github.com/mhr3/veloz|github.com/petar-dambovaliev/aho-corasick'

deps="$(go list -deps -test ./... 2>/dev/null || true)"
if printf '%s\n' "$deps" | grep -qE "^($FIELD)"; then
  echo "FAIL: the candidate module reaches a field competitor:" >&2
  printf '%s\n' "$deps" | grep -E "^($FIELD)" | sed 's/^/  /' >&2
  echo >&2
  echo "Baselines belong to ./arena only. A candidate that calls a competitor" >&2
  echo "is ineligible regardless of its benchmark result." >&2
  exit 1
fi

native_field="$(find . -path './.git' -prune -o -path './arena' -prune -o \
  -type f \( -name '*.go' -o -name '*.c' -o -name '*.h' -o -name '*.s' \) -print0 \
  | xargs -0 -r grep -nEi 'pcre2|rure|librure|vectorscan|hyperscan|libhs|stringzilla|libstringzilla|rustac|casei_ac' || true)"
if [ -n "$native_field" ]; then
  echo "FAIL: the candidate module reaches a native field competitor:" >&2
  printf '%s\n' "$native_field" | sed 's/^/  /' >&2
  echo >&2
  echo "Native field engines belong to ./arena only. A candidate that calls a competitor" >&2
  echo "is ineligible regardless of its benchmark result." >&2
  exit 1
fi
echo "ok: candidate module is isolated from the field"
