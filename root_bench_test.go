//go:build go1.24

package casei

import (
	"strings"
	"testing"
)

// BenchmarkPairShuftiMatcher covers the dense two-group root projection on a
// non-ASCII miss stream. The short case keeps the length guard scalar; longer
// cases enter the AVX-512 transition when it is runtime-enabled.
func BenchmarkPairShuftiMatcher(b *testing.B) {
	matcher := NewMatcher([]string{"Kелвин0", "ſекрет1", "Σигма2", "ςигма3", "щупальце4"})
	if !matcher.plan.filter.shufti.usable() {
		b.Fatal("pair Shufti filter was not compiled")
	}
	for _, tc := range []struct {
		name     string
		haystack string
	}{
		{"64B", strings.Repeat("ж", 32)},
		{"1KiB", strings.Repeat("ж", 512)},
		{"64KiB", strings.Repeat("ж", 32<<10)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.haystack)))
			if _, ok := matcher.Find(tc.haystack); ok {
				b.Fatal("unexpected match")
			}
			for b.Loop() {
				_, _ = matcher.Find(tc.haystack)
			}
		})
	}
}

func BenchmarkTripleSkipBytes(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		_ = tripleSkipBytes(haystack, 0, &plan.triples)
	}
}

func BenchmarkTriplePlan(b *testing.B) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	haystack := strings.Repeat("x", 1<<20)
	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		_, _ = plan.find(haystack)
	}
}
