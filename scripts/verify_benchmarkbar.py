#!/usr/bin/env python3
"""Fail unless a BenchmarkBar transcript satisfies the publication contract."""

import argparse
from collections import defaultdict
from pathlib import Path
import math
import re
from statistics import median
import sys


PREFIX = "BenchmarkBar/"
EXPECTED_ROWS = frozenset(
    {
        "multi/multi_N2_miss_log_1mb",
        "multi/multi_N512_miss_hazard_64kb",
        "multi/multi_N512_miss_log_64kb",
        "multi/multi_N64_miss_log_64kb",
        "multi/multi_N64_miss_ru_64kb",
        "multi/multi_N8_hazard_hit_1mb",
        "multi/multi_N8_hit_log_1mb",
        "multi/multi_N8_miss_hazard_1mb",
        "multi/multi_N8_miss_log_1mb",
        "multi/multi_N8_miss_ru_1mb",
        "single/code_hit_brackets_256kb",
        "single/code_miss_256kb",
        "single/kelvin_hazard_1mb",
        "single/latency_match_end_1kb",
        "single/latency_match_mid_1kb",
        "single/latency_match_start_1kb",
        "single/latency_miss_1kb",
        "single/log_hit_sparse_1mb",
        "single/log_miss_1kb",
        "single/log_miss_1mb",
        "single/log_miss_64kb",
        "single/log_needle16_64kb",
        "single/log_needle32_64kb",
        "single/log_needle3_64kb",
        "single/log_needle8_64kb",
        "single/periodic_miss_64kb",
        "single/prose_hit_dense_1mb",
        "single/prose_miss_1mb",
        "single/ru_hit_sparse_1mb",
        "single/ru_latency_miss_1kb",
        "single/ru_miss_1mb",
        "single/samechar_miss_64kb",
        "single/torture_miss_64kb",
    }
)
REQUIRED_METRICS = (
    "x_vs_best",
    "entrants",
    "candidate_active",
    "candidate_vector_bits",
    "vectorscan_active",
    "vectorscan_vector_bits",
    "vectorscan_vbmi",
)


class VerificationError(ValueError):
    pass


def benchmark_name(raw):
    return re.sub(r"-[0-9]+$", "", raw)


def parse(path):
    rows = defaultdict(list)
    with Path(path).open() as source:
        for line_number, line in enumerate(source, 1):
            fields = line.split()
            if not fields or not fields[0].startswith(PREFIX):
                continue
            sample = {}
            for metric in REQUIRED_METRICS:
                try:
                    at = fields.index(metric)
                except ValueError as err:
                    raise VerificationError(
                        f"{path}:{line_number}: missing {metric}"
                    ) from err
                if at == 0:
                    raise VerificationError(
                        f"{path}:{line_number}: {metric} has no value"
                    )
                try:
                    value = float(fields[at - 1])
                except ValueError as err:
                    raise VerificationError(
                        f"{path}:{line_number}: invalid {metric} value {fields[at - 1]!r}"
                    ) from err
                if not math.isfinite(value):
                    raise VerificationError(
                        f"{path}:{line_number}: non-finite {metric} value"
                    )
                sample[metric] = value
            name = benchmark_name(fields[0])[len(PREFIX):]
            rows[name].append(sample)
    return rows


def verify(path, expected_samples=3):
    rows = parse(path)
    found = set(rows)
    if found != EXPECTED_ROWS:
        raise VerificationError(
            f"{path}: row inventory differs; "
            f"missing={sorted(EXPECTED_ROWS - found)}, "
            f"unexpected={sorted(found - EXPECTED_ROWS)}"
        )

    wrong_counts = {
        name: len(samples)
        for name, samples in rows.items()
        if len(samples) != expected_samples
    }
    if wrong_counts:
        raise VerificationError(f"{path}: wrong sample counts: {wrong_counts}")

    for name, samples in rows.items():
        for sample_number, sample in enumerate(samples, 1):
            label = f"{name} sample {sample_number}"
            ratio = sample["x_vs_best"]
            if not 0 < ratio < 1:
                raise VerificationError(
                    f"{path}: {label} loses with x_vs_best={ratio:g}"
                )
            if sample["entrants"] < 2:
                raise VerificationError(
                    f"{path}: {label} is unmeasured with entrants={sample['entrants']:g}"
                )
            expected = {
                "candidate_active": 1,
                "candidate_vector_bits": 512,
                "vectorscan_active": 1,
                "vectorscan_vector_bits": 512,
                "vectorscan_vbmi": 1,
            }
            for metric, want in expected.items():
                if sample[metric] != want:
                    raise VerificationError(
                        f"{path}: {label} has {metric}={sample[metric]:g}, want {want}"
                    )

    medians = {
        name: median(sample["x_vs_best"] for sample in samples)
        for name, samples in rows.items()
    }
    worst_row = max(medians, key=medians.get)
    worst_sample = max(
        sample["x_vs_best"]
        for samples in rows.values()
        for sample in samples
    )
    median_speedup = median(1 / ratio for ratio in medians.values())
    entrant_counts = [
        int(sample["entrants"])
        for samples in rows.values()
        for sample in samples
    ]
    return (
        f"PASS: 33/33 rows; worst median {worst_row}={medians[worst_row]:.4f}; "
        f"worst sample={worst_sample:.4f}; median speedup={median_speedup:.2f}x; "
        f"entrants={min(entrant_counts)}-{max(entrant_counts)}; "
        "casei=512-bit; Vectorscan=512-bit VBMI"
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("transcript", type=Path)
    parser.add_argument("--samples", type=int, default=3)
    args = parser.parse_args()
    if args.samples < 1:
        parser.error("--samples must be positive")
    try:
        print(verify(args.transcript, args.samples))
    except (OSError, VerificationError) as err:
        print(f"FAIL: {err}", file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
