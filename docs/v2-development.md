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
  the CLI protocol contract for Node wrappers.
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

## Milestone 6 build review

- Replaced CircleCI, Nancy and obsolete lint tooling with native GitHub Actions,
  minimum/current Go checks, govulncheck, staticcheck, CodeQL and archive fuzzing.
- Added statement-union coverage across native jobs and instrumented subprocesses;
  the 90% gate uses pipefail and is a required release dependency.
- Release automation is master-only, idempotent per commit, starts in alpha, and
  publishes six CGO-disabled CLIs with checksums, SBOM and build attestations.
- Added repeated real lifecycle checks for retained goroutines, heap and workspaces,
  plus TestMain success/failure/cleanup-failure subprocess regressions.
- Native Windows confirmed password-file environment restoration; early integration
  failures then exposed undersized test deadlines during disk-heavy initdb.
- Linux concurrent launches exposed a writable-descriptor inheritance race
  (ETXTBSY). Executable copies now hold Go's Linux fork lock until their writable
  descriptor closes. Root test fixtures now use traversable temporary parents.
- Coverage has improved from 61% to 82% locally; final native union is pending.

- All six native Go test suites passed in the replacement workflow. Windows report
  generation then found a zero-statement stub division-by-zero; added a merger
  regression and kept the stub neutral in the weighted denominator.
- Upstream musl bundles require ICU 74. CI now pins the compatible Alpine 3.21
  runtime and a separate current Go toolchain, with immutable image digests.
- govulncheck's older published release was incompatible with Go 1.27; selected
  the current v1.8.0 source tag. CodeQL analysis already passed.
- Rewrote README/contribution docs and mapped all 8 open issues, 5 open PRs and
  25 closed-unmerged proposals. FreeBSD remains outside the supported matrix.

## Final review

- Aggregate native production coverage reached **90.60% (1445/1595 statements)**
  at `eb6a687`, including CLI, supervisor and platform-specific implementations.
  The final workflow recalculates coverage and enforces 90% for every release.
- TestMain coverage is flushed after suite cleanup so failure-return paths are
  measured. Test-scoped cleanup also reports failures from a prior context-driven
  close. Regression subprocesses cover assertion, startup and cleanup failures.
- Native Windows fault injection verifies token/job setup failures release their
  handles. Typed syscall boundaries preserve pointer lifetimes across injected
  calls; native successful launches exercise the real API.
- Full local race tests pass against PostgreSQL 18.6. The six-target release dry
  build succeeds and all artifact checksums verify; nothing has been published.
- Bash installer fixtures cover all six targets, checksum rejection and paths with
  spaces. They consume complete release listings to avoid SIGPIPE under pipefail.
  Native Windows jobs also exercise the PowerShell installer with HTTP fixtures.
- Alpine now reaches the native suite with ICU 74. Its startup-failure regression
  uses an invalid encoding because musl accepts arbitrary locale names.
- Final ownership review added a per-launch PostgreSQL setting to readiness checks:
  a listener with the same credentials on a contested port cannot satisfy another
  instance's probe. Exhausted port retries preserve their startup error.
- A panicking log writer now follows the same cleanup policy as a panicking hook:
  release resources first, then propagate the panic. Socket cleanup is registered
  immediately after creating the private directory, before changing ownership.

## PR review: Unix identity conversions

- Addressed all four CodeQL integer-conversion findings with one checked UID/GID
  conversion shared by identity validation and ownership changes. Values that do
  not fit the host's int are rejected before conversion; 64-bit hosts retain large
  valid Unix IDs. Root UID and unchanged-ID sentinels remain rejected.
- Added boundary and pre-chown rejection regressions, including an executed Linux
  386 test in native CI. Its profile contributes to the aggregate coverage gate.
- CodeQL confirmed all four alerts fixed and automatically resolved their review
  threads. Native/platform tests and the aggregate coverage gate passed.
- Disabled the obsolete CircleCI GitHub webhook through the GitHub API after the
  CircleCI CLI identified the legacy OAuth pipeline. Branch protection is unchanged.

## PR review: lifecycle and platform follow-up

Reviewed all twelve Claude comments submitted under the repository owner's account
on PR #171. Eleven correctness or maintenance findings are addressed:

- `4181773940`: Windows background children use `DETACHED_PROCESS`. A native test
  starts an attached console parent and verifies its detached child has no console.
- `4181773951`: pseudo-versions (all three Go forms) are rejected before supervisor
  release acquisition and use the source-build configuration hint.
- `4181773962`: readiness rejects permanent authentication/HBA failures promptly.
  Real reused-cluster tests cover changed passwords and missing roles.
- `4181773970`: unsupported binary targets name the requested OS, not the host OS.
- `4181774010`: CLI exec forwards Unix interruption, allows bounded teardown while
  PostgreSQL remains available, and returns signal-specific exit codes. Tests cover
  SIGINT, SIGTERM, a teardown SQL query, natural signal exits and forced escalation.
- `4181774022`, `4181774054`: scoped test cleanup has one normal owner and reads
  logs after shutdown. Subprocess tests verify hook panic propagation and final logs.
- `4181774028`: StopContext deadlines include time waiting behind startup. The
  queued cleanup still finishes; inspection-method serialization is documented.
- `4181774046`: the owner no longer reconstructs a process handle from a remembered
  PostgreSQL PID. Only the supervisor owns PostgreSQL termination; fallback uses
  the retained supervisor handle and reports that cleanup could not be confirmed.
- `4181774063`: RunAs rejects a missing socket parent before binary acquisition.
  Existing caller-owned parents are not modified; native root CI covers this case.
- `4181774079`: executable suffix handling lives in internal/platform for both
  provider and supervisor use.

`4181774071` (sharing RunAs installation copies) is deferred, not treated as a
correctness fix. Per-instance copies retain predictable ownership and cleanup for
custom provider leases. The non-root guide documents this cost and the existing
suite-level wrapper/non-root-runner alternatives. A future shared-copy design needs
explicit identity and immutability keys and owner-death cleanup before adoption.

Local validation: full race-enabled PostgreSQL integration suite passes on macOS
ARM64, including the new authentication, queued shutdown, scoped cleanup and CLI
signal tests. Windows console and root identity tests run in the native CI matrix.

## Final RunAs review

- `4191391592`: root callers must explicitly set a nonzero GID as well as UID.
  The check runs before provider acquisition. Non-root callers may still retain
  their own GID 0. Native Linux CI runs the platform tests as root and verifies
  the arbitrary-UID/GID-0 case in a credential-isolated subprocess.
- `4191391593`: the missing socket-parent diagnostic now describes only its
  existence/type check. The non-root guide makes clear that native tools validate
  traversal permissions; no ownership or permission changes are made to ancestors.
