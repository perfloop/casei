#!/usr/bin/env python3

from pathlib import Path
import tempfile
import unittest

from measure_benchmarkbar import summarize
from verify_benchmarkbar_test import row
import verify_benchmarkbar as verify


def transcript(ratio=0.5, **override):
    return "".join(
        row(name, ratio=ratio, **override)
        for name in sorted(verify.REQUIRED_ROWS)
    )


class MeasureBenchmarkBarTest(unittest.TestCase):
    def summarize_text(self, text):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bar.txt"
            path.write_text(text)
            return summarize(path)

    def test_strict_verifier_marks_a_complete_winning_board(self):
        metrics = self.summarize_text(transcript())
        self.assertEqual(metrics["benchmarkbar_all_rows_winning"], 1)
        self.assertEqual(metrics["benchmarkbar_rows_below_one"], len(verify.REQUIRED_ROWS))

    def test_strict_verifier_marks_any_losing_row(self):
        metrics = self.summarize_text(transcript(ratio=2.0))
        self.assertEqual(metrics["benchmarkbar_all_rows_winning"], 0)
        self.assertEqual(metrics["benchmarkbar_rows_below_one"], 0)

    def test_non_win_contract_failure_remains_an_error(self):
        with self.assertRaisesRegex(verify.VerificationError, "vectorscan_vector_bits"):
            self.summarize_text(transcript(vectorscan_bits=256))


if __name__ == "__main__":
    unittest.main()
