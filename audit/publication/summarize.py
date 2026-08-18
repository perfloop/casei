#!/usr/bin/env python3
"""Recompute the publication-audit summaries from the checked-in Go output."""

from collections import defaultdict
from pathlib import Path
import re
from statistics import median


ROOT = Path(__file__).resolve().parent / "results"
HOSTS = {"ice": "Ice Lake", "spr": "Sapphire Rapids"}
GDB_RUNS = (
    "gdb-normal.txt",
    "gdb-avx512-off.txt",
    "gdb-bmi2-off.txt",
)


def benchmark_name(raw):
    return re.sub(r"-\d+$", "", raw)


def metric_rows(path, metric, prefix):
    rows = defaultdict(list)
    with path.open() as source:
        for line in source:
            fields = line.split()
            if not fields or not fields[0].startswith(prefix):
                continue
            for at, field in enumerate(fields):
                if field == metric and at > 0:
                    name = benchmark_name(fields[0])[len(prefix):]
                    rows[name].append(float(fields[at - 1]))
                    break
    return rows


def require(rows, count, label):
    if len(rows) != 33:
        raise SystemExit(f"{label}: found {len(rows)} rows, expected 33")
    bad = {name: len(values) for name, values in rows.items() if len(values) != count}
    if bad:
        raise SystemExit(f"{label}: wrong sample counts: {bad}")


def candidate_rows(path):
    rows = {}
    for prefix in ("BenchmarkIndexFold/", "BenchmarkMatcher/"):
        for name, values in metric_rows(path, "ns/op", prefix).items():
            if not name.endswith("/candidate"):
                continue
            row = name.removesuffix("/candidate")
            if row in rows:
                raise SystemExit(f"{path}: duplicate candidate row {row}")
            rows[row] = values
    return rows


def ratios(left, right, label):
    if left.keys() != right.keys():
        raise SystemExit(f"{label}: row inventories differ")
    require(left, 5, f"{label}/left")
    require(right, 5, f"{label}/right")
    return {name: median(right[name]) / median(left[name]) for name in left}


def field_summary(root):
    rows = metric_rows(root / "benchmarkbar.txt", "x_vs_best", "BenchmarkBar/")
    require(rows, 3, f"{root.name}/field")

    dispatch = {
        metric: metric_rows(root / "benchmarkbar.txt", metric, "BenchmarkBar/")
        for metric in (
            "entrants",
            "candidate_vector_bits",
            "vectorscan_vector_bits",
            "vectorscan_vbmi",
        )
    }
    for metric, values in dispatch.items():
        require(values, 3, f"{root.name}/{metric}")
    flat = lambda values: [value for samples in values.values() for value in samples]
    entrants = flat(dispatch["entrants"])
    if min(entrants) < 2:
        raise SystemExit(f"{root.name}: unmeasured row with {min(entrants)} entrants")
    if set(flat(dispatch["candidate_vector_bits"])) != {512}:
        raise SystemExit(f"{root.name}: candidate did not stay on 512-bit dispatch")
    if set(flat(dispatch["vectorscan_vector_bits"])) != {512}:
        raise SystemExit(f"{root.name}: Vectorscan did not stay on 512-bit dispatch")
    if set(flat(dispatch["vectorscan_vbmi"])) != {1}:
        raise SystemExit(f"{root.name}: Vectorscan VBMI dispatch was not active")

    medians = {name: median(values) for name, values in rows.items()}
    worst_row = max(medians, key=medians.get)
    worst_sample = max(value for values in rows.values() for value in values)
    if worst_sample >= 1:
        raise SystemExit(f"{root.name}: losing field sample {worst_sample}")
    speedup = median([1 / value for value in medians.values()])
    return worst_row, medians[worst_row], worst_sample, speedup, int(min(entrants)), int(max(entrants))


def reachability_summary(root):
    observed = set()
    counts = []
    for name in GDB_RUNS:
        path = root / name
        lines = path.read_text().splitlines()
        if lines.count("PASS") != 1:
            raise SystemExit(f"{path}: expected exactly one PASS line")
        hits = []
        for line in lines:
            match = re.fullmatch(r"HIT ([0-9]+)", line)
            if match:
                hits.append(int(match.group(1)))
            elif line != "PASS":
                raise SystemExit(f"{path}: unexpected transcript line {line!r}")
        if len(hits) != len(set(hits)):
            raise SystemExit(f"{path}: duplicate breakpoint hit")
        observed.update(hits)
        counts.append(len(hits))

    expected = set(range(1, 37))
    if observed != expected:
        missing = sorted(expected - observed)
        extra = sorted(observed - expected)
        raise SystemExit(f"assembly reachability: missing={missing}, extra={extra}")
    return counts


def main():
    for host, title in HOSTS.items():
        root = ROOT / host
        row, middle, worst, speedup, min_entrants, max_entrants = field_summary(root)
        print(
            f"{title} field: worst-row={row} median-x={middle:.4f}, "
            f"worst-sample={worst:.4f}, median-speedup={speedup:.2f}x, "
            f"entrants={min_entrants}-{max_entrants}"
        )

        filters = ratios(
            candidate_rows(root / "filters-on.txt"),
            candidate_rows(root / "filters-bypassed.txt"),
            f"{host}/filters",
        )
        filter_worst = max(filters, key=filters.get)
        neutral = sorted(name for name, value in filters.items() if value < 1)
        print(
            f"{title} filters: median={median(filters.values()):.2f}x, "
            f"max={filters[filter_worst]:.2f}x ({filter_worst}), "
            f"bypassed-faster={neutral}"
        )

        isa = ratios(
            candidate_rows(root / "avx512-on.txt"),
            candidate_rows(root / "avx512-off.txt"),
            f"{host}/isa",
        )
        exceptions = sorted((name, value) for name, value in isa.items() if value < 1)
        rendered = ", ".join(f"{name}={value:.3f}x" for name, value in exceptions)
        print(f"{title} AVX-512: median={median(isa.values()):.2f}x, AVX2-faster=[{rendered}]")

    reachability = reachability_summary(ROOT / "ice")
    print(
        "Assembly reachability: 36/36 across normal, AVX-512-off, and "
        f"BMI2-off runs; per-run hits={reachability}; all PASS"
    )


if __name__ == "__main__":
    main()
