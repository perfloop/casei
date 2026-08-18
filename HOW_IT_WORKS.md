# How casei works

Most of the speed comes from one decision: do not decode Unicode where a few
raw-byte tests can prove that a match is impossible.

## Ten seconds

`casei` wins by refusing to do Unicode work almost everywhere.

It compiles each pattern set into two parts that agree:

```text
                         exact fold-token plan
                       /                       \
patterns -> compile once                         -> first correct match
                       \                       /
                         cheap raw-byte sieve
                         (64 starts per block)
```

The sieve rejects impossible starting positions. It never declares a match.
Usually it rejects all 64 positions, so the exact Unicode plan does no work for
that block. If positions survive, the plan checks them and makes the decision.

The sieve can be wrong in only one direction. It may admit a position that
later fails, which costs time. It may never reject a real match. That split
gives `casei` both properties it needs:

- **Speed:** most input is handled as raw bytes in wide vectors.
- **Correctness:** every possible match is decided by the complete Unicode
  plan, never by a lossy shortcut.

## One minute: searching one block

Imagine looking for `fatal panic` without case sensitivity.

Any matcher that checks every possible start will spend time on positions that
could never match. Fast search engines use filters to avoid that work. `casei`
can build its filters around a narrower promise than a general regex engine:
literal strings, Unicode simple folding, and one leftmost answer.

`casei` knows at compile time that this is a literal. It can choose several
useful byte positions in the pattern. It might pick positions far enough apart
that an accidental alignment is rare. On AVX-512 it loads those positions for
64 possible starts at once, compares their case-normalized bytes, and intersects
the resulting 64-bit masks.

```text
candidate starts       0 1 2 3 4 5 ... 63
probe at offset A      0 0 1 0 0 0 ...  0
probe at offset B      0 0 1 0 1 0 ...  0
probe at offset C      0 0 0 0 1 0 ...  0
                       -------------------- AND
survivors              0 0 0 0 0 0 ...  0
```

No survivor means none of those 64 starts can match, so the search advances by
a block. When a bit survives, `casei` replays the complete pattern at that byte
position. False positives only cost time; they cannot change the answer.

The compiler has several sieves because one shape is not best for every pattern
set: dispersed single-byte probes, adjacent pairs, pair-pair anchors, triples,
and bounded Shufti/Teddy-style tables. Route selection depends on facts proved
about the compiled patterns, not benchmark names.

## Why Unicode does not break the sieve

Unicode simple folding is not “lowercase both strings.” A pattern position is a
small orbit of equivalent runes, and those runes can have different UTF-8 byte
lengths:

```text
k    K    K       one byte, one byte, three bytes
s    S    ſ       one byte, one byte, two bytes
σ    ς    Σ       three different runes in one orbit
```

So `casei` first compiles the complete relation into tokens. Valid runes map to
their fold-orbit token. Invalid UTF-8 bytes map to separate opaque tokens. The
shared state machine advances over those tokens and owns all semantic decisions.

Raw-byte filters are then derived only where they are safe:

- A fixed ASCII probe is used only when its offsets remain valid for every
  relevant rendering.
- Pair and triple tables contain every raw UTF-8 form that could begin the
  corresponding token sequence.
- Low-bit table aliases and normalized Shufti buckets are allowed to add
  survivors, because the exact plan follows them.
- A route is disabled when the compiler cannot prove that its filter covers
  every possible start.

That rule keeps the sieve safe: **the filter may say “maybe” too often, but it
may never say “impossible” about a real match.**

## Why many patterns do not mean many scans

`Matcher` does not call single-pattern search once per needle. `NewMatcher`
compiles all patterns into one trie with failure transitions over fold tokens.
Small products of states and tokens become dense transition tables; larger
plans keep sparse edges. Both representations are the same state machine.

The filters summarize possible starts across the whole pattern set. One pass
over the haystack therefore serves one pattern or hundreds:

```text
N=1       one compiled plan, one filter, one scan
N=512     one shared plan, shared filters, one scan
```

Terminals record pattern IDs. The plan delays its answer only as far as needed
to prove the leftmost byte position; ties go to the lowest original pattern
index.

## Where the advantage comes from

Vectorscan, PCRE2, rust/regex, StringZilla, and veloz are fast at their own
contracts. `casei` has a narrower one. The compiler knows the patterns are
literals, the matching relation is Unicode simple folding, and the caller wants
the first leftmost answer. It can choose the raw-byte sieve and exact verifier
together, then share both across the whole pattern set.

The measured advantage has four layers. “It uses AVX-512” is true but
incomplete.

| layer | what it buys | evidence that it matters |
|---|---|---|
| Work avoidance | Whole blocks are rejected without decoding or advancing the exact plan at every byte. | Bypassing the shape-selected routes made the median row 3.88× slower on Ice Lake and 4.28× slower on Sapphire Rapids. Three rows were unchanged within 1%; the worst Unicode multi-pattern rows were 401× and 424× slower. |
| Shared construction | One pattern set becomes one transition plan and one traversal instead of `N` separate searches. | The full engine Case moved the worst full-field row from `x_vs_best=6.718` to `0.9123` while preserving the semantic suite. |
| Wider native transition | AVX-512 BW handles 64 candidate starts and keeps set arithmetic in mask registers; VBMI performs byte-table lookup in registers. | Masking AVX-512 off while retaining the same plans reduced median throughput by 1.72× on Ice Lake and 1.89× on Sapphire Rapids. One Ice Lake row and three Sapphire Rapids rows favored AVX2, mostly short or Unicode verification-heavy cases. |
| Kernel scheduling | Fewer dependent operations keep the wide sieve fed. | Fusing one four-way Shufti reduction improved its 64 KiB kernel by 21.6% and the field row using it by 21.8%. Replacing the complete assembly backend with Go's experimental SIMD package then regressed a required 1 MiB row. |

The AVX2 backend keeps the same plan and semantics. The published field lead is
specifically the AVX-512 implementation.

### Why the hand-written kernels matter

A 512-bit register gives the kernel 64 lanes. It does not hide latency. On Ice
Lake, the `VPERMB` lookup used by the hot long-literal filter takes three cycles
to produce an answer, although the core can start one lookup each cycle. The
generated Go SIMD loop starts one 64-byte block and then waits on that block's
answer. The assembly loop starts four independent blocks, keeping the lookup
unit busy while the first answer arrives.

The assembly also sends lookup results straight into AVX-512 mask registers
and asks whether any of four masks survived. The generated loop builds another
vector, converts it to a mask, and moves that mask to a general register on
every 64-byte block. In larger Shufti kernels, the generated code spills lookup
tables to 144- and 512-byte stack frames; the assembly keeps them in vector
registers.

This was tested as a complete backend replacement, not inferred from selected
instructions. Correctness passed, but six alternating-order Ice Lake runs put
the experimental backend at 20.8--23.3 µs/op on
`single/log_miss_1mb`, versus 18.4--20.8 µs/op for assembly. The public
[archsimd Case](https://app.perfloop.ai/t/oss/case_37sjyc8f94) and the
[negative-result record](NOVELTY.md#complete-experimental-go-simd-backend-negative-result)
contain the acceptance decision and falsifier.

## Where it differs from the field

Every scoring implementation was built, profiled, and read at the level where
its answer lives: source, generated code, or hot disassembly. The detailed
prior-art record is in [`CONTEXT.md`](CONTEXT.md), and the provenance of each
adopted technique is in [`NOVELTY.md`](NOVELTY.md).

| entrant | its strength | the verified distinction |
|---|---|---|
| Vectorscan | A mature multi-regex engine with Teddy/FDR-style literal machinery and a 512-bit VBMI target. | It ran at the same vector width on the same CPUs. Beating it on full-scan miss rows proves the result is not explained by register width alone. Its all-match API is a different home field; see [`REBAR.md`](REBAR.md). |
| PCRE2-JIT | Strong prefix analysis and native JIT code for a broad regex language. | `casei` spends its compile budget on the narrower literal-set contract and can derive filters that need not preserve general regex behavior. |
| rust/regex | Byte-oriented automata plus strong literal extraction and Teddy prefilters. | It carries a general regex representation; `casei` shares one fold-token literal plan and specializes raw filters around its exact start/tie contract. |
| StringZilla | Dedicated SIMD UTF-8 search, including full-fold expansions. | Full folding is a different relation. The arena times the verification needed to reduce its candidates to simple-fold semantics; `casei` represents that relation directly. |
| veloz | Excellent hand-written AVX2 ASCII single-literal search. | It does not answer Unicode simple-fold or multi-pattern queries. Against it, both specialization and `casei`'s wider ISA contribute, so the benchmark does not pretend to separate them. |
| Rust Aho-Corasick | Strong ASCII multi-pattern DFA with a Teddy prefilter. | `casei` extends the shared-plan shape to Unicode fold orbits, variable UTF-8 widths, opaque invalid bytes, and 512-bit filters. |

The measured result comes from a complete Unicode plan behind shape-specific
raw-byte rejection, shared across the pattern set and accelerated by AVX-512
kernels. Shufti, Teddy, rare anchors, tries, failure links, `VPERMB`, and
confirmation after a candidate are known techniques. Their combination under
this contract is what produced the new result.

## The evidence ladder

1. **Semantics:** millions of single- and multi-pattern differential cases plus
   fuzzing compare every backend with Go `regexp (?i)` on valid UTF-8 and the
   opaque-byte oracle on invalid input.
2. **Mechanism:** the filter-route and ISA ablations measure work avoidance and
   vector width separately; the Shufti case isolates one assembly change. The
   [raw two-host audit](audit/publication/README.md) includes every sample, the
   exact ablation, file hashes, and a recomputation script.
3. **Field result:** every native entrant is rebuilt from pinned source and
   checked for its actual dispatched width before it can enter `x_vs_best`.
4. **Two CPUs:** all 33 first-match rows lead on both Ice Lake and Sapphire
   Rapids, not one favorable microarchitecture.
5. **Counterexample hunt:** the direct rebar integration exposes the count-all
   rows where the current API loses. Those results narrow the claim instead of
   being excluded from the record.

The verified measurements are the [engine Case](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)
and the [Shufti refinement](https://app.perfloop.ai/t/oss/case_hqryrfd6j4).
The full local field is reproduced by [`scripts/reproduce.sh`](scripts/reproduce.sh).

## Limits

- **Counting every match:** `Find` returns one leftmost match; there is no public
  iterator yet. More importantly, rebar's worst count-all row exposes a
  multi-pattern plan that filters on common Cyrillic starts instead of rare
  interior pairs. A measured one-pass enumerator did not improve it. See
  [`REBAR.md`](REBAR.md) for the counters, profiles, and controls.
- **Tiny one-shot searches:** compiling a plan costs more than
  `strings.Index` on a single short lookup.
- **Other CPUs:** AVX2 and scalar paths are correct, but the published lead is
  not claimed for them. There is no NEON kernel yet.
- **Full folding:** expansions such as `ß -> ss` are outside the relation.
- **Bad filters:** adversarial data, or a pattern set whose useful anchors are
  not yet expressible in one shared filter, can create many survivors. The exact
  plan keeps correctness and linearity, but throughput falls; the arena includes
  periodic, same-byte, and torture rows, while rebar contributes a real Russian
  multi-pattern counterexample.

## Map from the model to the code

- [`plan.go`](plan.go) compiles fold tokens, the shared state machine, and every
  conservative filter; it also contains the exact fallback transitions.
- [`matcher.go`](matcher.go) exposes the one-plan `Matcher` API.
- [`root_amd64.go`](root_amd64.go) performs runtime dispatch and connects plan
  shapes to block transitions.
- [`root_amd64.s`](root_amd64.s) contains the AVX2 and AVX-512 kernels.
- [`root_other.go`](root_other.go) is the portable implementation of the same
  filter contracts.
- [`arena/field.yaml`](arena/field.yaml) defines who is allowed into the field
  and what semantics and ISA each entrant actually ran.
