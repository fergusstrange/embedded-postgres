# CLI and wrapper protocol

The CLI runs the same lifecycle implementation as the Go library. It supports
Linux, macOS and Windows on AMD64 and ARM64. Windows ARM64 uses x64 PostgreSQL
under OS emulation. Native PostgreSQL runtime prerequisites still apply; see
[non-root execution](non-root.md).

## Install

After the first v2 release is published:

```bash
curl -fsSL https://raw.githubusercontent.com/fergusstrange/embedded-postgres/master/install/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
```

Set `EP_VERSION` to pin a published v2 tag and `EP_INSTALL_DIR` to choose a
writable location. The script verifies the release SHA-256 before installing.
Windows users can run it in Git Bash or use [install.ps1](../install/install.ps1).
The default selects the newest published v2 release, including prereleases during
v2 development. Downloads do not bundle PostgreSQL; `prefetch` prepares an offline
cache. Build from source with `go build ./cmd/embedded-postgres`.

```bash
embedded-postgres prefetch
embedded-postgres exec -- npm test
embedded-postgres run --json --parent-stdin
embedded-postgres start --state-file ./private/postgres.json --json
embedded-postgres status --state-file ./private/postgres.json --json
embedded-postgres stop --state-file ./private/postgres.json --json
embedded-postgres prune
```

`run` stays in the foreground and handles SIGINT/SIGTERM. `exec` injects
`DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` and
`PGSSLMODE` into the test command, preserves its standard streams and exit status,
and closes PostgreSQL before returning. A test command that starts detached child
processes must manage those processes itself.

`start` intentionally outlives its launcher. The state file contains a random
control token and a loopback address, never a PID used for killing. Keep it in a
private directory. `status` and `stop` authenticate to that instance; `stop`
returns after lifecycle cleanup. Concurrent use of the same state file is
rejected by a process-held lease. Stale files can be removed after an interrupted
owner; no command signals a process based on a stale PID. Diagnostic output is
appended to `STATE_FILE.log`. Windows file privacy follows the parent directory's
ACL; choose a directory restricted to your account.

## Configuration

Precedence: defaults, JSON file (`--config` / `EP_CONFIG`), `EP_*` environment,
explicit flags. Unknown JSON fields and flags are errors. `COMMAND --help` lists
all flags. JSON field names and matching environment names include:

| JSON | Flag | Environment |
|---|---|---|
| `postgres_version` | `--postgres-version` | `EP_VERSION` |
| `port` | `--port` | `EP_PORT` |
| `database`, `username`, `password` | corresponding flag | `EP_DATABASE`, `EP_USERNAME`, `EP_PASSWORD` |
| `cache_dir`, `work_dir`, `data_dir` | `--cache-dir`, `--work-dir`, `--data-dir` | corresponding uppercase `EP_*` |
| `binaries`, `mirror`, `offline` | corresponding flag | `EP_BINARIES`, `EP_MIRROR`, `EP_OFFLINE` |
| `user` | `--user UID:GID` | `EP_USER` |
| `socket_dir` | `--socket-dir` | `EP_SOCKET_DIR` |
| `start_timeout`, `stop_timeout` | corresponding flag | corresponding uppercase `EP_*` |
| `state_file`, `json`, `parent_stdin` | corresponding flag | corresponding uppercase `EP_*` |
| `parameters` object | repeated `--set NAME=VALUE` | — |

The CLI defaults to an available port and a random password. Persisted clusters
must be restarted with the same explicit credentials. Prefer a private config
file or environment variable for passwords; command-line flags appear in process
listings. `data_dir` is persistent and never automatically erased. `work_dir` is
only a parent for private disposable children.

```json
{
  "postgres_version": "18.6.0",
  "port": 0,
  "database": "app_test",
  "start_timeout": "30s",
  "parameters": { "max_connections": "40" }
}
```

## Protocol 1 for Node and other wrappers

Spawn `run --json --parent-stdin` with stdin/stdout pipes and inherited or captured
stderr. Keep the stdin write end open for the entire database lifetime. Parse
stdout as JSON lines and require `protocol == 1`; tolerate unknown extra fields.

```json
{"protocol":1,"event":"ready","connection_url":"postgresql://...","port":54321}
{"protocol":1,"event":"stopped"}
```

Startup/runtime failures emit an `error` event containing an `error` string and
exit nonzero. `ready` means an authenticated query succeeded and the requested
database exists. Treat its connection URL as a credential. Do not log it in
shared CI output. Server diagnostics use separate files, never protocol stdout.

On teardown, close stdin and wait for `stopped` plus process exit. Apply your
wrapper's outer deadline longer than `stop_timeout + 2s`. An unexpected wrapper
exit closes the pipe; an unrecovered panic or forced termination of the Go/CLI
owner closes a second pipe watched by the independent supervisor. Whole-machine
failure or simultaneous termination of the supervisor and PostgreSQL cannot be
made graceful by a userspace library.

## Native Go → Node binding assessment

[Node-API](https://nodejs.org/api/n-api.html) exposes a stable **C** ABI. Go can
export C entry points with [cgo](https://pkg.go.dev/cmd/cgo) and `-buildmode=c-shared`,
but this requires an additional Node addon, a C toolchain, native artifact builds,
and careful runtime/thread ownership. It does not remove the need for independent
process supervision. There is no standard-library Go-to-Node binding.

Use the CLI pipe protocol for the first Node wrapper: it shares the exact tested
Go lifecycle, works without a compiler on the consuming machine, and isolates Go
runtime failures from the Node test runner. A native addon can remain an external
consumer later; no Node implementation is included in this milestone.
