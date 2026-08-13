// Package casei is an open benchmark arena for UTF-8 case-insensitive
// substring search.
//
// IndexFold is the function under optimization. The tests define its
// semantics, the benchmarks in bench_test.go define the competition, and
// CONTEXT.md catalogs every previously known technique.
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

// IndexFold returns the byte index of the first occurrence of needle in
// haystack under Unicode simple case folding, or -1 if needle is not present.
// It is the one-pattern instantiation of the same compiled plan used by
// Matcher.Find. An empty needle matches at index 0.
func IndexFold(haystack, needle string) int {
	match, ok := cachedSinglePlan(needle).find(haystack)
	if !ok {
		return -1
	}
	return match.Start
}

// RuntimeVectorBits reports the widest runtime-gated block transition this
// package can dispatch on the running machine: 0 for the portable path, 256
// for AVX2, and 512 for AVX-512 BW. It reports a package path, rather than an
// advertised CPU feature, so the arena can place this engine beside the field
// under the same GODEBUG feature controls.
func RuntimeVectorBits() int { return runtimeVectorBits() }
