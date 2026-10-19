"""Check a public Go module and CLI in an isolated consumer (no source override).

Usage: python scripts/check-published.py v2.0.0 COMMIT [EVIDENCE_DIR]
Requires Go, Python and authenticated gh for public build-attestation verification.
"""
import hashlib
import json
import os
import pathlib
import re
import shutil
import socket
import subprocess
import sys
import tempfile

source = pathlib.Path(__file__).resolve().parent / "testdata/published"
tag, revision = sys.argv[1:3]
if not re.fullmatch(r"v2\.\d+\.\d+(?:-[a-z]+\.\d+)?", tag) or not re.fullmatch(r"[0-9a-f]{40}", revision):
    raise SystemExit("expected an explicit v2 tag and full source commit")
evidence = pathlib.Path(sys.argv[3] if len(sys.argv) > 3 else "published-evidence").resolve()
evidence.mkdir(parents=True, exist_ok=False)
work = pathlib.Path(tempfile.mkdtemp(prefix="ep-published-"))
# Python's Windows private directory can be owned by Administrators. Grant the
# actual user explicitly so disabling Administrators does not deny the fixture.
# This changes only this test's new directory, never a consumer's existing ACL.
if os.name == "nt":
    identity = json.loads(subprocess.check_output([
        "pwsh", "-NoProfile", "-Command",
        "[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value | ConvertTo-Json",
    ], text=True))
    if not re.fullmatch(r"S-1-[0-9-]+", identity):
        raise SystemExit("unexpected current user SID")
    subprocess.run(["icacls", str(work), "/grant", f"*{identity}:(OI)(CI)F"], check=True)
env = {k: v for k, v in os.environ.items() if not k.startswith("EP_") and k != "EMBEDDED_POSTGRES_CLI"}
env.update(GOMODCACHE=str(work / "modules"), GOCACHE=str(work / "build-cache"),
           GOWORK="off", GOPROXY="https://proxy.golang.org,direct", GOSUMDB="sum.golang.org")
commands = []


def run(args, label, cwd, extra=None):
    with (evidence / (label + ".log")).open("w") as log:
        result = subprocess.run(args, cwd=cwd, env=dict(env, **(extra or {})),
                                stdout=log, stderr=subprocess.STDOUT, timeout=600)
    commands.append({"command": args, "label": label, "exit": result.returncode})
    (evidence / "commands.json").write_text(json.dumps(commands, indent=2) + "\n")
    if result.returncode:
        raise RuntimeError(f"{label} failed; see {evidence / (label + '.log')}")


try:
    go = work / "go"
    go.mkdir()
    shutil.copy2(source / "release_test.go", go / "release_test.go")
    (go / "go.mod").write_text("module example.com/published-consumer\n\ngo 1.26.0\n")
    run(["go", "version"], "go-version", go)
    module = "github.com/fergusstrange/embedded-postgres/v2@" + tag
    run(["go", "get", module], "go-get", go)
    run(["go", "mod", "verify"], "go-mod-verify", go)
    run(["go", "mod", "download", "-json", module], "go-module", go)
    metadata = json.loads((evidence / "go-module.log").read_text())
    assert metadata["Origin"]["Hash"] == revision, metadata
    shutil.copy2(go / "go.sum", evidence / "go.sum")
    run(["go", "test", "-v", "-count=1", "-timeout=6m", "./..."], "go-consumer", go,
        {"EP_CONSUMER_EVIDENCE": str(go), "EP_CONSUMER_TAG": tag})
    system, arch = subprocess.check_output(["go", "env", "GOOS", "GOARCH"], env=env, text=True).split()
    asset = f"embedded-postgres_{system}_{arch}" + (".exe" if system == "windows" else "")
    cli = go / "cache" / (tag + "-" + asset)
    digest = hashlib.sha256(cli.read_bytes()).hexdigest()
    run(["gh", "release", "download", tag, "--repo", "fergusstrange/embedded-postgres",
         "--pattern", "checksums.txt", "--dir", str(evidence)], "release-checksums", go)
    checksums = {line.split()[1].lstrip("*"): line.split()[0]
                 for line in (evidence / "checksums.txt").read_text().splitlines()}
    assert checksums[asset] == digest
    run(["gh", "attestation", "verify", str(cli), "--repo", "fergusstrange/embedded-postgres",
         "--source-ref", "refs/heads/master", "--source-digest", revision,
         "--signer-workflow", "fergusstrange/embedded-postgres/.github/workflows/v2.yml", "--signer-digest", revision,
         "--deny-self-hosted-runners", "--format", "json"], "cli-attestation", go)
    version = json.loads(subprocess.check_output([str(cli), "version"], text=True))
    assert version["version"] == tag
    cli_work = work / "cli"
    cli_work.mkdir()
    query_record = cli_work / "query.json"
    run([str(cli), "exec", "--cache-dir", str(cli_work / "cache"), "--work-dir", str(cli_work / "work"),
         "--", sys.executable, str(source / "cli_query.py"), str(cli_work / "cache"), str(query_record)],
        "cli-exec", cli_work)
    query = json.loads(query_record.read_text())
    shutil.copy2(query_record, evidence / "cli-query.json")
    with socket.socket() as connection:
        connection.settimeout(2)
        assert connection.connect_ex(("127.0.0.1", query["port"])) != 0, "CLI port survived cleanup"
    assert list((cli_work / "work").iterdir()) == [], "CLI workspace survived cleanup"
    record = {"status": "passed", "tag": tag, "revision": revision, "module_sum": metadata["Sum"],
              "system": system, "arch": arch, "cli_asset": asset, "cli_sha256": digest,
              "postgres_version": "18.6.1", "commands": commands}
finally:
    # Go makes downloaded modules read-only. Use its supported cache cleanup
    # before removing this script's isolated temporary directory.
    subprocess.run(["go", "clean", "-modcache"], cwd=work, env=env, check=True, timeout=120)
    shutil.rmtree(work)
(evidence / "result.json").write_text(json.dumps(record, indent=2) + "\n")
print("Published module, automatic supervisor, CLI, fresh downloads, SQL, offline reuse and cleanup passed")
