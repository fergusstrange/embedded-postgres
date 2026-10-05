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
