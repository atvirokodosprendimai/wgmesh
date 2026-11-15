# ADR-0003: Add one observe, diff, apply step between the desired kernel state and the kernel

**Status:** Accepted
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md` and `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` are amended by T9
**Cross-references:** `docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md` (Accepted; decides what enters the PeerStore and the receiver-side `accept-routes` list; this record starts after the store), `docs/adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md`, `docs/adr/BACKLOG.md`; issues #859, #872, #862 (open on 2026-11-15), #867 (closed; checked in the code below); PR #848 (closed, not merged; its change is already on main, see Context)
**Governs:** `pkg/routes/routes.go`, `pkg/daemon/routes.go`, `pkg/daemon/helpers.go`, `pkg/daemon/metrics.go`
**Enforced-by:** None — the check is `TestKernelWritesOnlyInConverge` in `pkg/daemon/converge_guard_test.go`, which does not exist until T3 adds it red-first; it fails when a production file in `pkg/daemon` other than the interface setup and teardown helpers runs `ip route`, `sysctl`, `iptables`, `route` or calls `wireguard.SetPeer` or `wireguard.RemovePeer`
**Invalidates:** none — checked (`adr-state` on 2026-11-15 reports ADR-0001 and ADR-0002 only; ADR-0002 governs `pkg/daemon/collision.go`, `cache.go`, `config.go` and `pkg/node/*.go`, none of which this record edits; it edits `pkg/daemon/daemon.go`, which ADR-0002's tasks T3 to T10 also edit, hence the ordering in Inter-task Contracts, and it keeps ADR-0002 decision item 4a (e): `routes.ExcludeOwn` stays an apply-side rule)
**Served-path change:** The reconcile tick (`pkg/daemon/daemon.go` `reconcile`) now reads the kernel's peers, routes, `ip_forward`, FORWARD rule and interface address, computes one desired state, and writes only the difference, so a shared subnet or a default route stops being deleted and re-added every 5 seconds, a failed peer is replaced as owner of its subnet, forwarding is applied once and a missing `iptables` is logged once, and macOS installs the routes of peers' advertised networks.

## Decisions

M's decisions of 2026-11-15 that this record keeps, in M's words as quoted in the brief of this draft:

1. **An advertised default route `0.0.0.0/0` stays a feature** (#859, M 2026-11-15, restated in ADR-0002 decision 6).
2. **Two HA exit nodes each drop the other's default route** through `routes.ExcludeOwn` in reconcile (`pkg/routes/routes.go:122-139`); this apply-side rule stays (ADR-0002 decision item 4a (e)). Here it moves into the one function that builds the desired state and is neither duplicated nor weakened.
3. **ADR-0002 decides what enters the PeerStore and the receiver-side `accept-routes` list**; this record does not redo route approval and treats the store as already approved.

4. **Owner of a network that two live peers advertise** (M 2026-11-15: "Pagal visas rekomendacijas, ADR po ADR (Recommended)"): option A. A peer with a handshake newer than `HandshakeStaleAfter` (150 s) beats one without; among fresh peers the owner read back from the kernel keeps the network; otherwise the lowest mesh IP, compared as a number. No automatic fail-back.
5. **Forwarding scope** (same answer): option A. `ip_forward` and the FORWARD rule are wanted only on an introducer or a node with `advertise-routes`; at N a once-only warning (`forward-scope`), at N+1 the node stops forcing them.
6. **A peer's `0.0.0.0/0` over a host default route on another device** (same answer): option A. The route is still installed with `replace`, since it is the feature, and one line and one counter (`host-default`) say a host default exists. The missing `fwmark` or policy rule (a node's own WireGuard UDP may be routed into `wg0`; not run) is a BACKLOG row, not part of this record.
7. **Revert at graceful exit** (same answer): option A. The FORWARD rule is removed only if this run added it, `ip_forward` is restored only if this run changed it from 0; warn at N, act at N+1 behind the third switch `EnforceRevertOnExit`. T8 stays in the plan.
8. **Staging** (same answer): option A. One step, warn at N and enforce at N+1, because the rules change only this node's own kernel and no byte on the wire.
9. **A route for a network the host is directly attached to** (same answer): option A. Equal or inside is skipped, a supernet is kept; on macOS from the first release, on Linux warn at N and skip at N+1.
10. **Interface address convergence** (same answer): option A. T7 is included.

## Open Questions for M

None. M answered all seven questions of the first draft on 2026-11-15 with "Pagal visas rekomendacijas, ADR po ADR (Recommended)" (decisions 4 to 10 above).

## Context

**The pipe.** The reconcile tick is `PeerStore.GetActive -> derive desired (three times) -> apply (wg, ip, sysctl, iptables, ifconfig)`. In ETL terms the target of the load, the kernel, is extracted by one source only, the routes (`ip route show dev <iface>`, `pkg/daemon/routes.go:117-153`), and even that one is partial; the transform from peers to desired state is written three times; and the load step is five writers with five error policies. The missing step is **converge**: read what the kernel has, compute one desired state, write only the difference, report per item.

**Class audit** (read at `b8b83f3`, 2026-11-15). Class: *every place the daemon writes kernel network state (a WireGuard peer's AllowedIPs, a route, `ip_forward`, the FORWARD rule, the interface address) from a value derived from the PeerStore or the local config, without reading that state back in the same cycle and diffing it against one shared desired state.*

| Command | Count | Members |
|---|---|---|
| `grep -nE 'wireguard\.(SetPeer\|RemovePeer)\(' pkg/daemon/*.go \| grep -v _test.go \| wc -l` | 5 | `daemon.go:778` (obsolete peer), `:814` (apply, guarded by a string cache), `:915` (`removePeer`, called by stale cleanup at `:944`), `:1387` (forced reconnect), `:1403` (health eviction) |
| `grep -nE 'cmdExecutor\.Command\("(ip\|sysctl\|iptables)"' pkg/daemon/routes.go \| wc -l` | 7 | `routes.go:78` and `:90` (restore a connected route), `:118` (read routes), `:159` (delete), `:166` (replace), `:172` (`ip_forward`), `:183` and `:188` (iptables check and append) |
| `grep -rn "setInterfaceAddress(" --include=*.go pkg \| grep -v _test \| grep -v "func setInterfaceAddress" \| wc -l` | 3 | `daemon.go:483`, `:487` (setup), `collision.go:123` (collision resolution) |
| `grep -rn "ExcludeOwn(" --include=*.go pkg \| grep -v _test.go \| grep -v "func ExcludeOwn" \| grep -v "// " \| wc -l` | 10 | derivations of "which peer networks do I install": `daemon.go:625`, `:639`, `:1364`, `routes.go:39`, `:58` (5 in `pkg/daemon`) and `pkg/mesh/deploy.go:190`, `:205`, `:254`, `:317`, `:338` (5 in the centralized pipeline) |
| `grep -rn "lastAppliedPeerConfigs" --include=*.go pkg \| grep -v _test.go \| wc -l` | 8 | the string cache that stands in for reading the kernel: `daemon.go:54`, `:191`, `:782`, `:806`, `:811`, `:817`, `:1392`, `:1407` |

Left out on purpose: the one-shot interface lifecycle in `pkg/daemon/helpers.go` (`createInterface`, `resetInterface`, `setInterfaceUp`, `setInterfaceDown`, `deleteInterface`, `configureInterface`, `grep -nE 'cmdExecutor\.Command\("(ip|ifconfig)"' pkg/daemon/helpers.go` lists them; they run at setup and teardown, not on the tick); the five `pkg/mesh/deploy.go` derivations and `pkg/ssh/routes.go`, `pkg/wireguard/persist.go`, which are the centralized pipeline (a sibling draft, branch `task/adr-0006-central-deploy`, is not on main; this record only changes the pure `pkg/routes` code both pipelines call, see Wiring); `pkg/node` (ADR-0002).

**What each claim of the issues is, checked in the code** (all `b8b83f3`; "run" means a test I ran on 2026-11-15 and deleted, a scratch test against a fake kernel table through `MockCommandExecutor`; "read" means read in the source only):

- *#859 duplicate subnet flaps.* `CalculateDiff` keeps every desired entry in `desiredMap` keyed by network and gateway, but `currentByNetwork` holds one entry per network (`pkg/routes/routes.go:56-105`). With two peers advertising `192.168.50.0/24` and a kernel that keeps one route, run: cycle 1 two `replace`, then from cycle 2 on every cycle one `del` and one `replace`, alternating the gateway (`.3` then `.2`), forever (4 cycles shown, 2 route mutations each). Confirmed.
- *#859 failover.* Read: `buildDesiredPeerConfigsWithHandshakes` gives the same network to both peers' AllowedIPs (`daemon.go:635-644`, `addAllowedIP` at `:760`); `applyDesiredPeerConfigs` calls `wg set` per peer in map order and skips a peer whose cached string `endpoint|allowedIPs` is unchanged (`:802-811`). WireGuard keeps a prefix on the peer set last (documented WireGuard behaviour, not run), so one peer holds it; if that peer is removed the survivor's string is unchanged and it is never set again. Confirmed by reading; the kernel half is not run.
- *#859 default route.* The kernel prints `default`; `getCurrentRoutes` keeps it as `default` (`routes.go:136`, `NormalizeNetwork` only handles bare hosts), the desired side says `0.0.0.0/0`. Run: cycle 1 one `replace`, from cycle 2 on every cycle `del default` and `replace 0.0.0.0/0`. Confirmed. `ExcludeOwn` already maps `default` for the HA check (`routes.go:111-120`, commit `72e3c5d`), so the HA rule works and only the diff does not.
- *#859 IPv6.* `getCurrentRoutes` runs `ip route show dev` once, which is IPv4 only (`routes.go:118`); the gateway of every desired route is the peer's IPv4 mesh IP (`routes.go:33`). Run: `ip route replace 2001:db8::/64 via 10.99.0.2 dev wg0` is issued every cycle and `applyRouteDiff` returns at the first failed add (`:167-169`), so the other adds and the forwarding calls after it are skipped in that cycle. The kernel's refusal text in my fake is not measured; that `ip` refuses an IPv4 gateway for an IPv6 route is not run.
- *#859 "replace can displace the host's default route".* Read: `ip route replace` (`routes.go:166`) matches on prefix and metric and the current routes are read with `dev <iface>` only, so a default route on another device is invisible to the diff. Not run.
- *#872.* Run: with no peer routes at all, every cycle issues `sysctl -w net.ipv4.ip_forward=1` and `iptables -C ...` (2 commands per cycle, 3 cycles shown); `applyRouteDiff` is called from every `syncPeerRoutes`, which `reconcile` calls every tick on Linux (`daemon.go:547`). The log line for a missing `iptables` is printed at `routes.go:190` on every `-A` failure; read, not run without an `iptables`. Nothing reverts them: `teardownWireGuard` deletes the interface (`daemon.go:503-516`) and does not touch the sysctl or the rule. Confirmed.
- *#862 macOS.* `syncPeerRoutes` returns `nil` at once when `runtime.GOOS != "linux"` (`routes.go:14-16`); the Darwin code in `helpers.go:223-263` installs only the mesh subnet. The daemon support spec states it ("Only runs on Linux (no-op on other platforms)", line 45). Confirmed by reading; not run on macOS.
- *A fifth case the issues do not name.* Read: `staleCleanupLoop` removes a peer through `removePeer` (`daemon.go:944` to `:915`) and does not delete its entry in `lastAppliedPeerConfigs`, unlike `evictPeerFromPool` (`:1406-1408`). If the peer returns with the same endpoint and AllowedIPs its string matches and `wg set` is not run, so it is absent from the kernel until the string changes. Not run.
- *#867 (closed).* `peersEqual` ignores a live endpoint when the desired endpoint is empty (`pkg/wireguard/config.go:160-164`). Fixed on main; centralized pipeline; nothing to do here.
- *PR #848 (closed, not merged).* Its change, "do not route a node's own networks into its mesh interface" and "restore the connected route", is on main in other commits: `pkg/mesh/deploy.go:177-259`, `pkg/daemon/routes.go:57-95`, `pkg/routes/routes.go:141-168` (commits `60dffa3`, `454ca97`, `74334be`, `7753e9a`). The daemon and the centralized pipeline both drop own networks through `ExcludeOwn` and restore the connected route. Read as input; its remaining idea, never emit an own network as a route, is already how `ExcludeOwn` is used. Nothing to continue.

**Debt pulled in.** `adr-debt docs/adr` on 2026-11-15 reports 15 deferred rows, none about the apply step. This record adds its own rows (Follow-ups).

## Existing Primitives Audit

- `routes.CalculateDiff` and `routes.Entry` (`pkg/routes/routes.go`): reuse as the route diff; T1 makes the compare canonical and makes a duplicate desired network deterministic. `routes.ExcludeOwn` and `routes.canonicalNetwork`: reuse; T1 exports the canonical form and calls `ExcludeOwn` from the one function that resolves owners.
- `desiredPeerConfig`, `addAllowedIP` and the relay mapping in `buildDesiredPeerConfigsWithHandshakes` (`daemon.go:566-648`): reuse as the peer part of the desired state; T2 adds the route list as a second product of the same loop, so AllowedIPs and routes cannot disagree.
- `getCurrentRoutes`, `applyRouteDiff`, `restoreConnectedRoute` (`pkg/daemon/routes.go`): reshape into the observe, plan and apply of the route target; the restore logic (ADR-less, commits `74334be`, `6c29fc1`) is kept as is.
- `wireguard.GetPeers`, `GetLatestHandshakes`, `SetPeer`, `RemovePeer` (`pkg/wireguard/apply.go`): `SetPeer` and `RemovePeer` reuse; `GetPeers` and the reconcile's use of `GetLatestHandshakes` are replaced by one `wg show <iface> dump` (peers, endpoints, AllowedIPs and handshakes in one fork); the functions stay for their other callers (`daemon.go:1068`, `:1285`, `pkg/discovery/dht.go:1136`, `:1298`, `exchange.go:902`).
- `CommandExecutor` and `MockCommandExecutor` (`executor.go`, `helpers_test.go`): reuse for every new command; the Darwin adapter is selected by `runtime.GOOS` as `helpers.go` already does, so it is testable on Linux with the mock.
- `getLocalSubnets` and `detectLocalSubnets` (`daemon.go:860-890`): reuse for the attached-network rule (decision 9).
- `HandshakeStaleAfter`, `isTemporarilyOffline`, `relayRoutes` (`daemon.go`): reuse as the liveness input of the owner rule; this record defines no new liveness (a sibling draft, branch `task/adr-0004-liveness`, may change what "alive" means; only the predicate passed to `routes.Resolve` would change).
- `lastAppliedPeerConfigs`: replaced for AllowedIPs by the kernel read; a smaller `pushedEndpoint` map stays for the endpoint, because a peer's endpoint is also written by WireGuard when the peer roams, so the kernel value cannot be compared with the desired one.
- `reconcile` runs on one goroutine (`reconcileLoop`) and once more from `handleSIGHUP` (`daemon.go:1680`) on the signal goroutine; the step keeps its small state under one mutex so those two do not race.
- Nothing like an observe step exists: `grep -rn -i "observe" --include=*.go pkg/daemon` finds no kernel reader (checked 2026-11-15).

## Decision

1. **One step, `converge`, with four parts, in `pkg/daemon`.** Per reconcile tick: (a) `observeKernel` reads the kernel (read-only, per OS); (b) `desire` computes one `desiredState` from the active peers, the handshakes just read, the relay table and the local node (pure); (c) `planKernel(desired, observed)` returns what to change (pure); (d) `applyKernel(plan)` writes it, one item at a time, an error on one item never skips the others, and returns a result per item. It lives in `pkg/daemon` because the seam it needs, `cmdExecutor` and its mock, is there and there is one consumer; the pure rules that the centralized pipeline also calls live in `pkg/routes`.
2. **One desired state feeds AllowedIPs and routes.** `desire` is the only place that calls `routes.ExcludeOwn`, so ADR-0002's apply-side rule stays exactly one rule (decision 2 above). A network is installed in the kernel route table if and only if it is in exactly one peer's AllowedIPs, which removes the case where the two disagree (a route to a network no peer is allowed to carry).
3. **One owner per network, chosen by `routes.Resolve`** (pure): candidates are the peers that advertise the canonical network after `ExcludeOwn`; the owner is chosen by decision 4, recommendation A. Peers keep their own `/32` and `/128`. If the owner is relayed, its networks go to the relay's AllowedIPs and the route gateway is the relay's mesh IP, as today.
4. **The routes are canonical and per family.** `default` is read as `0.0.0.0/0` for `ip -4` and `::/0` for `ip -6`; bare hosts get `/32` or `/128`; a desired route whose network is IPv6 uses the peer's mesh IPv6 as gateway (a peer without one is skipped, logged once); the observe reads `-4` and `-6`. `CalculateDiff` compares canonical forms and, if the caller still passes two desired entries for one network (the centralized pipeline does), keeps the one with the numerically lowest gateway, so that path stops flapping too.
5. **Forwarding is converged, not re-applied.** Observe reads `/proc/sys/net/ipv4/ip_forward` and runs `iptables -C` only when forwarding is wanted; apply writes the sysctl only when it reads 0, appends the rule only when `-C` says it is absent, treats a missing `iptables` as an outcome ("unavailable"), not an error, and logs a result only when it changes. Whether a node is wanted to forward is decision 5, staged by decision 8.
6. **Peers are compared with the kernel, not with a string.** Observe reads `wg show <iface> dump`; a peer is set when it is missing or its AllowedIPs set differs; shrinking peers are applied before growing ones so a moved network is never held by two peers; an observe that fails skips the peer target for that tick and never reads as "no peers" (T3's mutant). The endpoint is pushed when it differs from the last pushed one (`pushedEndpoint`), not compared with the kernel.
7. **macOS gets the same step.** The Darwin adapter reads routes with `netstat -rn -f inet` and `-f inet6`, adds with `route -n add -net <cidr> -interface <utun>` and deletes with `route -n delete`, with no gateway because WireGuard picks the peer by AllowedIPs; the unit tests run through the mock, the real command formats are verified by a human on a Mac (T6) before the task is done. Forwarding is not configured on macOS (out of scope).
8. **Staging: the two-release rule where behaviour changes.** Release N applies everything that cannot make a node worse: the idempotent compare, canonical routes, the IPv6 gateway, error isolation, one owner (the old outcome was "whichever peer was written last", so any fixed choice is one of the old outcomes), the edge-triggered log, and macOS routes (new capability). Two rules that stop doing something the old daemon did are computed and counted but not enforced at N: `forward-scope` (a node that neither relays nor advertises still gets forwarding) and `attached-network` on Linux (a route to an attached network is still installed); each logs once and increments `wgmesh_kernel_warnings_total{rule}`. N+1 flips `EnforceForwardScope` and `EnforceRouteGuard`, one line each, per decision 8. Attached-network on macOS is enforced from N because there is no old behaviour. Revert on exit follows decision 7 and has a third switch `EnforceRevertOnExit`, `false` at N and flipped at N+1.

What would make this decision fail: (a) a steady state that still writes: `TestConvergeSteadyStateIssuesNoWrites` (T5) runs 10 cycles against a fake kernel and asserts zero writes after the first; (b) an observe error read as an empty kernel, which would remove every peer: `TestObserveFailureSkipsApply` (T3); (c) AllowedIPs and routes disagreeing: `TestDesireRoutesAndAllowedIPsFromOneOwner` (T2); (d) a new writer outside `converge`: `TestKernelWritesOnlyInConverge` (T3). Only (a) and (c) can be red today; the others exist once their first step has run, against compiling stubs so the first red run fails on assertions.

## Alternatives Considered

- **Fix each issue where it shows.** Rejected: that is how the class arose (five peer writers, three derivations, one string cache, one blind sysctl). The next writer, or the macOS route code of #862, starts without observe and without the owner rule; a guard test that finds a direct writer is cheaper than reviewing each one.
- **Only canonicalise `CalculateDiff` and add an owner pick in `syncPeerRoutes`.** Rejected: it fixes the kernel route flap and leaves AllowedIPs, which `applyDesiredPeerConfigs` derives separately, giving the same network to two peers; the failover half of #859 needs both from one choice.
- **Routes without a gateway (`<net> dev wg0`).** Rejected: WireGuard picks the peer by AllowedIPs, so the gateway is not what carries the traffic and the route flap would disappear, but `getCurrentRoutes` skips gateway-less routes on purpose (connected routes must never be removed, `routes.go:145-147`, `CalculateDiff` doc), so those routes would become unmanaged; the table shape also changes for every operator.
- **A new package `pkg/kernelstate` behind an interface.** Rejected: one consumer, and the seam (`cmdExecutor` and its mock, `runtime.GOOS` switches) already exists in `pkg/daemon`; a package would need its own runner interface and an adapter to the existing one. The pure part that two pipelines share goes into `pkg/routes`, which already says it holds only shared pure logic (`routes.go:1-4`).
- **Observe every table: `ip route show table all` each tick.** Rejected: its size is unbounded on a router; the daemon reads the routes of its own interface and queries other devices only when it is about to install a default route (decision 6).
- **Keep `lastAppliedPeerConfigs` and add the kernel read beside it.** Rejected: two sources of truth; the cache is the thing that hid the failover and the stale-cleanup case. Only the endpoint, which the kernel value cannot answer, keeps a map.
- **Netlink instead of the `ip` binary.** Rejected here: it would add a dependency (`go.mod` is on the review list in `CLAUDE.md`) and does not change the shape of the decision; observe, plan and apply are the same behind either.

## Component / Boundary Impact

- `pkg/routes`: gains `Canonical`, `Resolve` and a deterministic `CalculateDiff`; one reason to change, how a set of advertised networks becomes a set of routes. `pkg/ssh` and `pkg/mesh` call `CalculateDiff` unchanged in signature; their tests must stay green (T1 invariant).
- `pkg/daemon`: gains `desired.go`, `observe.go`, `converge.go` and the counters in `metrics.go`; `routes.go` shrinks to the route adapter; `daemon.go` loses the two derivations and the cache, `reconcile` calls `converge`. One reason to change for the new files: what the kernel must look like and how to get it there. `daemon` still does not import `discovery`.
- `pkg/node`, `pkg/discovery`, `pkg/crypto`: untouched.
- `pkg/wireguard`: gains a `dump` parser used only by `pkg/daemon` (T3); its centralized functions are untouched.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| `routes.Canonical`, `routes.Resolve`, deterministic `routes.CalculateDiff` | new and tightened, pure | `pkg/routes` | `pkg/daemon/desired.go`; `pkg/ssh`, `pkg/mesh` through `CalculateDiff` |
| `desiredState`, `kernelObserved`, `kernelPlan`, `applyResult` | new, package-internal | `pkg/daemon` | `reconcile`, `attemptPeerReconnect`, `evictPeerFromPool`, `staleCleanupLoop` |
| `wireguard.ParseDump` (reads `wg show <iface> dump`) | new | `pkg/wireguard` | `pkg/daemon/observe.go` |
| `wgmesh_kernel_changes_total{target}` and `wgmesh_kernel_warnings_total{rule}` | new Prometheus counters; the first stops growing in steady state | `pkg/daemon/metrics.go` | operators, the N+1 go/no-go |
| `EnforceForwardScope`, `EnforceRouteGuard`, and `EnforceRevertOnExit` | new package variables, `false` at N | `pkg/daemon` | later releases flip them; tests set them |
| Log lines for kernel apply failures | now printed on change of result, not every tick | `pkg/daemon/converge.go` | operators |
| Kernel: peers' AllowedIPs, routes, `ip_forward`, FORWARD rule, address | same commands, issued only on a difference; shared networks have one owner; IPv6 routes use an IPv6 gateway; macOS gains routes | `pkg/daemon` | the host |
| Flags, config file, reload file, wire, envelope, announcement JSON, persisted files | None — no field, flag or byte is added | — | — |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `routes.Canonical`, `routes.Resolve`, deterministic `CalculateDiff` | T1 | T2, T5, T6 | No: signatures of existing functions are unchanged; `pkg/ssh` and `pkg/mesh` tests are the proof |
| `desiredState` built once per tick, `reconnectAllowedCSV` reading it | T2 | T3, T4, T5, T6, T7 | No: internal |
| `wireguard.ParseDump`, `kernelObserved`, `planKernel`, `applyKernel`, `pushedEndpoint`, the result log, `TestKernelWritesOnlyInConverge`, the two counters | T3 | T4, T5, T7, T8 | No: the peer apply keeps its commands |
| forwarding converge, `EnforceForwardScope` | T4 | T5, T8 | No: applies as today at N |
| route observe, plan and apply for both families, `EnforceRouteGuard`, the attached-network rule | T5 | T6 | No: removes the forwarding calls from `applyRouteDiff` only after T4 has moved them |
| Darwin adapter | T6 | T9 | No: new capability |
| address converge | T7 | T9 | No |
| revert on exit | T8 | T9 | No: staged by decision 7 |

## Implementation

See `tasks/README.md`. Nine tasks in five waves; the tasks that edit `pkg/daemon/daemon.go` run one after another on one branch, and all of them start after ADR-0002's T10 has landed (see the order paragraph below). T1 the pure canonical and owner code in `pkg/routes`; T2 one desired state; T3 the peer observe and diff plus the guard test, the counters and the result log; T4 forwarding; T5 routes for both families; T6 macOS; T7 interface address; T8 revert on exit (waits for decision 7); T9 specs, release note and backlog rows.

**Order against ADR-0002 and the sibling drafts.** ADR-0002's T3 (`evictPeerFromPool` calls `Withdraw`, `daemon.go:1402`), T4, T5, T6 and T10 edit `daemon.go`; T3 here edits the same function (`evictPeerFromPool`, `:1406-1408`, to drop the cache delete). T1 here waits for ADR-0002-T10 because T10's fence asserts an empty diff of `pkg/routes` and its test `TestHAExitNodesDropEachOthersDefault` in `pkg/daemon/routes_test.go` pins the HA rule that T2 here moves; that test must stay green unchanged. T7 waits for ADR-0002-T5 (it persists the collision-resolved IP in `collision.go`) and does not edit `collision.go`; the call at `collision.go:123` becomes a no-op next to T7 and a follow-up row removes it. T9 waits for ADR-0002-T11 because both amend the daemon support spec. The centralized pipeline draft (`task/adr-0006-central-deploy`) calls `routes.CalculateDiff`; T1 changes only what it returns for duplicate or `default` input. The liveness draft (`task/adr-0004-liveness`) and the atomic-state draft (`task/adr-0005-atomic-state`) touch `daemon.go` liveness and the unlocked read of `localNode.RoutableNetworks` that `reconcile` does (`daemon.go:625`, written at `:1719` without a lock); that read stays as it is here.

## Consequences

- **Positive:** the kernel is read before it is written, so a steady node issues no writes (counter `wgmesh_kernel_changes_total` flat); a shared subnet has one owner and fails over; a default route no longer flaps; IPv6 routes work or are skipped with one log line; one failing item cannot stop the rest; forwarding and the firewall rule are converged once and a missing `iptables` is logged once; a peer removed by stale cleanup is re-added when it returns; macOS routes peers' networks; `wg` is forked once per tick instead of twice (`GetPeers` and `GetLatestHandshakes` become one `dump`).
- **Negative:** failover of a shared network takes up to `HandshakeStaleAfter` (150 s) plus one tick, not measured; the owner rule is local, so two nodes can pick different owners for the same network (each node's choice is about its own outbound traffic, and no wire byte carries it); the real `netstat` formats on macOS are unverified until T6's human step; `observeKernel` adds reads per tick (a file read and, when forwarding is wanted, one `iptables -C`, which the old code already ran every tick); the endpoint map keeps one small cache.
- **Neutral:** `reconcile` calls one function instead of three; the existing assertion that `applyRouteDiff` runs `sysctl` (`pkg/daemon/routes_test.go:234`, `TestApplyRouteDiff_AddAndRemove`) is deliberately replaced by T5, which is the change itself and not a regression; `restoreConnectedRoute` and `ExcludeOwn` behave as before.

## Out of Scope

- The centralized pipeline's own apply (`pkg/mesh/deploy.go`, `pkg/ssh/routes.go`, `pkg/wireguard/persist.go`) and #867, #865, #866, #868 (permanent: boundary: another group owns the centralized pipeline; #867 is already fixed at `pkg/wireguard/config.go:160-164`, PR #848's change is already on main).
- What enters the PeerStore, route approval, `accept-routes` (permanent: boundary: ADR-0002 decides it and this record starts after the store).
- Liveness: when a peer counts as alive, the eviction, the relay choice (permanent: boundary: another group owns liveness; this record passes the existing handshake and temporary-offline facts to `routes.Resolve` as a predicate).
- Transport exemption for an accepted default route (`fwmark` or a policy rule) so a node's own WireGuard UDP is not routed into `wg0` (deferred: docs/adr/BACKLOG.md).
- Forwarding, NAT and firewall policy on macOS, `pf` rules, and `nftables` (permanent: boundary: this record converges the one rule the daemon already installs; other firewall management is not wgmesh's decision to make).
- Netlink instead of `ip` and `route` (permanent: boundary: shape of the decision is the same behind either; `go.mod` changes need review).
- Removing the `setInterfaceAddress` call at `pkg/daemon/collision.go:123` (deferred: docs/adr/BACKLOG.md).
- Making `reconcile` take locks around `localNode` fields (permanent: boundary: single-writer and atomic-load work owned by another group).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| A failed or partial observe is read as an empty kernel and the plan removes every peer or route | Med | High | each target is observed separately; an error skips that target for the tick and is logged; T3 and T5 mutants make the observe return empty and the tests must fail |
| `wg show dump` format differs on an old `wireguard-tools` | Low | Med | T3 parses by field count and has a fixture with `(none)` fields; the previous per-field commands stay in `pkg/wireguard` and are the fallback of a parse error, which skips the tick |
| The owner rule fails over later than operators expect | Med | Med | 150 s is the existing `HandshakeStaleAfter` used by relay and eviction; documented in the note (T9); decision 4 offers B |
| A node that does not advertise or relay loses forwarding at N+1 and an operator's other service stops | Med | Med | warn once at N with the reason; the counter is the go/no-go; `EnforceForwardScope` flips back in one line; decision 5 offers B |
| The attached-network rule skips a route an operator wanted | Low | Med | equal or inside only, a supernet is kept; warn at N on Linux; decision 9 |
| macOS command formats differ from the fixtures I could not record here | Med | Med | T6 is human-observed on a Mac and carries the PoC commands; it is not `done` without the sign-off |
| `reconcile` runs twice at once (`handleSIGHUP`) and two applies interleave | Low | Med | the step's state (`pushedEndpoint`, last results) is under one mutex; `go test -race` in every fence |
| Existing tests that pin the old shape (`TestApplyRouteDiff_AddAndRemove` expects `sysctl`) fail | High | Low | named in T5 as an intended change, not a regression |

## Rollback

All kernel state is runtime state; nothing persisted changes shape (no flag, file or byte is added). Reverting the release restores the old tick: it re-applies everything blindly, which is today's behaviour. The kernel left by N is the kernel the old daemon leaves (routes on `wg0`, `ip_forward`, the FORWARD rule); the old daemon's first tick reads routes as before and converges them; `pushedEndpoint` and the result log are memory only. A revert after N+1 returns the two rules to warn mode by flipping `EnforceForwardScope` and `EnforceRouteGuard` back, no restart state to clear. If T8 ships, a revert re-introduces the leak of #872 and nothing else.

## Follow-ups

- [ ] N+1: set `EnforceForwardScope = true`, `EnforceRouteGuard = true` and `EnforceRevertOnExit = true`, with a release note; `wgmesh_kernel_warnings_total{rule}` read as the go/no-go (deferred: docs/adr/BACKLOG.md)
- [ ] Transport exemption for an accepted default route (deferred: docs/adr/BACKLOG.md)
- [ ] Remove the now redundant `setInterfaceAddress` call in collision resolution once T7 and ADR-0002 are released (deferred: docs/adr/BACKLOG.md)
