# Moving from v1 to v2

Change imports to `github.com/fergusstrange/embedded-postgres/v2`. v1 remains
available under its original module path. v2 requires Go 1.26 or newer.

| v1 usage | v2 behaviour or replacement |
| --- | --- |
| `NewDatabase`, `DefaultConfig`, `Start`, `Stop` | Retained; add `StartContext`, `StopContext`, or idempotent `Close` |
| `DefaultConfig().Port(5432)` | Same; choose `Port(0)` for an automatically selected port |
| `Config.GetConnectionURL()` | Static configuration only; use instance `ConnectionURL()` after startup |
| `RuntimePath(path)` | Parent for a uniquely named workspace; the supplied path is never erased |
| `CachePath(path)` | Shared extracted binaries; the v1 archive cache is not reused |
| `DataPath(path)` | Persistent data; never erased or upgraded automatically |
| `BinariesPath(path)` | `LocalProvider(path)`; must include postgres, initdb, pg_ctl, psql, createdb |
| `BinaryRepositoryURL(url)` | gzip mirror layout `/VERSION/ASSET.tar.gz`; Maven/JAR URLs are not compatible |
| `CacheLocator`, `RemoteFetchStrategy`, `VersionStrategy` | Replace with `BinaryProvider` or `DownloadProvider` configuration |
| `V9` through `V14` | Removed from the supported catalogue; v2 supports PostgreSQL 15–18 |
| `StartParameters` | Retained; lifecycle-owned paths/endpoints use dedicated settings |
| `Logger(io.Writer)` | Retained; receives redacted server output during cleanup |
| Implicit lib/pq registration | Removed; explicitly import/register your own driver |

Defaults are PostgreSQL 18.6, database/user/password `postgres`, locale `C`,
encoding `UTF8`, startup timeout two minutes, shutdown timeout ten seconds,
loopback TCP, and no console logging. `Start` remains compatible with fixed port
5432. `eptest.Start` without a config uses an automatic port and automatic cleanup.
If you supply a config to the test helper, explicitly choose `Port(0)` if wanted.

`StartContext(ctx)` binds the entire instance lifetime to ctx. Cancellation after
readiness stops the server. `Close` uses its own bounded cleanup context, so an
already cancelled test context does not skip shutdown. `Stop` retains the
`ErrServerNotStarted` behaviour; `Close` can safely be called repeatedly.

The supervisor is an independent executable. Released module consumers acquire
its matching release automatically. When using a source checkout or `replace`,
build `go build -o /absolute/path/embedded-postgres ./cmd/embedded-postgres` and set
`EP_SUPERVISOR` or `Config.Supervisor`. Offline users must prefetch PostgreSQL and
provide an already installed supervisor. No system Go compiler is used at runtime.

On Unix, a root caller must supply `RunAs(User{UID: ..., GID: ...})` for an existing
non-root OS user. That user needs access to any caller-supplied persistent data or
workspace parent. The library never creates accounts or changes caller-owned data
permissions. Windows uses restricted tokens and rejects Unix UID/GID configuration.
