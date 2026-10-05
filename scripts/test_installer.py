import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest


@unittest.skipUnless(shutil.which("bash"), "Bash installer requires bash")
class InstallerTest(unittest.TestCase):
    def run_installer(self, system, arch, corrupt=False):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            tools = root / "tools"
            tools.mkdir()
            destination = root / "install with spaces"
            (tools / "uname").write_text('#!/bin/sh\nif [ "$1" = "-s" ]; then printf "%s\\n" "$FIXTURE_OS"; else printf "%s\\n" "$FIXTURE_ARCH"; fi\n')
            (tools / "curl").write_text('''#!/usr/bin/env python3
import hashlib, os, pathlib, sys
args=sys.argv[1:]
if "-o" not in args:
 # Consume a full release listing without an early-reader SIGPIPE under pipefail.
 print('  "tag_name": "v2.0.0-alpha.1",' + '\\n' + '  "tag_name": "v2.0.0-alpha.0",\\n' * 10000)
else:
 output=pathlib.Path(args[args.index("-o")+1])
 if output.name=="checksums.txt":
  name=os.environ["FIXTURE_ASSET"]
  digest="0"*64 if os.environ.get("FIXTURE_CORRUPT")=="1" else hashlib.sha256(b"test binary").hexdigest()
  output.write_text(digest+"  "+name+"\\n")
 else:output.write_bytes(b"test binary")
''')
            for tool in tools.iterdir():
                tool.chmod(0o755)
            target = {"Linux": "linux", "Darwin": "darwin", "MINGW64_NT": "windows"}.get(system, "unknown")
            machine = {"x86_64": "amd64", "aarch64": "arm64"}.get(arch, "unknown")
            suffix = ".exe" if target == "windows" else ""
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"], EP_INSTALL_DIR=str(destination), FIXTURE_OS=system, FIXTURE_ARCH=arch, FIXTURE_ASSET=f"embedded-postgres_{target}_{machine}{suffix}", FIXTURE_CORRUPT=str(int(corrupt)))
            env.pop("EP_VERSION", None)
            script = pathlib.Path(__file__).resolve().parent.parent / "install/install.sh"
            result = subprocess.run(["bash", str(script)], env=env, capture_output=True, text=True)
            installed = destination / ("embedded-postgres" + suffix)
            if corrupt or target == "unknown" or machine == "unknown":
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(installed.exists())
            else:
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(installed.read_bytes(), b"test binary")

    def test_six_platform_targets(self):
        for system in ("Linux", "Darwin", "MINGW64_NT"):
            for arch in ("x86_64", "aarch64"):
                with self.subTest(system=system, arch=arch):
                    self.run_installer(system, arch)

    def test_checksum_and_platform_rejection(self):
        self.run_installer("Linux", "x86_64", corrupt=True)
        self.run_installer("Unknown", "x86_64")
        self.run_installer("Linux", "unknown")
