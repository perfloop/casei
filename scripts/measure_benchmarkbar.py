#!/usr/bin/env python3
"""Adapt a complete one-sample BenchmarkBar run to claim metrics.

The acceptance marker is deliberately derived by the repository verifier rather
than by an aggregate ratio calculation. A baseline may legitimately lose rows,
so the adapter first validates the complete field and dispatch contract without
the win predicate, then emits zero when the strict verifier finds any losing
row. A claim requiring this marker to rise from zero to one therefore requires
every row to satisfy x_vs_best < 1.
"""

import argparse
import json
from pathlib import Path

import verify_benchmarkbar as verify


def summarize(path: Path) -> dict[str, float]:
    # This pass still requires the exact row inventory, one sample per row,
    # entrant counts, and every dispatch invariant on both comparison arms.
    verify.verify(path, expected_samples=1, require_wins=False)
    try:
        # Keep the strict repository verifier as the authority for the binding
        # every-row win rule. The expected baseline is allowed to produce zero;
        # the declared binary marker makes a losing candidate fail the claim.
        verify.verify(path, expected_samples=1)
    except verify.VerificationError:
        all_rows_winning = 0.0
    else:
        all_rows_winning = 1.0

    rows = verify.parse(path)
    samples = [sample for row in rows.values() for sample in row]
    ratios = [sample["x_vs_best"] for sample in samples]
    return {
        "benchmarkbar_all_rows_winning": all_rows_winning,
        "benchmarkbar_worst_x_vs_best": max(ratios),
        "benchmarkbar_rows_below_one": float(sum(ratio < 1 for ratio in ratios)),
        "benchmarkbar_min_entrants": min(sample["entrants"] for sample in samples),
        "benchmarkbar_candidate_vector_bits": min(
            sample["candidate_vector_bits"] for sample in samples
        ),
        "benchmarkbar_vectorscan_vector_bits": min(
            sample["vectorscan_vector_bits"] for sample in samples
        ),
        "benchmarkbar_vectorscan_vbmi": min(
            sample["vectorscan_vbmi"] for sample in samples
        ),
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    for metric, value in summarize(args.report).items():
        print(json.dumps({"metric": metric, "value": value}))


if __name__ == "__main__":
    main()
