package arena_test

// An entrant that does not answer the same question is not a competitor, it is
// a faster program solving an easier problem. PCRE2 claims simple-fold caseless
// UTF-8 semantics and leftmost/lowest-ID ties; this proves it on the hazards
// that actually separate fold implementations, and refuses it as an entrant if
// it disagrees.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tsenart/casei"
	"github.com/tsenart/casei/arena"
)

func TestPCRE2AgreesWithOracle(t *testing.T) {
	if !arena.PCRE2Available {
		t.Skip("built without -tags pcre2; the UTF-8 tier has no entrant in this build")
	}

	cases := []struct{ hay, needle string }{
		// The fold hazards. Kelvin sign and long s change UTF-8 width under
		// folding, which is where naive implementations diverge.
		{"temperature 300K today", "300k"},
		{"the ſilent letter", "silent"},
		{"Σigma and ςigma", "σigma"},
		{"ς", "Σ"},
		{"STRASSE", "strasse"},
		{"K", "k"},
		{"K", "K"},
		{"ſ", "S"},
		// Plain and negative cases.
		{"Hello, World", "world"},
		{"hello", "HELLO"},
		{"abcdef", "xyz"},
		{"", "a"},
		{"aaaa", "aaaa"},
		{"абвГД", "гд"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%q_in_%q", c.needle, c.hay), func(t *testing.T) {
			want := casei.IndexFold(c.hay, c.needle)
			p, err := arena.NewPCRE2([]string{c.needle})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			defer p.Close()
			if got := p.FirstIndex(c.hay); got != want {
				t.Errorf("pcre2 = %d, oracle = %d", got, want)
			}
		})
	}
}

func TestPCRE2MultiLeftmostLowestID(t *testing.T) {
	if !arena.PCRE2Available {
		t.Skip("built without -tags pcre2; the UTF-8 tier has no entrant in this build")
	}
	// Both patterns match at the same offset. The arena rule is lowest pattern
	// ID wins, and PCRE2 alternation order must reproduce it.
	hay := "the ABCDEF end"
	patterns := []string{"abc", "abcdef"}
	p, err := arena.NewPCRE2(patterns)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer p.Close()

	m := casei.NewMatcher(patterns)
	want, wantFound := m.Find(hay)
	if !wantFound {
		t.Fatalf("oracle found nothing in %q", hay)
	}
	if got := p.FirstIndex(hay); got != want.Start {
		t.Errorf("pcre2 offset = %d, oracle = %d (pattern %d)", got, want.Start, want.Pattern)
	}
}

func TestPCRE2JITEngaged(t *testing.T) {
	if !arena.PCRE2Available {
		t.Skip("built without -tags pcre2")
	}
	// An interpreted PCRE2 would understate the field and flatter casei, so a
	// silent JIT failure has to be a test failure rather than a slow baseline.
	p, err := arena.NewPCRE2([]string{"needle", "haystack"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	defer p.Close()
	if !p.JIT() {
		t.Error("PCRE2 JIT did not engage; this baseline would understate the field")
	}
}

func TestPCRE2QuotesMetacharacters(t *testing.T) {
	if !arena.PCRE2Available {
		t.Skip("built without -tags pcre2")
	}
	// A needle is a literal. If quoting leaked, these would match everywhere.
	for _, needle := range []string{".*", "a|b", "(x)", `\E`, `a\Eb`, "[a-z]+"} {
		t.Run(needle, func(t *testing.T) {
			hay := "harmless " + needle + " text"
			want := casei.IndexFold(hay, needle)
			p, err := arena.NewPCRE2([]string{needle})
			if err != nil {
				t.Fatalf("compile %q: %v", needle, err)
			}
			defer p.Close()
			if got := p.FirstIndex(hay); got != want {
				t.Errorf("pcre2 = %d, oracle = %d (quoting leaked?)", got, want)
			}
			if got := p.FirstIndex(strings.Repeat("z", 64)); got != -1 {
				t.Errorf("pcre2 matched %q in an unrelated subject at %d", needle, got)
			}
		})
	}
}
