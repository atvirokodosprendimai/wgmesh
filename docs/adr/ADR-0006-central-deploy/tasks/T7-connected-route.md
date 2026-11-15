# Task ADR-0006-T7: Restore the connected route for every removed own-network route, whatever its gateway (#894 item 1)

**Depends-on:** T5
**Covers:** none — no spec
**Estimated scope:** S (one function and its tests)
**Owner:** unassigned
**Produces:** `restoreOwnConnectedRoutes` in `pkg/mesh/deploy.go`
**Consumes:** `pkg/ssh/sshtest` (T2), `ssh.RestoreConnectedRoute`
**Data dependency:** hermetic
**Proof map:** v1

## Goal

After a centralized deploy removes a route `<own network> via <peer> dev <mesh interface>`, the node's connected route for that network is put back, as the daemon already does for every removed own-network entry (`pkg/daemon/routes.go`, `restoreConnectedRoute`).

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/mesh/deploy.go` | edit | `syncRoutesForNode` restores the connected route only for `ownNetworkRoutes(currentRoutes, ...)`, which collects gateway-less routes (`route.Gateway == ""`); the HA route of releases before #879 carries a gateway (`<net> via <other node> dev wg0`), is removed through `CalculateRouteDiff` and never restored. `restoreOwnConnectedRoutes(client, toRemove, ownNetworks)` calls `ssh.RestoreConnectedRoute` once per distinct network of `toRemove` whose network is one of the node's own (`netroutes.ExcludeOwn([]string{network}, own)` is empty), whatever the gateway; `ownNetworkRoutes` stays for the removal of stale gateway-less routes |
| `pkg/mesh/deploy_routes_test.go` | add | tests below |

Selection: `syncRoutesForNode` is called from `Deploy` for every node; deleting the call to `restoreOwnConnectedRoutes` turns `TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork` red. Issue #894 (open on 2026-11-15, item 1, "inferred from the code, not run") is verified here by reading `deploy.go` at `b8b83f3`: the restore loop ranges over `stale`, the gateway-less subset.

## Ordered Steps

1. [S1] Add the tests with the `sshtest` server (the server answers `ip route show dev wg0` with a gatewayed own-network route) against the unchanged code; run red: the gatewayed own-network case sees no `ip -4 -o addr show to 10.1.0.0/24` command.
2. [S2] Add `restoreOwnConnectedRoutes` and call it from `syncRoutesForNode` in place of the loop over `stale`; every distinct own network of `toRemove` is restored once, after the removals and also when an add failed (the comment of the old loop says why).
3. [S3] Run `go test -race ./pkg/mesh -count=1` and record the exit line in the commit message. [proof: acceptance]
4. [S4] Run the mutants with `adr-verify --mutant`: the restore ranges only over gateway-less routes (the old behaviour); the restore is skipped; the network is not de-duplicated. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/mesh -run '^(TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork|TestSyncRoutesRestoresGatewaylessOwnNetworkOnce|TestSyncRoutesDoesNotRestoreForeignNetwork)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -eq 3 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork` | `pkg/mesh/deploy_routes_test.go` | the node's own network `10.1.0.0/24` is present as `10.1.0.0/24 via 10.99.0.3` on the mesh interface: the recorded commands hold the `ip route del` of it, and after it the address query and the connected-route add for `10.1.0.0/24` | — | S1, S2 |
| `TestSyncRoutesRestoresGatewaylessOwnNetworkOnce` | `pkg/mesh/deploy_routes_test.go` | a gateway-less own-network route (removed both as stale and by the diff) triggers exactly one restore | — | S1, S2 |
| `TestSyncRoutesDoesNotRestoreForeignNetwork` | `pkg/mesh/deploy_routes_test.go` | a removed route to a network that is not the node's own triggers no address query | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the three tests |
| 2 — something selects it | `syncRoutesForNode` calls it; deleting the call turns the first test red |
| 3 — the caller can discover it | the deploy prints `Restored connected route: <net> dev <iface>` as today |
| 4 — it is used | every centralized HA upgrade whose nodes advertise the same subnet |

## Mutation Log

## Invariants

- A gateway-less own-network route is restored exactly once, as before.
- A removed route to a foreign network never triggers a restore.
- Restoring is best effort (a warning, never an error), as `RestoreConnectedRoute` is today.
- The daemon's own restore (`pkg/daemon/routes.go`) is not touched.

## Risks

- `ApplyRouteDiff` removes with `|| true`, so a failed `ip route del` is not seen; the restore then runs for a route that was never removed and answers `File exists`, which `RestoreConnectedRoute` accepts.
- Not run on a real HA mesh: the issue's claim is verified by reading and by the test against the recording server (stated, not measured).
- Test doubles: the `sshtest` server answers the route and address queries from a table.

## Stop Condition

Stop and ask if `CalculateRouteDiff` does not put a gatewayed own-network route into `toRemove` (then the issue's premise is wrong).

## Out of Scope

- Issue #894 items 2 to 4 (`PeerExchange` start and stop races, IPv6 entries in `RoutableNetworks`, first insert of a transitive peer, shallow copies from `PeerStore`) (deferred: docs/adr/BACKLOG.md; T6 writes the rows; items 2 and 4 belong to the daemon packages ADR-0002 and ADR-0005 govern).
- `routes.ConnectedRouteCmd` and the daemon restore (permanent: boundary: not part of the centralized pipeline).

## Verification Log
