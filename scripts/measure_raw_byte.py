#!/usr/bin/env python3
"""Adapt the shipped raw-byte Go benchmarks to the measurement contract."""

import argparse
import json
import re
from pathlib import Path


SPECS = {
    "steady_state": (
        "BenchmarkRawByteDensity",
        (
            "two_4KiB_one_in_32",
            "two_4KiB_zero_admission",
            "five_one_in_64",
            "five_one_in_4",
        ),
    ),
    "construction": (
        "BenchmarkRawByteConstruction",
        (
            "two_patterns_512B",
            "two_patterns_4KiB",
            "five_patterns_512B",
            "five_patterns_513B_zero_admission",
            "five_patterns_5MiB_raw_install_and_first_use",
            "two_shared_100_units_1KiB_zero_admission",
            "two_mixed_folded_ascii_root_1KiB",
        ),
    ),
    "fresh_find": (
        "BenchmarkRawByteFreshFind",
        (
            "five_long_false_one_in_256",
            "five_long_near_miss_one_in_256",
            "five_long_sparse_one_in_8192",
            "five_long_clustered",
            "five_long_early_match_one_in_256",
            "five_long_late_match_one_in_256",
        ),
    ),
    "reused_find": (
        "BenchmarkRawByteFindAfterFreshFind",
        (
            "five_long_reused_near_miss_one_in_256",
            "five_long_reused_late_match_one_in_256",
        ),
    ),
}

BENCHMARK = re.compile(
    r"^(?P<name>Benchmark\S+?)(?:-\d+)?\s+\d+\s+"
    r"(?P<ns>[0-9.]+)\s+ns/op(?:\s+[0-9.]+\s+MB/s)?\s+"
    r"(?P<bytes>[0-9.]+)\s+B/op\s+(?P<allocs>[0-9.]+)\s+allocs/op\s*$"
)


def parse(path: Path, mode: str) -> dict[str, dict[str, float]]:
    prefix, labels = SPECS[mode]
    expected = {f"{prefix}/{label}" for label in labels}
    result: dict[str, dict[str, float]] = {}
    for line in path.read_text().splitlines():
        match = BENCHMARK.match(line)
        if match is None or match["name"] not in expected:
            continue
        name = match["name"]
        if name in result:
            raise ValueError(f"duplicate benchmark result for {name}")
        result[name] = {
            "ns_per_op": float(match["ns"]),
            "B_per_op": float(match["bytes"]),
            "allocs_per_op": float(match["allocs"]),
        }
    missing = expected.difference(result)
    if missing:
        raise ValueError(f"missing benchmark results: {', '.join(sorted(missing))}")
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=sorted(SPECS))
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    for name, values in parse(args.report, args.mode).items():
        label = name.removeprefix(SPECS[args.mode][0] + "/").replace("/", "_")
        for suffix, value in values.items():
            print(json.dumps({"metric": f"{label}_{suffix}", "value": value}))


if __name__ == "__main__":
    main()
