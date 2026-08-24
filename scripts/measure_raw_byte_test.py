#!/usr/bin/env python3

import tempfile
import unittest
from pathlib import Path

from measure_raw_byte import parse


class MeasureRawByteTest(unittest.TestCase):
    def report(self, lines: list[str]) -> Path:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        path = Path(directory.name) / "bench.txt"
        path.write_text("\n".join(lines) + "\n")
        return path

    def test_steady_state(self) -> None:
        labels = (
            "two_4KiB_one_in_32",
            "two_4KiB_zero_admission",
            "five_one_in_64",
            "five_one_in_4",
        )
        report = self.report(
            [
                f"BenchmarkRawByteDensity/{label}-1  100  12.5 ns/op  327.68 MB/s  0 B/op  0 allocs/op"
                for label in labels
            ]
        )
        got = parse(report, "steady_state")
        self.assertEqual(len(got), len(labels))
        self.assertEqual(got["BenchmarkRawByteDensity/five_one_in_4"]["ns_per_op"], 12.5)

    def test_construction(self) -> None:
        labels = (
            "two_patterns_512B",
            "two_patterns_4KiB",
            "five_patterns_512B",
            "five_patterns_513B_zero_admission",
            "five_patterns_5MiB_raw_install_and_first_use",
            "two_shared_100_units_1KiB_zero_admission",
            "two_mixed_folded_ascii_root_1KiB",
        )
        report = self.report(
            [
                f"BenchmarkRawByteConstruction/{label}-1  100  12.5 ns/op  0 B/op  0 allocs/op"
                for label in labels
            ]
        )
        got = parse(report, "construction")
        self.assertEqual(len(got), len(labels))
        self.assertEqual(got["BenchmarkRawByteConstruction/two_mixed_folded_ascii_root_1KiB"]["ns_per_op"], 12.5)

    def test_fresh_find(self) -> None:
        labels = (
            "five_long_false_one_in_256",
            "five_long_near_miss_one_in_256",
            "five_long_sparse_one_in_8192",
            "five_long_clustered",
            "five_long_early_match_one_in_256",
            "five_long_late_match_one_in_256",
        )
        report = self.report(
            [
                f"BenchmarkRawByteFreshFind/{label}-1  100  12.5 ns/op  0 B/op  0 allocs/op"
                for label in labels
            ]
        )
        got = parse(report, "fresh_find")
        self.assertEqual(len(got), len(labels))
        self.assertEqual(
            got["BenchmarkRawByteFreshFind/five_long_near_miss_one_in_256"]["ns_per_op"],
            12.5,
        )

    def test_reused_find(self) -> None:
        labels = (
            "five_long_reused_near_miss_one_in_256",
            "five_long_reused_late_match_one_in_256",
        )
        report = self.report(
            [
                f"BenchmarkRawByteFindAfterFreshFind/{label}-1  100  12.5 ns/op  0 B/op  0 allocs/op"
                for label in labels
            ]
        )
        got = parse(report, "reused_find")
        self.assertEqual(len(got), len(labels))
        self.assertEqual(
            got["BenchmarkRawByteFindAfterFreshFind/five_long_reused_late_match_one_in_256"]["ns_per_op"],
            12.5,
        )

    def test_rejects_missing_result(self) -> None:
        report = self.report(["BenchmarkRawByteConstruction/two_patterns_512B-1  100  12 ns/op  0 B/op  0 allocs/op"])
        with self.assertRaisesRegex(ValueError, "missing benchmark results"):
            parse(report, "construction")


if __name__ == "__main__":
    unittest.main()
