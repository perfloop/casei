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
	{"empty set", "abc", []string{}, Match{}, false},
	{"fold pair across patterns", "the Kelvin scale", []string{"KELVIN", "scale"}, Match{0, 4}, true},
	{"kelvin sign haystack multi", "xxKelvin scale", []string{"scale", "kelvin"}, Match{1, 2}, true},
	{"cyrillic multi", "доктор Ватсон", []string{"ШЕРЛОК", "ватсон"}, Match{1, 13}, true},
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
