# embedded-postgres v2

A testing-first lifecycle and CLI with no third-party production Go dependencies.

- Private temporary instances, shared verified binary cache and persistent data
  protection; custom binary providers can own download/extract/cache policy.
- PostgreSQL 15–18 and native AMD64/ARM64 Go executables for Linux, macOS and
  Windows. Windows ARM64 uses x64 PostgreSQL under emulation.
- Explicit Unix UID/GID launching, Windows restricted tokens and independent
  parent-pipe supervision, including startup/client commands.
- Test helpers, lifecycle hooks, authenticated readiness and dynamic test ports.
- CLI run/exec/start/status/stop, versioned JSON pipe protocol, verified installers.

Read [the migration guide](https://github.com/fergusstrange/embedded-postgres/blob/master/docs/migration-v2.md)
before upgrading. Source builds must configure a companion CLI. PostgreSQL native
runtime libraries are OS prerequisites. Existing data is never automatically
removed. v2 alpha releases remain prereleases until the release channel is changed
explicitly after migration review.
