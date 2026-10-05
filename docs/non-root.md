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
choose a nonzero UID. An unprivileged caller cannot select another identity.
Windows uses restricted security tokens plus a kill-on-close Job Object; UID/GID
selection is a Unix feature.

The library creates a private workspace, assigns its ownership, and copies the
immutable binary installation and supervisor there when RunAs is used. This avoids
requiring access through `/root` or changing permissions on a caller's home/cache.
Data, password files, and sockets created by the library receive the same owner.

Caller-supplied workspace parents, socket parents, and persistent directories must
already be accessible to that account. Existing ownership is validated by the
native tools and is never recursively changed. A missing persistent directory may
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
