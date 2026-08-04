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

The goal is an `IndexFold` that, on this repository's benchmark suite:

0. **`x_vs_best < 1` on every row.** `BenchmarkBar` reports, per scenario,
   the candidate's time divided by the time of the strongest *existing*
   implementation available on the machine — including hand-written SIMD
   engines. Above 1 means something that already exists is faster. This is
   the scoreboard, and every other clause below is subordinate to it.
1. **Exploit the instruction set.** The strongest baselines here are
   NEON/AVX2 kernels; scalar code cannot reach them, and `x_vs_best` on the
   ASCII rows is unreachable without data-parallel work. Architecture-specific
   kernels are expected, not merely permitted — each with a correct portable
   fallback and identical semantics under every differential.
2. **wins every scenario** against every baseline present — realistic
   corpora and adversarial inputs alike, no cherry-picking;
3. **ASCII tier**: at least **2×** the strongest baseline (geometric mean)
   and within **10% of the exact-match ceiling** — case-insensitivity
   effectively free;
4. **UTF-8 tier**: within **2×** of the ASCII tier's throughput on matched
   corpus shapes — Unicode folding must not cost more than one doubling
   over ASCII folding;
   and, on cased non-Latin scripts, at least **2× StringZilla** measured on
   the same machine under this repository's semantics;
5. **multi-needle**: beat every baseline across N=2…512 on both tiers —
   the `simple-fold × multi-needle × SIMD` and `× linear-worst-case` cells
   have no shipped or published occupant (see `CONTEXT.md` §1d);
6. keeps a **linear worst case** — the adversarial scenarios (`periodic`,
   `samechar`, `torture`) exist so throughput cannot be bought with a
   quadratic cliff;
7. passes every test, differential, and the fuzzer, on every architecture it
   claims. Architecture-specific fast paths need a correct portable
   fallback.

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
