#!/usr/bin/env python3

from pathlib import Path
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
import verify_benchmarkbar as verify


def row(name, ratio=0.5, entrants=5, candidate_bits=512, vectorscan_bits=512, vbmi=1):
    return (
        f"BenchmarkBar/{name}-8 30 100 ns/op "
        f"{ratio} x_vs_best {entrants} entrants "
        f"1 candidate_active {candidate_bits} candidate_vector_bits "
        f"1 vectorscan_active {vectorscan_bits} vectorscan_vector_bits "
        f"{vbmi} vectorscan_vbmi\n"
    )


def transcript(**override):
    lines = []
    for index, name in enumerate(sorted(verify.EXPECTED_ROWS)):
        values = override if index == 0 else {}
        for _ in range(3):
            lines.append(row(name, **values))
    return "".join(lines)


class VerifyBenchmarkBarTest(unittest.TestCase):
    def verify_text(self, text):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bar.txt"
            path.write_text(text)
            return verify.verify(path)

    def test_accepts_complete_winning_full_width_board(self):
        summary = self.verify_text(transcript())
        self.assertIn("PASS: 33/33 rows", summary)
        self.assertIn("casei=512-bit", summary)

    def test_rejects_losing_sample(self):
        with self.assertRaisesRegex(verify.VerificationError, "loses"):
            self.verify_text(transcript(ratio=1.0))

    def test_rejects_handicapped_vectorscan(self):
        with self.assertRaisesRegex(verify.VerificationError, "vectorscan_vector_bits"):
            self.verify_text(transcript(vectorscan_bits=256))

    def test_rejects_unmeasured_row(self):
        with self.assertRaisesRegex(verify.VerificationError, "unmeasured"):
            self.verify_text(transcript(entrants=1))

    def test_rejects_missing_row(self):
        text = "".join(
            row(name)
            for name in sorted(verify.EXPECTED_ROWS)[:-1]
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_wrong_sample_count(self):
        with self.assertRaisesRegex(verify.VerificationError, "wrong sample counts"):
            self.verify_text(
                transcript() + row(sorted(verify.EXPECTED_ROWS)[0])
            )


if __name__ == "__main__":
    unittest.main()
