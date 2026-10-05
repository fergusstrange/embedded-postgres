# Tests and suites

```go
func TestStore(t *testing.T) {
    t.Parallel()
    pg := eptest.Start(t)
    // Connect your chosen PostgreSQL driver to pg.ConnectionURL().
}
```

Import `github.com/fergusstrange/embedded-postgres/v2/eptest`. Each call gets a
unique workspace, data directory, and automatically selected port. Binaries are
shared. Startup failures fail the test; cleanup is registered before startup.
Close your database connection pool before PostgreSQL cleanup. Since cleanups
run in reverse registration order, registering pool cleanup after `Start` does so.

To configure the instance, pass `embeddedpostgres.DefaultConfig().Port(0)` with
your chosen builders. Driver packages such as pgx, lib/pq, and migration tools
belong to your application, not to embedded-postgres.

## One server for TestMain

```go
var connectionURL string

func TestMain(m *testing.M) {
    os.Exit(eptest.Run(m, func(pg *embeddedpostgres.EmbeddedPostgres) {
        connectionURL = pg.ConnectionURL()
    }))
}
```

`Run` closes the server before returning the exit code. The supervisor handles
owner-process death when an unrecovered panic prevents TestMain from returning.
Tests sharing a server must still isolate their schemas/databases and close pools.
An instance per test is the default recommendation. Context-bound lifetime limits
are available through the core `StartContext` API.

For migration tools, open your chosen driver after Start, run migrations, then
execute tests. Alternatively use a Ready hook and return a cleanup that closes
the connection pool. Hooks run synchronously and must honour their context.

`eptest.Start` uses `t.Cleanup` as its normal teardown owner. It deliberately does
not bind the server lifetime to `t.Context()`, which is cancelled before cleanup
callbacks run. Hook cleanup panics therefore follow the direct `Close` policy
consistently, and failure diagnostics include the final shutdown log tail.

Lifecycle and inspection methods on one instance serialise behind an in-flight
`Start` or `Close`. `StopContext` is the exception for caller waiting: its deadline
also applies while startup holds the lifecycle lock; the queued cleanup continues
after startup finishes. Hooks must honour their contexts.
