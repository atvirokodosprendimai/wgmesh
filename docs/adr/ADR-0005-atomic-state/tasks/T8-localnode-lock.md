# Task ADR-0005-T8: Put the local node's mutable fields under one lock

**Depends-on:** T7
**Covers:** none — no spec
**Estimated scope:** L (multi-file)
**Owner:** unassigned
**Produces:** `LocalNode.GetMeshIP/SetMeshIP`, `GetMeshIPv6/SetMeshIPv6`, `GetNATType/SetNATType`, `Snapshot()`; `TestLocalNodeFieldsNotWrittenDirectly`
**Consumes:** the one `LocalNode` type (T7)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`MeshIP`, `MeshIPv6` and `NATType` of the local node are written and read under one new `sync.RWMutex` (`stateMu`), beside the two locks that already exist (`endpointMu` for the endpoint, `routesMu` for `RoutableNetworks`, which PR #895 added with `GetRoutableNetworks`/`SetRoutableNetworks`, merged 2026-11-15, `14a89ff`); one mutex per field group, no lock is held across two groups. `go test -race` over the packages that read them in goroutines is clean, and a direct assignment from outside `pkg/node` fails a test.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/types.go` | edit | add `stateMu` for `MeshIP`, `MeshIPv6`, `NATType` (the existing `endpointMu` and `routesMu` stay as #895 left them); the three fields become unexported (`meshIP`, `meshIPv6`, `natType`), `RoutableNetworks` becomes unexported (`routableNetworks`) so the compiler selects every site; getters and setters for the three; `Snapshot() LocalSnapshot` reads each group under its own lock in turn (never nested), so a group is consistent, the groups are not read at one instant; `GetRoutableNetworks`/`SetRoutableNetworks` already copy (#895) and move with the type |
| `pkg/daemon/daemon.go` | edit | the writes at `:378`, `:380`, `:391`, `:397`, `:490`, `:1719` call setters; the reads at `:224`, `:712`, `:974`, `:1273`, `:1534`, `:1847` call getters; the construction in `initLocalNode` (`:422`) sets the fields through the constructor `node.NewLocalNode(...)` or setters before any goroutine starts |
| `pkg/daemon/collision.go` | edit | `:119-120` use `GetMeshIP` and `SetMeshIP` (ADR-0002 T4 and T5 edit nearby lines: this follows them) |
| `pkg/discovery/exchange.go` | edit | announcement builders at `:507`, `:560`, `:563`, `:1170` read one `Snapshot()` |
| `pkg/discovery/lan.go`, `pkg/discovery/gossip.go` | edit | `lan.go:161`, `gossip.go:90`, `:228` read through getters or `Snapshot()` |
| `pkg/discovery/dht.go` | edit | `NATType` writes at `:256`, `:272`, `:285`, `:422`, `:445` call `SetNATType` |
| `pkg/node/localnode_lock_test.go` | add | the three behaviour tests |
| `pkg/daemon/localnode_guard_test.go` | add | `TestLocalNodeFieldsNotWrittenDirectly` |

The list of sites is the output of the two greps in the parent audit (2026-11-15, `b8b83f3`: 12 writes, 62 reads); the task re-runs both at its start and its end, and the end run must print 0 direct writes outside `pkg/node`. `grep -rn "RoutableNetworks" --include=*.go . | grep -v _test.go` also lists announcement structs and `PeerInfo.RoutableNetworks`, which are not the local node's and are not touched.

Selection: with the fields unexported the compiler selects every site; the guard test is the second selector for a reintroduced exported field.

Ordering note: follows ADR-0002 T9, T7 of this record (which moves the type, with `ObserveEndpoint`, into `pkg/node/types.go`) and ADR-0002 T4 and T5 (`collision.go`, `daemon.go`); ADR-0002 T9 also edits `dht.go` and `exchange.go`.

## Ordered Steps

1. [S1] Add the four tests with compiling stubs (getters and setters over the exported fields as they are, guard test that parses the files) and run red: the concurrency test under `-race` reports the race that exists today, the copy tests fail, the guard lists the 12 writes.
2. [S2] Implement the lock, accessors, `Snapshot`, constructor; unexport the four fields.
3. [S3] Switch the sites file by file, one commit per package, and fix the compile errors the unexported fields cause. [proof: acceptance]
4. [S4] Run both greps again and paste the counts. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: `SetMeshIP` without the lock (under `-race`); `GetRoutableNetworks` returns the internal slice; one direct field write reintroduced in `dht.go` (a test-only exported escape hatch is not allowed). [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node ./pkg/daemon -run '^(TestLocalNodeAccessorsConcurrent|TestRoutableNetworksAreCopied|TestSnapshotMatchesAccessors|TestLocalNodeFieldsNotWrittenDirectly)$' -count=1 -race -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go build ./... && go vet ./... && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLocalNodeAccessorsConcurrent` | `pkg/node/localnode_lock_test.go` | 8 goroutines set and get `MeshIP`, `MeshIPv6`, `NATType`, the routes and the endpoint for 200 iterations each; clean under `-race` | — | S1, S2 |
| `TestRoutableNetworksAreCopied` | `pkg/node/localnode_lock_test.go` | mutating the slice passed to the setter, or returned by the getter, does not change the node | — | S1, S2 |
| `TestSnapshotMatchesAccessors` | `pkg/node/localnode_lock_test.go` | `Snapshot` returns what the getters return at that moment and its routes are a copy | — | S1, S2 |
| `TestLocalNodeFieldsNotWrittenDirectly` | `pkg/daemon/localnode_guard_test.go` | parses the module's non-test files outside `pkg/node` and fails on an assignment to a selector named `MeshIP`, `MeshIPv6`, `NATType` or `RoutableNetworks` whose receiver is a local-node expression (`localNode`, `d.localNode`, `pe.localNode`, `GetLocalNode()`); composite literals and `PeerInfo` fields are not flagged | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | unexported fields force every site through the accessors; the guard catches an exported field returning |
| 3 — the caller can discover it | doc comments on the accessors; T11 amends the daemon lifecycle spec |
| 4 — it is used | every announcement, probe, reconcile and STUN refresh |

## Mutation Log

## Invariants

- What is announced, logged and reported by `daemon.status` is the same string as before for one node at rest.
- `WGPubKey`, `WGPrivateKey`, `Introducer` and `Hostname` stay plain fields: the audit shows them written only in `initLocalNode` before goroutines start (grep of 2026-11-15; `Introducer` and `Hostname` at `daemon.go:398-399`).
- No setter calls a getter under the lock, no lock is held across a call out of the type (a deadlock rule).
- `pkg/node` still imports nothing from `pkg/daemon` or `pkg/discovery`.

## Risks

- A write the grep misses (a different receiver name) compiles only if the field were exported; making the fields unexported turns any miss into a compile error.
- The guard's receiver-name heuristic can be evaded by an alias of the node; it states that limit and the compiler is the real guard.
- `Snapshot` returns a copy of the routes on every announcement; the lists are short (a handful of CIDRs), not measured.
- Test doubles: none; the tests use the real type.

## Stop Condition

Stop and ask if ADR-0002 T9 changed `LocalNode` so `stateMu` cannot stand beside `endpointMu` (for example if `ObserveEndpoint` took a lock of its own that must also cover `MeshIP`), or if an external embedder of `pkg/daemon` sets the fields directly. Before starting, `git fetch origin` and re-run both greps of the audit: PR #895 (merged 2026-11-15) already moved the `RoutableNetworks` sites to its accessors, so the write count at the start is lower than the 12 of the audit.

## Out of Scope

- `ObserveEndpoint` and the STUN rule (ADR-0002 T9; permanent: boundary: owned by ADR-0002).
- The `PeerStore`, which already has its own lock (permanent: boundary: owned by ADR-0002 and `pkg/node/store.go`).

## Verification Log
