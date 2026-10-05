"""Native integration + subprocess coverage. Runs on all six supported hosts."""
import os
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parent.parent
os.chdir(root)
output = pathlib.Path(os.environ.get("EP_CI_OUTPUT", "ci-output")).resolve()
output.mkdir(parents=True, exist_ok=True)
raw = output / "raw"
raw.mkdir(exist_ok=True)
suffix = ".exe" if os.name == "nt" else ""
helper = output / ("embedded-postgres" + suffix)
env = os.environ.copy()
env["EP_COVERDIR"] = str(raw)
env["EP_SUPERVISOR"] = str(helper)
env["EP_TEST_BIN"] = str((root / "pg/bin").resolve())
env["EP_TEST_ARCHIVE"] = str((root / "pg.tar.gz").resolve())
env["EP_POSTGRES_VERSION"] = env.get("EP_TEST_VERSION", "18.6.0")


def run(*args, **kwargs):
    subprocess.run(args, env=env, check=True, **kwargs)


run("go", "build", "-cover", "-covermode=atomic", "-coverpkg=./...", "-o", str(helper), "./cmd/embedded-postgres")
# Race is unsupported on Windows ARM64; native non-race coverage still runs there.
race = [] if sys.platform == "win32" and subprocess.check_output(["go", "env", "GOARCH"], text=True).strip() == "arm64" else ["-race"]
run("go", "test", *race, "-count=1", "-p=2", "-timeout=8m", "-coverpkg=./...", "-covermode=atomic", "-coverprofile=" + str(output / "unit.out"), "./...")
if sys.platform.startswith("linux") and os.environ.get("EP_TEST_ROOT") == "1":
    for package, name in [(".", "core"), ("./internal/supervisor", "supervisor")]:
        binary = output / (name + ".test")
        run("go", "test", "-c", "-cover", "-covermode=atomic", "-coverpkg=./...", "-o", str(binary), package)
        selected = [f"{key}={value}" for key, value in env.items() if key.startswith("EP_")]
        run("sudo", "env", *selected, f"EP_TEST_UID={os.getuid()}", f"EP_TEST_GID={os.getgid()}", str(binary), "-test.v", "-test.timeout=8m", "-test.gocoverdir=" + str(raw))
run("go", "tool", "covdata", "textfmt", "-i=" + str(raw), "-o=" + str(output / "subprocess.out"))
# The global job enforces 90% on the union of native OS implementations.
run(sys.executable, "scripts/coverage.py", str(output / "unit.out"), str(output / "subprocess.out"), "--output", str(output / "combined.out"), "--minimum", "0")
