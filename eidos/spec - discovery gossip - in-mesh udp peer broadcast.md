---
tldr: Every 10s, gossip picks a random active peer and sends an encrypted UDP packet containing the full local peer list to its mesh IP; the receiver extracts direct and transitive peers.
category: core
---

# Discovery — in-mesh gossip

## Target

The broadcast mechanism that propagates peer knowledge within the already-established WireGuard
mesh, distributing entries that may not be reachable via DHT or LAN.

## Behaviour

- Binds a UDP socket to the node's mesh IP on `gossipPort` (or all interfaces as fallback).
  Traffic is sent to peers' **mesh IPs** — all packets flow through the WireGuard tunnel.
- Every 10 seconds, selects one peer at random from the active store (excluding self and peers
  without a mesh IP) and sends an encrypted announcement to it.
- The announcement contains the local node's own info and the full list of known peers
  (`KnownPeers`), excluding the target peer itself.
- Announcements are envelope-encrypted with the mesh gossip key (`crypto.SealEnvelope`) —
  same key material as LAN discovery.
- Inbound rate-limited per source IP before decryption (prevents message flood from a single peer).

### Peer discovery from inbound messages

On receiving a gossip packet:
1. Decrypt with gossip key; discard on failure.
2. Skip if sender is local node.
3. Deliver sender peer to store as `"gossip"`.
4. For each peer in `KnownPeers`: deliver to store as `"gossip-transitive"`.
   Transitive entries carry lower endpoint ranking than direct gossip entries (see peer store spec).

### Exchange-integrated mode

`MeshGossip` can operate in two modes:

- **Standalone** — own UDP socket, builds and sends packets directly.
- **Exchange-integrated** — no own socket; delegates sends to `PeerExchange.SendAnnounce()`.
  The PeerExchange server owns the socket and calls `gossip.HandleAnnounceFrom()` for inbound
  gossip packets.

Mode is selected at construction time: `NewMeshGossipWithExchange` vs `NewMeshGossip`.

## Design

- Gossip targets mesh IPs (not external WireGuard endpoints) — a peer must already be in the
  mesh (WireGuard tunnel established) to receive gossip. This is intentional: gossip is a
  within-mesh propagation mechanism, not a bootstrap discovery.
- Sending to mesh IPs means the source port of inbound packets is the gossip port on the mesh
  IP, not the peer's WireGuard underlay endpoint — the sender address is not used for endpoint
  learning (discarded).
- Rate limiting happens before decryption to prevent CPU-intensive decryption from being
  triggered by a flood.

## Interactions

- `pkg/crypto.SealEnvelope` / `OpenEnvelope` — encrypt/decrypt gossip packets.
- `pkg/daemon.PeerStore.GetActive()` — source of peers to gossip to and to include in payloads.
- `pkg/daemon.PeerStore.Update(peer, "gossip")` / `Update(peer, "gossip-transitive")` — deliver learned peers.
- `PeerExchange.SendAnnounce()` — delegated send in exchange-integrated mode.
- `pkg/ratelimit.IPRateLimiter` — inbound rate limiting.

## Mapping

> [[pkg/discovery/gossip.go]]
