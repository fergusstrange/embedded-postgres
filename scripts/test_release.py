import unittest
from release import next_version


class ReleaseVersionTest(unittest.TestCase):
    def test_first_and_next_prerelease(self):
        config = {"base": "2.0.0", "prerelease": "alpha"}
        self.assertEqual(next_version(config, ["v1.34.0"]), "v2.0.0-alpha.1")
        self.assertEqual(next_version(config, ["v2.0.0-alpha.9", "v2.0.0-alpha.10", "v2.0.0-beta.30"]), "v2.0.0-alpha.11")

    def test_stable_patch_and_explicit_minor(self):
        self.assertEqual(next_version({"base": "2.0.0", "prerelease": ""}, []), "v2.0.0")
        self.assertEqual(next_version({"base": "2.0.0", "prerelease": ""}, ["v2.0.9"]), "v2.0.10")
        self.assertEqual(next_version({"base": "2.1.0", "prerelease": ""}, ["v2.0.9"]), "v2.1.0")

    def test_refuses_invalid_config(self):
        for config in [{"base": "1.0.0", "prerelease": ""}, {"base": "2.0.0", "prerelease": "../bad"}]:
            with self.assertRaises(ValueError):
                next_version(config, [])
