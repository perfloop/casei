#!/usr/bin/env python3
"""Verify and summarize the three two-host publication ablations."""

from collections import defaultdict
import csv
from pathlib import Path
from statistics import median
import re
import sys


sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent
HOSTS = ("ice", "spr")
VARIANTS = ("origin", "confirm", "tags")
REBAR_ROWS = {
    "curated/01-literal/sherlock-casei-ru",
    "curated/02-literal-alternate/sherlock-casei-ru",
    "hyperscan/literal-casei-russian-nosom",
    "hyperscan/literal-casei-russian-som",
    "opt/prefilter/literal-casei-russian",
}


def duration_ns(raw):
    for suffix, scale in (("ns", 1), ("us", 1_000), ("ms", 1_000_000), ("s", 1_000_000_000)):
        if raw.endswith(suffix):
            return float(raw[:-len(suffix)]) * scale
    raise ValueError(f"unrecognized duration {raw!r}")


def field_rows(host, variant):
    path = ROOT / "field" / host / f"{variant}.txt"
    rows = defaultdict(list)
    with path.open() as source:
        for line in source:
            fields = line.split()
            if not fields or not fields[0].startswith("BenchmarkBar/"):
                continue
            name = re.sub(r"-[0-9]+$", "", fields[0])
            try:
                at = fields.index("x_vs_best")
                ratio = float(fields[at - 1])
            except (ValueError, IndexError) as err:
                raise SystemExit(f"{path}: malformed benchmark row") from err
            rows[name].append(ratio)
    expected_rows = 1 if variant == "confirm" else 2
    if len(rows) != expected_rows or any(len(values) != 8 for values in rows.values()):
        raise SystemExit(f"{path}: expected {expected_rows} rows with eight samples, got {dict(rows)}")
    return rows


def rebar_rows(host, variant):
    ratios = defaultdict(list)
    paths = sorted((ROOT / "rebar" / host).glob(f"{variant}-pass*.csv"))
    if len(paths) != 3:
        raise SystemExit(f"{host}/{variant}: expected three Rebar passes, found {len(paths)}")
    for path in paths:
        rows = defaultdict(dict)
        with path.open(newline="") as source:
            for row in csv.DictReader(source):
                if row["err"]:
                    raise SystemExit(f"{path}: {row['name']}/{row['engine']}: {row['err']}")
                rows[row["name"]][row["engine"]] = duration_ns(row["median"])
        if set(rows) != REBAR_ROWS:
            raise SystemExit(f"{path}: wrong row inventory: {sorted(rows)}")
        for name, engines in rows.items():
            casei = engines.pop("casei", None)
            if casei is None or not engines:
                raise SystemExit(f"{path}: {name}: missing casei or competitor")
            ratios[name].append(casei / min(engines.values()))
    return ratios


def main():
    print("Focused BenchmarkBar ablations (median of eight paired samples):")
    for host in HOSTS:
        for variant in VARIANTS:
            rows = field_rows(host, variant)
            values = [median(samples) for samples in rows.values()]
            print(
                f"  {host}/{variant}: rows={len(values)}, "
                f"wins={sum(value < 1 for value in values)}/{len(values)}, "
                f"worst-median={max(values):.4f}, "
                f"worst-sample={max(max(samples) for samples in rows.values()):.4f}"
            )

    print("Same-contract Rebar ablations (median of three passes):")
    for host in HOSTS:
        for variant in ("confirm", "tags"):
            rows = rebar_rows(host, variant)
            values = [median(samples) for samples in rows.values()]
            print(
                f"  {host}/{variant}: wins={sum(value < 1 for value in values)}/5, "
                f"worst={max(values):.4f}"
            )


if __name__ == "__main__":
    main()
