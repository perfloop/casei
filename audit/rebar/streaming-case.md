# Streaming enumerator Case receipt

Status: negative result

Perfloop Case `case_7241g226cv` tested one construction for the remaining
Rebar gap. This file keeps the public-safe result in the repository. The Case
has no selected Candidate, so its anonymous view cannot expose this unselected
Candidate or its internal review record.

## Contract

Replace repeated `Matcher.Each` restarts with one plan-owned block scan. Carry
position, survivor masks, confirmed widths, tags, and non-overlap state across
matches. Preserve Unicode simple folding, byte offsets, leftmost order, lowest
pattern index on ties, and one compiled engine. Win all 18 pinned Rebar rows
without losing any of the 36 arena rows.

## Candidate

Candidate `cand_w18rjmtajp` compiled at most eight ASCII-spellable literals
into three tagged `VPERMB` prefix tables. It classified 4096-byte regions into
survivor masks and used exact confirmation plus bounded decoded halos for
Unicode matches.

The recorded source identity is:

| Field | Value |
| --- | --- |
| Candidate commit | `791b796a088ed5f44b357f8ce4f51bebdcf95552` |
| Source SHA-256 | `3dfd4ae560097bba8c19095ef8f84c978e5b4d2e439bfd5f0cb3e392c07e9f25` |
| Comparison commit | `769d9e90f65e4ed3e38a91ca51983c8aafc3a41f` |
| Verification | `ver_b404s4zdn6` |

This receipt transcribes the immutable Perfloop Candidate and Verification
records read on 2026-09-01.

The Candidate passed `go test`, race, vet, pure-Go, AVX-512-disabled, 386 and
arm64 compile, formatting, baseline-isolation, plan-shape, and enumeration
contract checks.

## Independent result

The verifier rejected the complete result:

- 9 of 18 Rebar rows were below the fastest eligible field entrant;
- the worst Sapphire Rapids `x_vs_best` was 4.9320;
- one same-command paired loss was 111.78 us for the Candidate versus 32.95 us
  for Hyperscan;
- the separate 36-row arena run stayed below 1.0, which did not satisfy the
  all-Rebar contract.

The field command used Rebar commit
`463d00f31887e84c38467805b9e3122c314b9521`, core 2, and the four eligible
entrants `casei`, Hyperscan, PCRE2-JIT, and rust/regex. The exact field setup and
row filter are in [this audit's README](README.md).

Later Sapphire Rapids counters also falsified the proposed cost mechanism. The
five losing rows already used the intended direct AVX-512/VBMI routes. The
single-pattern losses had no decoded restarts. The five-pattern loss decoded
only about 14 KiB of an 899 KiB corpus and still lost by 6.62x. Generic restart
or decoded-window replay could not explain the remaining multiple.

## What to carry forward

[`NOVELTY.md`](../../NOVELTY.md) records the refuted cells, the unproven lead,
and the evidence that would falsify this negative result.

The complete 18/18 result remains owned by the
[Casei Rebar Initiative](https://app.perfloop.ai/t/oss/init_y6kff75c02).
