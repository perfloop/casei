# How casei works

`casei` gets most of its speed by using a few raw-byte tests before it decodes
Unicode.

## Ten seconds

It compiles each pattern set into two parts that agree:

```text
                         exact fold-token plan
                       /                       \
patterns -> compile once                         -> first correct match
                       \                       /
                         cheap raw-byte sieve
                         (64 starts per block)
```

The sieve rejects impossible starting positions without declaring a match. It
usually rejects all 64 positions in a block. The complete Unicode plan checks
any survivors. Every real start reaches that plan, along with false positives
that cost an extra check.

## One minute: searching one block

Imagine looking for `fatal panic` without case sensitivity.

Checking every possible start spends time on positions that cannot match. Fast
search engines use filters to skip that work. `casei` specializes its filters
for literal strings under Unicode simple folding. The caller asks for one
leftmost answer.

`casei` knows at compile time that this is a literal. It chooses useful byte
positions far enough apart to make accidental alignment rare when the pattern
permits it. On AVX-512 it loads those positions for 64 possible starts at once,
compares their case-normalized bytes, and intersects the resulting 64-bit masks.

```text
candidate starts       0 1 2 3 4 5 ... 63
probe at offset A      0 0 1 0 0 0 ...  0
probe at offset B      0 0 1 0 1 0 ...  0
probe at offset C      0 0 0 0 1 0 ...  0
                       -------------------- AND
survivors              0 0 0 0 0 0 ...  0
```

When no bit survives, the search advances by a block. Otherwise `casei` replays
the complete pattern at each surviving byte position.

The compiler chooses among dispersed single-byte probes, adjacent pairs,
pair-pair anchors, triples, and bounded Shufti/Teddy-style tables. The choice is
made from facts proved about the compiled patterns.

## Why Unicode does not break the sieve

Lowercasing both strings cannot implement Unicode simple folding. Each pattern
position represents an orbit of equivalent runes, whose UTF-8 encodings may
have different lengths:

```text
k    K    K       one byte, one byte, three bytes
s    S    ſ       one byte, one byte, two bytes
σ    ς    Σ       three different runes in one orbit
```

`casei` first compiles the complete relation into tokens. Valid runes map to
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

Every filter must cover each encoding of every real start. Extra survivors are
checked by the exact plan.

## Many patterns in one scan

`NewMatcher` compiles every pattern into one trie with failure transitions over
fold tokens. Depending on its size, the same state machine uses a dense
transition table or sparse edges.

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

`casei` works under a narrower contract than the general regex engines in the
field. The compiler receives literals under Unicode simple folding. It also
knows that the caller wants the first leftmost answer, which lets it design the
raw-byte sieve and exact verifier together for the whole pattern set.

Four measured layers contribute to the result:

| layer | what it buys | evidence that it matters |
|---|---|---|
| Work avoidance | Whole blocks are rejected without decoding or advancing the exact plan at every byte. | Bypassing the shape-selected routes made the median row 3.88× slower on Ice Lake and 4.28× slower on Sapphire Rapids. Three rows were unchanged within 1%; the worst Unicode multi-pattern rows were 401× and 424× slower. |
| Shared construction | One pattern set becomes one transition plan and one traversal. | Ten paired measurements compared the shared-plan candidate with the earlier per-pattern `IndexFold` loop. The median maximum `x_vs_best` across the 33 rows fell from 6.718 to 0.9123 while the semantic suite stayed green. |
| Wider native transition | AVX-512 BW handles 64 candidate starts and keeps set arithmetic in mask registers; VBMI performs byte-table lookup in registers. | Masking AVX-512 off while retaining the same plans reduced median throughput by 1.72× on Ice Lake and 1.89× on Sapphire Rapids. One Ice Lake row and three Sapphire Rapids rows favored AVX2, mostly short or Unicode verification-heavy cases. |
| Kernel scheduling | Fewer dependent operations keep the wide sieve fed. | Fusing one four-way Shufti reduction improved its 64 KiB kernel by 21.6% and the field row using it by 21.8%. Replacing the complete assembly backend with Go's experimental SIMD package then regressed a required 1 MiB row. |

The AVX2 backend keeps the same plan and semantics. The published field lead is
specifically the AVX-512 implementation.

### Why the hand-written kernels matter

A 512-bit register gives the kernel 64 lanes, while each instruction still has
latency. On Ice Lake, the `VPERMB` lookup used by the hot long-literal filter
takes three cycles to produce an answer. The core can start one lookup each
cycle. The generated Go SIMD loop starts one 64-byte block and waits for its
answer. The assembly loop keeps four independent blocks in flight, filling the
lookup unit while earlier answers arrive.

The assembly also sends lookup results straight into AVX-512 mask registers
and asks whether any of four masks survived. The generated loop builds another
vector, converts it to a mask, and moves that mask to a general register on
every 64-byte block. In larger Shufti kernels, the generated code spills lookup
tables to 144- and 512-byte stack frames; the assembly keeps them in vector
registers.

The complete-backend A/B passed correctness. Six alternating-order Ice Lake
runs put the experimental backend at 20.8–23.3 µs/op on
`single/log_miss_1mb`, versus 18.4–20.8 µs/op for assembly. The public
[archsimd Case](https://app.perfloop.ai/t/oss/case_37sjyc8f94) and the
[negative-result record](NOVELTY.md#complete-experimental-go-simd-backend-negative-result)
contain the acceptance decision and falsifier.

## Where it differs from the field

Every scoring implementation was built from source and profiled. I then read
the source, generated code, or hot disassembly where its behavior lived. The
detailed prior-art record is in [`CONTEXT.md`](CONTEXT.md). [`NOVELTY.md`](NOVELTY.md)
records the provenance of each adopted technique.

| entrant | its strength | the verified distinction |
|---|---|---|
| Vectorscan | A mature multi-regex engine with Teddy/FDR-style literal machinery and a 512-bit VBMI target. | It ran at the same vector width on the same CPUs. The full-scan miss rows therefore compare two 512-bit engines. Its all-match API is covered separately in [`REBAR.md`](REBAR.md). |
| PCRE2-JIT | Strong prefix analysis and native JIT code for a broad regex language. | `casei` spends its compile budget on the narrower literal-set contract and can derive filters that need not preserve general regex behavior. |
| rust/regex | Byte-oriented automata plus strong literal extraction and Teddy prefilters. | It carries a general regex representation; `casei` shares one fold-token literal plan and specializes raw filters around its exact start/tie contract. |
| StringZilla | Dedicated SIMD UTF-8 search, including full-fold expansions. | Full folding is a different relation. The arena times the verification needed to reduce its candidates to simple-fold semantics; `casei` represents that relation directly. |
| veloz | Excellent hand-written AVX2 ASCII single-literal search. | Its contract covers one ASCII literal. The comparison with `casei` includes both specialization and the wider ISA. |
| Rust Aho-Corasick | Strong ASCII multi-pattern DFA with a Teddy prefilter. | `casei` extends the shared-plan shape to Unicode fold orbits, variable UTF-8 widths, opaque invalid bytes, and 512-bit filters. |

The measured result comes from a complete Unicode plan behind shape-specific
raw-byte rejection, shared across the pattern set and accelerated by AVX-512
kernels. Shufti, Teddy, rare anchors, tries, failure links, `VPERMB`, and
confirmation after a candidate are known techniques. Their combination under
this contract is what produced the new result.

## How the claims are checked

1. Seeded single- and multi-pattern differentials and exhaustive
   byte-pair filter checks compare each dispatch mode with Go `regexp (?i)` on
   valid UTF-8 and the opaque-byte oracle on invalid input. Separate fuzz
   targets exercise the same oracles.
2. The filter-route and ISA ablations measure work avoidance and
   vector width separately; the Shufti case isolates one assembly change. The
   [raw two-host audit](audit/publication/README.md) includes every sample, the
   exact ablation, file hashes, and a recomputation script.
3. Every native entrant is rebuilt from pinned source and
   checked for its actual dispatched width before it can enter `x_vs_best`.
4. The 33 first-match rows run on both Ice Lake and Sapphire Rapids.
5. The direct rebar integration exposes the count-all rows where the current
   API loses and keeps those results in the record.

The verified measurements are the [engine Case](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)
and the [Shufti refinement](https://app.perfloop.ai/t/oss/case_hqryrfd6j4).
The full local field is reproduced by [`scripts/reproduce.sh`](scripts/reproduce.sh).

## Limits

- `Find` returns one leftmost match. Rebar's worst count-all row exposes a
  multi-pattern plan that filters on common Cyrillic starts. The current plan
  cannot combine rare interior pairs from several patterns. A measured one-pass
  enumerator left that cost in place. See
  [`REBAR.md`](REBAR.md) for the counters, profiles, and controls.
- Compiling a plan costs more than `strings.Index` on a single short lookup.
- AVX2 and scalar paths run the same correctness suite. The published lead
  covers AVX-512, and there is no NEON kernel yet.
- Full-fold expansions such as `ß -> ss` are outside the relation.
- Adversarial data can leave many filter survivors. The arena includes
  periodic, same-byte, and torture rows. Rebar contributes a Russian
  multi-pattern example from a real corpus.

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
  and what semantics and ISA each entrant ran.
