# Contributing

Open pull requests against `master`. Keep changes reviewable; the v2 migration
uses milestone commits and review notes in one PR. Behavior changes need useful
regression tests and matching API/reference documentation. Production Go imports
must stay within this module and the standard library; test-only dependencies are
allowed when they improve the tests.

## Local setup

Install Go 1.26 or newer, then build the companion. Example for Apple Silicon:

```bash
go build -o /tmp/embedded-postgres ./cmd/embedded-postgres
python3 scripts/fetch-postgres.py 18.6.0 aarch64-apple-darwin pg.tar.gz
mkdir pg
tar -xzf pg.tar.gz -C pg --strip-components=1
export EP_SUPERVISOR=/tmp/embedded-postgres
export EP_TEST_BIN="$PWD/pg/bin"
export EP_TEST_ARCHIVE="$PWD/pg.tar.gz"
go test -race -count=1 -timeout 8m ./...
```

Select the pinned target for your platform from `internal/binaries/manifest.json`.
Windows uses `x86_64-pc-windows-msvc`, including on ARM64. Linux targets distinguish
`gnu` and `musl`. Install the native prerequisites in [non-root execution](docs/non-root.md).
Without fixture variables, real PostgreSQL tests skip; that is not full validation.

## Coverage and checks

After extracting the native fixture to `pg`, run:

```bash
python3 scripts/ci-test.py
go vet ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
```

The script builds an instrumented companion, runs race-enabled tests where
supported, and combines unit/subprocess coverage locally. CI merges all native
OS profiles and enforces **at least 90% production statement coverage**. Do not
exclude difficult production paths to meet the threshold. Include meaningful
failure, cancellation, filesystem-boundary and lifecycle tests. The repeated
lifecycle test checks retained goroutines, heap and temporary workspaces; race
checks and native process-death tests complement it.

The full workflow covers minimum/current Go, PostgreSQL 15–18, all six native
release platforms, Linux root-to-non-root use and Alpine. Tooling is pinned; do not
add CI tools to production imports. Go vulnerability analysis does not inspect
native PostgreSQL libraries, so binary updates also require upstream release and
support-policy review.

## Binary and release changes

Update distribution versions and SHA-256 pins together. Verify upstream assets
for every supported platform, run the matrix, and document any native prerequisite
change. Test mirror/download failures, immutable cache leases, unsafe archives and
offline behavior when changing acquisition code.

The master workflow publishes automatically only after every required job passes.
It starts with alpha prereleases. See [release policy](docs/releases.md) before
changing `.github/release.json`. Do not merge unfinished migrations merely to
exercise publishing. Keep provider and hook reference code compilable with the
normal module tests.
