package casei

import "testing"

// TestEachHarnessContract fixes the public enumeration reduction that the
// Rebar adapter times. The optimized implementation must preserve this
// repeated-Find baseline's non-overlap order and source widths.
func TestEachHarnessContract(t *testing.T) {
	matcher := NewMatcher([]string{"ss", "s"})
	want := []struct {
		match Match
		width int
	}{
		{Match{Pattern: 0, Start: 0}, len("SS")},
		{Match{Pattern: 0, Start: len("SS")}, len("ſs")},
	}
	var got []struct {
		match Match
		width int
	}
	if complete := matcher.Each("SSſs", func(match Match, width int) bool {
		got = append(got, struct {
			match Match
			width int
		}{match, width})
		return true
	}); !complete {
		t.Fatal("Each stopped before completing enumeration")
	}
	if len(got) != len(want) {
		t.Fatalf("Each returned %d matches, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("match %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
