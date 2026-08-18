# casei

`casei` searches UTF-8 text without lowercasing it first. `IndexFold` finds one
literal. A compiled `Matcher` finds the leftmost of many literals in one scan.
Both use Unicode simple case folding, the same relation as Go's `regexp (?i)`
on valid UTF-8.

On Intel Ice Lake and Sapphire Rapids with AVX-512F/BW/VBMI, `casei` finished
first on all 33 rows of its open first-match benchmark. The median speedup over
the fastest correct alternative was 1.9x on Ice Lake and 1.7x on Sapphire
Rapids. That claim covers the AVX-512 path only.

I built the engine as a hard, self-contained test for
[Perfloop](https://app.perfloop.ai). I supplied the problem and constraints;
Perfloop generated candidates and measured them against the field. An independent
verifier then tried to break the survivor. [The full engine Case](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1)
is public.

## Use it

```sh
go get github.com/tsenart/casei
```

```go
// One needle. Cache hits allocate nothing.
if casei.ContainsFold(line, "payment declined") {
    alert(line)
}

// Byte offset instead of a bool.
at := casei.IndexFold(line, "payment declined") // -1 when absent

// Many needles, one pass. Leftmost match wins; ties go to the lowest
// pattern index.
m := casei.NewMatcher([]string{"fatal panic", "oom killed", "segfault"})
if match, ok := m.Find(line); ok {
    fmt.Println(m.Patterns()[match.Pattern], match.Start)
}
```

`NewMatcher` compiles the pattern set once. Reuse the `*Matcher` across searches
and share it freely; `Find` is safe for concurrent use. Every published
benchmark path allocates nothing after compilation, including cache-hit
`IndexFold`. Compiling a plan can allocate. During a search, the generic Unicode
plan allocates an offset ring only when its longest pattern needs more than the
256 entries kept inline.

On valid UTF-8, matching is Unicode **simple** case folding, identical to Go's
`regexp` with `(?i)`: `k` matches the Kelvin sign U+212A, `ſ` matches `s`,
`σ`/`ς`/`Σ` all match, and `ß` matches `ẞ` but never `ss`. Invalid bytes are
matched as opaque one-byte units. Lowercasing both sides does not have these
semantics. [Here is why](HOW_IT_WORKS.md#why-unicode-does-not-break-the-sieve).

Requires Go 1.22+. The AVX-512 and AVX2 paths are chosen at runtime on x86-64;
every other platform runs the portable path, which returns identical results
(see [Limitations](#limitations) for what that costs).

## Why it is fast

`casei` gets most of its speed by proving where a match cannot start.

Compilation produces two views of the same patterns:

```text
patterns -> complete simple-fold plan -> exact answer
        \-> conservative byte filters -> 64 starts at once -> survivors only
```

The byte sieve rejects 64 impossible starts at a time. A surviving bit means
only “maybe.” The complete fold plan checks every survivor and decides Unicode
equivalence, byte offsets, leftmost order, and pattern ties, so a false positive
only adds work.

Both APIs use the same fold-token state machine. A multi-needle matcher walks
the haystack once, and AVX-512 runs its sieve over 64 possible starts at a time.
A scheduling change in one hand-written Shufti kernel improved its contested
row by 21.8%. Replacing the complete backend with Go's experimental SIMD
package passed correctness and slowed a required field row. Across all 33 rows,
bypassing the shape-selected filters made the median row 3.88x slower on Ice
Lake and 4.28x slower on Sapphire Rapids.

[The one-page explanation](HOW_IT_WORKS.md) walks from that mental model to the
actual plan, kernels, competitor differences, causal measurements, and limits.

## Results

`BenchmarkBar` timed `casei` and every eligible competitor built from pinned
source in the same benchmark process, with each entrant dispatching its widest
eligible path. The complete tables below come from three fresh passes on each
GCP host, one exposing Ice Lake and one exposing Sapphire Rapids. Perfloop also
ran ten co-measured pairs of the pre-engine and final source, choosing the two
source arms' order randomly inside each pair; that public Case used the worst
`x_vs_best` across all 33 rows as its acceptance metric.

Perfloop's verified runs put **casei first on every one of 33 rows, on both
microarchitectures**. The median speedup over the fastest correct alternative
was **1.9x** on Ice Lake and **1.7x** on Sapphire Rapids. The narrowest lead was
1.07x and the widest was 25.8x. The short table below shows selected Sapphire
Rapids rows; both complete tables follow. Throughput is in GB/s; **bold =
casei**. Values are rounded to one decimal, so `0.0` means below 0.05 GB/s.
`casei vs #2` is casei over the fastest other engine on that row.

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `log_miss_1mb` | **56.4** | 51.3 | 8.3 | 23.3 | 12.3 | 9.0 | **1.10×** |
| `code_miss_256kb` | **56.1** | 29.1 | 8.3 | 23.3 | 11.5 | 9.1 | **1.93×** |
| `prose_miss_1mb` | **56.3** | 19.5 | 8.3 | 23.2 | 12.1 | 9.0 | **2.43×** |
| `ru_miss_1mb` | **27.5** | 16.5 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.5 | 0.0 | 0.5 | **1.42×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 4.6 | – | 0.0 | 0.0 | 0.5 | **2.14×** |
| `latency_match_start_1kb` | **118.1** | 2.9 | 70.1 | 4.6 | 4.4 | 3.3 | **1.68×** |
| `samechar_miss_64kb` | **67.6** | 44.7 | 8.3 | 22.3 | 11.0 | 0.5 | **1.51×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.4 | 11.0 | 0.5 | **1.25×** |
| `torture_miss_64kb` | **13.1** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.76×** |
| `log_hit_sparse_1mb` | **32.1** | 1.5 | 8.0 | 7.2 | 10.3 | 6.6 | **3.11×** |

<details>
<summary><b>Full 33-row tables for both CPUs</b></summary>

The visible columns show the six engines with lanes on all or most rows;
`rust/regex` is the rure adapter. Go `regexp` and Rust Aho-Corasick are omitted
from this display, but both are timed and enter `x_vs_best` wherever eligible.
The ratio therefore still includes every eligible scoring entrant.

#### Sapphire Rapids (Xeon 8481C), GB/s (higher is better; **bold** = casei)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **118.1** | 2.9 | 70.1 | 4.6 | 4.4 | 3.3 | **1.68×** |
| `samechar_miss_64kb` | **67.6** | 44.7 | 8.3 | 22.3 | 11.0 | 0.5 | **1.51×** |
| `log_miss_1mb` | **56.4** | 51.3 | 8.3 | 23.3 | 12.3 | 9.0 | **1.10×** |
| `prose_miss_1mb` | **56.3** | 19.5 | 8.3 | 23.2 | 12.1 | 9.0 | **2.43×** |
| `code_miss_256kb` | **56.1** | 29.1 | 8.3 | 23.3 | 11.5 | 9.1 | **1.93×** |
| `log_miss_64kb` | **53.4** | 45.2 | 8.3 | 22.3 | 12.3 | 8.9 | **1.18×** |
| `log_needle3_64kb` | **53.3** | 45.0 | 8.3 | 22.1 | 18.0 | 13.8 | **1.19×** |
| `log_needle32_64kb` | **53.3** | 6.8 | 8.3 | 21.0 | 11.0 | 8.9 | **2.54×** |
| `log_needle16_64kb` | **53.3** | 36.0 | 8.3 | 22.1 | 11.8 | 8.9 | **1.48×** |
| `log_needle8_64kb` | **53.0** | 6.8 | 8.3 | 20.9 | 18.0 | 8.9 | **2.53×** |
| `multi_N8_miss_ru_1mb` | **38.6** | 5.7 | – | 23.3 | 0.8 | 9.0 | **1.66×** |
| `multi_N64_miss_ru_64kb` | **37.0** | 7.2 | – | 21.8 | 0.1 | 0.5 | **1.70×** |
| `multi_N8_hazard_hit_1mb` | **35.5** | 6.7 | – | 2.7 | 0.9 | 31.6 | **1.13×** |
| `periodic_miss_64kb` | **35.5** | 0.6 | 8.3 | 28.4 | 11.0 | 0.5 | **1.25×** |
| `log_hit_sparse_1mb` | **32.1** | 1.5 | 8.0 | 7.2 | 10.3 | 6.6 | **3.11×** |
| `multi_N8_miss_log_1mb` | **29.3** | 6.8 | – | 14.3 | 1.6 | 9.0 | **2.04×** |
| `multi_N64_miss_log_64kb` | **27.7** | 6.8 | – | 22.0 | 0.2 | 0.5 | **1.26×** |
| `multi_N512_miss_log_64kb` | **27.7** | 6.8 | – | 19.5 | 0.0 | 0.5 | **1.42×** |
| `ru_miss_1mb` | **27.5** | 16.5 | – | 22.8 | 6.5 | 9.0 | **1.21×** |
| `ru_hit_sparse_1mb` | **24.5** | 0.8 | – | 19.3 | 6.5 | 8.5 | **1.27×** |
| `latency_match_mid_1kb` | **22.7** | 2.4 | 14.5 | 2.6 | 3.8 | 2.5 | **1.57×** |
| `kelvin_hazard_1mb` | **20.3** | 1.8 | – | 1.2 | 12.8 | 8.5 | **1.58×** |
| `multi_N8_miss_hazard_1mb` | **18.3** | 6.8 | – | 0.3 | 0.9 | 2.8 | **2.67×** |
| `multi_N2_miss_log_1mb` | **15.2** | 11.5 | – | 0.7 | 5.8 | 5.5 | **1.32×** |
| `log_miss_1kb` | **13.7** | 5.5 | 7.9 | 5.4 | 5.5 | 4.0 | **1.74×** |
| `latency_match_end_1kb` | **13.5** | 2.4 | 7.5 | 1.7 | 3.2 | 2.0 | **1.80×** |
| `latency_miss_1kb` | **13.4** | 4.6 | 7.9 | 4.9 | 5.4 | 3.9 | **1.69×** |
| `prose_hit_dense_1mb` | **13.1** | 0.0 | 6.8 | 1.0 | 4.2 | 2.9 | **1.93×** |
| `torture_miss_64kb` | **13.1** | 0.1 | 0.5 | 0.3 | 0.1 | 0.3 | **25.76×** |
| `code_hit_brackets_256kb` | **11.1** | 0.0 | 6.0 | 1.1 | 1.3 | 0.9 | **1.86×** |
| `multi_N8_hit_log_1mb` | **10.2** | 5.7 | – | 2.0 | 1.8 | 2.5 | **1.78×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 4.6 | – | 0.0 | 0.0 | 0.5 | **2.14×** |
| `ru_latency_miss_1kb` | **8.5** | 3.6 | – | 5.1 | 3.9 | 3.6 | **1.65×** |

#### Ice Lake (Xeon @ 2.6 GHz), GB/s (higher is better; **bold** = casei)

| row | casei | Vectorscan | veloz | PCRE2-JIT | StringZilla | rust/regex | casei vs #2 |
|---|---|---|---|---|---|---|---|
| `latency_match_start_1kb` | **118.2** | 2.5 | 63.6 | 4.2 | 3.7 | 3.1 | **1.86×** |
| `samechar_miss_64kb` | **71.7** | 39.0 | 6.9 | 22.7 | 11.0 | 0.6 | **1.84×** |
| `prose_miss_1mb` | **57.2** | 16.7 | 6.8 | 16.4 | 12.2 | 9.5 | **3.43×** |
| `log_miss_1mb` | **57.1** | 45.0 | 6.8 | 21.8 | 12.2 | 9.4 | **1.27×** |
| `code_miss_256kb` | **56.9** | 23.1 | 6.9 | 19.1 | 11.5 | 9.6 | **2.47×** |
| `log_miss_64kb` | **54.5** | 38.9 | 6.8 | 19.8 | 11.9 | 9.3 | **1.40×** |
| `log_needle32_64kb` | **52.8** | 6.9 | 6.9 | 16.0 | 11.0 | 9.4 | **3.31×** |
| `log_needle8_64kb` | **52.8** | 6.9 | 6.9 | 16.1 | 15.5 | 9.2 | **3.28×** |
| `log_needle16_64kb` | **52.8** | 28.2 | 6.9 | 15.8 | 11.7 | 9.3 | **1.87×** |
| `log_needle3_64kb` | **52.6** | 38.9 | 6.9 | 16.5 | 15.3 | 14.3 | **1.35×** |
| `multi_N8_miss_ru_1mb` | **37.0** | 6.1 | – | 16.5 | 0.8 | 9.5 | **2.24×** |
| `multi_N8_hazard_hit_1mb` | **35.4** | 7.7 | – | 3.0 | 1.0 | 33.0 | **1.07×** |
| `multi_N64_miss_ru_64kb` | **35.2** | 5.8 | – | 15.7 | 0.1 | 0.5 | **2.24×** |
| `multi_N8_miss_log_1mb` | **31.1** | 7.0 | – | 13.3 | 1.6 | 9.5 | **2.33×** |
| `periodic_miss_64kb` | **30.9** | 0.5 | 6.9 | 23.5 | 11.0 | 0.6 | **1.32×** |
| `multi_N64_miss_log_64kb` | **29.5** | 6.9 | – | 20.4 | 0.2 | 0.5 | **1.45×** |
| `multi_N512_miss_log_64kb` | **29.5** | 6.9 | – | 13.8 | 0.0 | 0.5 | **2.13×** |
| `log_hit_sparse_1mb` | **27.6** | 1.5 | 6.7 | 6.9 | 10.3 | 6.8 | **2.67×** |
| `ru_miss_1mb` | **21.7** | 17.4 | – | 16.4 | 6.4 | 9.6 | **1.25×** |
| `kelvin_hazard_1mb` | **21.0** | 1.9 | – | 1.1 | 12.3 | 8.9 | **1.70×** |
| `multi_N8_miss_hazard_1mb` | **18.5** | 7.5 | – | 0.3 | 0.9 | 3.1 | **2.46×** |
| `latency_match_mid_1kb` | **18.5** | 2.0 | 12.0 | 2.3 | 3.2 | 2.4 | **1.54×** |
| `ru_hit_sparse_1mb` | **18.3** | 0.9 | – | 16.0 | 6.2 | 8.8 | **1.15×** |
| `multi_N2_miss_log_1mb` | **15.0** | 11.6 | – | 0.7 | 5.9 | 5.8 | **1.29×** |
| `log_miss_1kb` | **13.3** | 4.4 | 6.6 | 4.6 | 4.8 | 3.6 | **2.03×** |
| `latency_miss_1kb` | **12.8** | 3.8 | 6.6 | 4.2 | 4.7 | 3.6 | **1.95×** |
| `prose_hit_dense_1mb` | **12.0** | 0.0 | 5.8 | 1.0 | 3.6 | 2.8 | **2.06×** |
| `latency_match_end_1kb` | **11.0** | 2.0 | 6.2 | 1.6 | 2.9 | 2.0 | **1.79×** |
| `torture_miss_64kb` | **10.2** | 0.1 | 0.4 | 0.2 | 0.1 | 0.3 | **25.51×** |
| `multi_N8_hit_log_1mb` | **10.1** | 5.9 | – | 1.9 | 1.7 | 2.8 | **1.71×** |
| `multi_N512_miss_hazard_64kb` | **9.8** | 3.9 | – | 0.0 | 0.0 | 0.5 | **2.53×** |
| `code_hit_brackets_256kb` | **9.1** | 0.0 | 5.0 | 1.0 | 1.1 | 0.7 | **1.82×** |
| `ru_latency_miss_1kb` | **8.0** | 3.1 | – | 4.6 | 3.5 | 3.4 | **1.74×** |

Diagnostic baselines (`ToLower`+`Index`, the Go Aho-Corasick port, and the
exact-match `ceiling`) are omitted from the “fastest” comparison. The
[methodology](#benchmark-method) explains why. Rebuild the field and rerun the
local board with `./scripts/reproduce.sh`.
</details>

All 33 rows cover first-match search. They include ASCII and UTF-8 workloads,
with one needle or many, on both processors. Vectorscan used its 512-bit
AVX-512 VBMI path on the same machines. This gives the field an equal-width
control alongside narrower engines such as AVX2 veloz and 128-bit PCRE2-JIT.
Every benchmark row reports the width each entrant used.

Rebar measures `count` and `count-spans`. On the five performance rows that
share `casei`'s Unicode contract, the loop-over-`Find` adapter wins two and
loses three on both hosts. The worst row spends its time behind a weak shared
filter choice; a stateful enumerator left that cost in place. The
[complete rebar audit](REBAR.md) lists every applicable row and the controls
used to trace those losses.

On valid UTF-8, correctness is pinned to Go `regexp` `(?i)` by deterministic
single- and multi-pattern differentials. The suite runs under both x86 vector
paths. The portable scalar dispatch runs it too. It includes tens of thousands
of randomized searches and two exhaustive 65,536-pair filter checks.
`FuzzIndexFold` and `FuzzMatcher` run separately. Invalid-byte inputs are checked
against the opaque-unit contract.

## Reproduce it

With Go 1.24+ on an x86-64 Linux host **with AVX2 and AVX-512F/BW/VBMI** (pin a
GCP `n2` to Ice Lake, use `c3` for Sapphire Rapids, or use equivalent recent
Intel hardware), one script builds the entire competitor field from source and
runs the scoreboard. Apple Silicon does not meet this performance-host
contract. CI rebuilds and checks the same pinned field for correctness on every
push.

```sh
git clone https://github.com/tsenart/casei && cd casei
./scripts/reproduce.sh          # ~15 min: builds pcre2, vectorscan (VBMI), rure,
                                # rust-regex, stringzilla, then runs the benchmark
```

It prints, for all 33 rows, every entrant's local throughput and the vector
width it dispatched, plus `x_vs_best` (`casei`'s time ÷ the fastest *correct*
competitor). It then fails unless all three samples of every row are below 1,
every row has at least two entrants, and both `casei` and Vectorscan report
512-bit dispatch with Vectorscan's VBMI path active. Perfloop's public Case
separately records ten co-measured pre-engine/final-source pairs, with random
source-arm order, for the board's worst `x_vs_best`.

The [publication audit](audit/publication/README.md) records a fresh three-pass
acceptance run on both CPU models, the work-avoidance and AVX-512 ablations,
raw samples, and the script that recomputes their summaries.

## Benchmark method

<details>
<summary><b>Read the field, scoring, and measurement rules</b></summary>

The arena applies the following rules:

- A baseline enters `x_vs_best` after its output passes the agreement tests for
  that tier. `ToLower` plus `Index` and the Go Aho-Corasick port remain
  diagnostic lanes.
- Each row is scored against its fastest eligible competitor.
- Entrants report the ISA and vector width they dispatched. Vectorscan is built
  with `BUILD_AVX512VBMI`, and the arena checks that its 512-bit path ran.
- The workload set includes `periodic`, `samechar`, and `torture` inputs that
  expose data-dependent cliffs.
- The measured answer is a first byte offset, or a leftmost match with ties
  resolved by pattern order. An entrant with an enumeration API performs that
  reduction inside its timed operation.
- [`arena/field.yaml`](arena/field.yaml) pins nine engines to source versions
  and build flags. Perfloop's engine Case records ten co-measured source pairs.
  `reproduce.sh` rebuilds the field and runs the local board.

I wrote the arena alongside `casei`. Its source, workloads, field manifest, and
measurements are open for independent runs.

</details>

## Limitations

- The measured performance result covers AVX-512. Other x86 machines use the
  AVX2 path. ARM uses the portable scalar path, with no NEON kernel yet. All
  three dispatch modes run the correctness suite.
- `NewMatcher` is meant to be reused. On a tiny one-shot lookup, plan setup
  gives `strings.Index` the advantage.
- The contract is Unicode simple folding. Full-fold expansions such as
  `ß` to `ss` are outside it.
- The public API returns the first match. The rebar adapter enumerates by
  calling `Find` again on each suffix and loses three of five comparable rows
  on both measured hosts. [`REBAR.md`](REBAR.md) includes those results and the
  ASCII-only rows that ask for weaker folding semantics.

## How it was built

I used `casei` as an operator-directed Perfloop case. I supplied the hypotheses
and audited the field and host ISA; Perfloop generated candidates and killed or
kept them by measurement. The public trails cover the
[engine](https://app.perfloop.ai/t/oss/case_9r9ntnxjd1) and a later
[kernel-scheduling refinement](https://app.perfloop.ai/t/oss/case_hqryrfd6j4).
The repository contains the resulting source, field manifest, correctness
tests, and reproduction scripts.

## Details

- [`HOW_IT_WORKS.md`](HOW_IT_WORKS.md): the short mental model first, followed
  by the exact plan, assembly contribution, competitor comparison, and
  evidence.
- [`REBAR.md`](REBAR.md): every applicable third-party rebar workload, the
  semantic map, both-host measurements, real losses, and their diagnosis.
- [`arena/field.yaml`](arena/field.yaml): the field, versions, build flags,
  ISA, corpus hashes, semantic status.
- [`CONTEXT.md`](CONTEXT.md): every technique known to this problem, with
  sources and measured numbers (including rebar's published results).
- [`NOVELTY.md`](NOVELTY.md): the construction and prior-art assessment; the fold-orbit
  representation is *not* claimed as novel, and says why.
- [`AGENTS.md`](AGENTS.md): the arena's rules of engagement, baseline isolation,
  single-engine identity, and the acceptance bar a candidate must clear.
