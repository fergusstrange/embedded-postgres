# v2 issue and pull-request disposition

The v2 review covered the 8 open issues, 5 open feature PRs and 25 closed-unmerged
PRs present on 2026-10-05. After the native matrix, migration checks and publication
of v2.0.0-alpha.1, all 8 issues and 4 superseded feature PRs were closed on
2026-10-06. Each received an individual explanation and an invitation to confirm
that the v2 solution meets the original use case. Reporter confirmation is welcome;
closure records implementation, not confirmation on every downstream setup.

FreeBSD #169 remains open because its supported distribution and native CI work
are not complete. The separate Alpine update #172 also remains open: the current
musl PostgreSQL bundle requires ICU 74, which the proposed Alpine 3.24 image lacks.

## Requests reviewed for v2

| Issue / PR | v2 disposition | Validation / contract |
|---|---|---|
| [#170](https://github.com/fergusstrange/embedded-postgres/issues/170), Windows `.exe` lookup | Addressed | Platform suffix applied to every required tool; native Windows cache and lifecycle tests |
| [#168](https://github.com/fergusstrange/embedded-postgres/pull/168), [#167](https://github.com/fergusstrange/embedded-postgres/issues/167), cleanup | Addressed with clarified ownership | Disposable children removed; shared cache intentionally retained; explicit lease-aware prune |
| [#164](https://github.com/fergusstrange/embedded-postgres/issues/164), update policy | Addressed | Pinned supported major releases, security checks, automatic tested master releases and explicit prerelease channel |
| [#161](https://github.com/fergusstrange/embedded-postgres/pull/161), [#128](https://github.com/fergusstrange/embedded-postgres/issues/128), dynamic ports | Addressed | `Port(0)`, `GetPort`, actual connection URL and bounded collision retries; parallel native tests |
| [#160](https://github.com/fergusstrange/embedded-postgres/issues/160), parent death | Addressed | Independent owner-pipe supervision, Unix owned process groups, Windows Job Objects; forced termination and panic tests |
| [#157](https://github.com/fergusstrange/embedded-postgres/issues/157), test timeouts | Addressed | Startup deadline, lifetime context, bounded shutdown, `eptest.Start` and `eptest.Run` |
| [#155](https://github.com/fergusstrange/embedded-postgres/pull/155), own process group | Addressed internally | Mandatory process ownership rather than another configuration switch |
| [#154](https://github.com/fergusstrange/embedded-postgres/issues/154), erased downloads | Addressed | Binary cache is separate from disposable work; reuse and concurrent lease regressions |
| [#152](https://github.com/fergusstrange/embedded-postgres/pull/152), Unix sockets | Addressed | Socket-only mode in a private child directory; authenticated query regression |
| [#95](https://github.com/fergusstrange/embedded-postgres/issues/95), non-root execution | Addressed as core functionality | Explicit UID/GID for all Unix utilities and supervisor, restricted Windows token, native root integration and reference guide |
| [#169](https://github.com/fergusstrange/embedded-postgres/pull/169), FreeBSD 13/14 | Deferred from supported releases | No selected upstream gzip distribution or native hosted runner matrix; custom provider and source-built companion remain possible |

The implementation addresses all 8 open issue topics and 4 of 5 open PR topics.
The cache-cleanup request is resolved by an explicit shared-cache contract, rather
than deleting reusable binaries whenever a test stops.

## Closed or unmerged proposals revisited

| Prior work | v2 treatment |
|---|---|
| [#127](https://github.com/fergusstrange/embedded-postgres/pull/127), [#88](https://github.com/fergusstrange/embedded-postgres/pull/88), own/direct PostgreSQL process | Supervisor starts `postgres` directly and uses `pg_ctl` only for orderly shutdown |
| [#81](https://github.com/fergusstrange/embedded-postgres/pull/81), [#80](https://github.com/fergusstrange/embedded-postgres/pull/80), [#61](https://github.com/fergusstrange/embedded-postgres/pull/61), process attributes | Typed Unix identity contract; caller remains unchanged; arbitrary OS attributes are not exposed |
| [#52](https://github.com/fergusstrange/embedded-postgres/pull/52), [#51](https://github.com/fergusstrange/embedded-postgres/pull/51), [#109](https://github.com/fergusstrange/embedded-postgres/pull/109), custom binaries | Full `BinaryProvider` acquisition/release contract and compiled examples; covers non-network and nonstandard installations |
| [#99](https://github.com/fergusstrange/embedded-postgres/pull/99), [#67](https://github.com/fergusstrange/embedded-postgres/pull/67), [#75](https://github.com/fergusstrange/embedded-postgres/pull/75), races/cache | Cross-process leases, private extraction, atomic publication, copied configuration values and independent instance storage |
| [#134](https://github.com/fergusstrange/embedded-postgres/pull/134), dynamic ports | Incorporated in core with authenticated readiness and resolved endpoints |
| [#102](https://github.com/fergusstrange/embedded-postgres/pull/102), [#64](https://github.com/fergusstrange/embedded-postgres/pull/64), custom settings / TimescaleDB | Standard settings plus external extension distributions, preparation hooks and environment overrides |
| [#135](https://github.com/fergusstrange/embedded-postgres/pull/135), [#49](https://github.com/fergusstrange/embedded-postgres/pull/49), dependency updates | XZ/pq removed; gzip extraction and bundled native clients use standard Go APIs |
| [#119](https://github.com/fergusstrange/embedded-postgres/pull/119), Windows rename | Staged immutable installation and native Windows cache regression coverage |
| [#93](https://github.com/fergusstrange/embedded-postgres/pull/93), [#92](https://github.com/fergusstrange/embedded-postgres/pull/92), [#91](https://github.com/fergusstrange/embedded-postgres/pull/91), [#89](https://github.com/fergusstrange/embedded-postgres/pull/89), ARM macOS | Native ARM64 gzip distribution and native CI; no Rosetta requirement on the supported macOS path |
| [#138](https://github.com/fergusstrange/embedded-postgres/pull/138), [#104](https://github.com/fergusstrange/embedded-postgres/pull/104), versions | Explicit major compatibility checks and current pinned supported distributions |
| [#28](https://github.com/fergusstrange/embedded-postgres/pull/28), [#24](https://github.com/fergusstrange/embedded-postgres/pull/24), CI/Alpine | CircleCI removed; native GitHub Actions and separate musl integration |

Additional historical regressions inform tests: unsafe data deletion
[#15](https://github.com/fergusstrange/embedded-postgres/issues/15), persistence
[#151](https://github.com/fergusstrange/embedded-postgres/issues/151), panic cleanup
[#29](https://github.com/fergusstrange/embedded-postgres/issues/29), chunked downloads
[#123](https://github.com/fergusstrange/embedded-postgres/issues/123), quoted database
names [#126](https://github.com/fergusstrange/embedded-postgres/issues/126), and leak
report [#58](https://github.com/fergusstrange/embedded-postgres/issues/58).
CLI requests [#9](https://github.com/fergusstrange/embedded-postgres/issues/9) and
[#114](https://github.com/fergusstrange/embedded-postgres/issues/114) now have a
versioned lifecycle protocol. Extension requests, including
[#163](https://github.com/fergusstrange/embedded-postgres/issues/163), stay outside
core through the documented provider/hook contracts.
