package casei

import (
	"strings"
	"testing"
)

func TestRootSkipASCII(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		kind   uint8
		needle byte
		want   int
	}{
		{"all rootless", strings.Repeat("x", 257), rootASCIIFold, 'z', 257},
		{"case-fold root", strings.Repeat("x", 130) + "Z" + strings.Repeat("x", 130), rootASCIIFold, 'z', 130},
		{"exact root", strings.Repeat("x", 130) + "!" + strings.Repeat("x", 130), rootExact, '!', 130},
		{"non-ascii stop", strings.Repeat("x", 130) + "K" + strings.Repeat("x", 130), rootASCIIFold, 'z', 130},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rootSkipASCII(tc.input, 0, tc.kind, tc.needle); got != tc.want {
				t.Fatalf("rootSkipASCII(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestLiteralSkipASCII(t *testing.T) {
	input := strings.Repeat("x", 63) + "K" + strings.Repeat("x", 63) + "Z"
	if got, want := literalSkipASCII(input, 0, rootASCIIFold, 'z'), len(input)-1; got != want {
		t.Fatalf("literalSkipASCII = %d, want %d", got, want)
	}
	if got, want := literalSkipASCII("\xff\x80abc", 0, rootASCIIFold, 'z'), 5; got != want {
		t.Fatalf("literalSkipASCII malformed = %d, want %d", got, want)
	}
}

func TestTripleFilterKeepsOtherRoots(t *testing.T) {
	patterns := []string{"~", "aZm]"}
	plan := newSearchPlan(patterns)
	if plan.triples.usable() || !plan.filter.usable() {
		t.Fatalf("partial triples were not retained by the complete filter: triples=%+v filter=%+v", plan.triples, plan.filter)
	}
	if got, ok := plan.find(strings.Repeat("x", 10) + "~" + strings.Repeat("x", 10)); !ok || got != (Match{Pattern: 0, Start: 10}) {
		t.Fatalf("Find = %+v,%v; triples=%+v filter=%+v", got, ok, plan.triples, plan.filter)
	}
}

func TestTripleFilterSkip(t *testing.T) {
	plan := newSearchPlan([]string{"fatal panic", "segfault detected"})
	if !plan.triples.usable() {
		t.Fatalf("no triples: filter=%+v", plan.filter)
	}
	t.Logf("triples=%+v roots=%v fallback=%+v", plan.triples, plan.tripleRoots, plan.filter)
	if got := tripleSkipBytes(strings.Repeat("x", 257), 0, &plan.triples); got != 255 {
		t.Fatalf("triple skip = %d, want 255; triples=%+v", got, plan.triples)
	}
	if got := filterSkipBytes(strings.Repeat("x", 257), 0, &plan.filter); got != 257 {
		t.Fatalf("fallback skip = %d, want 257; filter=%+v", got, plan.filter)
	}
	if got := tripleSkipBytes(strings.Repeat("x", 130)+"FAT", 0, &plan.triples); got != 130 {
		t.Fatalf("triple match skip = %d, want 130; triples=%+v", got, plan.triples)
	}
}

func TestTripleMixedFilter(t *testing.T) {
	patterns := []string{"fatal panic", "segfault detected"}
	plan := newSearchPlan(patterns)
	if plan.triples.n != 3 || plan.triples.values[0].fold != 7 || plan.triples.values[1].fold != 7 ||
		plan.triples.values[2].fold != 4 || plan.triples.values[2].first < 0x80 {
		t.Fatalf("mixed triple plan = %+v", plan.triples)
	}
	prefix := strings.Repeat("x", 130)
	for _, tc := range []struct {
		haystack string
		want     Match
	}{
		{prefix + "FATAL PANIC", Match{Pattern: 0, Start: 130}},
		{prefix + "ſEGFAULT DETECTED", Match{Pattern: 1, Start: 130}},
	} {
		if got, ok := plan.find(tc.haystack); !ok || got != tc.want {
			t.Fatalf("Find(%q) = %+v,%v, want %+v,true", tc.haystack[130:], got, ok, tc.want)
		}
	}
	for _, haystack := range []string{
		prefix + "\xc5\xffegfault detected",
		prefix + "\xbfegfault detected",
	} {
		if got, ok := plan.find(haystack); ok {
			t.Fatalf("Find malformed %q = %+v,%v", haystack[130:], got, ok)
		}
	}
}

func TestOpaqueRootFilter(t *testing.T) {
	plan := newSearchPlan([]string{"\xe2"})
	if !plan.filter.usable() {
		t.Fatalf("opaque root did not get a filter: %+v", plan.filter)
	}
	if got := filterSkipBytes("xxxxxxxxxxx\xe2", 0, &plan.filter); got != 11 {
		t.Fatalf("opaque filter skip = %d, want 11; filter=%+v", got, plan.filter)
	}
	if got := IndexFold("xxxxxxxxxxx\xe2", "\xe2"); got != 11 {
		t.Fatalf("IndexFold opaque root = %d, want 11; filter=%+v", got, plan.filter)
	}
}

func TestFilterSkipBytes(t *testing.T) {
	var filter rootFilter
	if !filter.addOne('z') || !filter.addPair(0xce, 0xa3) ||
		!filter.addFoldPair('f', 'a', rootPairFoldFirst|rootPairFoldSecond) {
		t.Fatal("could not build root filter")
	}
	for _, tc := range []struct {
		input string
		want  int
	}{
		{strings.Repeat("x", 130) + "z" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 130) + "Σ" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 63) + "Σ" + strings.Repeat("x", 130), 63},
		{strings.Repeat("x", 130) + "FA" + strings.Repeat("x", 130), 130},
		{strings.Repeat("x", 257), 257},
	} {
		if got := filterSkipBytes(tc.input, 0, &filter); got != tc.want {
			t.Fatalf("filterSkipBytes(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestASCIIPairSkip(t *testing.T) {
	plan := newSearchPlan([]string{"fatal panic"})
	probe := &plan.asciiPair
	if !probe.usable() {
		t.Fatal("fixed long ASCII literal did not compile an aligned pair probe")
	}

	for _, offset := range []int{0, 1, 55, 56, 62, 63, 64, 65, 119, 120, 127, 128, 191, 255, 256, 257, 319, 320, 383, 1023, 1024, 1025, 1087} {
		haystack := strings.Repeat("x", offset) + "FaTaL pAnIc" + strings.Repeat("x", 193)
		candidates := len(haystack) - len(plan.asciiNeedle) + 1
		want := 0
		for want < candidates && !asciiPairAt(haystack, want, probe) {
			want++
		}
		if got := asciiPairSkipBytes(haystack, 0, candidates, probe); got != want {
			t.Fatalf("offset %d: pair skip=%d want %d", offset, got, want)
		}
		match, ok := plan.find(haystack)
		if !ok || match.Start != offset || match.Pattern != 0 {
			t.Fatalf("offset %d: find=%+v ok=%t", offset, match, ok)
		}
	}

	// An all-miss stream covers the steady-state four-block loop rather than
	// returning from an early candidate.
	miss := strings.Repeat("x", 2048)
	missCandidates := len(miss) - len(plan.asciiNeedle) + 1
	if got := asciiPairSkipBytes(miss, 0, missCandidates, probe); got != missCandidates {
		t.Fatalf("all-miss pair skip=%d want %d", got, missCandidates)
	}

	// The light pair transition admits this non-match, then must continue to
	// the later full literal rather than changing leftmost semantics.
	haystack := strings.Repeat("x", 63) + "fxxxxxxxn" + strings.Repeat("x", 64) + "fatal panic"
	match, ok := plan.find(haystack)
	want := 63 + len("fxxxxxxxn") + 64
	if !ok || match.Start != want || match.Pattern != 0 {
		t.Fatalf("false pair candidate: find=%+v ok=%t want start %d", match, ok, want)
	}
}

func TestASCIIOnlyStructuredProbe(t *testing.T) {
	needle := "[keys[i%"
	plan := newSearchPlan([]string{needle})
	if !plan.asciiOnly || !plan.asciiOnlyLong ||
		plan.asciiOnlyProbe.secondAt != plan.asciiOnlyProbe.thirdAt {
		t.Fatalf("structured ASCII probe = %+v long=%t", plan.asciiOnlyProbe, plan.asciiOnlyLong)
	}

	for _, offset := range []int{0, 1, 55, 56, 62, 63, 64, 65, 119, 120, 127, 128, 191, 255} {
		haystack := strings.Repeat("x", offset) + "[KeYs[i%" + strings.Repeat("x", 193)
		match, ok := plan.find(haystack)
		if !ok || match != (Match{Pattern: 0, Start: offset}) {
			t.Fatalf("ASCII offset %d: find=%+v ok=%t", offset, match, ok)
		}
	}

	// A light pair survivor must be fully confirmed before it can hide a later
	// match, and width-changing fold forms must take the Unicode fallback.
	falseAt := 63
	haystack := strings.Repeat("x", falseAt) + "[xxxxxx%" + strings.Repeat("x", 64) + needle
	match, ok := plan.find(haystack)
	want := falseAt + len("[xxxxxx%") + 64
	if !ok || match != (Match{Pattern: 0, Start: want}) {
		t.Fatalf("false structured pair: find=%+v ok=%t want=%d", match, ok, want)
	}
	for _, spelling := range []string{"[\u212Aeys[i%", "[key\u017f[i%"} {
		haystack := strings.Repeat("x", 130) + spelling + strings.Repeat("x", 193)
		match, ok := plan.find(haystack)
		if !ok || match != (Match{Pattern: 0, Start: 130}) {
			t.Fatalf("Unicode spelling %q: find=%+v ok=%t", spelling, match, ok)
		}
	}
}

func TestPairShuftiSkip(t *testing.T) {
	plan := newSearchPlan([]string{"Kелвин0", "ſекрет1", "Σигма2", "ςигма3", "щупальце4"})
	filter := &plan.filter
	if !filter.shufti.usable() {
		t.Fatalf("hazard roots did not compile the two-group pair filter: %+v", filter)
	}

	// The table projection is exact for its expanded raw-pair union. Exhaust
	// every two-byte input so case-expanded ASCII roots and UTF-8 prefixes
	// cannot silently lose a lane before the normal plan transition sees it.
	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			input := string([]byte{byte(first), byte(second)})
			want := filterSkipScalar(input, 0, filter) == 0
			if got := pairShuftiAt(byte(first), byte(second), &filter.shufti); got != want {
				t.Fatalf("pair %02x %02x: shufti=%t want %t", first, second, got, want)
			}
		}
	}

	// Exercise vector block boundaries and the scalar tail against the original
	// complete filter. The raw spellings cover both ASCII-folded and UTF-8 roots.
	pairs := []string{
		"k\xd0", "K\xd0", "s\xd0", "S\xd0", "\xe2\x84", "\xc5\xbf",
		"\xce\xa3", "\xcf\x82", "\xcf\x83", "\xd1\x89", "\xd0\xa9",
	}
	for _, offset := range []int{0, 1, 62, 63, 64, 65, 126, 127, 128, 191} {
		for _, pair := range pairs {
			haystack := strings.Repeat("x", offset) + pair + strings.Repeat("x", 193)
			got := pairShuftiSkipBytes(haystack, 0, filter)
			want := filterSkipScalar(haystack, 0, filter)
			if got != want {
				t.Fatalf("offset %d pair % x: shufti skip=%d want %d", offset, pair, got, want)
			}
		}
	}

	// A nontrivial byte stream checks that every stop returned by the vector
	// loop agrees with the scalar complete filter, including starts after a
	// failed candidate.
	bytes := make([]byte, 513)
	state := uint32(1)
	for i := range bytes {
		state = state*1664525 + 1013904223
		bytes[i] = byte(state >> 24)
	}
	haystack := string(bytes)
	for at := range haystack {
		got := pairShuftiSkipBytes(haystack, at, filter)
		want := filterSkipScalar(haystack, at, filter)
		if got != want {
			t.Fatalf("random at %d: shufti skip=%d want %d", at, got, want)
		}
	}
}

func TestPairShuftiWithOneRoots(t *testing.T) {
	patterns := []string{"щупальце", "kelvin", "zygomorphic", "ſecret", "Zq9xW", "grofse", "ΤΈΛΟΣ", "watchdog"}
	plan := newSearchPlan(patterns)
	filter := &plan.filter
	if filter.shufti.oneN != 2 || !filter.shufti.usable() {
		t.Fatalf("mixed shufti filter = %+v", filter.shufti)
	}

	// This normalized projection may stop early, but it must never skip any
	// byte where the complete root filter would stop. Exercise the AVX-512 loop
	// at every alignment with both one-byte and pair-root spellings.
	stream := strings.Repeat("x", 63) + "z" + strings.Repeat("x", 63) + "Z" +
		strings.Repeat("x", 63) + "\xd0\xa9" + strings.Repeat("x", 63) + "KE" +
		strings.Repeat("x", 63) + "\xe2\x84" + strings.Repeat("x", 193)
	for at := range stream {
		got := pairShuftiSkipBytes(stream, at, filter)
		want := filterSkipScalar(stream, at, filter)
		if got > want {
			t.Fatalf("at %d: shufti skip=%d skips complete-filter stop %d", at, got, want)
		}
	}

	for first := 0; first < 256; first++ {
		for second := 0; second < 256; second++ {
			input := string([]byte{byte(first), byte(second)})
			if filterSkipScalar(input, 0, filter) == 0 &&
				!pairShuftiAt(byte(first), byte(second), &filter.shufti) {
				t.Fatalf("pair %02x %02x lost from normalized projection", first, second)
			}
		}
	}
}

func TestHitTripleFilter(t *testing.T) {
	plan := newSearchPlan([]string{
		"fatal panic", "segfault detected", "oom killed", "disk full",
		"Payment Declined", "quota exceeded", "handshake failed", "watchdog fired",
	})
	t.Logf("triples=%+v roots=%v fallback=%+v", plan.triples, plan.tripleRoots, plan.filter)
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

func TestPairSkipASCII(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"all pairless", strings.Repeat("x", 257), 256},
		{"case-fold pair", strings.Repeat("x", 130) + "ZQ" + strings.Repeat("x", 130), 130},
		{"root without continuation", strings.Repeat("x", 130) + "Z!" + strings.Repeat("x", 130), 261},
		{"non-ascii stop", strings.Repeat("x", 130) + "K" + strings.Repeat("x", 130), 129},
		{"pair across vector boundary", strings.Repeat("x", 63) + "zq" + strings.Repeat("x", 130), 63},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pairSkipASCII(tc.input, 0, rootASCIIFold, 'z', rootASCIIFold, 'q'); got != tc.want {
				t.Fatalf("pairSkipASCII(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
