# Task ADR-0003-T5: Converge routes for both families, and stop re-adding what is already there (#859)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** L (cross-boundary)
**Owner:** unassigned
**Produces:** `routeTarget`, `EnforceRouteGuard`, rules `attached-network` and `host-default`
**Consumes:** `routes.Canonical`, deterministic `CalculateDiff` (T1); `desiredState.Routes` (T2); `kernelObserved`, the result log, the counters, the guard test (T3); the forwarding target (T4)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The route target reads `ip route show dev <if>` (IPv4, the default family of ip; built as `readRoutes` in `observe.go`) and `ip -6 route show dev <if>`, compares canonical networks, changes only the difference with one result per item (an IPv6 route with no usable gateway no longer aborts the others), no longer issues forwarding commands, skips or warns about a route to an attached network, warns once when a default route is installed over a host default on another device, and a steady node issues no writes at all.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/routes.go` | edit | `getCurrentRoutes` is replaced by `readRoutes` in `observe.go` (both families, canonical, `default` per family) and the route target is `convergeRoutes` in `converge.go` (`syncPeerRoutes` stays only as a test wrapper); `applyRouteDiff` loses the `sysctl` and `ensureWGForwardingRule` calls (`:172-175`, `:180-192` deleted) and returns one result per route instead of stopping at the first failed add (`:167-169`); `syncPeerRoutes` becomes the route target of `converge` |
| `pkg/daemon/desired.go` | edit | an IPv6 route uses the peer's `MeshIPv6` as gateway; a peer without one is skipped with one log line; routes equal to or inside an attached network (`getLocalSubnets`, `daemon.go:860`) get the rule `attached-network`: counted at N on Linux and dropped from routes and AllowedIPs when `EnforceRouteGuard` is true |
| `pkg/daemon/converge.go` | edit | `EnforceRouteGuard` (`atomic.Bool`, `false`; every access `Load`/`Store`); the rule `host-default`: before adding `0.0.0.0/0` or `::/0` one `ip -4 route show default` (or `-6`), and a default on another device is logged once and counted, the route is installed as today (decision 6, recommendation A) |
| `pkg/daemon/converge_guard_test.go` | edit | the pattern list gains `ip route` and `applyRouteDiff`; the route commands now live in `converge.go` (writes) and `observe.go` (reads), `routes.go` keeps `restoreConnectedRoute` called only from `converge.go`; the exception for `routes.go` is removed |
| `pkg/daemon/routes_test.go` | edit | `TestApplyRouteDiff_AddAndRemove` (`:197-237`) no longer expects `sysctl` (`:234-236`); this is the intended change, replaced by `TestApplyRouteDiffIssuesNoForwardingCommands` |
| `pkg/daemon/kernel_routes_test.go` | add | the ten tests below |

Selection: `reconcile` calls `converge`, which calls the route target; deleting the target turns `TestSharedSubnetNoFlapOverCycles` red; `EnforceRouteGuard` is read by `desire` and a test sets it.

## Ordered Steps

1. [S1] Add the ten tests below with compiling stubs. `TestSharedSubnetNoFlapOverCycles`, `TestDefaultRouteNoChurn` and `TestIPv6RouteUsesIPv6Gateway` reproduce the 2026-11-15 scratch runs (fake kernel table through `MockCommandExecutor`: two peers on `192.168.50.0/24` gave 2 route mutations every tick; a default route gave `del default` and `replace 0.0.0.0/0` every tick; an IPv6 network with an IPv4 gateway was issued every tick and stopped the adds after it). Run red.
2. [S2] Implement the route observe for both families and make the plan compare canonical networks.
3. [S3] Make apply return a result per route and remove the forwarding calls from `applyRouteDiff`.
4. [S4] Implement the IPv6 gateway rule and the `attached-network` and `host-default` rules.
5. [S5] Add `TestConvergeSteadyStateIssuesNoWrites`: ten ticks over a fake kernel holding peers, routes, forwarding and an address, asserting zero write commands after tick 1.
6. [S6] Run the mutants with `adr-verify --mutant`: the canonical form is not applied to `default`; apply returns at the first failed add; `attached-network` matches equal prefixes only and not inside; the forwarding calls are left in `applyRouteDiff`; the observe reads IPv4 only. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestObserveRoutesBothFamilies|TestSharedSubnetNoFlapOverCycles|TestDefaultRouteNoChurn|TestIPv6RouteUsesIPv6Gateway|TestPeerWithoutMeshIPv6SkippedLoggedOnce|TestRouteAddFailureDoesNotAbortOthers|TestApplyRouteDiffIssuesNoForwardingCommands|TestAttachedNetworkWarnsThenSkips|TestDefaultRouteWarnsWhenHostHasDefault|TestConvergeSteadyStateIssuesNoWrites)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 10 ] && [ "$(grep -rn 'ip_forward\|"iptables"' pkg/daemon/routes.go | wc -l)" -eq 0 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestObserveRoutesBothFamilies` | `pkg/daemon/kernel_routes_test.go` | `ip -4` and `ip -6 route show dev` output parse to canonical entries: `default` is `0.0.0.0/0` and `::/0` by the family queried, bare hosts get `/128` and `/32`, routes without a gateway are ignored as before | — | S1, S2 |
| `TestSharedSubnetNoFlapOverCycles` | `pkg/daemon/kernel_routes_test.go` | two fresh peers advertising one network over five ticks: one add on tick 1, zero route writes on ticks 2 to 5 | — | S1, S2 |
| `TestDefaultRouteNoChurn` | `pkg/daemon/kernel_routes_test.go` | a peer's `0.0.0.0/0` over three ticks: one add, then no `del default` or `replace` | — | S1, S2 |
| `TestIPv6RouteUsesIPv6Gateway` | `pkg/daemon/kernel_routes_test.go` | an IPv6 network is installed with the peer's mesh IPv6 as gateway and read back on the next tick without a write | — | S1, S4 |
| `TestPeerWithoutMeshIPv6SkippedLoggedOnce` | `pkg/daemon/kernel_routes_test.go` | an IPv6 network from a peer with no mesh IPv6 is not issued and one line is printed over ten ticks | — | S1, S4 |
| `TestRouteAddFailureDoesNotAbortOthers` | `pkg/daemon/kernel_routes_test.go` | when one `ip route replace` fails the other routes in the plan are still issued and the failure is one result | — | S1, S3 |
| `TestApplyRouteDiffIssuesNoForwardingCommands` | `pkg/daemon/routes_test.go` | `applyRouteDiff` issues no `sysctl` and no `iptables` command | — | S1, S3 |
| `TestAttachedNetworkWarnsThenSkips` | `pkg/daemon/kernel_routes_test.go` | a peer network equal to or inside an attached subnet is installed and counted `attached-network` at N, and dropped from the routes and the AllowedIPs with `EnforceRouteGuard`; a supernet is kept either way | — | S1, S4 |
| `TestDefaultRouteWarnsWhenHostHasDefault` | `pkg/daemon/kernel_routes_test.go` | with a default route on another device the mesh default is still installed and one `host-default` line and counter increment are produced | — | S1, S4 |
| `TestConvergeSteadyStateIssuesNoWrites` | `pkg/daemon/kernel_routes_test.go` | ten ticks over a converged fake kernel issue no write command after tick 1 | — | S1, S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the ten tests |
| 2 — something selects it | the route target is part of every plan; `EnforceRouteGuard` is read by `desire` |
| 3 — the caller can discover it | `wgmesh_kernel_warnings_total{rule}` and the once-only log lines; T9 documents them |
| 4 — it is used | every tick of every Linux node |

## Mutation Log
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/observe.go` · default not canonical: TestObserveRoutesBothFamilies must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/converge.go` · first failed add aborts: TestRouteAddFailureDoesNotAbortOthers must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/desired.go` · inside not matched: TestAttachedNetworkWarnsThenSkips must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/converge.go` · forwarding call in applyRouteDiff: TestApplyRouteDiffIssuesNoForwardingCommands must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/observe.go` · IPv6 never read: TestObserveRoutesBothFamilies must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · ff69c48* · mutant killed · exit 1 · `pkg/daemon/desired.go` · no skip: TestPeerWithoutMeshIPv6SkippedLoggedOnce must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/observe.go` · default not canonical: TestObserveRoutesBothFamilies must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/converge.go` · first failed add aborts: TestRouteAddFailureDoesNotAbortOthers must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/desired.go` · inside not matched: TestAttachedNetworkWarnsThenSkips must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/converge.go` · forwarding call in applyRouteDiff: TestApplyRouteDiffIssuesNoForwardingCommands must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/observe.go` · IPv6 never read: TestObserveRoutesBothFamilies must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90
- 2026-11-16 · 8e9f14f* · mutant killed · exit 1 · `pkg/daemon/desired.go` · no skip: TestPeerWithoutMeshIPv6SkippedLoggedOnce must go red · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90

## Invariants

- The `ip route del` and `ip route replace` commands keep their form (`<net> via <gw> dev <if>`), so the table looks as before.
- `restoreConnectedRoute` and the own-network rule behave as before (`ha_routes_test.go` runs unchanged).
- A route to a network the node advertises itself is never installed (`ExcludeOwn`, T2).
- An observe error on a family skips that family for the tick and removes nothing.

## Risks

- `ip -6 route show dev` on a host with IPv6 disabled fails: that family is skipped with one log line, as `--no-ipv6` already skips IPv6 endpoints.
- The attached-network rule drops a route an operator wanted; equal or inside only, a supernet is kept, warn first (decision 9).
- The `host-default` rule adds one `ip route show default` when a default route is about to be added; bounded, not per tick.
- Test doubles: `MockCommandExecutor` with a stateful fake route table.

## Stop Condition

Stop and ask if the observe cannot tell a route we installed from a foreign route on the same device, or if `getLocalSubnets` includes the mesh subnet itself (then the attached-network rule would drop the mesh's own routes; the test fixes the expectation).

## Out of Scope

- macOS (T6). T6 cannot reuse `routes.CalculateDiff` and the Linux `applyRouteDiff` as they are: macOS routes carry no gateway, `CalculateDiff` only removes routes with a gateway and keys on network and gateway, and the result key and command form are Linux-shaped; T6 needs its own plan and apply (or a managed-route predicate) behind the `goos` seam, and `convergeRoutes`, `observeRoutes` and `attachedSubnet` each return early on non-Linux today, three places to switch.
- Any transport exemption for a default route (deferred: docs/adr/BACKLOG.md).
- Policy routing and route metrics (permanent: boundary: decision 6 keeps today's `replace`).

## Verification Log
- 2026-11-16 · ff69c48* · exit 1 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:1901 · test-lock-sha256:b9824c4ebf27cfa9e7568581ebb8c698ee5aad498d7720b27cab2ba900d2f740 · test-lock-file:docs/adr/locks/b9824c4ebf27cfa9e7568581ebb8c698ee5aad498d7720b27cab2ba900d2f740.lock
  ```
  --- last 10 line(s) of stderr (of 67 after folding 67 raw)
      kernel_routes_test.go:549: the converged table lacks 192.168.60.0/24 via 10.99.0.3
  --- FAIL: TestConvergeSteadyStateIssuesNoWrites (0.00s)
  === RUN   TestApplyRouteDiffIssuesNoForwardingCommands
      routes_test.go:260: applyRouteDiff ran 5 commands for 3 routes, want 3: ["ip route del 10.0.2.0/24 via 10.0.0.1 dev wg0" "ip route replace 10.0.1.0/24 via 10.0.0.1 dev wg0" "ip route replace fd00:5::/64 via fd99::2 dev wg0" "sysctl -w net.ipv4.ip_forward=1" "iptables -C FORWARD -i wg0 -o wg0 -j ACCEPT"]
      routes_test.go:264: applyRouteDiff ran a forwarding command: "sysctl -w net.ipv4.ip_forward=1"
      routes_test.go:264: applyRouteDiff ran a forwarding command: "iptables -C FORWARD -i wg0 -o wg0 -j ACCEPT"
  --- FAIL: TestApplyRouteDiffIssuesNoForwardingCommands (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.016s
  FAIL
  ```
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:11819
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:12721
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:11844
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:14463
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:13226
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:12440
- 2026-11-16 · ff69c48* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:7747
- 2026-11-16 · 8e9f14f* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:0 · test-lock-sha256:1350f4a7c864e561c1a24372dc9b40f27bc99c430128ff7e0e8fa60d2f8fe18a · test-lock-file:docs/adr/locks/1350f4a7c864e561c1a24372dc9b40f27bc99c430128ff7e0e8fa60d2f8fe18a.lock · test-lock-kind:replace
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:9723
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:9815
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:9830
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:10257
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:10465
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:9969
- 2026-11-16 · 8e9f14f* · exit 0 · `set -o pipefail …` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:7951
- 2026-11-16 · f110717* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:0 · test-lock-sha256:1ba504914924ab1f3465d6e2c50456dde95478bf5fde169aebcf0638c04da199 · test-lock-file:docs/adr/locks/1ba504914924ab1f3465d6e2c50456dde95478bf5fde169aebcf0638c04da199.lock · test-lock-kind:replace
- 2026-11-16 · fe6f879* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:d0eba92d48ff9039d74abbc4373b7a0c2e455799e77f2c2a30a41d3d180a0f90 · ms:1 · test-lock-sha256:1ba504914924ab1f3465d6e2c50456dde95478bf5fde169aebcf0638c04da199 · test-lock-file:docs/adr/locks/1ba504914924ab1f3465d6e2c50456dde95478bf5fde169aebcf0638c04da199.lock · test-lock-kind:replace
