package arena_test

// The multi-needle arena. Pre-registered before any multi-needle claim.
// Baselines:
//
//   candidate  - casei.Matcher (naive reference form, under optimization)
//   regexpAlt  - precompiled (?i)(?:p0|p1|...): the stdlib answer and the
//                semantic anchor for leftmost-start
//   ac         - github.com/petar-dambovaliev/aho-corasick, DFA,
//                leftmost-first, AsciiCaseInsensitive (ASCII tier only:
//                the reference multi-pattern library renounces Unicode
//                folding in its own docs)
//   ceiling    - exact-match AC (DFA) over pre-folded haystack+patterns:
//                what multi-needle costs when folding is free

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	ac "github.com/petar-dambovaliev/aho-corasick"

	"github.com/tsenart/casei"
)

type multiScenario struct {
	name     string
	haystack string
	patterns []string
	utf8     bool
}

func genNeedles(n int, format string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(format, i)
	}
	return out
}

var multiScenarios = func() []multiScenario {
	logs1m := buildLogCorpus(1 << 20)
	cyr1m := buildWordCorpus(cyrillicWords, 1<<20)

	hit8 := []string{
		"fatal panic", "segfault detected", "oom killed", "disk full",
		"Payment Declined", "quota exceeded", "handshake failed", "watchdog fired",
	}
	return []multiScenario{
		{"multi_N2_miss_log_1mb", logs1m, []string{"fatal panic", "segfault detected"}, false},
		{"multi_N8_miss_log_1mb", logs1m, genNeedles(8, "Zq%03dxW vK"), false},
		{"multi_N64_miss_log_64kb", logs1m[:64<<10], genNeedles(64, "Zq%03dxW"), false},
		{"multi_N512_miss_log_64kb", logs1m[:64<<10], genNeedles(512, "Zq%03dxW"), false},
		{"multi_N8_hit_log_1mb", plant(logs1m, "Payment Declined", 4), hit8, false},
		{"multi_N8_miss_ru_1mb", cyr1m, genNeedles(8, "щупальце%d"), true},
		{"multi_N64_miss_ru_64kb", cyr1m[:64<<10], genNeedles(64, "щупальце%d"), true},
		{"multi_N8_hazard_hit_1mb", plant(buildProseCorpus(1<<20), "Kelvin", 8),
			[]string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}, true},
	}
}()

func regexpAltFor(patterns []string) *regexp.Regexp {
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`(?i)(?:` + strings.Join(quoted, "|") + `)`)
}

func acBuild(patterns []string, caseInsensitive bool) ac.AhoCorasick {
	b := ac.NewAhoCorasickBuilder(ac.Opts{
		AsciiCaseInsensitive: caseInsensitive,
		MatchKind:            ac.LeftMostFirstMatch,
		DFA:                  true,
	})
	return b.Build(patterns)
}

func acFirst(a *ac.AhoCorasick, h string) (casei.Match, bool) {
	it := a.Iter(h)
	m := it.Next()
	if m == nil {
		return casei.Match{}, false
	}
	return casei.Match{Pattern: m.Pattern(), Start: m.Start()}, true
}

func foldAll(patterns []string, utf8Tier bool) []string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		if utf8Tier {
			out[i] = canonFoldString(p)
		} else {
			out[i] = asciiLower(p)
		}
	}
	return out
}

// TestMultiBaselinesAgree pins the multi-needle contract: candidate and the
// ASCII-tier AC baseline against refFind; regexpAlt on match start.
func TestMultiBaselinesAgree(t *testing.T) {
	for _, s := range multiScenarios {
		want, wantOK := refFind(s.haystack, s.patterns)
		got, gotOK := casei.NewMatcher(s.patterns).Find(s.haystack)
		if gotOK != wantOK || (gotOK && got != want) {
			t.Errorf("%s/candidate: %+v,%v want %+v,%v", s.name, got, gotOK, want, wantOK)
		}
		re := regexpAltFor(s.patterns)
		reStart := -1
		if loc := re.FindStringIndex(s.haystack); loc != nil {
			reStart = loc[0]
		}
		wantStart := -1
		if wantOK {
			wantStart = want.Start
		}
		if reStart != wantStart {
			t.Errorf("%s/regexpAlt: start %d want %d", s.name, reStart, wantStart)
		}
		if !s.utf8 {
			a := acBuild(s.patterns, true)
			acGot, acOK := acFirst(&a, s.haystack)
			if acOK != wantOK || (acOK && acGot != want) {
				t.Errorf("%s/ac: %+v,%v want %+v,%v", s.name, acGot, acOK, want, wantOK)
			}
		}
	}
}

func BenchmarkMatcher(b *testing.B) {
	for _, s := range multiScenarios {
		m := casei.NewMatcher(s.patterns)
		b.Run(s.name+"/candidate", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, matcherFound = m.Find(s.haystack)
			}
		})
		re := regexpAltFor(s.patterns)
		b.Run(s.name+"/regexpAlt", func(b *testing.B) {
			b.SetBytes(int64(len(s.haystack)))
			for i := 0; i < b.N; i++ {
				matcherSink = len(re.FindStringIndex(s.haystack))
			}
		})
		if !s.utf8 {
			a := acBuild(s.patterns, true)
			b.Run(s.name+"/ac", func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				for i := 0; i < b.N; i++ {
					_, matcherFound = acFirst(&a, s.haystack)
				}
			})
		}
		lh := s.haystack
		if s.utf8 {
			lh = canonFoldString(s.haystack)
		} else {
			lh = asciiLower(s.haystack)
		}
		exact := acBuild(foldAll(s.patterns, s.utf8), false)
		b.Run(s.name+"/ceiling", func(b *testing.B) {
			b.SetBytes(int64(len(lh)))
			for i := 0; i < b.N; i++ {
				_, matcherFound = acFirst(&exact, lh)
			}
		})
	}
}

var (
	matcherSink  int
	matcherFound bool
)
