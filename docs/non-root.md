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

Install the selected binary distribution's native runtime libraries and create a
non-root account when building the image. The tested glibc distribution needs
OpenSSL 3, readline, libxml2, Kerberos, zlib, lz4, and zstd. On Debian Bookworm:

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 libreadline8 libxml2 libgssapi-krb5-2 \
    zlib1g liblz4-1 libzstd1 \
    && useradd --uid 10001 --no-create-home pgtest \
    && rm -rf /var/lib/apt/lists/*
```

The Go executable is built with CGO_ENABLED=0; PostgreSQL remains a native binary
with OS-library prerequisites. A custom BinaryProvider can supply a distribution
with additional bundled libraries. Native CI tests both ordinary callers and a
root test runner launching PostgreSQL as a separate user.

## Alpine / musl

The selected upstream musl bundle links ICU **74**, so the default-provider CI
uses Alpine **3.21** with `icu-libs` plus the other native libraries. Its package
[provides the required ICU 74 shared libraries](https://pkgs.alpinelinux.org/package/v3.21/main/x86_64/icu-libs).
The test image uses the current Go toolchain, independently of its runtime base.

Newer Alpine releases with a different ICU ABI need a compatible custom
PostgreSQL distribution through `LocalProvider`/`BinaryProvider`, or an upstream
bundle refresh. Do not symlink incompatible ICU major versions to satisfy the
loader. This is a native-distribution compatibility constraint, not a Go-module
dependency. Recheck it when updating PostgreSQL pins or the Alpine CI image.
