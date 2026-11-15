# Task ADR-0005-T8: Put the local node's mutable fields under one lock

**Depends-on:** T7
**Covers:** none — no spec
**Estimated scope:** L (multi-file)
**Owner:** unassigned
**Produces:** `LocalNode.GetMeshIP/SetMeshIP`, `GetMeshIPv6/SetMeshIPv6`, `GetNATType/SetNATType`, `GetRoutableNetworks/SetRoutableNetworks`, `Snapshot()`; `TestLocalNodeFieldsNotWrittenDirectly`
**Consumes:** the one `LocalNode` type (T7)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`MeshIP`, `MeshIPv6`, `NATType` and `RoutableNetworks` of the local node are written and read under one `sync.RWMutex`; `go test -race` over the packages that read them in goroutines is clean, and a direct assignment from outside `pkg/node` fails a test.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/types.go` | edit | rename `endpointMu` to `mu` and use it for the four fields and the endpoint; the four fields become unexported (`meshIP`, `meshIPv6`, `natType`, `routableNetworks`); getters, setters, `Snapshot() LocalSnapshot` (one `RLock`, copies the routes); `SetRoutableNetworks` copies its input; `GetRoutableNetworks` returns a copy |
| `pkg/daemon/daemon.go` | edit | the writes at `:378`, `:380`, `:391`, `:397`, `:490`, `:1719` call setters; the reads at `:224`, `:712`, `:974`, `:1273`, `:1534`, `:1847` call getters; the construction in `initLocalNode` (`:422`) sets the fields through the constructor `node.NewLocalNode(...)` or setters before any goroutine starts |
| `pkg/daemon/collision.go` | edit | `:119-120` use `GetMeshIP` and `SetMeshIP` (ADR-0002 T4 and T5 edit nearby lines: this follows them) |
| `pkg/discovery/exchange.go` | edit | announcement builders at `:507`, `:560`, `:563`, `:1170` read one `Snapshot()` |
| `pkg/discovery/lan.go`, `pkg/discovery/gossip.go` | edit | `lan.go:161`, `gossip.go:90`, `:228` read through getters or `Snapshot()` |
| `pkg/discovery/dht.go` | edit | `NATType` writes at `:256`, `:272`, `:285`, `:422`, `:445` call `SetNATType` |
| `pkg/node/localnode_lock_test.go` | add | the three behaviour tests |
| `pkg/daemon/localnode_guard_test.go` | add | `TestLocalNodeFieldsNotWrittenDirectly` |

The list of sites is the output of the two greps in the parent audit (2026-11-15, `b8b83f3`: 12 writes, 62 reads); the task re-runs both at its start and its end, and the end run must print 0 direct writes outside `pkg/node`. `grep -rn "RoutableNetworks" --include=*.go . | grep -v _test.go` also lists announcement structs and `PeerInfo.RoutableNetworks`, which are not the local node's and are not touched.

Selection: with the fields unexported the compiler selects every site; the guard test is the second selector for a reintroduced exported field.

Ordering note: follows ADR-0002 T9 (it edits `dht.go`, `exchange.go` and `pkg/node/types.go`) and ADR-0002 T4 and T5 (`collision.go`, `daemon.go`).

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
| `TestLocalNodeAccessorsConcurrent` | `pkg/node/localnode_lock_test.go` | 8 goroutines set and get the four fields and the endpoint for 200 iterations each; clean under `-race` | — | S1, S2 |
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

Stop and ask if ADR-0002 T9 changed `LocalNode` so the lock cannot be the one `endpointMu` already is (for example if `ObserveEndpoint` took its own mutex), or if an external embedder of `pkg/daemon` sets the fields directly.

## Out of Scope

- `ObserveEndpoint` and the STUN rule (ADR-0002 T9; permanent: boundary: owned by ADR-0002).
- The `PeerStore`, which already has its own lock (permanent: boundary: owned by ADR-0002 and `pkg/node/store.go`).

## Verification Log
