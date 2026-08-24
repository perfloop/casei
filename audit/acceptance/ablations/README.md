# Load-bearing mechanism ablations

These receipts answer the Distill question: does each new mechanism have a
real measured consumer, or is it complexity the final combination happens to
carry?

Each patch removes one fact from the accepted source:

- [`origin.patch`](origin.patch) bypasses the exact common-byte origin gate.
- [`confirm.patch`](confirm.patch) disables variable-width raw confirmation.
- [`tags.patch`](tags.patch) ignores the pattern-tag byte returned by the
  vector screen and rechecks every tag, matching the earlier Go replay shape.

The variants preserve match semantics. `confirm.patch` intentionally makes the
direct test that requires the specialization fail, so the performance binary
is built with `-run '^$'` after the unmodified source has passed correctness.

## Result

| removed mechanism | measured consumer | Ice Lake | Sapphire Rapids | decision |
|---|---|---:|---:|---|
| common-byte origin gate | focused N=5 field rows | worst median 1.0008, worst sample 1.066 | worst median 1.0780, worst sample 1.085 | required |
| variable-width raw confirmation | five same-contract Rebar rows | 3/5 wins, worst 2.4758 | 3/5 wins, worst 2.4500 | required |
| returned pattern tags | five same-contract Rebar rows | 4/5 wins, worst 1.1876 | 5/5 wins, worst 0.9726 | required by the two-host bar |

The synthetic N=1 miss row still wins without variable confirmation because it
has no survivors to confirm. The sparse N=5 miss rows also still win when Go
rechecks every pattern tag. Rebar enumeration is the consumer that makes both
pieces load-bearing.

## Verify

```sh
(cd audit/acceptance/ablations && sha256sum -c SHA256SUMS)
python3 audit/acceptance/ablations/summarize.py
git apply --unidiff-zero --check audit/acceptance/ablations/origin.patch
git apply --unidiff-zero --check audit/acceptance/ablations/confirm.patch
git apply --unidiff-zero --check audit/acceptance/ablations/tags.patch
```

[`field/`](field/) contains eight paired samples per focused row and host.
[`rebar/`](rebar/) contains three passes over the five same-contract rows per
variant and host. Both campaigns were pinned to core 2 and used the same native
fields as the accepted runs.
