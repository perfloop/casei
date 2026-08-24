package casei

import "unicode/utf8"

// Matcher searches for any of a set of patterns under the same Unicode
// simple-fold semantics as IndexFold. IndexFold is the one-pattern form of the
// same compiled search plan. The implementation scans the haystack once rather
// than running one independent search per pattern.
//
// Contract: Find returns the leftmost match by byte offset; ties at the
// same offset go to the lowest pattern index (regexp alternation order).
// An empty pattern matches at offset 0.

// Match identifies one pattern occurrence.
type Match struct {
	Pattern int // index into the pattern set
	Start   int // byte offset of the match start in the haystack
}

// Matcher searches for any of a fixed set of patterns. Construction compiles
// their shared fold-orbit transition plan; Find returns one answer and Each
// enumerates non-overlapping answers over the haystack.
type Matcher struct {
	patterns []string
	plan     *searchPlan
}

// NewMatcher builds a Matcher over the given pattern set. The set is copied;
// later mutation of the slice does not affect either the exposed pattern set
// or the compiled plan.
func NewMatcher(patterns []string) *Matcher {
	p := make([]string, len(patterns))
	copy(p, patterns)
	return &Matcher{patterns: p, plan: newSearchPlan(p)}
}

// Patterns returns a copy of the pattern set.
func (m *Matcher) Patterns() []string {
	patterns := make([]string, len(m.patterns))
	copy(patterns, m.patterns)
	return patterns
}

// Find returns the leftmost match across the pattern set, or ok=false when
// no pattern occurs.
func (m *Matcher) Find(haystack string) (Match, bool) {
	if m == nil || m.plan == nil {
		return Match{}, false
	}
	return m.plan.find(haystack)
}

// Each calls yield for each non-overlapping match in haystack, in the same
// leftmost and lowest-pattern-ID order as repeated calls to Find. width is the
// exact byte width consumed by this occurrence, which can differ from the
// matched pattern's byte length under Unicode simple folding. Returning false
// from yield stops enumeration and makes Each return false.
//
// A nil Matcher or nil yield has no matches and returns true. Each is safe for
// concurrent use when yield itself is safe.
func (m *Matcher) Each(haystack string, yield func(match Match, width int) bool) bool {
	if m == nil || m.plan == nil || yield == nil {
		return true
	}
	if m.plan.empty < 0 && m.plan.rawByteMulti.usable() {
		return m.plan.eachRawByteFixedAnchored(haystack, yield)
	}
	for at := 0; at <= len(haystack); {
		match, width, ok := m.plan.findWithWidth(haystack[at:])
		if !ok {
			return true
		}
		match.Start += at
		if width == 0 {
			units := utf8.RuneCountInString(m.patterns[match.Pattern])
			width = matcherMatchEnd(haystack, match.Start, units) - match.Start
		}
		end := match.Start + width
		if !yield(match, width) {
			return false
		}
		if width != 0 {
			at = end
			continue
		}
		if match.Start == len(haystack) {
			return true
		}
		_, size := utf8.DecodeRuneInString(haystack[match.Start:])
		at = match.Start + size
	}
	return true
}

func matcherMatchEnd(haystack string, start, units int) int {
	at := start
	for range units {
		_, size := utf8.DecodeRuneInString(haystack[at:])
		at += size
	}
	return at
}

// VectorBits reports the widest runtime-gated block transition available to
// this package, with the same contract as RuntimeVectorBits.
func (m *Matcher) VectorBits() int { return RuntimeVectorBits() }
