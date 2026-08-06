//go:build !pcre2

// Without the pcre2 build tag the UTF-8 tier has no entrant but Go's regexp.
// The scoreboard must say so rather than quietly report a number that looks
// like a field result, so PCRE2Available is what the bar reads to decide.
package arena

import "fmt"

// PCRE2Available reports whether this build has the PCRE2 entrant wired in.
const PCRE2Available = false

// PCRE2 is the unwired stand-in; every method is unreachable by construction.
type PCRE2 struct{}

// NewPCRE2 always fails in a build without the pcre2 tag.
func NewPCRE2(patterns []string) (*PCRE2, error) {
	return nil, fmt.Errorf("pcre2: not built (rebuild with -tags pcre2)")
}

// JIT reports whether the JIT compiler accepted the pattern set.
func (p *PCRE2) JIT() bool { return false }

// FirstIndex returns the leftmost match offset in bytes, or -1.
func (p *PCRE2) FirstIndex(haystack string) int { return -1 }

// Close releases the compiled pattern set.
func (p *PCRE2) Close() {}
