"""Exercise version detection through the real Go module loader, without network."""
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest
import zipfile


@unittest.skipUnless(shutil.which("go"), "requires Go")
class ModuleConsumerTest(unittest.TestCase):
    def test_tagged_dependency_version_in_test_binaries(self):
        root = pathlib.Path(__file__).resolve().parent.parent
        module = "github.com/fergusstrange/embedded-postgres/v2"
        version = "v2.0.0-fixture.1"
        with tempfile.TemporaryDirectory() as directory:
            stage = pathlib.Path(directory)
            proxy = stage / "proxy" / module / "@v"
            proxy.mkdir(parents=True)
            (proxy / f"{version}.mod").write_bytes((root / "go.mod").read_bytes())
            (proxy / f"{version}.info").write_text(json.dumps({"Version": version, "Time": "2026-01-01T00:00:00Z"}))
            files = subprocess.check_output(["git", "ls-files", "-z"], cwd=root).decode().split("\0")
            with zipfile.ZipFile(proxy / f"{version}.zip", "w") as archive:
                for name in files:
                    if name and (name.endswith(".go") or name in ("go.mod", "internal/binaries/manifest.json")):
                        archive.write(root / name, f"{module}@{version}/{name}")
                # Expose the private resolver only inside this temporary module
                # fixture, so an external consumer calls the production code.
                archive.writestr(f"{module}@{version}/release_fixture.go",
                                 "package embeddedpostgres\nfunc FixtureSupervisorVersion() string { return supervisorVersion() }\n")
            consumer = stage / "consumer"
            consumer.mkdir()
            (consumer / "go.mod").write_text(f"module example.com/consumer\n\ngo 1.26.0\n\nrequire {module} {version}\n")
            (consumer / "main.go").write_text(
                f'package main\nimport ("fmt"; postgres "{module}")\n'
                'func main() { fmt.Print(postgres.FixtureSupervisorVersion()) }\n')
            (consumer / "main_test.go").write_text(
                f'package main\nimport ("testing"; postgres "{module}")\n'
                'func TestSupervisorVersion(t *testing.T) { '
                f'if got := postgres.FixtureSupervisorVersion(); got != "{version}" {{ t.Fatalf("got %q", got) }} }}\n')
            env = dict(os.environ, GOPROXY=(stage / "proxy").as_uri(), GOSUMDB="off",
                       GOPRIVATE="", GONOPROXY="", GONOSUMDB="", GOWORK="off",
                       GOTOOLCHAIN="local", GOMODCACHE=str(stage / "modules"), GOFLAGS="-modcacherw")
            for args in [("run", "-mod=mod", "."), ("test", "-mod=mod", "-count=1", "."),
                         ("test", "-mod=mod", "-count=1", "-trimpath", ".")]:
                with self.subTest(command=args):
                    result = subprocess.run(["go", *args], cwd=consumer, env=env, text=True,
                                            capture_output=True, timeout=180)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    if args[0] == "run":
                        self.assertEqual(result.stdout, version)
