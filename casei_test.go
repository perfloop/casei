package casei

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// asciiLower folds only 'A'..'Z'. Deliberately NOT strings.ToLower, which is
// Unicode-aware and would fold non-ASCII (and re-encode invalid UTF-8).
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 0x20
		}
	}
	return string(b)
}

// reference is an independent second implementation: fold both strings with
// the ASCII-only fold, then exact search. Every other implementation in this
// repository must agree with it on every input.
func reference(haystack, needle string) int {
	return strings.Index(asciiLower(haystack), asciiLower(needle))
}

var trapCases = []struct {
	name     string
	haystack string
	needle   string
	want     int
}{
	{"empty needle", "abc", "", 0},
	{"empty both", "", "", 0},
	{"needle longer", "ab", "abc", -1},
	{"exact", "hello world", "world", 6},
	{"fold simple", "Hello World", "world", 6},
	{"fold needle upper", "hello world", "WORLD", 6},
	{"whole string case diff", "HELLO", "hello", 0},
	{"match at start", "Foobar", "foo", 0},
	{"match at end", "barFOO", "foo", 3},

	// 0x20-adjacent punctuation pairs must NOT fold.
	{"bracket brace", "[hello]", "{hello}", -1},
	{"brace bracket upper", "{hello}", "[HELLO]", -1},
	{"at backtick", "@X@x", "`x", -1},
	{"backtick literal", "@X`x", "`x", 2},
	{"backslash pipe", `a\b`, "a|b", -1},
	{"caret tilde", "a^b", "a~b", -1},
	{"bracket exact inside needle", "fn[T any](x)", "FN[t ANY](X)", 0},
	{"brace does not match bracket needle", "fn{T any}(x)", "fn[T any](x)", -1},

	// Bytes >= 0x80 are opaque: exact match only, never folded.
	{"utf8 exact", "na\xc3\xafve", "\xc3\xaf", 2},
	{"utf8 case NOT folded", "na\xc3\xafve", "\xc3\x8f", -1},
	{"high byte exact", "\x80abc", "\x80a", 0},
	{"high byte pair not folded", "\x80abc", "\xa0A", -1},
	{"high byte with letter fold", "\x80ABC", "\x80abc", 0},

	// Long needles with the case difference in the tail (the class of bug a
	// scalar-tail verify path gets wrong: see CONTEXT.md, "known traps").
	{"tail case diff 17B match", strings.Repeat("x", 100) + "abcdefghijklmnopQ", "abcdefghijklmnopq", 100},
	{"tail mismatch 17B", strings.Repeat("x", 100) + "abcdefghijklmnopQ", "abcdefghijklmnopr", -1},
	{"tail case diff 33B match", strings.Repeat("y", 64) + "abcdefghijklmnopqrstuvwxyzABCDEF" + "g", strings.Repeat("Y", 0) + "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefG", 64},

	// Candidate abutting the very end of the haystack.
	{"match flush at end", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzHELLO", "hello", 32},
	{"near miss at end", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzHELL", "hello", -1},

	// Periodic / repetitive inputs (adaptive-filter and budget territory).
	{"periodic hit", strings.Repeat("ab", 1000) + "abc", "ABC", 2000},
	{"periodic miss", strings.Repeat("ab", 1000), "abc", -1},
	{"samechar hit", strings.Repeat("a", 500) + "b", strings.Repeat("A", 4) + "B", 496},
	{"overlapping prefix", "aaab", "AAB", 1},
}

func TestIndexFoldTraps(t *testing.T) {
	for _, tc := range trapCases {
		if got := IndexFold(tc.haystack, tc.needle); got != tc.want {
			t.Errorf("%s: IndexFold(%q, %q) = %d, want %d", tc.name, tc.haystack, tc.needle, got, tc.want)
		}
		// The trap table itself must agree with the independent reference.
		if ref := reference(tc.haystack, tc.needle); ref != tc.want {
			t.Fatalf("%s: trap table wrong: reference = %d, table says %d", tc.name, ref, tc.want)
		}
	}
}

// alphabet is chosen to stress folding edges: letters at both case ends,
// digits, every 0x20-adjacent punctuation pair, spaces, and high bytes.
var alphabet = []byte("aAzZ mM09[{]}@`\\|^~\x80\xc3\xaf\x8f")

func randomString(rng *rand.Rand, maxLen int) string {
	n := rng.IntN(maxLen + 1)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.IntN(len(alphabet))]
	}
	return string(b)
}

func TestIndexFoldMatchesReferenceRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 42))
	for i := 0; i < 50000; i++ {
		h := randomString(rng, 48)
		n := randomString(rng, 8)
		got, want := IndexFold(h, n), reference(h, n)
		if got != want {
			t.Fatalf("iter %d: IndexFold(%q, %q) = %d, want %d", i, h, n, got, want)
		}
	}
	// A second pass planting a case-mangled needle so hits are common.
	for i := 0; i < 20000; i++ {
		n := randomString(rng, 12)
		if n == "" {
			continue
		}
		pre := randomString(rng, 32)
		post := randomString(rng, 32)
		h := pre + flipCases(rng, n) + post
		got, want := IndexFold(h, n), reference(h, n)
		if got != want {
			t.Fatalf("planted iter %d: IndexFold(%q, %q) = %d, want %d", i, h, n, got, want)
		}
	}
}

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

func FuzzIndexFold(f *testing.F) {
	f.Add("Hello World", "world")
	f.Add("[hello]", "{hello}")
	f.Add(strings.Repeat("ab", 64), "abc")
	f.Add("na\xc3\xafve", "\xc3\x8f")
	f.Add(strings.Repeat("a", 40)+"B", strings.Repeat("A", 3)+"b")
	f.Fuzz(func(t *testing.T, haystack, needle string) {
		got, want := IndexFold(haystack, needle), reference(haystack, needle)
		if got != want {
			t.Fatalf("IndexFold(%q, %q) = %d, want %d", haystack, needle, got, want)
		}
		if got >= 0 {
			window := haystack[got : got+len(needle)]
			if asciiLower(window) != asciiLower(needle) {
				t.Fatalf("reported match at %d is not a fold-equal window: %q vs %q", got, window, needle)
			}
		}
	})
}
