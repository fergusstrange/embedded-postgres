# Releases

v2 is stable. The Go module and CLI are released together on the
[releases page](https://github.com/fergusstrange/embedded-postgres/releases).
The [migration guide](migration-v2.md) explains the changes from v1.

## Install

For Go tests, add the newest stable v2 version:

```bash
go get github.com/fergusstrange/embedded-postgres/v2@latest
```

The library downloads the matching supervisor automatically. No separate CLI
installation is needed for a normal Go module consumer.

For other test runners, install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
embedded-postgres exec -- npm test
```

The installer selects the newest published v2 release. It can also install
explicit prerelease tags. To pin a version, set `EP_VERSION` on the `bash` command:

```bash
curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | EP_VERSION=v2.0.0 bash
```

Windows users can use Git Bash or the [PowerShell installer](../install/install.ps1).
See the [CLI guide](cli.md) for installation paths and configuration.

PostgreSQL is downloaded on first use and cached for later tests. The pinned
bundles include third-party runtime libraries; OS and loader requirements still
apply. See the [platform guide](non-root.md).

## What a release contains

Each release provides CLI binaries for Linux, macOS and Windows on AMD64 and
ARM64, installers, SHA-256 checksums, a PostgreSQL version/checksum manifest,
a software inventory and the license. Windows ARM64 uses x64 PostgreSQL under
emulation. The installers verify downloads against the release checksums.

GitHub build attestations identify the source commit and workflow. To verify a
downloaded binary with the GitHub CLI:

```bash
gh attestation verify ./embedded-postgres_linux_amd64 --repo fergusstrange/embedded-postgres
```

## Maintainer release process

Every successful `master` push publishes the next stable patch. No manual tag or version
commit is needed. The workflow waits for all native tests, PostgreSQL 15–18,
Alpine, minimum/current Go, security checks and **at least 90% aggregate production
coverage**. The coverage report is available in the workflow artifacts.

Pull requests must pass the required checks against an up-to-date base and have
all review conversations resolved. `master` disallows force pushes and deletion.

The release job builds and attests the binaries before publishing. Retrying the
same commit resumes its draft or leaves an already published release unchanged.
PR, manual and scheduled workflows do not publish. Stable v2 releases become
GitHub's latest release; v1 remains available under its original Go module path.

After publication, the public **Published consumers** workflow installs the exact
Go module from the public proxy, verifies its source revision and checksum,
downloads the automatic matching CLI, and checks the CLI's checksum and build
attestation. Fresh-cache SQL, offline reuse and cleanup run on all six native
hosts with both supported Go versions, plus native x64/ARM64 Alpine. Windows
checks start outside Git Bash. Logs and pinned identities remain public workflow
artifacts; this validation does not depend on a private repository.

To reproduce one host locally with Go, Python and an authenticated GitHub CLI:

```bash
python scripts/check-published.py RELEASE_TAG FULL_SOURCE_COMMIT
```

The command writes `published-evidence/`, requires an unused evidence directory,
and uses temporary caches. It changes permissions only on its own new Windows
test directory so the actual user retains access after administrator groups are
disabled. It never changes an existing consumer directory's ACL.

Release settings live in [`.github/release.json`](../.github/release.json):

- The current `base: "2.0.0"`, `prerelease: ""` starts at stable `v2.0.0`.
  Subsequent master builds increment the patch version.
- Set `base` explicitly when starting a new minor release.
- A nonempty `prerelease` channel is reserved for a deliberately reviewed future
  prerelease series; it is not the current release policy.

Actions are pinned to commit hashes and Dependabot groups their updates for
review. The workflow checks that production Go code uses only the standard
library, runs vet/staticcheck, govulncheck, CodeQL, race tests, archive fuzzing and
lifecycle resource checks. Native PostgreSQL libraries require separate upstream
security and compatibility review. Contributor commands are in
[CONTRIBUTING.md](../CONTRIBUTING.md).
