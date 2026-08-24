# Rebar audit artifacts

This directory contains the adapter and measurements behind
[`REBAR.md`](../../REBAR.md).

The current receipts say:

- `casei` wins all 5/5 rows with the same Unicode contract on both hosts;
- the worst same-contract ratio is 0.8766 on Ice Lake and 0.8919 on Sapphire
  Rapids;
- across all 18 representable stress rows, including 13 ASCII-only contracts,
  `casei` wins 9/18 on each host.

## What is here

- [`runner/main.go`](runner/main.go) compiles a `Matcher` once, validates full
  non-overlapping enumeration against an independent simple-fold oracle, then
  times `Matcher.Each` with only the count or span sink.
- [`prepare.py`](prepare.py) registers that runner on all 18 performance rows
  and three behavior checks in the pinned Rebar checkout.
- [`results/`](results/) contains three CSV passes from each host and their
  SHA-256 receipt file.
- [`summarize.py`](summarize.py) validates the inventory and error columns,
  selects the fastest competitor on each pass, and recomputes every ratio in
  `REBAR.md`.

Verify the checked-in record from any directory:

```sh
(cd audit/rebar/results && sha256sum -c SHA256SUMS)
python3 audit/rebar/summarize.py
```

## Reproduce on a qualifying Linux host

Check out `casei` and Rebar as siblings:

```sh
git clone https://github.com/tsenart/casei.git
git clone https://github.com/BurntSushi/rebar.git
git -C rebar checkout 463d00f31887e84c38467805b9e3122c314b9521
cd rebar
python3 ../casei/audit/rebar/prepare.py
cargo build --release --bin rebar
./target/release/rebar build -e '^(casei|hyperscan|pcre2/jit|rust/regex)$'
```

`prepare.py` refuses any other Rebar commit. It also omits the unused
`pcre2posix.c` wrapper because this pinned snapshot lacks its header. Rebar's
native PCRE2 API and JIT sources are unchanged.

The exact 18-row selection is:

```sh
filter='^(curated/(01-literal|02-literal-alternate)/sherlock-casei-(en|ru)|hyperscan/literal-casei-(english|russian)-(no)?som|imported/leipzig/(twain-insensitive|tom-sawyer-huckle-fin-insensitive)|imported/sherlock/(name-(sherlock|holmes|sherlock-holmes|alt3|alt5)-casei|the-casei)|opt/prefilter/literal-casei-(english|russian))$'

taskset -c 2 ./target/release/rebar measure \
  -e '^(casei|hyperscan|pcre2/jit|rust/regex)$' \
  -f "$filter" \
  --max-warmup-iters 100 --max-warmup-time 200ms \
  --max-iters 1000 --max-time 500ms > rebar-audit-pass1.csv
```

Repeat for passes two and three. The checked-in record used core 2 on both
hosts. The runner validates every answer in an untimed preflight; a mismatch
appears in the CSV `err` column and makes `summarize.py` fail.

The compatible behavior checks can be run directly:

```sh
taskset -c 2 ./target/release/rebar measure --verify --verbose \
  -e '^casei$' \
  -f '^test/unicode/case/(ascii-with-unicode|unicode)$'
```

The excluded `test/unicode/case/ascii-only` row expects `s` to miss `ſ`.
Unicode simple folding requires them to match.
