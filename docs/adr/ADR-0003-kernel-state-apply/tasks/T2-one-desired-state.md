# Task ADR-0003-T2: Build one desired state per tick for AllowedIPs and routes

**Depends-on:** T1
**Blocked-on:** ADR-0002's route approval task has landed on main: `grep -q 'func Approved' pkg/node/binding.go` exits 0 (it adds `TestHAExitNodesDropEachOthersDefault`, which this task keeps green, and edits `daemon.go`)
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `desiredState`, `Daemon.desire`, `reconnectAllowedCSV` reading the desired state
**Consumes:** `routes.Canonical`, `routes.Resolve`, deterministic `CalculateDiff` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`reconcile` computes the peers' AllowedIPs and the kernel routes from one function and one owner choice per network, so a network is in the route table if and only if it is in exactly one peer's AllowedIPs, and `routes.ExcludeOwn` is called from that function only.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/desired.go` | add | `desiredState{Peers map[string]*desiredPeerConfig; Routes []routes.Entry; relayRoutes map[string]string; directStable map[string]int}` and `(d *Daemon) desire(peers []*PeerInfo, handshakes map[string]int64, observedOwners map[string]string) *desiredState`; `Fresh` is a handshake newer than `HandshakeStaleAfter`; the relay mapping and hysteresis of `buildDesiredPeerConfigsWithHandshakes` move here unchanged |
| `pkg/daemon/daemon.go` | edit | `buildDesiredPeerConfigsWithHandshakes` stays as a thin wrapper returning the same three values (so `routes_test.go` relay tests run unchanged); `reconcile` (`:534-559`) calls `desire` once and passes `Routes` to `syncPeerRoutes`; `reconnectAllowedCSV` (`:1356-1371`) takes the allowed set from `desire` instead of calling `ExcludeOwn` itself |
| `pkg/daemon/routes.go` | edit | `syncPeerRoutes` takes the desired routes instead of deriving them (removes the loop at `:18-46` and the `ExcludeOwn` at `:58` moves to a helper in `desired.go`) |
| `pkg/daemon/desired_test.go` | add | the five tests below |
| `pkg/daemon/ha_routes_test.go`, `pkg/daemon/routes_test.go` | none | the pinned tests run unchanged |

Selection: `reconcile` calls `desire`; deleting that call turns `TestDesireRoutesAndAllowedIPsFromOneOwner` and every reconcile test red.

## Ordered Steps

1. [S1] Add `pkg/daemon/desired_test.go` with the five tests and a compiling stub (`desire` returns an empty state) so the first run fails on assertions. Run red.
2. [S2] Implement `desire`: peers and relay mapping as today, then the route list and the AllowedIPs of non-mesh networks from `routes.Resolve`, so both read the same owner; a skipped network is skipped in both.
3. [S3] Switch `reconcile`, `syncPeerRoutes` and `reconnectAllowedCSV` to the desired state.
4. [S4] Add `TestExcludeOwnOnlyInDesire`: it parses the non-test `.go` files of `pkg/daemon` and fails on a call to `routes.ExcludeOwn` outside `desired.go`. Run it red before S3 (`grep -rn "ExcludeOwn(" pkg/daemon/*.go | grep -v _test.go | grep -v desired.go | wc -l` is 5 at `b8b83f3`), green after.
5. [S5] Run the pinned tests `TestSharedSubnetNeverRoutedThroughMesh`, `TestSharedSubnetNotRelayedThroughMesh` and ADR-0002's `TestHAExitNodesDropEachOthersDefault` unchanged. [proof: acceptance]
6. [S6] Run the mutants with `adr-verify --mutant`: `desire` builds routes from every advertiser without `Resolve`; the route loop skips the relay mapping; the `ExcludeOwn` call is removed. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestDesireRoutesAndAllowedIPsFromOneOwner|TestDesireHAExitNodeDropsPeerDefault|TestDesireRelayedOwnerGoesToRelay|TestReconnectAllowedMatchesDesired|TestExcludeOwnOnlyInDesire)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] \
 && b=$(go test ./pkg/daemon -run '^(TestSharedSubnetNeverRoutedThroughMesh|TestSharedSubnetNotRelayedThroughMesh|TestHAExitNodesDropEachOthersDefault)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 3 ] \
 && [ "$(grep -rn 'ExcludeOwn(' pkg/daemon/*.go | grep -v _test.go | grep -v desired.go | wc -l)" -eq 0 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestDesireRoutesAndAllowedIPsFromOneOwner` | `pkg/daemon/desired_test.go` | two peers advertising `192.168.50.0/24`: the network is in exactly one peer's AllowedIPs and in exactly one route whose gateway is that peer's mesh IP | — | S1, S2 |
| `TestDesireHAExitNodeDropsPeerDefault` | `pkg/daemon/desired_test.go` | a node that advertises `0.0.0.0/0` has no `0.0.0.0/0` in any AllowedIPs or route from a peer; a node that does not advertise it has the peer's | — | S1, S2 |
| `TestDesireRelayedOwnerGoesToRelay` | `pkg/daemon/desired_test.go` | when the owner is relayed its networks are in the relay's AllowedIPs and the route gateway is the relay's mesh IP | — | S1, S2 |
| `TestReconnectAllowedMatchesDesired` | `pkg/daemon/desired_test.go` | the list used by a forced reconnect equals the desired AllowedIPs of that peer, including the case where the peer is not the owner of a shared network | — | S1, S3 |
| `TestExcludeOwnOnlyInDesire` | `pkg/daemon/desired_test.go` | no production file of `pkg/daemon` except `desired.go` calls `routes.ExcludeOwn` | — | S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `reconcile` calls `desire`; the removed-call mutant turns the first test red |
| 3 — the caller can discover it | doc comment on `desire`; T9 amends the reconciliation spec |
| 4 — it is used | every tick |

## Mutation Log

## Invariants

- Relay selection, hysteresis and `directStableCycles` give the same values as before for every scenario in `pkg/daemon/routes_test.go`.
- AllowedIPs of a peer always contain its mesh `/32` and `/128`; only non-mesh networks have a single owner.
- `ExcludeOwn` stays an apply-side rule inside reconcile (ADR-0002 decision item 4a (e)); no admit code is touched.

## Risks

- A shared network owned by a relayed peer moves to the relay's AllowedIPs, which also carries other peers' networks; covered by `TestDesireRelayedOwnerGoesToRelay`.
- `reconcile` reads `localNode.RoutableNetworks` without a lock (`daemon.go:625`, reload writes at `:1719`); unchanged here (permanent: boundary: another group owns it).
- Test doubles: none; the tests use a hand-built `Daemon` like `makeRelayTestDaemon`.

## Stop Condition

Stop and ask if a pinned test needs a changed expectation, or if `desire` cannot give the observed owners before the observe step exists (T3 passes them; until then `reconcile` passes an empty map).

## Out of Scope

- Reading the kernel (T3 for peers, T5 for routes).
- Forwarding and the interface address (T4, T7).

## Verification Log
