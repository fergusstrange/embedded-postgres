import pathlib
import tempfile
import unittest
from coverage import merge


class CoverageTest(unittest.TestCase):
    def test_native_union_without_duplicate_denominator(self):
        with tempfile.TemporaryDirectory() as directory:
            a, b = pathlib.Path(directory) / 'a.out', pathlib.Path(directory) / 'b.out'
            a.write_text('mode: atomic\nmodule/core.go:1.1,2.2 4 0\nmodule/process_unix.go:1.1,2.2 2 1\n')
            b.write_text('mode: atomic\nmodule/core.go:1.1,2.2 4 9\nmodule/process_windows.go:1.1,2.2 3 1\nmodule/examples/demo.go:1.1,2.2 100 0\n')
            result = merge([a, b])
            self.assertEqual(sum(size for _, size in result), 9)
            self.assertEqual(sum(size for (_, size), count in result.items() if count), 9)

    def test_zero_statement_platform_stub(self):
        import subprocess
        import sys
        with tempfile.TemporaryDirectory() as directory:
            profile = pathlib.Path(directory) / 'stub.out'
            profile.write_text('mode: set\nmodule/stub_windows.go:1.1,1.2 0 0\nmodule/core.go:1.1,2.2 1 1\n')
            result = subprocess.run([sys.executable, str(pathlib.Path(__file__).with_name('coverage.py')), str(profile), '--output', str(pathlib.Path(directory) / 'merged.out')], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('100.00%', result.stdout)
