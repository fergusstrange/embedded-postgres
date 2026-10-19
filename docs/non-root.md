# Running PostgreSQL as a different OS user

```go
pg := embeddedpostgres.NewDatabase(
    embeddedpostgres.DefaultConfig().Port(0).
        RunAs(embeddedpostgres.User{UID: 10001, GID: 10001}),
)
```

The account must already exist. On Unix the caller may remain root; only the
supervisor, initdb, postgres, and client utilities change identity. Supplementary
groups are cleared for a root caller. The Go application's credentials are never
changed, and the library never invokes sudo/su or creates users.

Default non-root callers use their own identity. Root callers must explicitly
choose both a nonzero UID and a nonzero GID; an omitted GID is not inferred from
the account. An unprivileged caller cannot select another identity, but may retain
its own GID 0 (for example, in an arbitrary-UID container).
Explicit IDs must fit Go's native `int`: at most 2147483647 on 32-bit hosts.
64-bit hosts retain the full Unix ID range except the unchanged-ID sentinel
4294967295. These bounds are checked before identity comparison or ownership changes.
Windows uses restricted security tokens plus a kill-on-close Job Object; UID/GID
selection is a Unix feature.

On Windows, supplied cache and workspace directories must grant access to the
actual user account. An ACL that grants access only through Administrators or
administrator ownership will not work after that group is disabled. The library
does not rewrite caller-owned directory permissions. Restricted child tokens and
the objects they create grant access to that user and SYSTEM, so tools can reopen
their own tokens without restoring administrator or power-user membership.

The library creates a private workspace, assigns its ownership, and copies the
immutable binary installation and supervisor there when RunAs is used. This avoids
requiring access through `/root` or changing permissions on a caller's home/cache.
Data, password files, and sockets created by the library receive the same owner.

The installation copy is per instance, including for a custom provider. For root
CI suites, use [`eptest.Run`](testing.md) to share one instance across the suite,
or run the test process itself as the non-root account to avoid `RunAs` copying.
A shared copy cache is deferred: it needs explicit identity, immutability and lease
rules for caller-owned providers, plus cleanup after owner death. Keeping a copy
inside the instance workspace preserves deterministic removal and avoids making
one user's cached installation accessible to another.

Caller-supplied workspace parents, socket parents, and persistent directories must
already be accessible to that account. With `RunAs`, a socket parent must exist
before startup; a missing parent is rejected before acquiring binaries or creating
a cluster. This preflight checks existence and directory type, not traversal
permissions on the parent or its ancestors; the native tools validate actual
access. The library only owns and removes its private child socket directory.
Existing ownership is never recursively changed. A missing persistent directory may
be created and assigned; existing contents are always preserved. Correct permissions
in your Dockerfile or provisioning step rather than granting broad access.

## Minimal Debian container

The corrected pinned distributions bundle their third-party runtime libraries.
The GNU bundles still need glibc 2.36 or newer and the matching ELF loader; the
musl bundles need musl 1.2.4 or newer and its dynamic loader. See the upstream
[support policy](https://github.com/theseus-rs/postgresql-binaries/blob/1d92c278f64f41ecc835b0e261abfb2d75ca2066/docs/support-policy.md)
for platform floors. Custom providers and explicitly selected older bundles can
have additional native-library requirements.

On Debian Bookworm, install CA certificates for verified downloads and create a
non-root account when building the image:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && useradd --uid 10001 --no-create-home pgtest \
    && rm -rf /var/lib/apt/lists/*
```

The Go executable is built with CGO_ENABLED=0; PostgreSQL remains a native binary.
Native CI tests both ordinary callers and a root test runner launching PostgreSQL
as a separate user.

## Arch Linux

The corrected PostgreSQL 15–18 pins include `libxml2.so.2`; installing
`libxml2-legacy` or setting `LD_LIBRARY_PATH` is not required. The published Go
module and CLI passed a clean Arch x64 container check with system libxml2 2.15,
including XML, `xml2`, `pgcrypto`, offline reuse, persistence and cleanup.
See the [results and reproduction steps](https://github.com/fergusstrange/embedded-postgres/issues/181#issuecomment-6259458842)
for the image pin and emulation scope. Explicitly selecting an old distribution
can still reproduce the missing-library error.

## Alpine / musl

The corrected musl bundles include ICU and the other third-party libraries;
there is no host ICU 74 package requirement for these pins. They remain dynamically
linked to musl, so the matching loader is required. The default-provider CI uses
Alpine 3.24 with the current Go toolchain and only its build/test prerequisites.
The published-binary regression campaign also exercised native x64 and ARM64
musl containers. Do not symlink incompatible library versions to satisfy a loader
error; check the selected distribution and its runtime requirements instead.
