#!/usr/bin/env bash
# The candidate must not be able to reach a competitor.
#
# An engine that calls veloz cannot beat veloz; it can only add overhead to it.
# The scoreboard cannot see the difference -- it reports a ratio either way --
# so this is enforced structurally instead. The field lives in ./arena, its own
# module, and nothing in the candidate module's import graph may reach it.
set -euo pipefail

# Every implementation the scoreboard measures against. Add a baseline here
# when you add it to arena/field.yaml.
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
echo "ok: candidate module is isolated from the field"
