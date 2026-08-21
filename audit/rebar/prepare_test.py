import os
from pathlib import Path
import tempfile
import unittest

import prepare


class AddEngineTest(unittest.TestCase):
    def test_refreshes_existing_casei_registration(self):
        with tempfile.TemporaryDirectory() as tmp:
            rebar = Path(tmp)
            engines = rebar / "benchmarks/engines.toml"
            engines.parent.mkdir()
            engines.write_text('[[engine]]\n  name = "other"\n')

            first = rebar / "first/runner"
            second = rebar / "second/runner"
            prepare.add_engine(rebar, first)
            prepare.add_engine(rebar, second)

            text = engines.read_text()
            cwd = Path(os.path.relpath(second, engines.parent)).as_posix()
            self.assertEqual(text.count('name = "casei"'), 1)
            self.assertIn(f'cwd = "{cwd}"', text)
            self.assertNotIn("first/runner", text)
            self.assertIn('name = "other"', text)


if __name__ == "__main__":
    unittest.main()
