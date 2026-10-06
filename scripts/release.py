"""Idempotent master-build releases. No release mutation occurs in PR builds."""
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
REPO = "fergusstrange/embedded-postgres"
TAG = re.compile(r"^v2\.(\d+)\.(\d+)(?:-([a-z]+)\.(\d+))?$")
TARGETS = [(system, arch) for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]


def command(*args, **kwargs):
    return subprocess.check_output(args, cwd=ROOT, text=True, **kwargs).strip()


def next_version(config, tags):
    base, channel = config["base"], config["prerelease"]
    if not re.fullmatch(r"2\.\d+\.\d+", base) or not re.fullmatch(r"[a-z]*", channel):
        raise ValueError("release config needs a v2 base and a lowercase prerelease channel")
    if channel:
        prefix = f"v{base}-{channel}."
        numbers = [int(tag[len(prefix):]) for tag in tags if tag.startswith(prefix) and tag[len(prefix):].isdigit()]
        return prefix + str(max(numbers, default=0) + 1)
    minimum = tuple(map(int, base.split(".")))
    stable = [tuple(map(int, tag[1:].split("."))) for tag in tags if re.fullmatch(r"v2\.\d+\.\d+", tag)]
    latest = max(stable, default=(2, 0, -1))
    return "v" + ".".join(map(str, max(minimum, (latest[0], latest[1], latest[2] + 1))))


def prepare():
    sha = os.environ["GITHUB_SHA"]
    if os.environ.get("GITHUB_REF") != "refs/heads/master" or os.environ.get("GITHUB_EVENT_NAME") != "push":
        raise SystemExit("release is restricted to master pushes")
    if command("git", "rev-parse", "HEAD") != sha:
        raise SystemExit("checkout does not match the tested commit")
    pages = json.loads(command("gh", "api", f"repos/{REPO}/releases?per_page=100", "--paginate", "--slurp"))
    releases = [release for page in pages for release in page]
    tags = set(command("git", "tag", "--list", "v2.*").splitlines()) | {r["tag_name"] for r in releases}
    pointed = set(command("git", "tag", "--points-at", sha).splitlines())
    matching = [r for r in releases if TAG.fullmatch(r["tag_name"]) and (r["target_commitish"] == sha or r["tag_name"] in pointed)]
    if matching:
        release = matching[0]
        tag, publish = release["tag_name"], release["draft"]
    else:
        config = json.loads((ROOT / ".github/release.json").read_text())
        tag = next_version(config, tags)
        command("gh", "release", "create", tag, "--repo", REPO, "--target", sha, "--draft", "--title", tag, "--notes-file", str(ROOT / "docs/release-notes-v2.md"))
        publish = True
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"tag={tag}\npublish={str(publish).lower()}\n")
    print(f"{tag}: {'prepare verified artifacts' if publish else 'already published; no changes'}")


def build(tag):
    if not TAG.fullmatch(tag):
        raise ValueError("invalid v2 release tag")
    dist = ROOT / "dist"
    dist.mkdir(exist_ok=True)
    if any(dist.iterdir()):
        raise SystemExit("dist must be empty before a release build")
    for system, arch in TARGETS:
        suffix = ".exe" if system == "windows" else ""
        name = f"embedded-postgres_{system}_{arch}{suffix}"
        env = dict(os.environ, CGO_ENABLED="0", GOOS=system, GOARCH=arch)
        command("go", "build", "-trimpath", "-ldflags", f"-s -w -X github.com/fergusstrange/embedded-postgres/v2.ReleaseVersion={tag}", "-o", str(dist / name), "./cmd/embedded-postgres", env=env)
    for source in ["install/install.sh", "install/install.ps1", "LICENSE"]:
        shutil.copyfile(ROOT / source, dist / pathlib.Path(source).name)
    shutil.copyfile(ROOT / "internal/binaries/manifest.json", dist / "postgresql-manifest.json")
    sbom = {
        "bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
        "metadata": {"component": {"type": "application", "name": "embedded-postgres", "version": tag}},
        "components": [{"type": "framework", "name": "Go standard library", "version": command("go", "env", "GOVERSION")}],
        "dependencies": [],
    }
    (dist / "sbom.cdx.json").write_text(json.dumps(sbom, indent=2) + "\n")
    entries = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n" for path in sorted(dist.iterdir())]
    (dist / "checksums.txt").write_text("".join(entries))


def publish(tag):
    if not TAG.fullmatch(tag):
        raise ValueError("invalid v2 release tag")
    # The REST endpoint for a tag only resolves published releases. The CLI
    # also finds drafts, including those whose Git tag has not been created.
    release = json.loads(command("gh", "release", "view", tag, "--repo", REPO, "--json", "isDraft,targetCommitish"))
    if not release["isDraft"]:
        print("Release is already published; artifacts are immutable")
        return
    if release["targetCommitish"] != os.environ["GITHUB_SHA"]:
        raise SystemExit("draft belongs to a different commit")
    assets = sorted(str(path) for path in (ROOT / "dist").iterdir())
    if len(assets) != 12:
        raise SystemExit("incomplete release assets")
    command("gh", "release", "upload", tag, "--repo", REPO, "--clobber", *assets)
    prerelease = "-" in tag
    command("gh", "release", "edit", tag, "--repo", REPO, "--draft=false", "--prerelease=" + str(prerelease).lower(), "--latest=" + str(not prerelease).lower())
    print(f"Published {tag}")


if __name__ == "__main__":
    if sys.argv[1:] == ["prepare"]:
        prepare()
    elif len(sys.argv) == 3 and sys.argv[1] in ("build", "publish"):
        {"build": build, "publish": publish}[sys.argv[1]](sys.argv[2])
    else:
        raise SystemExit("usage: release.py prepare | build TAG | publish TAG")
