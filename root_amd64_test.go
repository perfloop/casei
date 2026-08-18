//go:build amd64

package casei

import (
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func TestPairSetSkip(t *testing.T) {
	filter := rootFilter{
		pairs: [16]rootPair{
			{first: 0xce, second: 0xa3},
			{first: 0xd0, second: 0xaf},
		},
		pairN: 2,
	}
	for _, offset := range []int{0, 1, 31, 32, 63, 64, 65, 127, 128, 191} {
		stream := strings.Repeat("x", offset) + "Σ" + strings.Repeat("x", 193)
		got := pairSetSkipBytes(stream, 0, &filter)
		want := filterSkipScalar(stream, 0, &filter)
		if got != want || got != offset {
			t.Fatalf("offset %d: pair-set skip=%d want %d", offset, got, want)
		}
	}
	miss := strings.Repeat("x", 257)
	if got, want := pairSetSkipBytes(miss, 0, &filter), filterSkipScalar(miss, 0, &filter); got != want {
		t.Fatalf("pair-set miss=%d want %d", got, want)
	}
}

func TestASCIIPairShortSkip64TailPosition(t *testing.T) {
	if !cpu.X86.HasAVX512F || !cpu.X86.HasAVX512BW {
		t.Skip("AVX-512 BW pair path is disabled")
	}

	plan := newSearchPlan([]string{"fatal panic"})
	probe := &plan.asciiPair
	if !probe.usable() {
		t.Fatal("fixed long ASCII literal did not compile an aligned pair probe")
	}

	for _, prefix := range []int{0, 128} {
		candidates := prefix + 64
		for lane := 0; lane < 64; lane++ {
			want := prefix + lane
			input := []byte(strings.Repeat("x", candidates+64))
			input[want] = probe.first
			input[want+probe.secondAt] = probe.second
			haystack := string(input)

			if got := asciiPairShortSkip64(unsafe.StringData(haystack), candidates, probe); got != want {
				t.Fatalf("prefix %d lane %d: direct skip = %d, want %d", prefix, lane, got, want)
			}
			if got := asciiPairSkipBytes(haystack, 0, candidates, probe); got != want {
				t.Fatalf("prefix %d lane %d: wrapped skip = %d, want %d", prefix, lane, got, want)
			}
		}
	}
}
