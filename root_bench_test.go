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

// BenchmarkUnicodePairConfirm makes the pair-pair filter admit 2,134
// near-full false survivors across a 1.5 MiB UTF-8 miss. The needle has only
// width-stable simple-fold forms, so this isolates confirmation after the
// AVX-512 filter rather than width-changing fold handling.
func BenchmarkUnicodePairConfirm(b *testing.B) {
	if !asciiPairVBMIEnabled() {
		b.Skip("requires AVX-512F/BW/VBMI")
	}

	const needle = "приключения лилий"
	const haystackBytes = 1_570_556
	const survivors = 2_134

	matcher := NewMatcher([]string{needle})
	plan := matcher.plan
	if plan.unicodePairN == 0 || plan.unicodePairs[0].pairPair.valid == 0 {
		b.Fatalf("no pair-pair anchor for %q: %+v", needle, plan.unicodePairs)
	}
	anchor := plan.unicodePairs[0]
	filter := anchor.pairPair
	// Only the last rune differs, so every surviving anchor traverses the
	// whole width-stable literal before confirmation rejects it.
	falseLiteral := needle[:len(needle)-len("й")] + "я"
	bytes := []byte(strings.Repeat("x", haystackBytes))
	step := len(bytes) / survivors
	inserted := 0
	for at := anchor.at; at-anchor.at+len(falseLiteral) <= len(bytes) && inserted < survivors; at += step {
		copy(bytes[at-anchor.at:], falseLiteral)
		inserted++
	}
	if inserted != survivors {
		b.Fatalf("inserted %d false survivors, want %d", inserted, survivors)
	}
	haystack := string(bytes)
	// The decoded baseline calls the byte scanner once initially and once after
	// every rejected survivor, so this also pins the intended reentry shape.
	actual, confirms, scannerCalls := 0, 0, 0
	for at := 0; at+int(filter.offset)+1 < len(haystack); {
		scannerCalls++
		at += pairPairSkipBytes(haystack, at, &filter)
		if at+int(filter.offset)+1 >= len(haystack) {
			break
		}
		actual++
		if start := at - anchor.at; start >= 0 && plan.matchesSingleAt(haystack, start) {
			confirms++
		}
		at++
	}
	if actual != survivors || confirms != 0 || scannerCalls != survivors+1 {
		b.Fatalf("pair-pair scanner calls=%d survivors=%d confirms=%d, want %d false survivors and %d calls",
			scannerCalls, actual, confirms, survivors, survivors+1)
	}
	if match, ok := matcher.Find(haystack); ok {
		b.Fatalf("false-survivor miss = %+v", match)
	}

	b.SetBytes(int64(len(haystack)))
	for b.Loop() {
		_, _ = matcher.Find(haystack)
	}
}
