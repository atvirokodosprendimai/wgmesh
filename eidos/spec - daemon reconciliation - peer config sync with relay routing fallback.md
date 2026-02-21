---
tldr: Every 5 seconds, the daemon builds desired WireGuard peer configs from active peers and applies them; relay routing redirects unreachable peers through an introducer node.
category: core
---

# Daemon reconciliation

## Target

The core loop that keeps WireGuard peer configuration in sync with discovered peers,
and the relay routing decision that handles peers unreachable by direct path.

## Behaviour

- The reconciliation loop runs every 5 seconds and on SIGHUP.
- Each cycle: read active peers → build desired WireGuard peer configs → apply changes → sync kernel routes → check IP collisions.
- A peer is configured as a WireGuard peer only if it has a non-empty endpoint.
  IPv6 endpoints are skipped when `--no-ipv6` is set.
- AllowedIPs per peer: mesh IPv4 `/32` always, mesh IPv6 `/128` if present, plus any advertised routable networks.
- Changes are applied only when endpoint or AllowedIPs change — a signature check (`endpoint|allowedIPs`) prevents redundant `wg set` calls.
- Obsolete peers (in WireGuard but not in desired config) are removed via `wg set peer … remove`.

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
- `applyDesiredPeerConfigs` uses an optimistic lock pattern: mark signature before applying, roll back on failure, to prevent stale cache entries.
- `configMu` read lock is held when reading `AdvertiseRoutes` and `DisableIPv6` inside reconcile.
- Peers marked temporarily offline (by the mesh probe) are excluded from both the desired config and route sync.

## Interactions

- `PeerStore.GetActive()` — source of truth for which peers to configure.
- `wireguard.GetPeers`, `wireguard.SetPeer`, `wireguard.RemovePeer` — apply live WireGuard changes.
- `wireguard.GetLatestHandshakes` — read per-peer handshake times to inform relay decision.
- `routes.go syncPeerRoutes` — called after peer config is applied; uses relay table for gateway selection.
- `collision.go CheckAndResolveCollisions` — called at end of each reconcile cycle.

## Mapping

> [[pkg/daemon/daemon.go]]
