# Kernel state: how the daemon writes the kernel

Decision record: [ADR-0003](adr/ADR-0003-kernel-state-apply/ADR-0003-kernel-state-apply.md). This page is for the operator of a mesh and is the release note for the releases that bring it in. The release after the first is a row in [the backlog](adr/BACKLOG.md), not a task.

Every tick the daemon reads what the kernel holds, computes one desired state from the active peers, and writes only the difference. No byte on the wire changes, no flag or file changes shape, so old and new nodes talk to each other during the whole rollout.

## The converge step, once per tick (every 5 seconds and on SIGHUP)

1. **Observe.** One `wg show <iface> dump` (peers, endpoints, AllowedIPs, handshakes); the routes of the mesh interface for both families (`ip route show dev <iface>` and `ip -6 route show dev <iface>`; `netstat -rn -f inet|inet6` on macOS); `ip -o addr show dev <iface>`; and, on Linux when forwarding is wanted, `/proc/sys/net/ipv4/ip_forward` and `iptables -C FORWARD -i <iface> -o <iface> -j ACCEPT`.
2. **Desire.** One desired state: every peer's AllowedIPs and every route come from the same owner choice, so a route to a network no peer may carry cannot exist.
3. **Plan, apply.** Per target the difference only, one item at a time; a failed item never skips the others. Removals run before adds for addresses; peers that shrink are set before peers that grow, so a moved network is never held by two peers.
4. **Result.** One result per item. A result is logged when it differs from the last one of that item: a failure once, a recovery once (`<op> <target> <key>... done` for a write, `... recovered` after a failure). A steady node writes nothing and logs nothing.

A target that cannot be read is skipped for that tick and never read as empty. A failed or unparsable `wg show dump` skips the whole tick: no peer, route, address or forwarding change, and the relay table is not updated (one line, `Cannot read the WireGuard interface, skipping this tick`, then `Reading the WireGuard interface works again`).

## One owner for a shared network

When several peers advertise one network, the node picks one owner:

1. A peer with a handshake newer than `HandshakeStaleAfter` (150 s) beats one without.
2. Among fresh peers the owner read back from the kernel keeps the network (no automatic fail-back).
3. Otherwise the lowest mesh IP wins, compared as a number (`10.99.0.9` before `10.99.0.10`), then the lowest public key.

- The choice is per node: two nodes may pick different owners, because it only decides this node's outbound traffic.
- Failover bound: the network moves at the first tick after the owner's last handshake is `HandshakeStaleAfter` old, so at most `HandshakeStaleAfter` plus one `ReconcileInterval` (5 s) after that handshake. This is read from the constants (`pkg/daemon/daemon.go`), not measured.
- A network this node advertises itself is owned by nobody: two HA exit nodes each drop the other's default route (`routes.ExcludeOwn`, unchanged).
- Limit: when the owner is relayed, the kernel holds the network under the relay's key, so the "observed owner stays" step never matches and the lowest-IP rule decides. The owner is still one and stable, but it does not stay on a fresh peer that is not the lowest (BACKLOG).
- The owner that lost is logged once per change: `Shared network <net>: owner <key> via <gw>, ignored <key> via <gw>`.

## Release switches

All four are `false` in release N; flipping one is a one-line change with its own release note. Flip one only when every node runs the release before it.

| Switch (rule) | At N | At N+1 | Counter to read before flipping |
|---|---|---|---|
| `EnforceForwardScope` (`forward-scope`) | every Linux node still gets `ip_forward=1` and the FORWARD rule; one line and one count when the node neither is an introducer nor has `advertise-routes` | only an introducer or a node with `advertise-routes` is forced; others are left alone | `wgmesh_kernel_warnings_total{rule="forward-scope"}` zero, or every node that counts it should forward (set `--introducer` or `advertise-routes`) |
| `EnforceRouteGuard` (`attached-network`) | a peer's route to a network this host is attached to (equal or inside) is still installed; one line and one count | the route is left out of the routes and the AllowedIPs; the line is logged, the Linux counter no longer counts | `{rule="attached-network"}` zero |
| `EnforceAddressRemoval` (`stale-address`) | an address on the mesh interface inside the mesh network that is not this node's is warned about, once per address while it holds, and kept | it is removed (`removing <addr>: inside the mesh network and not this node's address`) | `{rule="stale-address"}` zero |
| `EnforceRevertOnExit` | planned, T8: not built yet. {>> to be completed by T8: what is reverted at exit, the line and the counter} | planned, T8 | planned, T8 |

- `attached-network` on macOS is enforced from the first release, whatever `EnforceRouteGuard` says (there was no old behaviour), and is counted there.
- `ip route replace` of an attached network overwrites the host's own connected route for it (reproduced on a Linux 6.17 kernel in a network namespace, 2026-11-16: `192.168.50.0/24 dev dum0 proto kernel` was replaced). The daemon restores the connected route when it later removes the route. A supernet of an attached network is kept; the mesh subnet, the mesh IPv6 prefix and link-local networks are never counted as attached.
- Addresses: the daemon adds a missing mesh address and never flushes; an address inside the mesh network that it did not derive would be removed at N+1, including one an operator added by hand. A point-to-point address (`inet A peer B`) is never touched. `setInterfaceAddress` still flushes the family once at daemon start.

## Reading the counters

Both need `--metrics` (see the README, Metrics): enable it on every node and check that each answers before reading a zero.

- `wgmesh_kernel_changes_total{target}`: writes that changed the kernel. `target` is `peer`, `forward`, `route` or `address`. In a steady state it is flat; a restart adds one `peer` count per peer, because the pushed endpoints are memory only and every peer's endpoint is pushed once.
- `wgmesh_kernel_warnings_total{rule}`: `rule` is `forward-scope`, `host-default`, `attached-network`, `foreign-route` (macOS) or `stale-address`. A case is counted once when it starts and again only after it stopped and returned, not on every tick. Only `peer` of the changes counter exists at start; every warning series is absent until counted, so write an alert as `sum by (rule) (wgmesh_kernel_warnings_total) or vector(0)`. Counters reset on restart: the gate is "zero since process start on every node".

## Default route

An advertised `0.0.0.0/0` (or `::/0`) is still installed with `replace`: it is a feature. When the host already has a default route of that family on another device, one line and one count say so (`host-default: this host already has an IPv4 default route on <dev>; the mesh's ... is installed as well`).

Caveat, not built and not run (BACKLOG): the node's own WireGuard UDP traffic may be routed into the mesh interface once a default route is accepted. A transport exemption (an `fwmark` on the socket and a policy rule) is a separate decision.

## IPv6 and macOS

- An IPv6 route uses the peer's mesh IPv6 as gateway (`ip` refuses an IPv4 gateway for an IPv6 network, reproduced 2026-11-16 on a Linux 6.17 kernel). A peer without a mesh IPv6 is skipped for that network (one line); with `--no-ipv6` IPv6 networks are not routed (one line per network).
- IPv6 routes on the mesh interface are now managed: a route `ip -6 route ... via` that an operator added on it is removed when it is not desired. Before, only IPv4 was read.
- macOS routes (`route -n add|change|delete -net <cidr> -interface <utun>`, no gateway) are built and tested against hand-written fixtures only. **They are not verified on a real Mac: the T6 sign-off is open.** Every format assumption (the `netstat` columns, taken from the header; a `link#N` gateway or the interface name; the flags) is written from the man page and is unverified.
- macOS deletes only routes this daemon run installed (an in-memory set; the interface is recreated at every start), so a route left by hand or by an earlier run stays.
- macOS rule `foreign-route`: a peer's network that another interface already routes (the Mac's own default via `en0`, another VPN) is not re-pointed into the mesh; it is skipped, logged once and counted. A mesh-interface row the parser does not know is logged once (`unrecognised route row`).
- `EnforceRouteGuard` is ignored on macOS: attached networks are always dropped there. Forwarding is not configured on macOS.

## Log lines

`Failed to configure peer` and `Failed to remove obsolete peer` are gone. Grep now for `Failed to set peer`, `Failed to remove peer` (and `Failed to <op> route|address|forward ...` for the other targets: the key in the prefix is cut to 16 characters, the error text carries the whole route), plus the once-only lines named above: `forward-scope:`, `host-default:`, `attached-network:`, `foreign-route:`, `stale address`, `Cannot read the ... skipping`. Stale cleanup and eviction still log `[Peers] Failed to remove stale peer` and `[Health] Failed to remove evicted peer`.

## Known limits

- A forced reconnect never touches a relayed peer (its networks sit under the relay): `[Health] Peer ... has no WireGuard peer entry of its own`.
- `reconcile` is serialised; health writes (reconnect, eviction, stale cleanup) take the same converge lock, so a tick and a health write never interleave.
- The `wg` forks have no deadline. A hung `wg` also holds the converge lock and so blocks eviction. A deadline belongs to ADR-0007 (not in this tree).
- Real `netstat` output on a Mac has not been recorded (BACKLOG).

## Mixed fleet

Each node chooses its own owner and writes only its own kernel, so a node with this change and a node without it interoperate. Flip a switch only when every node runs the release before it: a node that forwarded for no reason is the one an N+1 node stops.

## Rollback

All kernel state is runtime state. Reverting the release restores the old tick, which re-applies everything blindly; the kernel left by N is the kernel the old daemon leaves. After N+1, set the switches back to `false` in the next release: nothing persisted needs rewriting.
