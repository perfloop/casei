package casei

// Matcher is the multi-needle face of the arena: search for any of a set of
// patterns under the same Unicode simple-fold semantics as IndexFold.
// IndexFold is the N=1 special case. Like IndexFold, the implementation
// below is the deliberately naive reference form — the goal of this
// repository is a single adaptive engine for which this API and the tests
// define the contract: per-position sets of UTF-8 encodings under simple
// folding (elastic-degenerate byte patterns), exact search as the singleton
// case, multi-needle as the union, one anchoring/verification theory,
// linear worst case.
//
// Contract: Find returns the leftmost match by byte offset; ties at the
// same offset go to the lowest pattern index (regexp alternation order).
// An empty pattern matches at offset 0.

// Match identifies one pattern occurrence.
type Match struct {
	Pattern int // index into the pattern set
	Start   int // byte offset of the match start in the haystack
}

// Matcher searches for any of a fixed set of patterns. Construction is the
// place to analyze the set; Find must not re-analyze per call.
type Matcher struct {
	patterns []string
}

// NewMatcher builds a Matcher over the given pattern set. The set is
// copied; later mutation of the slice does not affect the Matcher.
func NewMatcher(patterns []string) *Matcher {
	p := make([]string, len(patterns))
	copy(p, patterns)
	return &Matcher{patterns: p}
}

// Patterns returns the pattern set (read-only view for tests and tooling).
func (m *Matcher) Patterns() []string { return m.patterns }

// Find returns the leftmost match across the pattern set, or ok=false when
// no pattern occurs.
func (m *Matcher) Find(haystack string) (Match, bool) {
	best := Match{Pattern: -1, Start: -1}
	for i, p := range m.patterns {
		pos := IndexFold(haystack, p)
		if pos < 0 {
			continue
		}
		if best.Pattern < 0 || pos < best.Start {
			best = Match{Pattern: i, Start: pos}
		}
		if best.Start == 0 {
			break // nothing can beat offset 0 at a lower index later
		}
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

// VectorBits reports the widest vector path this Matcher's compiled plan
// dispatches to on the running machine, with the same contract as
// RuntimeVectorBits. The reference implementation compiles no vector plan.
func (m *Matcher) VectorBits() int { return 0 }
