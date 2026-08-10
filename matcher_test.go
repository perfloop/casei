package casei

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

// refFind is the independent multi-needle reference: per-pattern canonical
// fold reference, leftmost start, ties to the lowest pattern index.
func refFind(haystack string, patterns []string) (Match, bool) {
	best := Match{Pattern: -1, Start: -1}
	for i, p := range patterns {
		pos := reference(haystack, p)
		if pos < 0 {
			continue
		}
		if best.Pattern < 0 || pos < best.Start {
			best = Match{Pattern: i, Start: pos}
		}
	}
	if best.Pattern < 0 {
		return Match{}, false
	}
	return best, true
}

var matcherTraps = []struct {
	name     string
	haystack string
	patterns []string
	want     Match
	ok       bool
}{
	{"single hit", "Hello World", []string{"world"}, Match{0, 6}, true},
	{"no hit", "Hello World", []string{"mars", "venus"}, Match{}, false},
	{"leftmost wins", "aa bravo alpha", []string{"ALPHA", "BRAVO"}, Match{1, 3}, true},
	{"tie goes to lower index", "xxABCxx", []string{"abc", "ABC"}, Match{0, 2}, true},
	{"empty pattern matches at zero", "abc", []string{"zzz", ""}, Match{1, 0}, true},
	{"non-empty lower ID beats empty tie", "ABC", []string{"abc", ""}, Match{0, 0}, true},
	{"longer terminal beats suffix terminal", "abc", []string{"BC", "ABC"}, Match{1, 0}, true},
	{"empty set", "abc", []string{}, Match{}, false},
	{"fold pair across patterns", "the Kelvin scale", []string{"KELVIN", "scale"}, Match{0, 4}, true},
	{"kelvin sign haystack multi", "xxKelvin scale", []string{"scale", "kelvin"}, Match{1, 2}, true},
	{"cyrillic multi", "доктор Ватсон", []string{"ШЕРЛОК", "ватсон"}, Match{1, 13}, true},
	{"opaque byte cannot match continuation", "K", []string{"\x84"}, Match{}, false},
	{"opaque byte matches opaque byte", "x\x84Y", []string{"\x84y"}, Match{0, 1}, true},
	{"bracket not brace multi", "fn{T}(x)", []string{"fn[T]", "(X)"}, Match{1, 5}, true},
	{"overlapping patterns leftmost", "abcd", []string{"BCD", "ABC"}, Match{1, 0}, true},
}

func TestMatcherTraps(t *testing.T) {
	for _, tc := range matcherTraps {
		m := NewMatcher(tc.patterns)
		got, ok := m.Find(tc.haystack)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%s: Find = %+v,%v want %+v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
		ref, refOK := refFind(tc.haystack, tc.patterns)
		if refOK != tc.ok || (refOK && ref != tc.want) {
			t.Fatalf("%s: trap table wrong: reference = %+v,%v", tc.name, ref, refOK)
		}
	}
}

func TestMatcherPairPrefixBoundaries(t *testing.T) {
	patterns := []string{"ZqA", "zQa", "ZqB"}
	for offset := 0; offset < 192; offset++ {
		haystack := strings.Repeat("x", offset) + "zQa" + strings.Repeat("x", 192-offset)
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("offset %d: Find = %+v,%v want %+v,%v", offset, got, gotOK, want, wantOK)
		}
	}

	// The pair probe may skip an ASCII root byte only when its required next
	// token is absent. High bytes remain scalar so width-changing folds and
	// opaque bytes preserve the regular transition semantics.
	for _, haystack := range []string{
		strings.Repeat("Z!", 128) + "zQa",
		strings.Repeat("x", 63) + "Kqa" + strings.Repeat("x", 128),
		strings.Repeat("x", 63) + "z\x80a" + strings.Repeat("x", 128),
	} {
		patterns := []string{"ZqA", "kqa", "z\x80a"}
		got, gotOK := NewMatcher(patterns).Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("Find(%q) = %+v,%v want %+v,%v", haystack, got, gotOK, want, wantOK)
		}
	}
}

func TestMatcherUnicodePairPairBoundaries(t *testing.T) {
	patterns := []string{"яр"}
	plan := newSearchPlan(patterns)
	if plan.unicodePairN < 2 || plan.unicodePairs[0].pairPair.valid == 0 {
		t.Fatalf("no dispersed pair transition: %+v", plan.unicodePairs[0])
	}
	for _, offset := range []int{0, 63, 64, 127, 128, 4093} {
		for _, rendering := range []string{"яр", "ЯР", "яР", "Яр"} {
			haystack := strings.Repeat("x", offset) + rendering + strings.Repeat("x", 4300-offset)
			got, gotOK := plan.find(haystack)
			want, wantOK := refFind(haystack, patterns)
			if gotOK != wantOK || (gotOK && got != want) {
				t.Fatalf("offset %d, %q: Find = %+v,%v want %+v,%v", offset, rendering, got, gotOK, want, wantOK)
			}
		}
	}

	// A primary pair without its dispersed partner must not enter the token
	// machine, including when an invalid byte replaces the partner.
	for _, haystack := range []string{
		strings.Repeat("яx", 2048),
		strings.Repeat("x", 63) + "я\xd1\xff" + strings.Repeat("x", 4096),
	} {
		got, gotOK := plan.find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("false-primary Find = %+v,%v want %+v,%v", got, gotOK, want, wantOK)
		}
	}
}

func TestMatcherMatchesReferenceRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 99))
	for i := 0; i < 20000; i++ {
		h := randomBytes(rng, 64)
		n := 1 + rng.IntN(5)
		pats := make([]string, n)
		for j := range pats {
			pats[j] = randomBytes(rng, 6)
		}
		got, gotOK := NewMatcher(pats).Find(h)
		want, wantOK := refFind(h, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("iter %d: Find(%q, %q) = %+v,%v want %+v,%v", i, h, pats, got, gotOK, want, wantOK)
		}
	}
	// Planted pass over rune strings so hits are common.
	for i := 0; i < 10000; i++ {
		n := 1 + rng.IntN(4)
		pats := make([]string, n)
		for j := range pats {
			pats[j] = randomRuneString(rng, 5)
		}
		pick := pats[rng.IntN(n)]
		h := randomRuneString(rng, 20) + flipCases(rng, pick) + randomRuneString(rng, 20)
		got, gotOK := NewMatcher(pats).Find(h)
		want, wantOK := refFind(h, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("planted iter %d: Find(%q, %q) = %+v,%v want %+v,%v", i, h, pats, got, gotOK, want, wantOK)
		}
	}
}

// TestMatcherMatchesRegexpAlternation pins the leftmost-start contract to
// the stdlib: (?i)(?:p0|p1|...) must agree on the match START (regexp
// cannot report which alternative won, so pattern identity is pinned by
// refFind above instead).
func TestMatcherMatchesRegexpAlternation(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260804, 123))
	for i := 0; i < 4000; i++ {
		n := 1 + rng.IntN(4)
		pats := make([]string, n)
		quoted := make([]string, n)
		nonEmpty := false
		for j := range pats {
			pats[j] = randomRuneString(rng, 5)
			quoted[j] = regexp.QuoteMeta(pats[j])
			if pats[j] != "" {
				nonEmpty = true
			}
		}
		if !nonEmpty {
			continue // regexp empty alternation semantics diverge trivially
		}
		h := randomRuneString(rng, 30)
		re := regexp.MustCompile(`(?i)(?:` + strings.Join(quoted, "|") + `)`)
		wantStart := -1
		if loc := re.FindStringIndex(h); loc != nil {
			wantStart = loc[0]
		}
		got, ok := NewMatcher(pats).Find(h)
		gotStart := -1
		if ok {
			gotStart = got.Start
		}
		if gotStart != wantStart {
			t.Fatalf("iter %d: Find(%q, %q) start = %d, regexp says %d", i, h, pats, gotStart, wantStart)
		}
	}
}

func FuzzMatcher(f *testing.F) {
	f.Add("Hello World", "world", "mars", "")
	f.Add("xxKelvin scale", "scale", "kelvin", "ſecret")
	f.Add("доктор Ватсон", "ватсон", "ШЕРЛОК", "z")
	f.Fuzz(func(t *testing.T, haystack, p0, p1, p2 string) {
		pats := []string{p0, p1, p2}
		got, gotOK := NewMatcher(pats).Find(haystack)
		want, wantOK := refFind(haystack, pats)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Fatalf("Find(%q, %q) = %+v,%v want %+v,%v", haystack, pats, got, gotOK, want, wantOK)
		}
	})
}
