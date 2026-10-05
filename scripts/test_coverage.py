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
