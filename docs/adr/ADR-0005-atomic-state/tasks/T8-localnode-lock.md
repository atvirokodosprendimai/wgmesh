# Task ADR-0005-T8: Put the local node's mutable fields under one lock

**Depends-on:** T7
**Covers:** none — no spec
**Estimated scope:** L (multi-file)
**Owner:** unassigned
**Produces:** `LocalNode.GetMeshIP/SetMeshIP`, `GetMeshIPv6/SetMeshIPv6`, `GetNATType/SetNATType`, `IsIntroducer/SetIntroducer`, `Identity() (pubKey, meshIP string)`, `Snapshot()`; `TestLocalNodeFieldsNotWrittenDirectly`
**Consumes:** the one `LocalNode` type (T7)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`MeshIP`, `MeshIPv6`, `NATType` and `Introducer` of the local node are written and read under one new `sync.RWMutex` (`stateMu`), beside the two locks that already exist (`endpointMu` for the endpoint, `routesMu` for `RoutableNetworks`, which PR #895 added with `GetRoutableNetworks`/`SetRoutableNetworks`, merged 2026-11-15, `14a89ff`); one mutex per field group, no lock is held across two groups. `WGPubKey`, `WGPrivateKey` and `Hostname` are written once in `initLocalNode`/`loadLocalNode` before any goroutine starts and stay plain fields (the guard test fails on an assignment to them anywhere else); this is the owner rule of M 2026-11-16 08:21 ("no field accessed by more than one goroutine may be bare"): a field with later writes has a lock, a field written once before the goroutines start has a single writer function and a test. `go test -race` over the packages that read them in goroutines is clean, and a direct assignment from outside `pkg/node` fails a test.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/types.go` | edit | add `stateMu` for `MeshIP`, `MeshIPv6`, `NATType`, `Introducer` (the existing `endpointMu` and `routesMu` stay as #895 left them); the four fields and `RoutableNetworks` become unexported (`meshIP`, `meshIPv6`, `natType`, `introducer`, `routableNetworks`) so the compiler selects every site; a constructor `NewLocalNode(pubKey, privKey, hostname, ...)` sets the write-once fields; getters and setters for the four; `Identity()` returns the public key and mesh IP read together under `stateMu` (the `Policy.Local` closure and `GetRPCStatus` use it); `Snapshot() LocalSnapshot` reads each group under its own lock in turn (never nested), so a group is consistent, the groups are not read at one instant; `GetRoutableNetworks`/`SetRoutableNetworks` already copy (#895) and move with the type |
| `pkg/daemon/daemon.go` | edit | every write of `MeshIP`, `MeshIPv6`, `NATType`, `Introducer` calls a setter (the sites are the output of the grep below: `initLocalNode`, `setupWireGuard`, the key-generation literal, the SIGHUP reload) and every read calls a getter, `Identity()` or `Snapshot()` (`Policy.Local`, the mesh probe dialer, `GetRPCStatus`, metrics, reconcile); `initLocalNode` builds the node through `node.NewLocalNode` before any goroutine starts |
| `pkg/daemon/collision.go` | edit | the `MeshIP` write and read in `CheckAndResolveCollisions` use `GetMeshIP` and `SetMeshIP` |
| `pkg/discovery/exchange.go` | edit | the announcement builders (HELLO, REPLY, ANNOUNCE) read one `Snapshot()` and the probe/offer paths read through getters |
| `pkg/discovery/lan.go`, `pkg/discovery/gossip.go` | edit | the announcement builders and the mesh-IP reads go through `Snapshot()` or getters |
| `pkg/discovery/dht.go` | edit | every `NATType` write (the synchronous discovery and the 60 s STUN refresh loop) calls `SetNATType` |
| `pkg/node/localnode_lock_test.go` | add | the three behaviour tests |
| `pkg/daemon/localnode_guard_test.go` | add | `TestLocalNodeFieldsNotWrittenDirectly` |

The list of sites is the output of `grep -rnE "\blocalNode\.(MeshIP|MeshIPv6|NATType|Introducer|RoutableNetworks)\b" --include=*.go . | grep -v _test.go` (2026-11-15 at `b8b83f3`: 12 writes and 62 reads of the first three plus routes, before #895 and the merge of ADR-0002; re-run at the start); the task re-runs it at its start and its end, and the end run must print 0 direct accesses outside `pkg/node`. `grep -rn "RoutableNetworks" --include=*.go . | grep -v _test.go` also lists announcement structs and `PeerInfo.RoutableNetworks`, which are not the local node's and are not touched.

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
| `TestLocalNodeAccessorsConcurrent` | `pkg/node/localnode_lock_test.go` | 8 goroutines set and get `MeshIP`, `MeshIPv6`, `NATType`, `Introducer`, the routes, the endpoint, `Identity()` and `Snapshot()` for 200 iterations each; clean under `-race` | — | S1, S2 |
| `TestRoutableNetworksAreCopied` | `pkg/node/localnode_lock_test.go` | mutating the slice passed to the setter, or returned by the getter, does not change the node | — | S1, S2 |
| `TestSnapshotMatchesAccessors` | `pkg/node/localnode_lock_test.go` | `Snapshot` returns what the getters return at that moment and its routes are a copy | — | S1, S2 |
| `TestLocalNodeFieldsNotWrittenDirectly` | `pkg/daemon/localnode_guard_test.go` | parses the module's non-test files outside `pkg/node` and fails on an assignment to a selector named `MeshIP`, `MeshIPv6`, `NATType`, `Introducer` or `RoutableNetworks` whose receiver is a local-node expression (`localNode`, `d.localNode`, `pe.localNode`, `GetLocalNode()`), and on an assignment to `WGPubKey`, `WGPrivateKey` or `Hostname` of such a receiver outside `initLocalNode` and `loadLocalNode`; composite literals and `PeerInfo` fields are not flagged | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | unexported fields force every site through the accessors; the guard catches an exported field returning |
| 3 — the caller can discover it | doc comments on the accessors; T11 amends the daemon lifecycle spec |
| 4 — it is used | every announcement, probe, reconcile and STUN refresh |

## Mutation Log
- 2026-11-16 · b0e81e1* · mutant killed · exit 1 · `pkg/node/types.go` · SetMeshIP writes without stateMu: TestLocalNodeAccessorsConcurrent must go red under -race · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7
- 2026-11-16 · b0e81e1* · mutant killed · exit 1 · `pkg/node/types.go` · GetRoutableNetworks returns the internal slice: TestRoutableNetworksAreCopied must go red · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7
- 2026-11-16 · b0e81e1* · mutant killed · exit 1 · `pkg/node/types.go` · SetNATType writes without stateMu: the concurrent test must go red under -race · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7

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
- 2026-11-16 · b0e81e1* · exit 1 · `set -o pipefail …` · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7 · ms:2303 · test-lock-sha256:5d78a1e4ef8a42bfbadd4cdc99f999b54fd66615366bc95e9194c54788426e68 · test-lock-file:docs/adr/locks/5d78a1e4ef8a42bfbadd4cdc99f999b54fd66615366bc95e9194c54788426e68.lock
  ```
  --- last 10 line(s) of stderr (of 449 after folding 461 raw)
      testing.go:1867: race detected during execution of test
  --- FAIL: TestLocalNodeAccessorsConcurrent (0.63s)
  === RUN   TestRoutableNetworksAreCopied
  --- PASS: TestRoutableNetworksAreCopied (0.00s)
  === RUN   TestSnapshotMatchesAccessors
  --- PASS: TestSnapshotMatchesAccessors (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.647s
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon [build failed]
  FAIL
  ```
- 2026-11-16 · b0e81e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7 · ms:33334
- 2026-11-16 · b0e81e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7 · ms:31873
- 2026-11-16 · b0e81e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7 · ms:28322
- 2026-11-16 · b0e81e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:5c8726f7489ef64d0d8e9f74b51885e4d8f47fca7c59886853fc0821fd1adac7 · ms:23800
