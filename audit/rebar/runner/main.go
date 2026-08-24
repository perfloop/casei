package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tsenart/casei"
)

type config struct {
	name            string
	model           string
	patterns        []string
	haystack        string
	caseInsensitive bool
	maxIters        uint64
	maxWarmupIters  uint64
	maxTime         time.Duration
	maxWarmupTime   time.Duration
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("casei-rebar 1")
		return nil
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	c, err := readConfig(raw)
	if err != nil {
		return err
	}
	if !c.caseInsensitive {
		return errors.New("casei runner only accepts case-insensitive benchmarks")
	}
	if c.model != "count" && c.model != "count-spans" {
		return fmt.Errorf("unsupported model %q", c.model)
	}
	patterns, err := literalAlternation(c.patterns)
	if err != nil {
		return err
	}
	matcher := casei.NewMatcher(patterns)
	spans := c.model == "count-spans"
	if _, err := verifyEnumeration(c.haystack, patterns, matcher, spans); err != nil {
		return err
	}
	bench := func() int {
		return countMatches(c.haystack, matcher, spans)
	}

	warmupStart := time.Now()
	for i := uint64(0); i < c.maxWarmupIters; i++ {
		_ = bench()
		if time.Since(warmupStart) >= c.maxWarmupTime {
			break
		}
	}

	out := bufio.NewWriter(os.Stdout)
	runStart := time.Now()
	for i := uint64(0); i < c.maxIters; i++ {
		start := time.Now()
		count := bench()
		duration := time.Since(start)
		fmt.Fprintf(out, "%d,%d\n", duration.Nanoseconds(), count)
		if time.Since(runStart) >= c.maxTime {
			break
		}
	}
	return out.Flush()
}

// countMatches is the timed Rebar operation: one compiled Matcher enumeration
// and only the count/span sink. verifyEnumeration runs the independent oracle
// once before the runner starts its warm-up and measurement loops.
func countMatches(haystack string, matcher *casei.Matcher, spans bool) int {
	total := 0
	matcher.Each(haystack, func(_ casei.Match, width int) bool {
		if spans {
			total += width
		} else {
			total++
		}
		return true
	})
	return total
}

// foldPrefixWidth is deliberately independent from Matcher.Each. The Rebar
// preflight uses it to verify every source width and match boundary.
func foldPrefixWidth(haystack, pattern string) (int, bool) {
	consumed := 0
	for len(pattern) > 0 {
		pr, pn := utf8.DecodeRuneInString(pattern)
		if len(haystack) == consumed {
			return 0, false
		}
		hr, hn := utf8.DecodeRuneInString(haystack[consumed:])
		if pr == utf8.RuneError && pn == 1 {
			if hr != utf8.RuneError || hn != 1 || pattern[0] != haystack[consumed] {
				return 0, false
			}
			pattern = pattern[1:]
			consumed++
			continue
		}
		if hr == utf8.RuneError && hn == 1 || !foldEqual(pr, hr) {
			return 0, false
		}
		pattern = pattern[pn:]
		consumed += hn
	}
	return consumed, true
}

func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

// nextMatch is a package-independent, source-boundary oracle for the next
// non-empty literal occurrence. At one source offset, the first matching
// pattern index wins; offsets are then considered left to right.
func nextMatch(haystack string, patterns []string, from int) (casei.Match, int, bool) {
	for at := from; at <= len(haystack); {
		for pattern, literal := range patterns {
			if width, ok := foldPrefixWidth(haystack[at:], literal); ok && width != 0 {
				return casei.Match{Pattern: pattern, Start: at}, width, true
			}
		}
		if at == len(haystack) {
			break
		}
		_, size := utf8.DecodeRuneInString(haystack[at:])
		at += size
	}
	return casei.Match{}, 0, false
}

// verifyEnumeration compares every result to the canonical source scan before
// timing begins. It validates Pattern bounds, leftmost/lowest-ID order, exact
// source width, and the non-overlapping resume point without using Matcher.Find.
func verifyEnumeration(haystack string, patterns []string, matcher *casei.Matcher, spans bool) (int, error) {
	at, total := 0, 0
	var verifyErr error
	complete := matcher.Each(haystack, func(match casei.Match, width int) bool {
		if match.Pattern < 0 || match.Pattern >= len(patterns) || match.Start < at || match.Start > len(haystack) || width <= 0 {
			verifyErr = fmt.Errorf("casei returned invalid match %+v with width %d", match, width)
			return false
		}
		want, expectedWidth, ok := nextMatch(haystack, patterns, at)
		if !ok || match != want || width != expectedWidth {
			verifyErr = fmt.Errorf("casei returned match %+v with width %d; canonical next match is %+v with width %d, ok=%t", match, width, want, expectedWidth, ok)
			return false
		}
		if spans {
			total += width
		} else {
			total++
		}
		at = match.Start + width
		return true
	})
	if verifyErr != nil {
		return 0, verifyErr
	}
	if !complete {
		return 0, errors.New("casei enumeration stopped during preflight")
	}
	if want, width, ok := nextMatch(haystack, patterns, at); ok {
		return 0, fmt.Errorf("casei omitted canonical match %+v with width %d", want, width)
	}
	return total, nil
}

func literalAlternation(raw []string) ([]string, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("expected one rebar regex, got %d", len(raw))
	}
	if strings.ContainsAny(raw[0], `\\[](){}.*+?^$`) {
		return nil, fmt.Errorf("not a literal alternation: %q", raw[0])
	}
	patterns := strings.Split(raw[0], "|")
	for _, pattern := range patterns {
		if pattern == "" {
			return nil, errors.New("empty alternation is unsupported")
		}
	}
	return patterns, nil
}

func readConfig(raw []byte) (config, error) {
	var c config
	for len(raw) > 0 {
		keyEnd := bytes.IndexByte(raw, ':')
		if keyEnd < 0 {
			return c, errors.New("invalid KLV key")
		}
		lengthEnd := bytes.IndexByte(raw[keyEnd+1:], ':')
		if lengthEnd < 0 {
			return c, errors.New("invalid KLV length")
		}
		lengthEnd += keyEnd + 1
		n, err := strconv.Atoi(string(raw[keyEnd+1 : lengthEnd]))
		if err != nil || n < 0 || lengthEnd+1+n >= len(raw) || raw[lengthEnd+1+n] != '\n' {
			return c, errors.New("invalid KLV value")
		}
		key := string(raw[:keyEnd])
		value := string(raw[lengthEnd+1 : lengthEnd+1+n])
		raw = raw[lengthEnd+1+n+1:]
		switch key {
		case "name":
			c.name = value
		case "model":
			c.model = value
		case "pattern":
			c.patterns = append(c.patterns, value)
		case "haystack":
			c.haystack = value
		case "case-insensitive":
			c.caseInsensitive, err = strconv.ParseBool(value)
		case "unicode":
			// casei always applies Unicode simple folding. The audit separately
			// records which rebar rows request ASCII-only case insensitivity.
		case "max-iters":
			c.maxIters, err = strconv.ParseUint(value, 10, 64)
		case "max-warmup-iters":
			c.maxWarmupIters, err = strconv.ParseUint(value, 10, 64)
		case "max-time":
			var nanos uint64
			nanos, err = strconv.ParseUint(value, 10, 64)
			c.maxTime = time.Duration(nanos)
		case "max-warmup-time":
			var nanos uint64
			nanos, err = strconv.ParseUint(value, 10, 64)
			c.maxWarmupTime = time.Duration(nanos)
		default:
			return c, fmt.Errorf("unrecognized KLV key %q", key)
		}
		if err != nil {
			return c, fmt.Errorf("invalid %s: %w", key, err)
		}
	}
	return c, nil
}
