# Direct rebar audit

> **Yes: the current `casei` API loses relevant rebar workloads.** Rebar times
> enumeration, meaning count every non-overlapping match, while `casei` is a first-match
> engine. The losses are real, they do not contradict the 33-row first-match
> result, and the worst one exposes a weak multi-pattern filter choice. A
> stateful iterator is still a missing API, but a measured streaming version did
> not repair the loss.

This page records the whole audit so “not yet in rebar” cannot become a way to
avoid unfavorable cases.

## The answer in 30 seconds

At rebar commit
[`463d00f`](https://github.com/BurntSushi/rebar/commit/463d00f31887e84c38467805b9e3122c314b9521),
the complete caseless literal/finite-alternation inventory is:

- **18 performance workloads**, all wired to `casei` and verified;
- **2 compatible Unicode behavior checks**, both passed;
- **1 deliberately incompatible ASCII-only behavior check**, which `casei`
  fails for the correct reason: rebar asks `s` not to match `ſ`, while Unicode
  simple folding requires the match.

Only five performance rows enable Unicode semantics and therefore ask the same
folding question as `casei`. Against rebar's three recorded leaders (Hyperscan,
PCRE2-JIT, and rust/regex), `casei` wins **2/5** and loses **3/5** on both Ice Lake
and Sapphire Rapids. Its median `time / best-other-time` is 1.27 and 1.28; the
worst row is roughly 9× slower.

The remaining 13 performance rows request ASCII-only case insensitivity.
`casei` was run anyway and produced the expected answers on those corpora, but
it always retains the stronger Unicode relation. Their timings are useful
stress tests, not semantic equivalents.

## Why this is a different benchmark contract

| | casei arena | rebar rows on this page |
|---|---|---|
| answer | first byte offset; or leftmost match, tie to lowest pattern ID | count or total span of every non-overlapping match |
| search state | one `IndexFold`/`Find` call | one compiled engine repeatedly enumerates until end of input |
| folding | Unicode simple folding on every row | Unicode on 5 rows, ASCII-only on 13 |
| measurement | randomized co-measurement through Perfloop for the published result | [rebar's sequential runner protocol](https://github.com/BurntSushi/rebar/blob/463d00f31887e84c38467805b9e3122c314b9521/METHODOLOGY.md), three independent passes here |

The audit adapter compiles `NewMatcher` outside the timed region. Inside each
iteration it calls `Find` on the remaining suffix, verifies the matched byte
width under simple folding, advances past the non-overlapping match, and
continues to the end. It supports rebar's `count` and `count-spans` models.
Nothing is recompiled per hit, but every hit exits and re-enters a first-match
search. A future iterator should retain the plan's scan state and vector
continuity across hits. Retaining that state would make a cleaner API. The
diagnosis below shows that iterator overhead is small on the worst row.

## Results

`casei / best` is median casei time divided by the fastest selected competitor
on the same row and pass, then the median across three passes. **Below 1.0 is a
win; above 1.0 is a loss.** The named competitor is the fastest by its
three-pass median.

| rebar row | requested folding | Ice Lake `casei / best` | Sapphire Rapids `casei / best` |
|---|---|---:|---:|
| `curated/01-literal/sherlock-casei-en` | ASCII-only* | 7.57× (Hyperscan) | 8.88× (Hyperscan) |
| `curated/01-literal/sherlock-casei-ru` | Unicode | 2.57× (PCRE2-JIT) | 2.47× (PCRE2-JIT) |
| `curated/02-literal-alternate/sherlock-casei-en` | ASCII-only* | 7.02× (Hyperscan) | 7.02× (Hyperscan) |
| `curated/02-literal-alternate/sherlock-casei-ru` | Unicode | 9.86× (Hyperscan) | 9.18× (Hyperscan) |
| `hyperscan/literal-casei-english-nosom` | ASCII-only* | 4.95× (Hyperscan) | 6.40× (Hyperscan) |
| `hyperscan/literal-casei-english-som` | ASCII-only* | 4.96× (Hyperscan) | 6.52× (Hyperscan) |
| `hyperscan/literal-casei-russian-nosom` | Unicode | **0.71×** (rust/regex) | **0.58×** (rust/regex) |
| `hyperscan/literal-casei-russian-som` | Unicode | **0.72×** (rust/regex) | **0.56×** (rust/regex) |
| `imported/leipzig/tom-sawyer-huckle-fin-insensitive` | ASCII-only* | 2.70× (Hyperscan) | 3.10× (Hyperscan) |
| `imported/leipzig/twain-insensitive` | ASCII-only* | 1.35× (Hyperscan) | 1.18× (Hyperscan) |
| `imported/sherlock/name-alt3-casei` | ASCII-only* | **0.78×** (rust/regex) | **0.69×** (rust/regex) |
| `imported/sherlock/name-alt5-casei` | ASCII-only* | 1.04× (rust/regex) | **0.94×** (rust/regex) |
| `imported/sherlock/name-holmes-casei` | ASCII-only* | **0.79×** (PCRE2-JIT) | **0.75×** (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-casei` | ASCII-only* | 1.57× (PCRE2-JIT) | 1.82× (PCRE2-JIT) |
| `imported/sherlock/name-sherlock-holmes-casei` | ASCII-only* | 2.64× (PCRE2-JIT) | 3.04× (PCRE2-JIT) |
| `imported/sherlock/the-casei` | ASCII-only* | 1.08× (PCRE2-JIT) | **0.90×** (PCRE2-JIT) |
| `opt/prefilter/literal-casei-english` | ASCII-only* | 1.85× (PCRE2-JIT) | 2.22× (PCRE2-JIT) |
| `opt/prefilter/literal-casei-russian` | Unicode | 1.27× (PCRE2-JIT) | 1.28× (PCRE2-JIT) |

`*` Rebar disables Unicode-aware case folding. `casei` cannot disable it, so it
does more work and would also match Unicode fold mates not present in these
particular corpora. These rows passed output verification but are not
contract-equivalent.

Across all 18 stress rows, including those ASCII-only rows, `casei` wins 4/18
on Ice Lake and 6/18 on Sapphire Rapids. Those counts are deliberately not
promoted as a product result because 13 rows ask different semantics.

## Why the worst row is 9× slower

The bad row searches a 1,570,556-byte Russian Sherlock Holmes corpus for five
names and counts 971 matches. It is a particularly clear counterexample to
`casei`'s usual advantage:

```text
one Russian pattern    rare interior byte pairs -> about 2,100 candidates
five Russian patterns  common starting letters -> about 80,000 filter stops
```

For one pattern, the compiler selects a fused pair-pair anchor from inside the
literal. The AVX-512 VBMI kernel skips 1,552,007 bytes and asks the exact
matcher to check 2,134 candidate positions.

For all five patterns, the current compiler cannot combine those interior
anchors into one shared filter. It falls back to a nine-pair Shufti filter over
the patterns' first UTF-8 bytes. Several of those starts are common Cyrillic
letters. Across the full count, the filter is invoked 79,950 times and admits
193,449 runes, or 21.7% of the corpus, into the exact plan. The plan performs
175,860 dense state transitions. The CPU profile is consequently dominated by
UTF-8 decoding and fold-token map lookup, not the AVX-512 filter.

Three controls separate that cause from plausible alternatives:

- A one-pass diagnostic enumerator, using the same plan without returning from
  `Find` after each hit, took 4.69 ms versus 4.65 to 4.99 ms for repeated `Find`.
  It returned the same 971 matches. Restarting the API is therefore noise on
  this row, not the 9× cause.
- Replacing Shufti with the exact nine-pair AVX-512 filter made the row slower.
  Disabling AVX-512 made the multi-pattern row roughly 25% to 33% slower. The
  assembly kernel is helping; it is being given an unselective question.
- Rebar's Hyperscan runner was rebuilt and checked independently. It returned
  971 matches with an AVX-512 VBMI database. Enabling start-of-match tracking
  changed its time by only about 2%, so the result is not explained by its
  no-start-offset lane. Hyperscan expands the folds at compile time into a
  byte-level database and scans the five literals continuously, without a Go
  rune-decoding and hash-map verification loop.

The smaller two losses have the same general shape at lower severity. On the
single Russian literal, `casei`'s rare interior anchor is effective, but each
survivor still enters a Go rune decoder and token map while PCRE2-JIT verifies
in native code (about 2.5×). On the sparse one-match Russian prefilter row,
that residual verification gap is only about 1.28×. The two wins are the same
sparse shape against rust/regex rather than PCRE2.

## Correctness and inventory closure

Every recorded runner invocation returned rebar's expected result on each of
the 18 performance workloads it entered. On both hosts, `casei` also passed the
two compatible behavior checks. The incompatible behavior row is:

```text
test/unicode/case/ascii-only: pattern "s", haystack "ſ"
rebar with unicode=false: 0 matches
casei Unicode simple fold: 1 match
```

The remaining case-insensitive rebar definitions were inspected rather than
silently ignored:

- `curated/03-date/*` and `wild/url/*` are general regex grammars.
- `curated/13-noseyparker/*` loads a large rule file whose caseless rules also
  use classes, boundaries, captures, and bounded repetition; its search and
  compile workloads are general-regex workloads, not finite literal sets.
- `imported/sherlock/name-alt4-casei` contains a character class and `+`.
- `wild/ruff`, `reported/p893-hir-case-folding`, and
  `reported/i988-cloudflare-compile` exercise inline flags, classes,
  repetition, captures, or compile time.

Those are outside a literal/finite-alternation API. Every caseless rebar row
that can be represented as one literal or a finite set of literals is accounted
for above.

## Measurement record

The exact adapter, pinned integration script, six raw CSVs, and an independent
ratio calculator are checked in under
[`audit/rebar/`](audit/rebar/README.md). Running
`python3 audit/rebar/summarize.py` reproduces this page's table and summaries
from those receipts.

- **casei source:** tree
  `d1f73802d35c29009a433eaaf9c2b51113ab5c95` (the tree merged as
  [`3954dbe`](https://github.com/tsenart/casei/commit/3954dbe40e8e21c4c7b2e2716f22647dd7cd880c))
- **rebar source:** `463d00f31887e84c38467805b9e3122c314b9521`
- **selected field:** Hyperscan 5.4.2, PCRE2 10.47 JIT, rust/regex 1.12.4
- **hosts:** GenuineIntel family 6/model 106 Ice Lake and family 6/model 143
  Sapphire Rapids; both exposed AVX-512F, AVX-512BW, and AVX-512VBMI
- **passes:** three per host; per benchmark, up to 100 warmups/200 ms and 1,000
  measured iterations/500 ms
- **plan setup:** outside rebar's timed iteration, as for the other engines
- **entrants per row:** three or four including `casei`, depending on which of
  the selected engines the upstream definition admits

These three competitors were selected because they are the leaders in rebar's
own published/current records for these literal rows. This was not a rebuild of
every general regex engine in rebar, so a displayed “win” is only against that
leader set; an unmeasured entrant could make a ratio worse, never repair a
reported loss.

One upstream build detail is recorded for reproducibility: rebar's vendored
PCRE2 10.47 snapshot contains `pcre2posix.c` but not its unused
`pcre2posix.h`. The audit omitted that POSIX wrapper from the build; rebar's
runner uses the native PCRE2 API, so no compiled search or JIT code changed.

## Required next work

The next construction must:

1. compile once and keep one package-owned plan;
2. combine selective interior anchors from several patterns into one shared
   byte-level filter instead of unioning common roots;
3. replace hot-path rune-to-token map lookups with compiled classification where
   the plan permits it;
4. expose exact non-overlapping enumeration, tentatively `Matcher.Scan` or an
   iterator, without introducing a second search engine; and
5. beat the fastest eligible entrant on all five Unicode-equivalent rows before
   any count-all performance claim is made.

Until that exists, the public performance claim remains first-match search.
