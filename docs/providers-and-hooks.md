# Binary providers and lifecycle hooks

`BinaryProvider.Acquire(ctx, request)` returns an immutable `Installation` and a
release callback. It owns fetching, verification, extraction, and caching. Release
runs exactly once after owned PostgreSQL processes stop, including failed startup.
Providers must support concurrent calls and honour cancellation.

## Built-in providers

- `DownloadProvider{}`: release-pinned gzip archives with SHA-256 verification,
  bounded extraction, atomic installation, and shared process leases.
- `DownloadProvider{CacheDir: path, Offline: true}`: cached installations only.
- `DownloadProvider{BaseURL: url, Client: client}`: mirror `/VERSION/ASSET.tar.gz`
  with a caller-supplied HTTP transport. Mirror bytes must match the pinned digest.
- `DownloadProvider{Target: "aarch64-unknown-linux-musl"}`: explicit distribution
  target for systems whose libc cannot be inferred from `/etc/alpine-release`.
- `LocalProvider(path)`: complete installation with postgres, initdb, pg_ctl,
  createdb, and psql under `bin/`. Native library requirements still apply.

A custom provider can implement any source or archive format, including existing
Maven/XZ assets. Such dependencies live in the provider's own module. The core does
not import them. See the compiled examples in `examples/custom`.

A provider lease must remain valid until Release: never prune or mutate binaries
being used by another instance. Cache keys should include version, platform,
architecture/libc, and any extension/patch identity. Do not patch the built-in
shared cache in place. A NixOS provider should prepare and cache its own patched
installation before returning it, then supply environment changes with Environment.

## Hooks

`BeforeStart` runs after initdb and before launching the server. It can prepare
instance configuration files. `Ready` runs after PostgreSQL accepts an authenticated
query and the requested database exists. Hooks receive an InstanceInfo snapshot.
The endpoint is available in Ready; BeforeStart must not assume an assigned port.

Every hook may return a cleanup, even alongside an error. Registered cleanups run
in reverse order after PostgreSQL stops, before the workspace and binary lease are
released. An error aborts startup. A panic triggers cleanup and is rethrown. Hooks
must honour context cancellation and must not call lifecycle methods on their own
instance. Cleanup uses an independent, bounded context. All registered cleanups run even
if one panics. Direct Close/Stop calls propagate cleanup panics after releasing
resources; asynchronous cleanup records them in Err instead.

## Extensions and migrations

For pgvector, AGE, PostGIS, or TimescaleDB, use a distribution/provider containing
the extension libraries and matching PostgreSQL ABI, plus its control/SQL files.
Then enable it from a Ready hook with your driver. Configure any required
`shared_preload_libraries` using StartParameters before startup. The project does
not download/build arbitrary extensions or promise support for their native code.

Migration frameworks can run in Ready with the resolved connection URL; return
pool cleanup from the hook. Template-database setup can use the same route. For
ordinary test setup, simply open a pool after eptest.Start and register its cleanup.

The provider and hooks are Go APIs. CLI JSON describes built-in providers and
PostgreSQL settings, not executable Go callbacks. Other-language wrappers can run
migrations after the ready event or supply their own prepared distribution.
