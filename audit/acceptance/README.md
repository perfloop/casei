# Current two-host acceptance record

This directory holds the raw `BenchmarkBar` transcripts used by the current
README claim.

| host | rows | worst median | worst sample | median speedup | entrants |
|---|---:|---:|---:|---:|---:|
| Ice Lake, family 6/model 106 | 36/36 | 0.9614 | 0.9622 | 1.79× | 5-7 |
| Sapphire Rapids, family 6/model 143 | 36/36 | 0.9693 | 0.9716 | 1.55× | 5-7 |

Every row has three samples. Every sample has `x_vs_best < 1`. `casei` reports
512-bit dispatch, Vectorscan reports a 512-bit VBMI database, and the verifier
checks every other active entrant against its declared width.

## Verify the receipts

```sh
(cd audit/acceptance && sha256sum -c SHA256SUMS)
sha256sum -c audit/acceptance/SOURCE_SHA256SUMS
python3 audit/acceptance/summarize.py
python3 scripts/verify_benchmarkbar.py \
  audit/acceptance/results/ice/benchmarkbar.txt
python3 scripts/verify_benchmarkbar.py \
  audit/acceptance/results/spr/benchmarkbar.txt
```

The verifier requires the exact 36-row inventory, three samples per row,
at least two entrants, all dispatch metrics, and every `x_vs_best` below 1.0.

[`ablations/`](ablations/README.md) removes the origin gate, variable raw
confirmation, and returned pattern tags one at a time. Each removal breaks its
claimed field or Rebar consumer on at least one target host.

## Measurement order

For each row, `BenchmarkBar` pairs `casei` separately with every eligible
competitor. A pair contains six 25 ms windows. The order alternates, giving
each operation three first positions. The median paired ratio is the row's
ratio against that competitor. The largest ratio is `x_vs_best` because it
corresponds to the fastest competitor.

The complete board was run three times with:

```sh
taskset -c 2 go test -run '^$' -bench '^BenchmarkBar$' \
  -benchtime 30x -count 3
```

Before timing, both hosts passed the full arena agreement suite against the
native field. The field was built from the pinned prepare scripts with
Vectorscan's AVX-512 VBMI target enabled.

## Native reachability

The three native entries added or materially changed by this result are
`literalSkipExact64`, `pairPairConfirmVBMI64`, and
`rawByteMultiAnchorSkip64`. One-shot GDB breakpoints observed all three while
their direct model tests passed on both hosts.

```sh
go test -c -o casei.test .
gdb -q -batch -x audit/acceptance/native-reachability.gdb ./casei.test
```

The command file is [`native-reachability.gdb`](native-reachability.gdb). The
captured outputs are [`results/ice/gdb-native.txt`](results/ice/gdb-native.txt)
and [`results/spr/gdb-native.txt`](results/spr/gdb-native.txt). Each receipt
must contain `HIT 1`, `HIT 2`, `HIT 3`, and a final `PASS`.
[`SOURCE_SHA256SUMS`](SOURCE_SHA256SUMS) pins the source, direct tests, and GDB
command file used to build both binaries. Both hosts reported the same checksum
stream, `e797ffa1edc73b5195db9f066ad549ec5b4ccf7e29777e35c567a11cb5f16f2e`.

## Why failed runs are kept

[`methodology/`](methodology/) contains the earlier sequential-window runs.
They measured the candidate and field in distant windows and produced unstable
near-parity N=5 rows, including losing samples. Those receipts failed the
publication bar. They are retained so the change to paired timing and the
reason for it remain auditable.

[`results/`](results/) contains the paired acceptance run. The acceptance rule
did not change: one row at or above 1.0 fails the board.
