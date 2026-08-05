package arena_test

// The competitive bar, expressed as a measurable quantity.
//
// The rest of the suite measures each implementation on its own. That is not
// the goal of this repository: the goal is to beat the field. BenchmarkBar
// therefore reports, for every scenario, the candidate's time divided by the
// time of the STRONGEST OTHER implementation available on this machine —
// including SIMD engines. The metric is `x_vs_best`, lower is better:
//
//	x_vs_best > 1  the candidate loses to something that already exists
//	x_vs_best = 1  parity with the best existing implementation
//	x_vs_best < 1  the candidate is the fastest thing here
//
// A run of this benchmark is the honest scoreboard. Driving `x_vs_best`
// below 1 on every row is the objective, and on the ASCII single-needle
// rows that is not reachable without exploiting the machine's SIMD
// instruction set: the strongest baseline there is a hand-written NEON/AVX2
// kernel. Scalar code cannot close that gap, so this metric is what makes
// instruction-level work a requirement rather than a preference.

import (
	"testing"
	"time"

	veloz "github.com/mhr3/veloz/ascii"

	"github.com/tsenart/casei"
)

// timeOp returns ns/op for one operation. It times manually rather than
// through testing.Benchmark, which cannot be nested inside a running
// benchmark, and takes the best of three samples so a loaded machine
// inflates every entrant equally rather than randomly.
func timeOp(op func()) float64 {
	const budget = 25 * time.Millisecond
	best := 0.0
	for sample := 0; sample < 3; sample++ {
		n := 0
		start := time.Now()
		for time.Since(start) < budget {
			op()
			n++
		}
		ns := float64(time.Since(start).Nanoseconds()) / float64(n)
		if best == 0 || ns < best {
			best = ns
		}
	}
	return best
}

// BenchmarkBar reports x_vs_best per scenario: candidate time relative to the
// fastest other implementation that can answer the same query correctly.
func BenchmarkBar(b *testing.B) {
	for _, s := range scenarios {
		s := s
		b.Run("single/"+s.name, func(b *testing.B) {
			cand := timeOp(func() { sink = casei.IndexFold(s.haystack, s.needle) })

			best := timeOp(func() { sink = indexRegexp(s.haystack, s.needle) })
			// veloz is a SIMD engine and fold-correct on pure-ASCII input.
			if !s.utf8 {
				if v := timeOp(func() { sink = veloz.IndexFold(s.haystack, s.needle) }); v < best {
					best = v
				}
			}
			// The tolower idiom is semantically wrong beyond ASCII, so it
			// counts as an entrant only on the ASCII tier.
			if !s.utf8 {
				if t := timeOp(func() { sink = indexToLower(s.haystack, s.needle) }); t < best {
					best = t
				}
			}
			for i := 0; i < b.N; i++ {
				sink = casei.IndexFold(s.haystack, s.needle)
			}
			b.ReportMetric(cand/best, "x_vs_best")
		})
	}

	for _, s := range multiScenarios {
		s := s
		b.Run("multi/"+s.name, func(b *testing.B) {
			m := casei.NewMatcher(s.patterns)
			cand := timeOp(func() { _, matcherFound = m.Find(s.haystack) })

			re := regexpAltFor(s.patterns)
			best := timeOp(func() { matcherSink = len(re.FindStringIndex(s.haystack)) })
			if !s.utf8 {
				a := acBuild(s.patterns, true)
				if v := timeOp(func() { _, matcherFound = acFirst(&a, s.haystack) }); v < best {
					best = v
				}
			}
			for i := 0; i < b.N; i++ {
				_, matcherFound = m.Find(s.haystack)
			}
			b.ReportMetric(cand/best, "x_vs_best")
		})
	}
}
