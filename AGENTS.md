# Contributing rules

These are binding rules for any change to the search implementation, not
advice. A change that violates one should not be proposed, however fast it is.

## 1. The deliverable is a result the field does not hold

This repository exists to produce a caseless search engine that does not
currently exist: the fastest correct one, on workloads the field does not
serve. Twelve constructions are closed by proof in `NOVELTY.md`, so a wholly
new state or transition is not the expected route and is no longer the bar.

The bar is a new **result**, reached however it is reached. Assembling known
techniques into an engine that holds a position nobody holds -- `x_vs_best < 1`
on rows the field contests, or correct caseless UTF-8 multi-needle search that
no shipped engine provides -- is the deliverable, and building it is required
rather than forbidden.

Two outcomes are still failures:

- a re-implementation of an existing engine's approach that produces no new
  result;
- a wrapper or port of a published technique.

## 2. Novelty must be argued, not assumed

A change claiming a new construction ships `NOVELTY.md` containing:

1. the claimed new state representation or block transition, stated precisely
   enough to disagree with;
2. the closest known constructions, each with a source (paper, repository, or
   engine), including the ones in `CONTEXT.md`;
3. what is combined from them and what result that combination produces.
   **Combining known techniques is legitimate and is how most real advances
   happen.** The test is whether the RESULT is new -- a capability nobody has,
   or a measured position nobody holds -- not whether every component is. A
   repacking that produces no new result is not enough; a combination that
   produces one is;
4. what evidence would falsify the claim, and the result of looking for it.

**Absence from `CONTEXT.md` is not evidence of novelty.** That document is one
sweep's catalog against eighty years of literature. Point 3 is where these
claims usually die.

## 3. A negative result is a result

If the assessment concludes every component is known art, record that in
`NOVELTY.md` with the sources -- and then build the thing anyway if the
combination reaches a result the field does not hold. Known components are not
a reason to stop; only a known *result* is.

Do not use a novelty finding as a reason to ship nothing.

State what would falsify the negative. That is what makes it usable by whoever
looks next, instead of merely discouraging.

## 3b. One refutation is not a round

A closed cell is a result, not a stopping condition. Refuting a construction
frees you to route around it, so generate the next one and refute that. Keep
going until the budget is spent or a construction survives.

Stopping after a single refutation is the failure mode this rule exists to
prevent: the reward for "still closed" and for "here is something that
survives" are the same, and the first is reachable in minutes. Report every cell
you closed, not just the last.

Record what you ruled out cheaply on paper as well as what you implemented. A
cell closed by a two-line argument is worth as much to the next reader as one
closed by a benchmark, and costs far less.

## 4. Measure against the field, not against yourself

The scoreboard is `BenchmarkBar` in the `arena/` module. It reports
`x_vs_best`: this implementation's time divided by the fastest correct
alternative present.

**Run it with `-tags pcre2`.** Without that tag the UTF-8 rows have no entrant
but Go's `regexp`, a scalar NFA, and a row measured that way inverts once a real
matcher enters it -- rows that read 0.31-0.90 against the floor measure 1.09 to
1202 against PCRE2. Install the library first if it is missing:

```
apt-get install -y libpcre2-dev   # or: brew install pcre2
cd arena && go test -tags pcre2 -run '^$' -bench BenchmarkBar
```

Every row also reports an `entrants` count. A UTF-8 row with `entrants` below 2
was measured against the floor alone: say so when reporting it, and treat
closing that gap as the work rather than the number as a win.

A measurement with no competitor in it is not evidence. Two ways to produce
one, both of which have happened here:

- a benchmark written for this change, compared against its own previous value;
- a per-implementation lane of an arena benchmark
  (`BenchmarkIndexFold/<row>/candidate`, and likewise `/veloz`, `/regexp`,
  `/ceiling`). Those lanes exist for profiling. Reporting `/candidate` borrows
  the arena's authority for a number that never looked at the field.

### The acceptance bar

**Every row of `BenchmarkBar` must measure `x_vs_best` below 1.0.** Not a
subset, not a listed few, not "the mandatory ones" -- every row, ASCII and
UTF-8, single needle and multi needle. The goal is to be the fastest thing in
existence at this problem, so a single row above 1.0 is a row the field still
wins and the work is not done.

There are exactly two ways a row is excused, both narrow:

- **Ceiling-limited.** The best field implementation is within 5% of the
  exact-match ceiling, so a large multiple would mean beating memory bandwidth.
  Such a row instead requires this engine within 5% of that ceiling, and is
  reported separately with the ceiling number shown.
- **Not yet measurable.** No entrant exists that answers the same question.
  Then the row is not excused so much as unmeasured: say so, and wiring an
  entrant in becomes the work.

**A row is only measured if at least two entrants ran in it.** `x_vs_best`
against Go's `regexp` alone is a comparison with a scalar NFA floor, and a
number below 1.0 there says nothing about the field. Report the entrant count
for every row. A UTF-8 row with one entrant is an unoccupied tier, which is
missing work in this repository -- go wire a competitor in and measure again.

Do not report a lost row as a guardrail, a non-regression, or a diagnostic. A
row above 1.0 is a row that is losing. Name it, give its number, and say what
you intend to do about it.

These rows are the ones most likely to expose a construction that only looks
fast, so lead with them -- but they are a starting point for reporting, never
the definition of passing:

| row | competitor |
|---|---|
| `single/log_miss_1mb` | veloz NEON/AVX2 |
| `single/latency_miss_1kb` | short-input shape where plan setup shows up |
| `single/samechar_miss_64kb` | adversarial; linearity |
| `single/periodic_miss_64kb` | adversarial; self-similar input |
| `multi/multi_N512_miss_log_64kb` | aho-corasick |
| `multi/multi_N512_miss_hazard_64kb` | width-changing folds at N=512 |

## 5. One engine

`IndexFold` and `Matcher.Find` must be one package-owned compiled search plan
and one block-transition state machine. A single needle is the `N=1` plan of
that machine.

Prohibited as alternate engines: per-pattern `IndexFold` loops, regex
delegation, `strings.Index` fallback lookup, an unrelated KMP or Aho-Corasick
engine reachable at runtime, and benchmark-specific dispatch.

## 6. Baseline isolation

Search code must not import, link, execute, embed, or delegate lookup to any
implementation in `arena/field.yaml`. Baselines live in the `arena/` module.
`scripts/check-baseline-isolation.sh` enforces this.

**Calling a field competitor disqualifies the result regardless of the
benchmark** -- an engine that calls `veloz` cannot beat `veloz`, it can only add
overhead to it, and a ratio cannot tell you no search was invented.

## 7. Scope

Target amd64, plus a correct portable fallback. arm64/NEON is out of scope for
now and its absence is not a defect.

**The measurement host has AVX-512, not merely AVX2.** Recorded CPU is
`genuineintel/6/85` (Skylake-SP class) and sessions observe `avx512f`,
`avx512bw`, `avx512cd`, `avx512dq`, `avx512vl`. Use them: 512-bit vectors,
byte-granularity compares under BW, `vpermb` for in-register table lookup, and
k-mask registers that make set/liveness arithmetic native rather than emulated.
An earlier version of this file said AVX2 and that was an unchecked assumption,
not a constraint -- state counts computed against 256-bit lanes were computed
against the wrong machine.

Gate every path on runtime feature detection with a portable fallback. Say
which ISA a measurement covers; cross-compilation proves portability, not
performance.

## 8. Correctness is not negotiable

Unicode simple folding, pinned to Go `regexp` `(?i)` by differential test and
fuzz. Byte offsets, leftmost match, and ties to the lowest pattern index are
part of the contract. The portable path must be exercised, not merely compiled.
