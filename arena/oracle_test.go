package arena_test

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tsenart/casei"
)

// ---- the arena's own semantic oracle ----------------------------------------
//
// Copied deliberately rather than shared with the candidate's tests. This is
// the definition of "correct" that every entrant, candidate and baseline
// alike, is held to. A candidate that could edit it could move the field it is
// being measured against.

// orbitMin maps a rune to the smallest member of its simple-fold orbit, so
// fold-equality becomes plain equality of canonical forms.
func orbitMin(r rune) rune {
	m := r
	for x := unicode.SimpleFold(r); x != r; x = unicode.SimpleFold(x) {
		if x < m {
			m = x
		}
	}
	return m
}

// canonFold decodes s into canonical fold form. Opaque (invalid-encoding)
// bytes become distinct negative sentinels so they compare byte-exactly and
// can never collide with a real rune. offs[i] is the byte offset in s of
// canonical element i; a final entry holds len(s).
func canonFold(s string) (canon []rune, offs []int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			canon = append(canon, -rune(s[i])-1)
		} else {
			canon = append(canon, orbitMin(r))
		}
		offs = append(offs, i)
		i += size
	}
	offs = append(offs, len(s))
	return canon, offs
}

// reference is a second, structurally different implementation: canonical
// fold both strings, then exact slice search. Every implementation in this
// repository must agree with it on every input.
func reference(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	ch, offs := canonFold(haystack)
	cn, _ := canonFold(needle)
	for i := 0; i+len(cn) <= len(ch); i++ {
		match := true
		for j := range cn {
			if ch[i+j] != cn[j] {
				match = false
				break
			}
		}
		if match {
			return offs[i]
		}
	}
	return -1
}

// canonFoldString rebuilds a string in canonical fold form (UTF-8 tier
// ceiling: what caseless search costs if folding were free).
func canonFoldString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteByte(s[i])
		} else {
			b.WriteRune(orbitMin(r))
		}
		i += size
	}
	return b.String()
}

// fold reference, leftmost start, ties to the lowest pattern index.
func refFind(haystack string, patterns []string) (casei.Match, bool) {
	best := casei.Match{Pattern: -1, Start: -1}
	for i, p := range patterns {
		pos := reference(haystack, p)
		if pos < 0 {
			continue
		}
		if best.Pattern < 0 || pos < best.Start {
			best = casei.Match{Pattern: i, Start: pos}
		}
	}
	if best.Pattern < 0 {
		return casei.Match{}, false
	}
	return best, true
}
