// Package casei is an open benchmark arena for ASCII case-insensitive
// substring search.
//
// IndexFold below is the function under optimization. It is deliberately the
// simple, obviously-correct reference form: the arena's tests define the
// semantics, the benchmarks in bench_test.go define the competition, and
// CONTEXT.md catalogs every previously known technique. The goal of this
// repository is an implementation of IndexFold that beats every baseline in
// the benchmark suite by a wide margin without losing a single test.
//
// Semantics: only the 52 ASCII letters fold ('A'..'Z' matches 'a'..'z').
// Every other byte value compares exactly — including the 0x20-adjacent
// punctuation pairs ('[' vs '{', '@' vs '`', ']' vs '}', '\' vs '|', '^' vs
// '~') and all bytes >= 0x80, which are passed through opaquely so UTF-8
// multibyte sequences are matched byte-exactly, never case-folded.
package casei

// IndexFold returns the index of the first occurrence of needle in haystack
// under ASCII case folding, or -1 if needle is not present. An empty needle
// matches at index 0.
func IndexFold(haystack, needle string) int {
	n := len(needle)
	switch {
	case n == 0:
		return 0
	case n > len(haystack):
		return -1
	}
	first := foldByte(needle[0])
	for i := 0; i+n <= len(haystack); i++ {
		if foldByte(haystack[i]) != first {
			continue
		}
		j := 1
		for j < n && foldByte(haystack[i+j]) == foldByte(needle[j]) {
			j++
		}
		if j == n {
			return i
		}
	}
	return -1
}

// foldByte maps 'A'..'Z' to 'a'..'z' and leaves every other byte unchanged.
func foldByte(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + 0x20
	}
	return b
}
