package casei

// The arena. Every implementation races on the same scenario matrix:
// realistic corpora (logs, prose, code), miss-heavy throughput scans,
// dense-hit counting, short-haystack latency, a needle-length sweep, and the
// adversarial family (periodic, samechar, torture) that punishes quadratic
// verification. Baselines:
//
//   candidate  - casei.IndexFold, the function under optimization
//   tolower    - strings.Index(asciiLower(h), asciiLower(n)): the idiom
//                everyone actually writes, allocation cost included
//   regexp     - precompiled (?i) literal via regexp: the stdlib answer
//   veloz      - github.com/mhr3/veloz/ascii.IndexFold: the strongest
//                published Go SIMD caseless search (NEON on arm64, AVX2/SSE
//                on amd64)
//   ceiling    - strings.Index on a pre-lowered corpus: exact-match physics,
//                what caseless search costs if folding were free
//
// Every miss scenario reports bytes/op over the full haystack (throughput);
// count scenarios scan the whole haystack through repeated calls.

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"

	veloz "github.com/mhr3/veloz/ascii"
)

// ---- corpora (deterministic; no testdata files) ----------------------------

func corpusRNG() *rand.Rand { return rand.New(rand.NewPCG(0xCA5E1, 0xA12E9A)) }

var logLevels = []string{"DEBUG", "INFO", "INFO", "INFO", "WARN", "ERROR"}
var logServices = []string{"checkout", "search", "ingest", "billing", "Gateway", "AuthZ", "replicator"}
var logMessages = []string{
	"request completed", "cache miss", "retry scheduled", "connection reset by peer",
	"slow query detected", "Payment Authorized", "token refreshed", "queue depth high",
	"compaction finished", "TLS handshake complete", "rate limit applied",
}

func buildLogCorpus(size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 256)
	for b.Len() < size {
		fmt.Fprintf(&b, "2026-08-04T%02d:%02d:%02d.%03dZ %s service=%s region=eu-west-%d trace=%08x%08x msg=%q latency_ms=%d\n",
			rng.IntN(24), rng.IntN(60), rng.IntN(60), rng.IntN(1000),
			logLevels[rng.IntN(len(logLevels))],
			logServices[rng.IntN(len(logServices))],
			1+rng.IntN(3), rng.Uint32(), rng.Uint32(),
			logMessages[rng.IntN(len(logMessages))],
			rng.IntN(2000))
	}
	return b.String()[:size]
}

var proseWords = strings.Fields(`the of and to in a is that it was for on are with as his they at be
this have from or one had by word but not what all were we when your can said there use an each which
she do how their if will up other about out many then them these so some her would make like him into
time has look two more write go see number no way could people my than first water been call who oil
its now find long down day did get come made may part over new sound take only little work know place
year live me back give most very after thing our just name good sentence man think say great where
help through much before line right too mean old any same tell boy follow came want show also around
form three small set put end does another well large must big even such because turn here why ask went
men read need land different home us move try kind hand picture again change off play spell air away
animal house point page letter mother answer found study still learn should America world`)

func buildProseCorpus(size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 128)
	for b.Len() < size {
		n := 6 + rng.IntN(9)
		for i := 0; i < n; i++ {
			w := proseWords[rng.IntN(len(proseWords))]
			if i == 0 {
				w = strings.Title(w) //nolint:staticcheck // ASCII words only
			}
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(w)
		}
		b.WriteString(". ")
	}
	return b.String()[:size]
}

func buildCodeCorpus(size int) string {
	rng := corpusRNG()
	var b strings.Builder
	b.Grow(size + 256)
	for b.Len() < size {
		id := rng.IntN(10000)
		fmt.Fprintf(&b, "func handleReq%d(ctx context.Context, in []byte) (map[string]int, error) {\n", id)
		fmt.Fprintf(&b, "\tout := make(map[string]int, %d)\n\tfor i, v := range in {\n", rng.IntN(64))
		fmt.Fprintf(&b, "\t\tif v&0x%02x != 0 {\n\t\t\tout[keys[i%%%d]] += int(v)\n\t\t}\n\t}\n", rng.IntN(256), 1+rng.IntN(16))
		fmt.Fprintf(&b, "\tif len(out) == 0 {\n\t\treturn nil, fmt.Errorf(\"empty%d: %%w\", errSentinel)\n\t}\n\treturn out, nil\n}\n\n", id)
	}
	return b.String()[:size]
}

// plant returns corpus with occ case-flipped copies of needle spliced in at
// even spacing, so hit scenarios have known, realistic density.
func plant(corpus, needle string, occ int) string {
	rng := rand.New(rand.NewPCG(7, 7))
	if occ <= 0 {
		return corpus
	}
	step := len(corpus) / (occ + 1)
	var b strings.Builder
	b.Grow(len(corpus) + occ*len(needle))
	prev := 0
	for i := 1; i <= occ; i++ {
		pos := i * step
		b.WriteString(corpus[prev:pos])
		b.WriteString(flipCases(rng, needle))
		prev = pos
	}
	b.WriteString(corpus[prev:])
	return b.String()
}

// ---- scenario matrix --------------------------------------------------------

type scenario struct {
	name     string
	haystack string
	needle   string
	count    bool // count all (overlap-allowed) occurrences instead of first
}

var scenarios = func() []scenario {
	logs1m := buildLogCorpus(1 << 20)
	prose1m := buildProseCorpus(1 << 20)
	code256k := buildCodeCorpus(256 << 10)

	return []scenario{
		// Miss-heavy scans: full-haystack throughput.
		{"log_miss_1kb", logs1m[:1024], "fatal panic", false},
		{"log_miss_64kb", logs1m[:64<<10], "fatal panic", false},
		{"log_miss_1mb", logs1m, "fatal panic", false},
		{"prose_miss_1mb", prose1m, "zygomorphic", false},
		{"code_miss_256kb", code256k, "goto retryLabel", false},

		// Needle-length sweep, all misses, letters included so folding is live.
		{"log_needle3_64kb", logs1m[:64<<10], "vQx", false},
		{"log_needle8_64kb", logs1m[:64<<10], "vQxKz9Jw", false},
		{"log_needle16_64kb", logs1m[:64<<10], "vQxKz9JwPl2Rt7Ym", false},
		{"log_needle32_64kb", logs1m[:64<<10], "vQxKz9JwPl2Rt7YmNb4Cd8Fg1Hk5Ls0Z", false},

		// Hits: sparse (first-match latency over distance) and dense (count).
		{"log_hit_sparse_1mb", plant(logs1m, "payment declined by issuer", 16), "payment declined by issuer", true},
		{"prose_hit_dense_1mb", prose1m, "The ", true},
		{"code_hit_brackets_256kb", code256k, "[keys[i%", true},

		// Short-haystack latency (the per-row / per-line call shape).
		{"latency_match_start_1kb", "Needle in front " + prose1m[:1008], "needle in front", false},
		{"latency_match_mid_1kb", prose1m[:500] + "NeEdLe MiDwAy" + prose1m[500:1011], "needle midway", false},
		{"latency_match_end_1kb", prose1m[:1009] + "NEEDLE AT END", "needle at end", false},
		{"latency_miss_1kb", prose1m[:1024], "absent needle", false},

		// Adversarial: repetitive structure, near-matches, quadratic traps.
		{"periodic_miss_64kb", strings.Repeat("ab", 32<<10), "abababababababac", false},
		{"samechar_miss_64kb", strings.Repeat("a", 64<<10), "aaaaaaaaaaaaaaab", false},
		{"torture_miss_64kb", strings.Repeat(strings.Repeat("a", 31)+"b", 2048), strings.Repeat("a", 32), false},
	}
}()

// ---- implementations under test ----------------------------------------------

func indexToLower(h, n string) int {
	return strings.Index(asciiLower(h), asciiLower(n))
}

var regexpCache = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(scenarios))
	for _, s := range scenarios {
		if _, ok := m[s.needle]; !ok {
			m[s.needle] = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(s.needle))
		}
	}
	return m
}()

func indexRegexp(h, n string) int {
	loc := regexpCache[n].FindStringIndex(h)
	if loc == nil {
		return -1
	}
	return loc[0]
}

var impls = []struct {
	name  string
	index func(h, n string) int
}{
	{"candidate", IndexFold},
	{"tolower", indexToLower},
	{"regexp", indexRegexp},
	{"veloz", veloz.IndexFold},
}

// countAll counts overlap-allowed occurrences by repeated first-match calls,
// so every implementation is measured through the same access pattern.
func countAll(index func(h, n string) int, h, n string) int {
	c, off := 0, 0
	for off <= len(h) {
		i := index(h[off:], n)
		if i < 0 {
			return c
		}
		c++
		off += i + 1
	}
	return c
}

// TestBaselinesAgree pins every implementation to the reference on the whole
// scenario matrix, so a benchmark win can never come from semantic drift.
func TestBaselinesAgree(t *testing.T) {
	for _, s := range scenarios {
		want := reference(s.haystack, s.needle)
		wantCount := countAll(reference, s.haystack, s.needle)
		for _, im := range impls {
			if got := im.index(s.haystack, s.needle); got != want {
				t.Errorf("%s/%s: first = %d, want %d", s.name, im.name, got, want)
			}
			if s.count {
				if got := countAll(im.index, s.haystack, s.needle); got != wantCount {
					t.Errorf("%s/%s: count = %d, want %d", s.name, im.name, got, wantCount)
				}
			}
		}
	}
}

func BenchmarkIndexFold(b *testing.B) {
	for _, s := range scenarios {
		for _, im := range impls {
			b.Run(s.name+"/"+im.name, func(b *testing.B) {
				b.SetBytes(int64(len(s.haystack)))
				b.ReportAllocs()
				if s.count {
					for i := 0; i < b.N; i++ {
						sink = countAll(im.index, s.haystack, s.needle)
					}
				} else {
					for i := 0; i < b.N; i++ {
						sink = im.index(s.haystack, s.needle)
					}
				}
			})
		}
		// The exact-match ceiling: same scenario, folding pre-paid outside
		// the timed region. This is the physics target the winning caseless
		// implementation is judged against (see CONTEXT.md).
		lh, ln := asciiLower(s.haystack), asciiLower(s.needle)
		b.Run(s.name+"/ceiling", func(b *testing.B) {
			b.SetBytes(int64(len(lh)))
			if s.count {
				for i := 0; i < b.N; i++ {
					sink = countAll(strings.Index, lh, ln)
				}
			} else {
				for i := 0; i < b.N; i++ {
					sink = strings.Index(lh, ln)
				}
			}
		})
	}
}

var sink int
