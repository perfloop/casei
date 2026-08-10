package arena_test

// The competitive bar reports the candidate's time divided by the fastest
// eligible field entrant on each scenario. `x_vs_best` is lower-is-better:
// one is parity and values below one are a win. The accompanying dispatch
// metrics make the row's ISA contract explicit instead of treating unlike
// native paths as one unnamed field.

import (
	"fmt"
	"testing"
	"time"

	veloz "github.com/mhr3/veloz/ascii"
	"golang.org/x/sys/cpu"

	"github.com/tsenart/casei"
	pcre2jit "github.com/tsenart/casei/arena/pcre2"
	rustac "github.com/tsenart/casei/arena/rustac"
	stringzilla "github.com/tsenart/casei/arena/stringzilla"
	vectorscan "github.com/tsenart/casei/arena/vectorscan"
)

// timeOp returns ns/op for one operation. It times manually rather than
// through testing.Benchmark, which cannot be nested inside a running
// benchmark, and takes the best of three samples so a loaded machine inflates
// every entrant equally rather than randomly.
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

// velozVectorBits is deliberately strict: Veloz has an SSE/scalar fallback,
// but the field's x86 entrant is its source-audited AVX2 path. Do not race a
// weaker fallback under the same name.
func velozVectorBits() int {
	if cpu.X86.HasAVX2 {
		return 256
	}
	return 0
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func reportSingleDispatch(b *testing.B, s scenario) {
	vscan := vectorscanSingles[s.needle]
	if vscan == nil {
		panic(fmt.Sprintf("Vectorscan baseline was not compiled for %q", s.needle))
	}
	velozBits := 0
	if !s.utf8 {
		velozBits = velozVectorBits()
	}
	stringZillaBits := 0
	if stringZillaAvailable {
		stringZillaBits = stringzilla.VectorBits()
	}
	rureBits := 0
	if re := rureSingles[s.needle]; re != nil {
		rureBits = re.VectorBits()
	}
	b.ReportMetric(1, "candidate_active")
	b.ReportMetric(float64(casei.NewMatcher([]string{s.needle}).VectorBits()), "candidate_vector_bits")
	b.ReportMetric(1, "regexp_active")
	b.ReportMetric(0, "regexp_vector_bits")
	b.ReportMetric(1, "pcre2_active")
	b.ReportMetric(float64(pcre2jit.VectorBits()), "pcre2_vector_bits")
	b.ReportMetric(boolMetric(rureBits == 256), "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(1, "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(boolMetric(stringZillaAvailable), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8 && velozBits == 256), "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
}

func reportMultiDispatch(b *testing.B, s multiScenario, candidateBits int, rure *rureRegex, rust *rustac.Matcher, vscan *vectorscan.Matcher) {
	velozBits := 0 // Veloz has no multi-pattern API.
	stringZillaBits := 0
	if stringZillaAvailable {
		stringZillaBits = stringzilla.VectorBits()
	}
	rureBits := rure.VectorBits()
	rustBits := rust.VectorBits()
	b.ReportMetric(1, "candidate_active")
	b.ReportMetric(float64(candidateBits), "candidate_vector_bits")
	b.ReportMetric(1, "regexp_active")
	b.ReportMetric(0, "regexp_vector_bits")
	b.ReportMetric(1, "pcre2_active")
	b.ReportMetric(float64(pcre2jit.VectorBits()), "pcre2_vector_bits")
	b.ReportMetric(boolMetric(rureBits == 256), "rure_active")
	b.ReportMetric(float64(rureBits), "rure_vector_bits")
	b.ReportMetric(1, "vectorscan_active")
	b.ReportMetric(float64(vscan.VectorBits()), "vectorscan_vector_bits")
	b.ReportMetric(boolMetric(vscan.HasVBMI()), "vectorscan_vbmi")
	b.ReportMetric(boolMetric(stringZillaAvailable), "stringzilla_active")
	b.ReportMetric(float64(stringZillaBits), "stringzilla_vector_bits")
	b.ReportMetric(0, "veloz_active")
	b.ReportMetric(float64(velozBits), "veloz_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8 && rustBits == 256), "rustac_active")
	b.ReportMetric(float64(rustBits), "rustac_vector_bits")
	b.ReportMetric(boolMetric(!s.utf8), "go_ac_active")
	b.ReportMetric(0, "go_ac_vector_bits")
}

// BenchmarkBar reports x_vs_best per scenario: candidate time relative to the
// fastest other implementation that can answer the same query correctly. The
// Go Aho-Corasick baseline remains visible as a supplemental scalar entrant,
// but it is intentionally not eligible to establish the winning bar; the
// direct Rust DFA is the native multi-pattern control.
func BenchmarkBar(b *testing.B) {
	for _, s := range scenarios {
		s := s
		b.Run("single/"+s.name, func(b *testing.B) {
			cand := timeOp(func() { sink = casei.IndexFold(s.haystack, s.needle) })

			best := timeOp(func() { sink = indexRegexp(s.haystack, s.needle) })
			competitors := 1
			if v := timeOp(func() { sink = indexPCRE2(s.haystack, s.needle) }); v < best {
				best = v
			}
			competitors++
			rure := rureSingles[s.needle]
			rureTime := timeOp(func() { sink = indexRure(s.haystack, s.needle) })
			// The Rust adapter records the backend reached by this exact query.
			// A query that did not reach memchr AVX2 is diagnostic only; it must
			// not race a target-width field entrant under a CPU-flag label.
			if rure.VectorBits() == 256 {
				if rureTime < best {
					best = rureTime
				}
				competitors++
			}
			if v := timeOp(func() { sink = indexVectorscan(s.haystack, s.needle) }); v < best {
				best = v
			}
			competitors++
			if stringZillaAvailable {
				if v := timeOp(func() { sink = indexStringZilla(s.haystack, s.needle) }); v < best {
					best = v
				}
				competitors++
			}
			if !s.utf8 && velozVectorBits() == 256 {
				if v := timeOp(func() { sink = veloz.IndexFold(s.haystack, s.needle) }); v < best {
					best = v
				}
				competitors++
			}
			for i := 0; i < b.N; i++ {
				sink = casei.IndexFold(s.haystack, s.needle)
			}
			b.ReportMetric(cand/best, "x_vs_best")
			b.ReportMetric(float64(competitors), "competitors")
			b.ReportMetric(float64(competitors+1), "entrants")
			reportSingleDispatch(b, s)
		})
	}

	for scenarioIndex, s := range multiScenarios {
		s := s
		scenarioIndex := scenarioIndex
		b.Run("multi/"+s.name, func(b *testing.B) {
			m := casei.NewMatcher(s.patterns)
			cand := timeOp(func() { _, matcherFound = m.Find(s.haystack) })

			re := regexpAltFor(s.patterns)
			best := timeOp(func() { matcherSink = len(re.FindStringIndex(s.haystack)) })
			competitors := 1
			pcre := pcre2Alts[scenarioIndex]
			if v := timeOp(func() { _, _, matcherFound = pcre.Find(s.haystack) }); v < best {
				best = v
			}
			competitors++
			rure := rureAlts[scenarioIndex]
			rureTime := timeOp(func() { _, _, matcherFound = rure.Find(s.haystack) })
			if rure.VectorBits() == 256 {
				if rureTime < best {
					best = rureTime
				}
				competitors++
			}
			vscan := vectorscanAlts[scenarioIndex]
			if v := timeOp(func() { _, _, matcherFound = vscan.Find(s.haystack) }); v < best {
				best = v
			}
			competitors++
			if stringZillaAvailable {
				stringzilla := stringZillaAlts[scenarioIndex]
				if v := timeOp(func() { _, _, matcherFound = stringzilla.Find(s.haystack) }); v < best {
					best = v
				}
				competitors++
			}
			supplemental := 0
			rust := rustACAlts[scenarioIndex]
			if !s.utf8 {
				rustTime := timeOp(func() { _, _, matcherFound = rust.Find(s.haystack) })
				// The direct Rust DFA exposes the memchr backend reached by this
				// exact prefilter query. Do not call an unobserved scalar/SSE path
				// an AVX2 field entrant merely because this process has AVX2.
				if rust.VectorBits() == 256 {
					if rustTime < best {
						best = rustTime
					}
					competitors++
				}

				goAC := acBuild(s.patterns, true)
				_ = timeOp(func() { _, matcherFound = acFirst(&goAC, s.haystack) })
				supplemental++
			}
			for i := 0; i < b.N; i++ {
				_, matcherFound = m.Find(s.haystack)
			}
			b.ReportMetric(cand/best, "x_vs_best")
			b.ReportMetric(float64(competitors), "competitors")
			b.ReportMetric(float64(competitors+1+supplemental), "entrants")
			reportMultiDispatch(b, s, m.VectorBits(), rure, rust, vscan)
		})
	}
}
