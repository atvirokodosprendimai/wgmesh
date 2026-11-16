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
a=$(go test ./pkg/mesh -run '^(TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork|TestSyncRoutesRestoresOwnNetworkOnceWhenRemovedTwice|TestSyncRoutesDoesNotRestoreForeignNetwork|TestSyncRoutesRestoresAfterFailedAdd)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -eq 4 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork` | `pkg/mesh/deploy_routes_test.go` | the node's own network `10.1.0.0/24` is present as `10.1.0.0/24 via 10.99.0.3` on the mesh interface: the recorded commands hold the `ip route del` of it, and after it the address query and the connected-route add for `10.1.0.0/24` | — | S1, S2 |
| `TestSyncRoutesRestoresOwnNetworkOnceWhenRemovedTwice` | `pkg/mesh/deploy_routes_test.go` | one network that appears in the route table both as a gateway-less and as a gatewayed route (so it is in the removed list twice: once as stale, once through the diff) triggers exactly one restore | — | S1, S2 |
| `TestSyncRoutesDoesNotRestoreForeignNetwork` | `pkg/mesh/deploy_routes_test.go` | a removed route to a network that is not the node's own triggers no address query | — | S1, S2 |
| `TestSyncRoutesRestoresAfterFailedAdd` | `pkg/mesh/deploy_routes_test.go` | an add that fails (exit 1) still ends with the restore: `syncRoutesForNode` returns the error and the address query and connected-route add follow the removal | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the three tests |
| 2 — something selects it | `syncRoutesForNode` calls it; deleting the call turns the first test red |
| 3 — the caller can discover it | the deploy prints `Restored connected route: <net> dev <iface>` as today |
| 4 — it is used | every centralized HA upgrade whose nodes advertise the same subnet |

## Mutation Log
- 2026-11-16 · b21ee0d* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the old behaviour: the gatewayed own-network route is not restored: TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork must go red · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6
- 2026-11-16 · b21ee0d* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · no restore at all: the restore tests must go red · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6
- 2026-11-16 · b21ee0d* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a network is restored once per route: TestSyncRoutesRestoresOwnNetworkOnceWhenRemovedTwice must go red · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6
- 2026-11-16 · b21ee0d* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a foreign network is restored: TestSyncRoutesDoesNotRestoreForeignNetwork must go red · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6
- 2026-11-16 · 5fb793a* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the old behaviour: the gatewayed own-network route is not restored · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa
- 2026-11-16 · 5fb793a* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · no restore at all · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa
- 2026-11-16 · 5fb793a* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a network is restored once per route · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa
- 2026-11-16 · 5fb793a* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a foreign network is restored · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa
- 2026-11-16 · 5fb793a* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · no restore after a failed add: TestSyncRoutesRestoresAfterFailedAdd must go red · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa

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
- 2026-11-16 · b21ee0d* · exit 1 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:850 · test-lock-sha256:6f584755fc04ff4d61b235a6aff146d482c2c38402987a183d5a2ff66f0326ad · test-lock-file:docs/adr/locks/6f584755fc04ff4d61b235a6aff146d482c2c38402987a183d5a2ff66f0326ad.lock
  ```
  --- last 10 line(s) of stderr (of 18 after folding 18 raw)
      Removed route: 10.1.0.0/24
      Restored connected route: 10.1.0.0/24 dev eth0
  --- PASS: TestSyncRoutesRestoresOwnNetworkOnceWhenRemovedTwice (0.00s)
  === RUN   TestSyncRoutesDoesNotRestoreForeignNetwork
    Route changes: 1 to remove, 0 to add
      Removed route: 10.5.0.0/24 via 10.99.0.3
  --- PASS: TestSyncRoutesDoesNotRestoreForeignNetwork (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	0.017s
  FAIL
  ```
- 2026-11-16 · b21ee0d* · exit 0 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:12799
- 2026-11-16 · b21ee0d* · exit 0 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:11989
- 2026-11-16 · b21ee0d* · exit 0 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:11766
- 2026-11-16 · b21ee0d* · exit 0 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:11038
- 2026-11-16 · b21ee0d* · exit 0 · `set -o pipefail …` · acceptance-sha256:b49efbbf748b1149b515259a7c78117b4507682d71db39895a6d159f4728c6e6 · ms:8344
- 2026-11-16 · 5fb793a* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:0 · test-lock-sha256:4889629738234964937dcc68b71b174b1e285032ad2c89b1ae809f061f523cb4 · test-lock-file:docs/adr/locks/4889629738234964937dcc68b71b174b1e285032ad2c89b1ae809f061f523cb4.lock · test-lock-kind:replace
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:11906
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:11481
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:12531
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:11878
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:12732
- 2026-11-16 · 5fb793a* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:7893
- 2026-11-16 · 164dd40* · exit 0 · `set -o pipefail …` · acceptance-sha256:03e45c3aae3b6a145699bffe26d39d7116da810f7e04cbc38fc4a0d9273587fa · ms:9625
