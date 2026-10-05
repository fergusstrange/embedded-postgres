# v2 implementation and review ledger

One implementation branch and one final PR. Each milestone has its own commit,
validation, and diff review. PostgreSQL 14 and older are excluded. CircleCI is
removed in favour of GitHub Actions. Non-root launching is a core requirement.

1. Binary distribution, OS identity, supervisor foundations, native CI.
2. Standard-library-only core, safe storage and cache ownership.
3. Compatibility and testing helpers.
4. Extension hooks and executable reference guides.
5. CLI, versioned protocol, installers.
6. Complete CI/security/coverage/release automation and integration review.

## Architecture decisions

- PostgreSQL archives use gzip; runtime extraction requires no XZ library.
- The supervisor is a separate executable. Importing the library does not install
  signal handlers or re-execute the calling application.
- The caller retains its identity. Unix children receive explicit UID/GID and no
  inherited supplementary groups when the caller is root. Windows children use a
  restricted token. The supervisor holds a kill-on-close Windows Job Object.
- Parent liveness is an inherited pipe, not PID polling. PostgreSQL never inherits
  its write end. EOF requests bounded shutdown of the owned process tree.
- Data supplied by callers is never automatically erased. Temporary workspaces
  and cached binary installations have separate lifetimes.

## Milestone 1 review

- Confirmed native macOS gzip bundle SHA-256 and bundled psql/createdb/initdb.
- macOS integration passed graceful close, forced parent termination, and an
  unrecovered panic in another goroutine. All cases removed postmaster.pid.
- Cross-compiled the supervisor for Windows AMD64/ARM64 and Linux ARM64.
- Archive rejection tests and shared/exclusive cross-process lock primitives pass.
- Linux packages require native runtime libraries (OpenSSL 3, readline, libxml2,
  Kerberos, zlib/lz4/zstd); these are OS prerequisites, not Go dependencies.
- Review caught a deadlocking test owner; it now stays alive with a timer and
  receives an explicit crash trigger after authenticated database readiness.
- Windows execution still requires native CI validation; compilation is not a
  claim that Job Objects and restricted tokens have been tested at runtime.

## Milestone 2 review

- Production module now has no require directives and no external imports.
- macOS race-enabled real integration tests pass parallel databases, custom SQL
  identifiers, cancellation, persistence, nonempty-directory protection, hooks
  that return errors, and hooks that panic.
- Cache installs publish only after checksum verification, extraction, and tool
  validation. Shared leases prevent pruning active installations.
- Review added post-extraction symlink validation, a gzip-footer check, and
  protection for unknown cache directories during pruning.
- Setup/client commands now also use an independent pipe-watching supervisor.
  Cancellation closes the owner pipe; only that supervisor's own child group is
  terminated. No supervisor sends a signal to its own/inherited process group.
- Native milestone-1 CI passed Linux AMD64/ARM64 (including root-to-non-root),
  macOS AMD64/ARM64, and Windows AMD64/ARM64 (x64 PostgreSQL under emulation).

## Milestone 3 review

- Added eptest.Start with cleanup registered before startup and t.Context lifetime.
- Added eptest.Run for TestMain; os.Exit occurs only after deferred cleanup.
- Replaced stale nested modules with compiled v2 examples and documented all
  incompatible path/provider/version/driver-registration changes explicitly.
- Verified test-helper cleanup and endpoint resolution in real integration tests.

## Coverage acceptance requirement

User requirement: at least 90% aggregate statement coverage for production Go
code, including library, CLI, supervisor and OS-specific implementations. Native
platform profiles and instrumented subprocess profiles are merged; releases fail
below 90%. Examples/development scripts are not production Go packages. Coverage
work must exercise real failure and regression scenarios, not remove hard code
from the denominator.

## Milestone 4 review

- Provider and hook contracts specify cancellation, immutability, cleanup order,
  errors and panic propagation. Added provider release-on-failure regression.
- Compiled custom-provider/migration/extension examples keep driver dependencies
  outside the core. Documented native-extension ABI and NixOS responsibilities.
- Non-root guide includes actual tested OS-library and directory requirements.

## Milestone 5 review

- Added foreground/parent-pipe, command-scoped and explicit background CLI modes,
  authenticated local status/stop, cache commands and strict config precedence.
- Added protocol 1 and checksum-verifying Bash/PowerShell installers; documented
  the Node wrapper contract and assessed Node-API/cgo tradeoffs.
- Real CLI regressions cover stdin EOF, detached startup/status/stop and child exit
  propagation. A broken-stdout regression found and fixed premature SIGPIPE exit
  before state-file cleanup. Server log tails now use bounded memory.
- Windows authenticated startup is under investigation in native CI. Coverage
  measurement is being established; 90% has not yet been demonstrated.

## Integration review corrections

- Concurrent cache acquisition found a lock-upgrade deadlock. Losing installers
  now re-check under shared leases rather than waiting behind active instances.
- Windows authentication diagnostics established that password files failed while
  URI authentication succeeded. Go's token environment default discarded the
  supervisor's curated environment. Child commands now explicitly inherit it;
  the permanent regression tests both forms with escaped credentials.
- Canonicalized installation roots before validating symlinks, handling macOS
  /var and /private/var aliases without accepting links outside the installation.
- Old lifecycle watchers cannot close a restarted instance. Canceled StopContext
  still finishes cleanup. Cleanup panics run remaining cleanups before propagating.
- Reserved PostgreSQL settings are protected case-insensitively. Unix unchanged-ID
  sentinels are rejected before any identity or ownership operation.
- Expanded local race-enabled integration suite passes, including concurrent
  verified cache installs, failure cases, socket-only use, and lifecycle restarts.
