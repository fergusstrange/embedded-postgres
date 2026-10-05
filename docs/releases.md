# Build, coverage and releases

GitHub Actions replaces CircleCI. Six native runners exercise Linux, macOS and
Windows on AMD64 and ARM64; separate jobs cover PostgreSQL 15–17, Alpine musl and
the minimum Go version. PostgreSQL 18 is exercised on every native runner. Windows
ARM64 runs its native Go CLI and x64 PostgreSQL through OS emulation.

Production uses only the standard library. CI checks that invariant independently
of module tidy, runs vet/staticcheck, govulncheck, CodeQL, race tests (where Go
supports them), bounded archive fuzzing, and lifecycle resource-retention tests.
The security tooling runs in CI and is not imported by the library. Dependencies
in PostgreSQL distributions remain upstream native dependencies; Go scanning does
not scan those native libraries.

Native unit profiles and `go build -cover` subprocess profiles are merged by
statement location. Each statement contributes once; execution on any tested
platform covers it. OS-specific code remains in the denominator. Only compiled
reference examples are outside production scope. The aggregate job fails below
**90%** and blocks publishing. Its coverage profile and summary are downloadable
workflow artifacts. The merger itself has regression tests.

On each successful `master` push, the release job selects a new tag, builds six
CGO-disabled CLIs, creates SHA-256 checksums and a CycloneDX inventory, attests the
artifacts with GitHub build provenance, then publishes the draft release. All
required test, coverage and security jobs must have passed for that exact commit.
Pull requests, manual builds and scheduled security scans cannot publish.

`.github/release.json` starts with base `2.0.0`, channel `alpha`. Builds produce
`v2.0.0-alpha.1`, `.2`, etc. Set channel to `""` to launch stable v2; subsequent
master builds increment the patch automatically. Change `base` explicitly for a
minor release. Versioning does not commit back to master or create recursive
builds. A retry resumes an existing draft for its commit, and an already published
commit is a no-op. Published release artifacts are never overwritten.

GitHub Actions uses immutable action commit references, minimal job permissions,
and credentials disabled on checkout. Only the release job receives contents,
attestation and OIDC write permissions. Dependabot maintains action references.
Configure branch protection to require the v2 jobs before merging. No repository
settings, production release, or branch protection are changed by this PR.
