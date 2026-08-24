#!/usr/bin/env python3
"""Verify and summarize the current two-host acceptance receipts."""

import hashlib
from pathlib import Path
from statistics import median
import sys


sys.dont_write_bytecode = True
SCRIPTS = Path(__file__).resolve().parents[2] / "scripts"
sys.path.insert(0, str(SCRIPTS))
import verify_benchmarkbar  # noqa: E402


ROOT = Path(__file__).resolve().parent / "results"
SOURCE_MANIFEST = Path(__file__).resolve().parent / "SOURCE_SHA256SUMS"
HOSTS = {"ice": "Ice Lake", "spr": "Sapphire Rapids"}


def main():
    source_digest = hashlib.sha256(SOURCE_MANIFEST.read_bytes()).hexdigest()
    for host, title in HOSTS.items():
        path = ROOT / host / "benchmarkbar.txt"
        try:
            verify_benchmarkbar.verify(path, expected_samples=3)
        except verify_benchmarkbar.VerificationError as err:
            raise SystemExit(err) from err
        rows = verify_benchmarkbar.parse(path)

        ratios = {
            name: [sample["x_vs_best"] for sample in samples]
            for name, samples in rows.items()
        }
        middle = {name: median(values) for name, values in ratios.items()}
        worst_row = max(middle, key=middle.get)
        worst_sample = max(value for values in ratios.values() for value in values)
        speedup = median(1 / value for value in middle.values())
        entrants = [
            sample["entrants"]
            for samples in rows.values()
            for sample in samples
        ]
        native = (ROOT / host / "gdb-native.txt").read_text().splitlines()
        required = {
            "HIT 1",
            "HIT 2",
            "HIT 3",
            "PASS",
            f"source checksum stream: {source_digest}",
        }
        missing = required.difference(native)
        if missing:
            raise SystemExit(
                f"{title} native receipt missing: {', '.join(sorted(missing))}"
            )
        print(
            f"{title}: rows={len(rows)}, samples={sum(map(len, rows.values()))}, "
            f"worst-row={worst_row}, median-x={middle[worst_row]:.4f}, "
            f"worst-sample={worst_sample:.4f}, median-speedup={speedup:.2f}x, "
            f"entrants={int(min(entrants))}-{int(max(entrants))}, native=3/3"
        )


if __name__ == "__main__":
    main()
