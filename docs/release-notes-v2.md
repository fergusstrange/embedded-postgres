# embedded-postgres v2 alpha

Run real PostgreSQL in Go tests with automatic cleanup, or use the CLI with any
test runner. v2 requires Go 1.26+ and has no third-party production Go dependencies.

## Start here

- **Go tests:** add `github.com/fergusstrange/embedded-postgres/v2` at this release's
  tag, then call `eptest.Start(t)`. PostgreSQL and the matching supervisor download
  automatically on first use and are cached for later tests.
- **Other test runners:** install the CLI and run `embedded-postgres exec -- YOUR_TEST_COMMAND`.
  It supplies `DATABASE_URL`, runs your tests and closes PostgreSQL afterward.
- **Upgrading from v1:** follow the [migration guide](https://github.com/fergusstrange/embedded-postgres/blob/master/docs/migration-v2.md).
  This is a major-version migration; use the `/v2` import path.

The [quick start](https://github.com/fergusstrange/embedded-postgres#readme) and
[CLI guide](https://github.com/fergusstrange/embedded-postgres/blob/master/docs/cli.md)
cover installation and configuration.

## Included in v2

- PostgreSQL 15–18; Linux, macOS and Windows on AMD64 and ARM64. Windows ARM64 uses
  x64 PostgreSQL under emulation.
- Isolated test instances, available ports, a shared verified binary cache and
  explicit persistent-data storage. Caller-supplied data is never automatically erased.
- Independent supervision for cleanup after owner exit or panic, and explicit
  Unix UID/GID support for tests running as root.
- Custom binary providers and lifecycle hooks for migrations and extensions.
- CLI foreground, command-scoped and background modes, plus protocol 1 for wrappers.

PostgreSQL's [native runtime requirements](https://github.com/fergusstrange/embedded-postgres/blob/master/docs/non-root.md)
still apply. Source checkouts and local module replacements need an explicitly
selected supervisor. v2 remains a prerelease while downstream migrations are validated.
