# Historical 33-row publication verification

This directory preserves the first August 2026 publication review. It covers
the 33-row board before the three Rebar-derived rows were added. The later
36-row historical snapshot and its raw transcripts live in
[`audit/acceptance/`](../acceptance/README.md). The current publication gate is
38 rows; no checked-in 38-row acceptance receipt exists yet.

Perfloop's original public Case co-measured the pre-engine and final source in
ten pairs with random source-arm order, using the worst `x_vs_best` across all
33 rows as its metric. This historical run rebuilt that field, checked every
row, and isolated the two main speed mechanisms with the same compiled plans.

The search source was commit
`781eb8c36413f9a23c2d1f279ad9ef6554cac8bf`. The publication review then
removed two unreachable assembly bodies that the linker had already omitted
and added direct tests for the remaining kernels. No reachable search path
changed.

## Hosts and field

| host | CPUID | required features |
|---|---|---|
| Ice Lake, GCP `n2` | GenuineIntel family 6 model 106 | AVX2, AVX-512F, AVX-512BW, AVX-512VBMI |
| Sapphire Rapids, GCP `c3` | GenuineIntel family 6 model 143 | AVX2, AVX-512F, AVX-512BW, AVX-512VBMI |

Both hosts used Go 1.26.4 from the official archive. Each native dependency was
built by its checked-in `arena/*/prepare.sh` script. Vectorscan reported its
512-bit VBMI database on every row. `casei` reported 512 bits on every row.

`BenchmarkBar` was run three times per host. All 99 samples per host were below
`x_vs_best=1`, with five to seven entrants per row.

| host | slowest row by median `x_vs_best` | worst sample | median local speedup |
|---|---:|---:|---:|
| Ice Lake | `single/ru_hit_sparse_1mb`, 0.8021 | 0.8463 | 1.92x |
| Sapphire Rapids | `single/log_miss_1mb`, 0.9281 | 0.9289 | 1.58x |

These local-board medians and raw rows support the published two-host tables.
The public Perfloop Case answers a different question: how the board's worst
row changed from the pre-engine source to the final engine under co-measurement.

## Work-avoidance ablation

[`filter-ablation.patch`](filter-ablation.patch) keeps the compiled plan and
exact state transition, but sends every non-empty plan directly to
`findUnfiltered`. This bypasses the shape-specific routes normally selected by
`find`. The generic path still contains exact root and pair block transitions,
so this is not a claim that every byte-level shortcut was removed.

The normal and bypassed binaries were alternated five times per host with
`GOMAXPROCS=1` and 300 ms per row. The ratio below is bypassed time divided by
normal time.

| host | median across 33 rows | largest slowdown | rows with no measured benefit |
|---|---:|---:|---|
| Ice Lake | 3.88x | 401.36x, `multi_N64_miss_ru_64kb` | three ASCII multi-pattern miss rows, all within 1% |
| Sapphire Rapids | 4.28x | 423.87x, `multi_N8_miss_ru_1mb` | the same three rows, all within 1% |

On the three neutral rows, the selected and bypassed routes measured the same
within noise. The large Unicode miss results are where the selected byte
filters prevent most rune decoding and exact-plan work.

## AVX-512 ablation

The same binary and plans were run normally and with
`GODEBUG=cpu.avx512f=off,cpu.avx512bw=off`, again in five alternating passes per
host. The ratio is AVX2 time divided by AVX-512 time.

| host | median across 33 rows | exceptions |
|---|---:|---|
| Ice Lake | 1.72x | `ru_latency_miss_1kb` favored AVX2 by 19%; `latency_match_start_1kb` was tied within 0.2% |
| Sapphire Rapids | 1.89x | three rows favored AVX2: `ru_latency_miss_1kb` by 20%, `multi_N8_miss_ru_1mb` by 13%, and `multi_N64_miss_ru_64kb` by 12% |

AVX-512 improves the median while the short and verification-heavy exceptions
above favor AVX2. The full field claim remains restricted to the AVX-512 path.

## Correctness and assembly reachability

The root suite passed on both hosts with the normal dispatch, AVX-512 disabled,
and AVX2 plus AVX-512 disabled. The complete pinned arena agreement and dispatch
suite passed on both hosts. `FuzzIndexFold` and `FuzzMatcher` each ran for 30
seconds on both hosts.

The amd64 source at the audited commit contained 36 linked assembly entry
points. One-shot GDB breakpoints observed all 36 across the normal,
AVX-512-disabled, and BMI2-disabled test runs. Before this check, `runSkip32`
and `runSkip64` were unreferenced source and absent from the linked test binary;
they were removed.
[`asm-reachability.gdb`](asm-reachability.gdb) is the command file used for the
check. The three files named `gdb-*.txt` in `results/ice` retain the `HIT` and
final `PASS` lines captured from these runs:

```sh
go test -c -o casei.test .
gdb -q -batch -x audit/publication/asm-reachability.gdb ./casei.test
gdb -q -batch -ex 'set environment GODEBUG cpu.avx512f=off,cpu.avx512bw=off' \
  -x audit/publication/asm-reachability.gdb ./casei.test
gdb -q -batch -ex 'set environment GODEBUG cpu.bmi2=off' \
  -x audit/publication/asm-reachability.gdb ./casei.test
```

For that historical source, the union of the reported breakpoint numbers must
be 1 through 36, and each test process must print `PASS`. `summarize.py` checks
both conditions from the captured receipts. Current-kernel reachability lives
with the current acceptance record in [`audit/acceptance/`](../acceptance/README.md).

## Recompute the summaries

From this directory:

```sh
sha256sum -c SHA256SUMS
python3 summarize.py
```

The raw files retain every Go benchmark sample and dispatch metric used by the
summary.
