# Contributing rules

These are binding rules for any change to the search implementation, not
advice. A change that violates one should not be proposed, however fast it is.

## 1. The deliverable is an invention, not an optimization

This repository exists to produce a caseless search construction that does not
currently exist. Making the reference implementation faster is not the
deliverable. A change that is a competent implementation of a known technique
has failed its purpose here, even when every benchmark improves.

Two specific outcomes are failures:

- a re-implementation of an existing engine's approach;
- a wrapper, port, or re-tuning of a published technique.

## 2. Novelty must be argued, not assumed

A change claiming a new construction ships `NOVELTY.md` containing:

1. the claimed new state representation or block transition, stated precisely
   enough to disagree with;
2. the closest known constructions, each with a source (paper, repository, or
   engine), including the ones in `CONTEXT.md`;
3. why this is not an equivalent combination, repacking, or threshold variation
   of those constructions;
4. what evidence would falsify the claim, and the result of looking for it.

**Absence from `CONTEXT.md` is not evidence of novelty.** That document is one
sweep's catalog against eighty years of literature. Point 3 is where these
claims usually die.

## 3. A negative result is a result

If the novelty assessment concludes the construction is known art, that is the
answer, not a failure to produce one. Say so, in `NOVELTY.md`, with the
sources. Do not fall back to optimizing the known construction so the work has
something to show; the proof is the thing to show.

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

A measurement with no competitor in it is not evidence. Two ways to produce
one, both of which have happened here:

- a benchmark written for this change, compared against its own previous value;
- a per-implementation lane of an arena benchmark
  (`BenchmarkIndexFold/<row>/candidate`, and likewise `/veloz`, `/regexp`,
  `/ceiling`). Those lanes exist for profiling. Reporting `/candidate` borrows
  the arena's authority for a number that never looked at the field.

Report `x_vs_best` on at least these rows:

| row | competitor |
|---|---|
| `single/log_miss_1mb` | veloz NEON/AVX2 |
| `single/latency_miss_1kb` | short-input shape where plan setup shows up |
| `single/samechar_miss_64kb` | adversarial; linearity |
| `multi/multi_N512_miss_log_64kb` | aho-corasick |

Report a row you currently lose. A row above 1.0 is the only thing that tells
you the field is still ahead while there is time to act on it.

A UTF-8 row whose only compatible competitor is Go's `regexp` is
**field-incomplete** and is not a competitive result. The naive reference
already scores 0.31-0.90 there, having beaten a scalar NFA and nothing else.

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

Target amd64 with AVX2, plus a correct portable fallback. arm64/NEON is out of
scope for now and its absence is not a defect. Say which ISA a measurement
covers; cross-compilation proves portability, not performance.

## 8. Correctness is not negotiable

Unicode simple folding, pinned to Go `regexp` `(?i)` by differential test and
fuzz. Byte offsets, leftmost match, and ties to the lowest pattern index are
part of the contract. The portable path must be exercised, not merely compiled.
