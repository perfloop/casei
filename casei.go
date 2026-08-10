// Package casei is an open benchmark arena for UTF-8 case-insensitive
// substring search.
//
// IndexFold below is the function under optimization. It is deliberately the
// simple, obviously-correct reference form: the tests define the semantics,
// the benchmarks in bench_test.go define the competition, and CONTEXT.md
// catalogs every previously known technique. The goal of this repository is
// an implementation of IndexFold that beats every baseline in the benchmark
// suite by a wide margin without losing a single test.
//
// Semantics — Unicode simple case folding over UTF-8:
//
//   - Two code points match when they belong to the same simple case-folding
//     orbit (unicode.SimpleFold). This is exactly the matching used by Go's
//     regexp with (?i) and by rust/regex: 'k' matches 'K' and the Kelvin
//     sign U+212A; 's' matches 'S' and long s U+017F; σ, ς and Σ all match;
//     ß matches ẞ (U+1E9E) but NOT "ss" (no full folding); İ (U+0130) and
//     ı (U+0131) fold only to themselves (locale-independent).
//   - Matching is per code point, so a match window's byte length can differ
//     from the needle's ("kelvin" is 6 bytes but matches a 8-byte window
//     starting with U+212A). IndexFold returns the byte offset of the first
//     match start; match starts are haystack rune boundaries.
//   - Bytes that are not part of a valid UTF-8 encoding are opaque units:
//     they match only an opaque occurrence of the identical byte, never a
//     fragment of a valid encoding, and are never folded.
//   - ASCII consequences: only the 52 ASCII letters fold within ASCII; the
//     0x20-adjacent punctuation pairs ('[' vs '{', '@' vs '`', ']' vs '}',
//     '\' vs '|', '^' vs '~') never match.
package casei

import (
	"unicode"
	"unicode/utf8"
)

// IndexFold returns the byte index of the first occurrence of needle in
// haystack under Unicode simple case folding, or -1 if needle is not
// present. An empty needle matches at index 0.
func IndexFold(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i < len(haystack); {
		if foldHasPrefix(haystack[i:], needle) {
			return i
		}
		_, size := utf8.DecodeRuneInString(haystack[i:])
		i += size
	}
	return -1
}

// foldHasPrefix reports whether s begins with a fold-equal rendering of
// needle.
func foldHasPrefix(s, needle string) bool {
	for len(needle) > 0 {
		rn, sn := utf8.DecodeRuneInString(needle)
		if rn == utf8.RuneError && sn == 1 {
			// Opaque needle byte: matches only an opaque occurrence of the
			// identical byte, never a fragment of a valid encoding.
			rs, ss := utf8.DecodeRuneInString(s)
			if rs != utf8.RuneError || ss != 1 || s[0] != needle[0] {
				return false
			}
			s, needle = s[1:], needle[1:]
			continue
		}
		rs, ss := utf8.DecodeRuneInString(s)
		if rs == utf8.RuneError && ss <= 1 {
			// s is exhausted (ss == 0) or holds an opaque byte (ss == 1):
			// it cannot match the valid needle rune rn.
			return false
		}
		if !foldEq(rs, rn) {
			return false
		}
		s, needle = s[ss:], needle[sn:]
	}
	return true
}

// foldEq reports whether a and b are in the same simple case-folding orbit.
func foldEq(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

// RuntimeVectorBits reports the widest vector path this package dispatches to
// on the running machine: 0 for the scalar/portable path, 256 for AVX2, 512
// for AVX-512. It is telemetry, not capability -- the answer is what the
// engine actually uses, never what the CPU offers. This reference
// implementation is scalar and says so; a candidate engine is expected to
// raise it to the machine's width, and the arena's dispatch report prints it
// beside every entrant's so a scalar engine can never borrow a vector field's
// credibility.
func RuntimeVectorBits() int { return 0 }
