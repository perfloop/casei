# casei

An open benchmark arena for **UTF-8 case-insensitive substring search**.

`grep -i`, SQL `ILIKE`, log-line filters, header lookups — caseless search is
one of the most executed operations in computing, and it is far slower than
it needs to be. For ASCII, engines pay 2–5× over exact matching. Beyond
ASCII it gets worse. Regex engines handle it as case-expanded literals
through general machinery — on the public record (rebar, Dec 2025),
Hyperscan drops from 32 GB/s exact to 7.4 GB/s on Russian caseless,
rust/regex to 8.4, and Go's `regexp` to ~49 MB/s. Dedicated engines exist
but not for these semantics: StringZilla v4.5 implements **full** folding
(ß→ss — a contract ClickHouse explicitly declined for substring search),
and ClickHouse's own UTF-8 caseless searcher surrenders
(`force_fallback = true`) whenever a character's case forms differ in
encoded length. **No dedicated engine implements simple folding — the
semantics of `regexp (?i)`.** The idiom everyone
actually writes — `ToLower` both strings and search — is not even correct:
`ToLower` is not case folding (it splits the σ/ς/Σ orbit, re-encodes, and
shifts byte offsets).

This repository holds one problem, in two faces:

```go
// IndexFold returns the byte index of the first occurrence of needle in
// haystack under Unicode simple case folding, or -1.
func IndexFold(haystack, needle string) int

// Matcher searches for any of a set of patterns under the same semantics;
// Find returns the leftmost match, ties to the lowest pattern index.
func NewMatcher(patterns []string) *Matcher
func (m *Matcher) Find(haystack string) (Match, bool)
```

They are the same problem: **a pattern position is a small set of UTF-8
encodings** (the fold orbit), exact search is the singleton case, and
multi-needle is the union. The goal of this repository is one adaptive
engine for that object — not two implementations sharing a package.

## Semantics

Unicode **simple case folding** over code points — exactly the matching of
Go `regexp` with `(?i)` and rust/regex, pinned by differential tests:

- `k` matches `K` and the Kelvin sign U+212A; `s` matches `S` and long s
  U+017F; `σ`, `ς`, `Σ` all match; `ß` matches `ẞ` but **not** `ss` (no full
  folding); `İ` and `ı` fold only to themselves (locale-independent).
- Matching is per code point, so a match window's byte length can differ
  from the needle's (`kelvin` is 6 bytes but matches an 8-byte window
  starting with U+212A). Matches start at haystack rune boundaries.
- Bytes outside valid UTF-8 are opaque units: they match only an opaque
  occurrence of the identical byte, never a fragment of a valid encoding.
- ASCII consequences: only the 52 ASCII letters fold within ASCII; the
  0x20-adjacent punctuation pairs (`[`/`{`, `@`/`` ` ``, `]`/`}`, `\`/`|`,
  `^`/`~`) never match.

`casei_test.go` is the executable definition: trap cases that have bitten
real SIMD implementations of this problem, a random differential against an
independent canonical-fold reference on arbitrary bytes, a random
differential against `regexp (?i)` on valid UTF-8, and a fuzz target
enforcing both.

## The bar

### Baseline isolation

Search code must not import, link, execute, embed, or delegate lookup to any
implementation in `arena/field.yaml`. Baselines live in the `arena/` module and
stay there; `scripts/check-baseline-isolation.sh` runs first in CI.

**A candidate that calls a field competitor is ineligible, whatever its
benchmark says.** This is not a style rule. An engine that calls `veloz` cannot
beat `veloz` — it can only add dispatch and verification overhead on top of it —
but `x_vs_best` reports a ratio either way, so the scoreboard cannot tell you
that no search was invented. The module boundary can.

### Engine identity

`IndexFold` and `Matcher.Find` must be one package-owned compiled search plan
and one block-transition state machine. A single needle is the `N=1` plan.
ASCII, UTF-8, scalar, AVX2 and NEON paths may differ only as representations of
that same transition.

Prohibited as alternate engines: per-pattern `IndexFold` loops, regex
delegation, `strings.Index` fallback lookup, an unrelated KMP or Aho-Corasick
engine reachable at runtime, and benchmark-specific dispatch. Instrumentation
must be able to show that single-needle and multi-needle searches enter the
same plan.

### Competitive acceptance

Results are scoped to the frozen field in `arena/field.yaml` — baseline
versions, build flags, ISA, corpus hashes, semantic status. The claim this
repository can support is *fastest among the audited field, on the declared
platform*. It cannot support "fastest in existence", and no longer asks for it.

A baseline's time enters `x_vs_best` only if its `semantic_status` says it
agrees with the arena oracle on that tier. Any adaptation needed to make it
comparable is timed as part of it.

**A row whose only compatible competitor is Go's `regexp` is field-incomplete
and cannot be reported as a competitive win.** Every UTF-8 row is currently in
that state: the naive reference in this repository already scores 0.31–0.90
there, having beaten nothing but a scalar NFA. Occupying `stringzilla`,
`pcre2-jit`, or `vectorscan` is what makes those rows mean something.

On every mandatory row that is not ceiling-limited, the upper bound of the 95%
confidence interval of `candidate / best-field` must be **≤ 0.67**, and the
geometric mean across those rows **≤ 0.50**.

A row is ceiling-limited when the best field implementation is within 5% of the
exact-match ceiling. Demanding a large multiple there is asking to beat memory
bandwidth; such a row instead requires the candidate within 5% of that ceiling,
and is reported separately.

Report raw paired samples with alternating order, not a best-of-N point
estimate. Ratios and intervals are computed from those samples.

### And it must also

1. **Exploit the instruction set.** The ASCII bar is a hand-written NEON/AVX2
   kernel. Scalar code does not reach it, and no wrapper reaches it either.
   Architecture-specific kernels are expected — each with a correct portable
   fallback and identical semantics under every differential.
2. **Keep a linear worst case.** The adversarial scenarios (`periodic`,
   `samechar`, `torture`) exist so throughput cannot be bought with a
   quadratic cliff.
3. **Pass every test, differential, and the fuzzer, on every architecture it
   claims.** Architecture-specific fast paths need a correct portable fallback.
4. **Be reproducible off this machine.** A field result ships with the frozen
   manifest, corpus hashes, toolchain and CPU feature detection, and the raw
   samples — enough for a third party to re-run it on their own hardware and
   get the same direction and confidence bounds.

## Where the reference stands

`BenchmarkBar` measures this repository's own reference implementation
against the field. `x_vs_best` is its time divided by the fastest correct
alternative present; below 1.0 means nothing that exists is faster.
Measured on an Apple M3 Max (loaded; directional):

| row | x_vs_best |
|---|---|
| multi/multi_N512_miss_log_64kb | 6421 |
| multi/multi_N64_miss_log_64kb | 734 |
| single/samechar_miss_64kb | 597 |
| single/periodic_miss_64kb | 325 |
| multi/multi_N8_miss_log_1mb | 97.7 |
| single/log_miss_1mb | 90.7 |
| single/torture_miss_64kb | 26.9 |
| multi/multi_N8_miss_ru_1mb | 7.1 |
| single/ru_miss_1mb | **0.85** |
| single/kelvin_hazard_1mb | **0.31** |

The reference is a deliberately naive rune-walking scan, so most rows are
one to four orders of magnitude behind. The two rows already below 1.0 are
not an achievement: on the UTF-8 tier the only in-arena competitor is Go's
`regexp`, which is itself slow. They mark where the field is weakest, not
where this code is strong.

Getting every row below 1.0 requires both a better algorithm and
data-parallel execution. The baselines winning the ASCII rows are
hand-written NEON/AVX2 kernels consuming 16 or 32 bytes per instruction.

## Baselines

| name | what it is | tiers |
|---|---|---|
| `candidate` | `casei.IndexFold` — the function under optimization | both |
| `tolower` | `strings.Index(ToLower(h), ToLower(n))` — the common idiom, allocations included; semantically wrong beyond ASCII, kept as a perf reference only | both (perf), ASCII (agreement) |
| `regexp` | precompiled `(?i)` literal — the stdlib answer and semantic anchor | both |
| `veloz` | [`mhr3/veloz`](https://github.com/mhr3/veloz) `ascii.IndexFold` — the strongest published Go SIMD caseless search | ASCII |
| `ceiling` | `strings.Index` on pre-folded input — exact-match physics, the target | both |

Multi-needle (`matcher_bench_test.go`):

| name | what it is |
|---|---|
| `candidate` | `casei.Matcher` |
| `regexpAlt` | precompiled `(?i)(?:p0\|p1\|…)` — stdlib answer, semantic anchor for leftmost-start |
| `ac` | [aho-corasick](https://github.com/petar-dambovaliev/aho-corasick) DFA, leftmost-first, ASCII-caseless (ASCII tier only — the reference multi-pattern libraries renounce Unicode folding) |
| `ceiling` | exact-match Aho-Corasick over pre-folded input |

## Running

```sh
go test ./...                      # correctness, differentials, agreement
go test -fuzz=FuzzIndexFold -fuzztime=30s
go test -bench=. -benchtime=200ms       # the arena (single- and multi-needle)
go test -bench=BenchmarkBar -benchtime=10ms  # the scoreboard: x_vs_best per row
```

## Prior art

[`CONTEXT.md`](CONTEXT.md) catalogs every technique known to this problem —
folding primitives, SIMD prefilter designs, candidate-extraction tricks on
movemask-less ISAs, vectorized rolling hashes, adaptive stage-escalation
budgets, rare-byte statistics, and what regex engines do for caseless UTF-8
today — with sources and measured numbers. It is the line between
engineering and invention here: **an approach only counts as new if it is
not already in that document.**
