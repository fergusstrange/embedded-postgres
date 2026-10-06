import json
import os
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import release
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


class ReleasePublicationTest(unittest.TestCase):
    tag = "v2.0.0-alpha.1"
    sha = "a" * 40

    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = pathlib.Path(temporary.name)
        self.dist = self.root / "dist"
        self.dist.mkdir()
        self.output = self.root / "output"
        self.enterContext(patch.object(release, "ROOT", self.root))
        self.enterContext(patch.dict(os.environ, GITHUB_SHA=self.sha,
                                     GITHUB_REF="refs/heads/master", GITHUB_EVENT_NAME="push",
                                     GITHUB_OUTPUT=str(self.output)))
        self.command = self.enterContext(patch.object(release, "command"))

    def draft(self, **changes):
        return dict(isDraft=True, targetCommitish=self.sha, **changes)

    def create_assets(self, count=12):
        for index in range(count):
            (self.dist / f"asset-{index}").write_text("built artifact")

    def test_publishes_draft_without_an_existing_tag(self):
        self.create_assets()

        def github(*args, **kwargs):
            # A draft without a tag is visible through release view, while
            # the published-release REST lookup fails with HTTP 404.
            if args[:3] == ("gh", "release", "view"):
                return json.dumps(self.draft())
            if args[:3] in (("gh", "release", "upload"), ("gh", "release", "edit")):
                return ""
            raise AssertionError(f"unsupported draft lookup: {args}")

        self.command.side_effect = github
        release.publish(self.tag)
        upload = self.command.call_args_list[1].args
        self.assertEqual(upload[:7], ("gh", "release", "upload", self.tag, "--repo", release.REPO, "--clobber"))
        self.assertEqual(set(upload[7:]), {str(path) for path in self.dist.iterdir()})
        self.command.assert_called_with("gh", "release", "edit", self.tag, "--repo", release.REPO,
                                        "--draft=false", "--prerelease=true", "--latest=false")

    def test_published_release_is_immutable_even_without_local_assets(self):
        self.dist.rmdir()
        self.command.return_value = json.dumps({"isDraft": False, "targetCommitish": self.sha})
        release.publish(self.tag)
        self.assertEqual(self.command.call_count, 1)

    def test_draft_for_another_commit_cannot_be_published(self):
        self.create_assets()
        self.command.return_value = json.dumps({"isDraft": True, "targetCommitish": "b" * 40})
        with self.assertRaisesRegex(SystemExit, "different commit"):
            release.publish(self.tag)
        self.assertEqual(self.command.call_count, 1)

    def test_incomplete_build_cannot_be_published(self):
        self.create_assets(11)
        self.command.return_value = json.dumps(self.draft())
        with self.assertRaisesRegex(SystemExit, "incomplete release assets"):
            release.publish(self.tag)
        self.assertEqual(self.command.call_count, 1)

    def test_prepare_reuses_draft_and_skips_published_release_on_retry(self):
        for draft in (True, False):
            with self.subTest(draft=draft):
                self.output.write_text("")
                self.command.reset_mock()
                self.command.side_effect = [self.sha, json.dumps([[{
                    "tag_name": self.tag, "target_commitish": self.sha, "draft": draft,
                }]]), "", ""]
                release.prepare()
                self.assertEqual(self.output.read_text(), f"tag={self.tag}\npublish={str(draft).lower()}\n")
                self.assertFalse(any(call.args[:2] == ("gh", "release") for call in self.command.call_args_list))
