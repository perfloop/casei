package main

import (
	"testing"

	"github.com/tsenart/casei"
)

func TestLiteralAlternation(t *testing.T) {
	got, err := literalAlternation([]string{"Sherlock|Holmes|Шерлок"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Sherlock", "Holmes", "Шерлок"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	for _, regex := range []string{"Sher[a-z]+", "a||b", "a\\|b"} {
		if _, err := literalAlternation([]string{regex}); err == nil {
			t.Fatalf("accepted non-literal alternation %q", regex)
		}
	}
}

func TestEachWidth(t *testing.T) {
	for _, tt := range []struct {
		haystack string
		pattern  string
		width    int
		ok       bool
	}{
		{"Kelvin", "k", 3, true},
		{"ſuffix", "S", 2, true},
		{"ς", "Σ", 2, true},
		{"шЕРЛОК хОЛМС", "Шерлок Холмс", len("шЕРЛОК хОЛМС"), true},
		{"Ёлка", "ёлка", len("Ёлка"), true},
		{"x", "s", 0, false},
		{string([]byte{0xff}), string([]byte{0xff}), 1, true},
		{string([]byte{0xfe}), string([]byte{0xff}), 0, false},
	} {
		expected, expectedOK := foldPrefixWidth(tt.haystack, tt.pattern)
		if expected != tt.width || expectedOK != tt.ok {
			t.Fatalf("foldPrefixWidth(%q, %q) = (%d, %v), want (%d, %v)",
				tt.haystack, tt.pattern, expected, expectedOK, tt.width, tt.ok)
		}
		got, ok := 0, false
		casei.NewMatcher([]string{tt.pattern}).Each(tt.haystack, func(_ casei.Match, width int) bool {
			got, ok = width, true
			return false
		})
		if got != expected || ok != expectedOK {
			t.Errorf("Each(%q, %q) = (%d, %v), want independently verified (%d, %v)",
				tt.haystack, tt.pattern, got, ok, expected, expectedOK)
		}
	}
}

func TestFindMatch(t *testing.T) {
	matcher := casei.NewMatcher([]string{"needle"})
	if got := findMatch("a needle", matcher); got != 1 {
		t.Fatalf("findMatch(hit) = %d, want 1", got)
	}
	if got := findMatch("a haystack", matcher); got != 0 {
		t.Fatalf("findMatch(miss) = %d, want 0", got)
	}
}

func TestCountMatches(t *testing.T) {
	patterns := []string{"ss", "s"}
	matcher := casei.NewMatcher(patterns)
	for _, tt := range []struct {
		spans bool
		want  int
	}{
		{false, 2},
		{true, 5},
	} {
		if got, err := verifyEnumeration("SSſs", patterns, matcher, tt.spans); err != nil || got != tt.want {
			t.Fatalf("verifyEnumeration(spans=%v) = %d, %v; want %d, nil", tt.spans, got, err, tt.want)
		}
		got := countMatches("SSſs", matcher, tt.spans)
		if got != tt.want {
			t.Fatalf("countMatches(spans=%v) = %d, want %d", tt.spans, got, tt.want)
		}
	}
}
