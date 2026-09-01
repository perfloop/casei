package casei

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRawByteTokenPlanDirectTransitions(t *testing.T) {
	plan := newSearchPlan(rawByteCyrillicPatterns)
	if !plan.hasRawByteTokenPlan() {
		t.Fatal("eligible Cyrillic plan did not retain a compact raw-byte map")
	}
	for state := range plan.nodes {
		for value := range utf8.RuneSelf {
			haystack := string([]byte{byte(value)})
			got, size, ok := plan.rawByteAdvance(haystack, 0, state)
			wantToken, wantSize := plan.haystackToken(haystack, 0)
			want := plan.advance(state, wantToken)
			if !ok || size != wantSize || got != want {
				t.Fatalf("state %d ASCII %02x: raw=(%d,%d,%t), decoded=(%d,%d)",
					state, value, got, size, ok, want, wantSize)
			}
		}
		for lead := 0xc2; lead <= 0xdf; lead++ {
			for trail := 0x80; trail <= 0xbf; trail++ {
				haystack := string([]byte{byte(lead), byte(trail)})
				got, size, ok := plan.rawByteAdvance(haystack, 0, state)
				wantToken, wantSize := plan.haystackToken(haystack, 0)
				want := plan.advance(state, wantToken)
				if !ok || size != wantSize || got != want {
					t.Fatalf("state %d UTF-8 %02x%02x: raw=(%d,%d,%t), decoded=(%d,%d)",
						state, lead, trail, got, size, ok, want, wantSize)
				}
			}
		}
	}

	for _, haystack := range []string{"\x80", "\xc2x", "€", "K"} {
		if _, _, ok := plan.rawByteAdvance(haystack, 0, 0); ok {
			t.Fatalf("unsupported input %x entered the raw-byte map", haystack)
		}
	}
}

// rawByteFilteredDirectTransitions mirrors the non-skipped portion of
// findFiltered for a no-match stream. It verifies that the public filtered
// route reaches the compact raw map rather than merely compiling it.
func rawByteFilteredDirectTransitions(t *testing.T, plan *searchPlan, haystack string) int {
	t.Helper()
	if !plan.hasRawByteTokenPlan() {
		t.Fatal("plan has no compact raw-byte map")
	}

	state, transitions := 0, 0
	for at := 0; at < len(haystack); {
		if state == 0 {
			skipped := len(haystack) - at
			if plan.pairSecond {
				skipped = pairSecondSkipBytes(haystack, at, &plan.filter)
			} else if plan.filter.usable() {
				skipped = filterSkipBytes(haystack, at, &plan.filter)
			}
			if plan.triples.usable() {
				if tripleSkipped := tripleSkipBytes(haystack, at, &plan.triples); tripleSkipped < skipped {
					skipped = tripleSkipped
				}
			}
			at += skipped
			if at == len(haystack) {
				break
			}
		}

		rawState, rawSize, rawOK := plan.rawByteAdvance(haystack, at, state)
		token, decodedSize := plan.haystackToken(haystack, at)
		decodedState := plan.advance(state, token)
		if !rawOK || rawSize != decodedSize || rawState != decodedState {
			t.Fatalf("transition at byte %d: raw=(state=%d,size=%d,ok=%t), decoded=(state=%d,size=%d)",
				at, rawState, rawSize, rawOK, decodedState, decodedSize)
		}
		state = rawState
		transitions++
		at += rawSize
	}
	return transitions
}

func rawByteSharedPrefixPatterns(n int) []string {
	patterns := make([]string, n)
	for i := range patterns {
		patterns[i] = "щупальце" + string(rune('0'+i))
	}
	return patterns
}

func TestRawByteMultiAnchorRequiresTagDiversity(t *testing.T) {
	for _, n := range []int{2, 4, 8} {
		patterns := rawByteSharedPrefixPatterns(n)
		plan := newSearchPlan(patterns)
		if !plan.hasRawByteTokenPlan() {
			t.Fatalf("N=%d shared-prefix plan lost the compact raw-byte map", n)
		}
		if plan.rawByteMulti.usable() {
			t.Fatalf("N=%d indistinguishable shared-prefix anchors entered rawByteMulti", n)
		}
		matcher := NewMatcher(patterns)
		for _, haystack := range []string{
			strings.Repeat("щ", 63) + "упальцеx",
			"ᲇупальце7", // width-changing fold spelling before the shared suffix.
			strings.Repeat("x", 71) + "\xff" + patterns[n-1],
		} {
			got, gotOK := matcher.Find(haystack)
			want, wantOK := refFind(haystack, patterns)
			if gotOK != wantOK || gotOK && got != want {
				t.Fatalf("N=%d Find(%x) = %+v,%t; want %+v,%t", n, haystack, got, gotOK, want, wantOK)
			}
		}
	}

	diverse := []string{
		"абвгде0", "ёжзийк1", "лмнопр2", "стуфхц3",
		"чшщьыъ4", "ыьэюяа5", "бвгдеж6", "зийклм7",
	}
	for _, patterns := range [][]string{rawByteCyrillicPatterns, diverse} {
		plan := newSearchPlan(patterns)
		if !plan.rawByteMulti.usable() || !plan.rawByteMulti.tagDiverse() {
			t.Fatalf("diverse anchors were not admitted: patterns=%q usable=%t diverse=%t", patterns, plan.rawByteMulti.usable(), plan.rawByteMulti.tagDiverse())
		}
	}
}

func TestRawByteFilteredDensityUsesDirectTransitions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		patterns []string
		period   int
		groups   int
	}{
		{"two_one_in_32", rawByteCyrillicPatterns[:2], 32, 4096 / 32},
		{"five_one_in_256", rawByteCyrillicPatterns, 256, rawBytePublicationCorpusBytes / 256},
		{"five_one_in_4", rawByteCyrillicPatterns, 4, rawByteBenchmarkCorpusBytes / 16 / 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			haystack := rawByteFalseCandidates(tc.period, tc.groups)
			plan := newSearchPlan(tc.patterns)
			if plan.unicodePairN != 0 || plan.unicodeAnchor.n != 0 || !plan.filter.usable() {
				t.Fatalf("fixture does not select generic filtered search: pairs=%d anchor=%d filter=%t",
					plan.unicodePairN, plan.unicodeAnchor.n, plan.filter.usable())
			}
			if got := rawByteFilteredDirectTransitions(t, plan, haystack); got != 2*tc.groups {
				t.Fatalf("direct non-skipped transitions = %d, want %d", got, 2*tc.groups)
			}

			matcher := NewMatcher(tc.patterns)
			if got, ok := matcher.Find(haystack); ok || got != (Match{}) {
				t.Fatalf("Find = %+v,%t, want no match", got, ok)
			}
			if !matcher.plan.hasRawByteTokenPlan() {
				t.Fatal("public Matcher.Find did not retain the compact raw-byte map")
			}
		})
	}
}

func TestRawByteTokenPlanPreservesFallbackOffsetsAndTies(t *testing.T) {
	patterns := append(append([]string(nil), rawByteCyrillicPatterns...), "ДЖОН УОТСОН")
	matcher := NewMatcher(patterns)
	if !matcher.plan.hasRawByteTokenPlan() {
		t.Fatal("eligible multi-pattern plan did not retain a raw-byte map")
	}

	for _, haystack := range []string{
		"ᲁЖОН УОТСОН", // U+1C81 is a three-byte simple-fold spelling of Д.
		strings.Repeat("x", 31) + "\xff" + strings.Repeat("x", 17) + "ДЖОН УОТСОН",
		strings.Repeat("x", 29) + "K" + strings.Repeat("x", 11) + "ШЕРЛОК ХОЛМС",
		strings.Repeat("x", 23) + "ДЖОН УОТСОН",
	} {
		got, gotOK := matcher.Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("Find(%x) = %+v,%t; want %+v,%t", haystack, got, gotOK, want, wantOK)
		}
	}

	rng := rand.New(rand.NewPCG(20260824, 1))
	units := []string{"x", " ", "Д", "д", "ᲁ", "Ж", "ж", "Ш", "ш", "K", "ſ", "\xff", "\x80", "€"}
	for iteration := 0; iteration < 1000; iteration++ {
		var haystack strings.Builder
		for range 96 {
			haystack.WriteString(units[rng.IntN(len(units))])
		}
		if iteration%3 == 0 {
			haystack.WriteString(patterns[rng.IntN(len(rawByteCyrillicPatterns))])
		}
		input := haystack.String()
		got, gotOK := matcher.Find(input)
		want, wantOK := refFind(input, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("iteration %d Find(%x) = %+v,%t; want %+v,%t", iteration, input, got, gotOK, want, wantOK)
		}
	}
}

func TestRawByteOriginDoesNotEnterASCIIOnlyPartition(t *testing.T) {
	plan := newSearchPlan(rawByteCyrillicPatterns)
	if plan.patternCount != len(rawByteCyrillicPatterns) || plan.asciiOnly {
		t.Fatalf("raw transition plan changed ASCII-only admission: patterns=%d asciiOnly=%t", plan.patternCount, plan.asciiOnly)
	}
	if plan.asciiOnlyPartitionUsable() {
		t.Fatal("raw transition plan entered the N=1 ASCII-only partition route")
	}
	if !plan.rawByteMulti.usable() || !plan.rawByteOrigin.usable() {
		t.Fatal("eligible plan did not compile the tagged filter and origin gate")
	}
}

func TestRawByteOriginGatePreservesFind(t *testing.T) {
	plan := newSearchPlan(rawByteCyrillicPatterns)
	if !plan.rawByteMulti.usable() || !plan.rawByteOrigin.usable() {
		t.Fatal("eligible plan did not compile the tagged filter and origin gate")
	}
	matcher := NewMatcher(rawByteCyrillicPatterns)
	check := func(name, haystack string) {
		t.Helper()
		want, wantOK := refFind(haystack, rawByteCyrillicPatterns)
		for _, route := range []struct {
			name string
			find func(string) (Match, bool)
		}{
			{"direct", plan.findRawByteOrigin},
			{"public", matcher.Find},
		} {
			got, gotOK := route.find(haystack)
			if gotOK != wantOK || gotOK && got != want {
				t.Fatalf("%s/%s: Find(%x) = %+v,%t; want %+v,%t", name, route.name, haystack, got, gotOK, want, wantOK)
			}
		}
	}

	check("absent", strings.Repeat("x", 5<<10))
	check("unrelated-earlier-gate", strings.Repeat("x", 97)+" "+strings.Repeat("x", 5<<10)+rawByteCyrillicPatterns[3])
	check("opaque-before-match", strings.Repeat("x", 5<<10)+"\xff"+rawByteCyrillicPatterns[2])
	for alignment := 0; alignment < 64; alignment++ {
		// U+1C81 is a three-byte rendering of the pattern's initial Д. Its
		// varying source width exercises the gate's maximum-prefix lookback.
		check("width-changing-prefix", strings.Repeat("x", 4096+alignment)+"ᲁЖОН УОТСОН")
	}

	if gate := rawByteOriginGateFor([]string{"абв", "где"}); gate.usable() {
		t.Fatalf("patterns with no common fold-invariant ASCII byte compiled gate %+v", gate)
	}
	if gate := rawByteOriginGateFor([]string{"абв ", "где \xff"}); gate.usable() {
		t.Fatalf("malformed pattern compiled gate %+v", gate)
	}
}

type rawByteEachResult struct {
	match Match
	width int
}

// rawByteReferenceEach is deliberately independent from Matcher.Find and the
// compiled transition plan. It reduces the canonical fold reference used by
// the package tests to the non-overlapping enumeration contract.
func rawByteReferenceEach(haystack string, patterns []string) []rawByteEachResult {
	var out []rawByteEachResult
	for at := 0; at <= len(haystack); {
		match, ok := refFind(haystack[at:], patterns)
		if !ok {
			return out
		}
		match.Start += at
		canon, _ := canonFold(patterns[match.Pattern])
		_, offsets := canonFold(haystack[match.Start:])
		width := offsets[len(canon)]
		out = append(out, rawByteEachResult{match, width})
		at = match.Start + width
	}
	return out
}

func rawByteCheckEach(t *testing.T, matcher *Matcher, haystack string) {
	t.Helper()
	want := rawByteReferenceEach(haystack, matcher.patterns)
	var got []rawByteEachResult
	if complete := matcher.Each(haystack, func(match Match, width int) bool {
		got = append(got, rawByteEachResult{match, width})
		return true
	}); !complete {
		t.Fatal("Each stopped before completing enumeration")
	}
	if len(got) != len(want) {
		t.Fatalf("Each(%x) returned %d matches, want %d: got=%+v want=%+v", haystack, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Each(%x) match %d = %+v, want %+v", haystack, i, got[i], want[i])
		}
	}
}

// TestRawByteMultiAnchorFindOrderingAndTail exercises the first-result view of
// the shared tagged scan. It keeps a duplicate fold-equivalent literal so the
// exact replay, rather than tag iteration order, must select the lowest ID.
func TestRawByteMultiAnchorFindOrderingAndTail(t *testing.T) {
	patterns := append(append([]string(nil), rawByteCyrillicPatterns...), strings.ToLower(rawByteCyrillicPatterns[0]))
	matcher := NewMatcher(patterns)
	if !matcher.plan.rawByteMulti.usable() {
		t.Fatal("eligible Find plan did not compile a raw multi-anchor filter")
	}

	check := func(name, haystack string) {
		t.Helper()
		got, gotOK := matcher.Find(haystack)
		want, wantOK := refFind(haystack, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("%s: Find(%x) = %+v,%t; want %+v,%t", name, haystack, got, gotOK, want, wantOK)
		}
	}

	// Shift the final literal through every vector-tail alignment. The prefix
	// is long enough to take the VBMI scan before its scalar tail reaches the
	// candidate; the duplicate pattern checks the lowest-ID tie at that tail.
	for alignment := 0; alignment < 64; alignment++ {
		check("tail", strings.Repeat("x", 128+alignment)+rawByteCyrillicPatterns[0])
	}

	check("earlier-start-wins", rawByteCyrillicPatterns[3]+" x "+rawByteCyrillicPatterns[0])
	check("opaque-before-match", strings.Repeat("x", 91)+"\xff"+rawByteCyrillicPatterns[2])
	// U+1C81 is the three-byte simple-fold spelling of the initial Д. The
	// compiled start offsets may nominate extras, but exact raw-plan replay must
	// still return the same ordinary byte offset as the reference.
	check("width-changing-before-anchor", strings.Repeat("x", 77)+"ᲁжон уотсон")
}

func TestRawByteMultiAnchorEnumeration(t *testing.T) {
	matcher := NewMatcher(rawByteCyrillicPatterns)
	if !matcher.plan.hasRawByteTokenPlan() || !matcher.plan.rawByteMulti.usable() {
		t.Fatalf("eligible plan did not compile raw multi-anchor state: raw=%t multi=%t", matcher.plan.hasRawByteTokenPlan(), matcher.plan.rawByteMulti.usable())
	}

	// Sweep every vector tail alignment. The second occurrence forces a resume
	// after a nonzero width, while the last one makes the selected anchor cross
	// from a vector block into the scalar tail.
	for alignment := 0; alignment < 64; alignment++ {
		haystack := rawByteCyrillicPatterns[0] + strings.Repeat("x", alignment) + rawByteCyrillicPatterns[2]
		rawByteCheckEach(t, matcher, haystack)
	}

	for _, haystack := range []string{
		"ᲁжон уотсон " + rawByteCyrillicPatterns[2], // width-changing Д form before an anchor.
		strings.Repeat("x", 71) + "\xff" + rawByteCyrillicPatterns[3],
		strings.Repeat("ж", 83) + rawByteCyrillicPatterns[4],
		strings.Repeat("x", 19) + rawByteCyrillicPatterns[1] + rawByteCyrillicPatterns[0],
	} {
		rawByteCheckEach(t, matcher, haystack)
	}

	rng := rand.New(rand.NewPCG(20260826, 3))
	units := []string{"x", " ", "Д", "д", "ᲁ", "Ж", "ж", "Ш", "ш", "О", "ᲂ", "о", "Н", "н", "И", "и", "€", "\xff", "\x80"}
	for iteration := 0; iteration < 512; iteration++ {
		var haystack strings.Builder
		for range 96 {
			haystack.WriteString(units[rng.IntN(len(units))])
		}
		for i := 0; i < iteration%5; i++ {
			haystack.WriteString(rawByteCyrillicPatterns[rng.IntN(len(rawByteCyrillicPatterns))])
			haystack.WriteString(" xx ")
		}
		rawByteCheckEach(t, matcher, haystack.String())
	}
}

func TestRawByteMultiAnchorSkipNeverPassesAConfirmedTag(t *testing.T) {
	plan := newSearchPlan(rawByteCyrillicPatterns)
	filter := &plan.rawByteMulti
	if !filter.usable() {
		t.Fatal("eligible plan did not compile a raw multi-anchor filter")
	}
	for alignment := 0; alignment < 64; alignment++ {
		haystack := strings.Repeat("x", alignment+128) + rawByteCyrillicPatterns[alignment%len(rawByteCyrillicPatterns)] + strings.Repeat("x", 96)
		for at := range haystack {
			skipped, _ := rawByteMultiAnchorSkipBytes(haystack, at, filter)
			if skipped < 0 || at+skipped > len(haystack) {
				t.Fatalf("alignment %d at %d: invalid skip %d", alignment, at, skipped)
			}
			for candidate := at; candidate < at+skipped; candidate++ {
				if tags := filter.tagsAt(haystack, candidate, 0xff); tags != 0 {
					t.Fatalf("alignment %d at %d: skip %d passed confirmed tag %08b at %d", alignment, at, skipped, tags, candidate)
				}
			}
		}
	}
}

func TestRawByteMultiAnchorScalarScreenUnionsConfirmationGroups(t *testing.T) {
	pair := func(first, second byte) rawByteMultiAnchorPairSet {
		return rawByteMultiAnchorPairSet{
			pairs: [rawByteMultiAnchorForms]uint16{uint16(first) | uint16(second)<<8},
			n:     1,
		}
	}
	const (
		aliasTag = byte(1 << iota)
		matchTag
	)
	aliasPrimary := pair(0x01, 0x02) // Low six bits alias "AB".
	matchPrimary := pair('A', 'B')
	aliasConfirm := pair('C', 'D')
	matchConfirm := pair('E', 'F')
	guard := pair('G', 'H')
	filter := rawByteMultiAnchorFilter{
		confirmOffset: [rawByteMultiAnchorConfirmGroups]uint8{2, 4},
		confirmN:      2,
		valid:         1,
	}
	rawByteMultiAnchorAddTable(&filter.first, aliasPrimary, false, aliasTag)
	rawByteMultiAnchorAddTable(&filter.second, aliasPrimary, true, aliasTag)
	rawByteMultiAnchorAddTable(&filter.first, matchPrimary, false, matchTag)
	rawByteMultiAnchorAddTable(&filter.second, matchPrimary, true, matchTag)
	rawByteMultiAnchorAddTable(&filter.confirmFirst[0], aliasConfirm, false, aliasTag)
	rawByteMultiAnchorAddTable(&filter.confirmSecond[0], aliasConfirm, true, aliasTag)
	rawByteMultiAnchorAddTable(&filter.confirmFirst[1], matchConfirm, false, matchTag)
	rawByteMultiAnchorAddTable(&filter.confirmSecond[1], matchConfirm, true, matchTag)
	filter.anchors[0] = rawByteMultiAnchor{
		primary:       aliasPrimary,
		confirm:       aliasConfirm,
		guard:         guard,
		starts:        [rawByteMultiAnchorStartOffsets]uint8{0},
		confirmOffset: [rawByteMultiAnchorConfirmGroups]uint8{2},
		guardOffset:   [rawByteMultiAnchorStartOffsets]uint8{6},
		startN:        1,
		confirmN:      1,
		guardN:        1,
	}
	filter.anchors[1] = rawByteMultiAnchor{
		primary:       matchPrimary,
		confirm:       matchConfirm,
		guard:         guard,
		starts:        [rawByteMultiAnchorStartOffsets]uint8{0},
		confirmOffset: [rawByteMultiAnchorConfirmGroups]uint8{4},
		guardOffset:   [rawByteMultiAnchorStartOffsets]uint8{6},
		startN:        1,
		confirmN:      1,
		guardN:        1,
	}

	haystack := "ABCDEFGH"
	skipped, candidates := rawByteMultiAnchorSkipScalar(haystack, 0, &filter)
	if skipped != 0 || candidates != aliasTag|matchTag {
		t.Fatalf("scalar screen = (%d, %08b), want (0, %08b)", skipped, candidates, aliasTag|matchTag)
	}
	if tags := filter.tagsAt(haystack, 0, candidates); tags != matchTag {
		t.Fatalf("exact tags = %08b, want %08b", tags, matchTag)
	}
}

// TestRawByteMultiAnchorScalarScreenNeverPassesConfirmedTag proves the table
// screen used after a vector tail (and on portable hosts) can stop early on an
// alias but never skips an exact tagged anchor. The later tagsAt replay remains
// the match authority.
func TestRawByteMultiAnchorScalarScreenNeverPassesConfirmedTag(t *testing.T) {
	plan := newSearchPlan(rawByteCyrillicPatterns)
	filter := &plan.rawByteMulti
	if !filter.usable() {
		t.Fatal("eligible plan did not compile a raw multi-anchor filter")
	}
	inputs := []string{
		strings.Repeat("x", 257) + rawByteCyrillicPatterns[0],
		strings.Repeat("x", 63) + "\xff\x80" + rawByteCyrillicPatterns[1],
		strings.Repeat("x", 17) + "ᲁжон уотсон",
	}
	rng := rand.New(rand.NewPCG(20260907, 8))
	for i := 0; i < 256; i++ {
		buf := make([]byte, i)
		for j := range buf {
			buf[j] = byte(rng.Uint32())
		}
		inputs = append(inputs, string(buf))
	}
	for inputIndex, haystack := range inputs {
		for at := range haystack {
			skipped, _ := rawByteMultiAnchorSkipScalar(haystack, at, filter)
			if skipped < 0 || at+skipped > len(haystack) {
				t.Fatalf("input %d at %d: invalid scalar skip %d", inputIndex, at, skipped)
			}
			for candidate := at; candidate < at+skipped; candidate++ {
				if tags := filter.tagsAt(haystack, candidate, 0xff); tags != 0 {
					t.Fatalf("input %d at %d: scalar skip %d passed confirmed tag %08b at %d", inputIndex, at, skipped, tags, candidate)
				}
			}
		}
	}
}

func TestRawByteTokenPlanFallsBackForUnsupportedPlans(t *testing.T) {
	for _, patterns := range [][]string{
		{"Шерлок"},
		{"Шерлок", "Δelta", "éclair"},
		{"\xffШерлок", "Джон"},
	} {
		matcher := NewMatcher(patterns)
		if matcher.plan.hasRawByteTokenPlan() {
			t.Fatalf("unsupported plan %q retained raw-byte tokens", patterns)
		}
		input := "… δELTA … ÉCLAIR … ШЕРЛОК \xffДЖОН"
		got, gotOK := matcher.Find(input)
		want, wantOK := refFind(input, patterns)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("Find(%q, %x) = %+v,%t; want %+v,%t", patterns, input, got, gotOK, want, wantOK)
		}
	}
}

func TestRawByteTokenPlanFindAllocatesNothingAfterConstruction(t *testing.T) {
	matcher := NewMatcher(rawByteCyrillicPatterns)
	haystack := rawByteFalseCandidatesAtLeast(64, 64<<10)
	if got, ok := matcher.Find(haystack); ok || got != (Match{}) {
		t.Fatalf("setup Find = %+v,%t", got, ok)
	}
	if allocs := testing.AllocsPerRun(100, func() { _, _ = matcher.Find(haystack) }); allocs != 0 {
		t.Fatalf("reused raw-byte Find allocations = %g, want 0", allocs)
	}
}
