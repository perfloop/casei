package arena_test

import "math/rand/v2"

// The arena keeps its own copies of the corpus helpers it needs. They are
// small, and the alternative is worse: sharing fixtures with the candidate's
// test files would let a candidate move the field by editing its own tests.

// asciiLower folds only 'A'..'Z'. Used by the ASCII-tier ceiling benchmark.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 0x20
		}
	}
	return string(b)
}

// flipCases randomly flips ASCII letter case (planting helper; fold-neutral).
func flipCases(rng *rand.Rand, s string) string {
	b := []byte(s)
	for i, c := range b {
		if rng.IntN(2) == 0 {
			continue
		}
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = c - 0x20
		case 'A' <= c && c <= 'Z':
			b[i] = c + 0x20
		}
	}
	return string(b)
}
