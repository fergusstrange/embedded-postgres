# Releases

v2 is available as an alpha. The Go module and CLI are released together on the
[releases page](https://github.com/fergusstrange/embedded-postgres/releases).
The [migration guide](migration-v2.md) explains the changes from v1.

## Install

For Go tests, add a published version:

```bash
go get github.com/fergusstrange/embedded-postgres/v2@v2.0.0-alpha.1
```

The library downloads the matching supervisor automatically. No separate CLI
installation is needed for a normal Go module consumer.

For other test runners, install the CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
embedded-postgres exec -- npm test
```

The installer selects the newest published v2 release, including alphas. To pin a
version, set `EP_VERSION` on the `bash` command:

```bash
curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | EP_VERSION=v2.0.0-alpha.1 bash
```

Windows users can use Git Bash or the [PowerShell installer](../install/install.ps1).
See the [CLI guide](cli.md) for installation paths and configuration.

PostgreSQL is downloaded on first use and cached for later tests. Its native
libraries must be available on the host; see the [platform guide](non-root.md).

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

Every successful `master` push publishes the next alpha. No manual tag or version
commit is needed. The workflow waits for all native tests, PostgreSQL 15–18,
Alpine, minimum/current Go, security checks and **at least 90% aggregate production
coverage**. The coverage report is available in the workflow artifacts.

Pull requests must pass the required checks against an up-to-date base and have
all review conversations resolved. `master` disallows force pushes and deletion.

The release job builds and attests the binaries before publishing. Retrying the
same commit resumes its draft or leaves an already published release unchanged.
PR, manual and scheduled workflows do not publish. Alpha releases do not replace
v1 as GitHub's latest stable release.

Release settings live in [`.github/release.json`](../.github/release.json):

- `base: "2.0.0"`, `prerelease: "alpha"` produces `v2.0.0-alpha.1`, `.2`, and so on.
- After migration and downstream validation, set `prerelease` to `""` to publish
  stable `v2.0.0`. Subsequent master builds increment the patch version.
- Set `base` explicitly when starting a new minor release.

Actions are pinned to commit hashes and Dependabot groups their updates for
review. The workflow checks that production Go code uses only the standard
library, runs vet/staticcheck, govulncheck, CodeQL, race tests, archive fuzzing and
lifecycle resource checks. Native PostgreSQL libraries require separate upstream
security and compatibility review. Contributor commands are in
[CONTRIBUTING.md](../CONTRIBUTING.md).
