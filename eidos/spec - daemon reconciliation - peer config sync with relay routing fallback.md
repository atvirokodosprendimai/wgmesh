---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Every 5 seconds, the daemon observes the kernel, computes one desired state from active peers and applies only the difference; relay routing redirects unreachable peers through an introducer node.
category: core
---

# Daemon reconciliation

## Target

The core loop that keeps WireGuard peer configuration in sync with discovered peers,
and the relay routing decision that handles peers unreachable by direct path.

## Behaviour

- The reconciliation loop runs every 5 seconds and on SIGHUP.
- Each cycle (`converge`): read active peers → observe the kernel → compute one desired state → plan the difference → apply it one item at a time → check IP collisions.
- Observe reads the kernel once per target: the peers with one `wg show <iface> dump` (peers, endpoints, AllowedIPs and handshakes in one fork), the routes of both families, the interface addresses and, when forwarding is wanted, `ip_forward` and the FORWARD rule. A target that cannot be read is skipped for that tick and never read as empty: a failed or unparsable `wg show dump` skips the whole tick, routes and relay table included.
- A peer is configured as a WireGuard peer only if it has a non-empty endpoint.
  IPv6 endpoints are skipped when `--no-ipv6` is set.
- AllowedIPs per peer: mesh IPv4 `/32` always, mesh IPv6 `/128` if present, plus any advertised routable networks.
- The desired state is computed once per cycle and feeds both the peers' AllowedIPs and the kernel routes, so the two cannot disagree. A network that several peers advertise has one owner, chosen by `routes.Resolve`.
- Only the difference between the dump and the desired state is written. A peer is set when it is missing in the kernel, when its AllowedIPs set differs, or when its desired endpoint differs from the one last pushed (`pushedEndpoint`, in memory, so a restart pushes every endpoint once; a set that only repairs AllowedIPs carries no endpoint, so a roamed endpoint is not overwritten). Shrinking peers are set before growing ones. A peer in the kernel that is not desired is removed via `wg set peer … remove`.

### Relay routing

When a peer cannot be reached by direct path, its traffic is tunnelled through an introducer relay:
- The peer's IPs are added to the **relay node's** AllowedIPs (not the peer's own WireGuard entry).
- The relay table (`relayRoutes`) is used by the route sync step to set the correct kernel gateway.

Relay is used when (any of):
- Local node and remote peer both have symmetric NAT (hole-punch unreliable).
- WireGuard handshake is stale (>150s) AND both sides are symmetric NAT.
- Peer was discovered only via transitive DHT and has no WireGuard handshake yet.
- `--force-relay` flag is set and at least one relay candidate exists.

Relay is never used when:
- Local node is an introducer.
- Target peer is an introducer.
- Peer was discovered via LAN or its endpoint is on a local subnet.
- No introducer relay candidates are available.

Relay selection: deterministic hash of `(local pubkey, peer pubkey)` over sorted introducers.
{>> FNV hash with sorted candidates avoids relay flapping across reconcile cycles}

## Design

- Relay candidates: introducers seen within the last 90 seconds with a known endpoint.
- Apply writes one item at a time and an error on one item never skips the others; each item has one result, and a result is logged only when it differs from the last one of that item (a failure once, a recovery once). Every kernel write of a tick, and the health paths (reconnect, eviction, stale cleanup), run under one converge lock; `reconcile` itself is serialised, so the ticker and SIGHUP never overlap.
- `configMu` read lock is held when reading `AdvertiseRoutes` and `DisableIPv6` inside reconcile.
- Peers marked temporarily offline (by the mesh probe) are excluded from both the desired config and route sync.

## Interactions

- `PeerStore.GetActive()` — source of truth for which peers to configure.
- `wireguard.Dump` (`wg show <iface> dump`) — the observe of peers, endpoints, AllowedIPs and the handshakes that inform the relay decision; `wireguard.SetPeer`, `wireguard.RemovePeer` — apply live WireGuard changes.
- `converge.go`, `observe.go`, `desired.go` — observe, desired state, plan and apply of peers, routes, forwarding and addresses; the relay table is a product of the desired state and is used for the kernel gateway.
- `collision.go CheckAndResolveCollisions` — called at end of each reconcile cycle.

## Mapping

> [[pkg/daemon/daemon.go]]
> [[pkg/daemon/converge.go]]
> [[pkg/daemon/observe.go]]
> [[pkg/daemon/desired.go]]
