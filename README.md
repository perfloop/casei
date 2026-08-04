# casei

An open benchmark arena for **ASCII case-insensitive substring search**.

`grep -i`, SQL `ILIKE`, log-line filters, HTTP header lookups — caseless
search is one of the most executed operations in computing, and it is far
slower than it needs to be. On the current public record, most engines pay
2–5× for case-insensitivity over exact matching, and the standard-library
answers pay much more (Go's `regexp` with `(?i)` runs about 90× slower than
its exact-match path on the same input). Exact-match search is a
heavily-optimized, well-mapped field; its caseless sibling is not.

This repository holds one function:

```go
// IndexFold returns the index of the first occurrence of needle in haystack
// under ASCII case folding, or -1 if needle is not present.
func IndexFold(haystack, needle string) int
```

Semantics: only the 52 ASCII letters fold. Every other byte compares exactly —
including the 0x20-adjacent punctuation pairs (`[`/`{`, `@`/`` ` ``, `]`/`}`,
`\`/`|`, `^`/`~`) and all bytes ≥ 0x80, so UTF-8 multibyte sequences match
byte-exactly and are never folded. `casei_test.go` and the fuzz target are
the executable definition, including the trap cases that have bitten real
SIMD implementations of this exact problem.

## The bar

The goal is an `IndexFold` that, on this repository's benchmark suite:

1. **wins every scenario** against every baseline — realistic corpora and
   adversarial inputs alike, no cherry-picking;
2. **at least doubles** the strongest baseline (geometric mean across the
   suite);
3. comes **within 10% of the exact-match ceiling** (`strings.Index` on
   pre-lowered input) — demonstrating that case-insensitivity can be
   effectively free;
4. keeps a **linear worst case** — the adversarial scenarios (`periodic`,
   `samechar`, `torture`) are in the suite precisely so that throughput
   cannot be bought with a quadratic cliff;
5. passes every test and the fuzzer, on every architecture it claims.

Pure Go and Go assembly are both in bounds. Architecture-specific fast paths
must come with a correct portable fallback.

## Baselines

| name | what it is |
|---|---|
| `candidate` | `casei.IndexFold` — the function under optimization |
| `tolower` | `strings.Index(lower(h), lower(n))` — the idiom everyone writes, allocations included |
| `regexp` | precompiled `(?i)` literal — the stdlib answer |
| `veloz` | [`mhr3/veloz`](https://github.com/mhr3/veloz) `ascii.IndexFold` — the strongest published Go SIMD caseless search (NEON on arm64, AVX2/SSE on amd64) |
| `ceiling` | `strings.Index` on pre-lowered input — exact-match physics, the target |

## Running

```sh
go test ./...                      # correctness + baseline agreement
go test -fuzz=FuzzIndexFold -fuzztime=30s
go test -bench=. -benchtime=200ms  # the arena
```

## Prior art

[`CONTEXT.md`](CONTEXT.md) catalogs every technique known to this problem —
folding primitives, SIMD prefilter designs, candidate-extraction tricks on
movemask-less ISAs, vectorized rolling hashes, adaptive stage-escalation
budgets, rare-byte statistics — with sources and measured numbers. It is the
line between engineering and invention here: **an approach only counts as new
if it is not already in that document.**
