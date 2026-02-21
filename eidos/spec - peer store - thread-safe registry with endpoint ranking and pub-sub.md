---
tldr: Thread-safe in-memory registry of discovered peers; merge semantics prefer higher-trust endpoints; subscribers receive new/updated events without blocking the store.
category: core
---

# Peer store

## Target

The runtime registry of all discovered mesh peers, shared between discovery, reconciliation, and health subsystems.

## Behaviour

- Peers are keyed by WireGuard public key. All operations are thread-safe.
- `Update(info, discoveryMethod)` merges incoming data into existing records:
  - Endpoint: updated only if the new source has higher or equal rank (see Endpoint ranking below).
  - RoutableNetworks, MeshIP, MeshIPv6, Hostname: last non-empty value wins.
  - Introducer flag: always overwritten by the latest announcement (a node can stop being an introducer).
  - NATType: last non-empty value wins.
  - DiscoveredVia: accumulates all methods used to find this peer (no duplicates).
  - LastSeen: refreshed on direct discovery; not refreshed for cache restores or transitive methods.
- New peer insertions are rejected when the store holds 1000 peers (flood protection). Updates to existing peers are always allowed through.
- **Dead timeout:** 5 minutes without an update → peer considered dead; excluded from `GetActive()`.
- **Remove timeout:** 10 minutes without an update → removed from store by the stale cleanup loop.
- Subscribers receive `PeerEvent` (new / updated) on a buffered channel (size 16). Events are sent outside the store lock to prevent deadlock. Non-blocking send: lagging subscribers drop events silently.

### Endpoint ranking

Higher rank wins when updating an endpoint. From highest to lowest:

| Rank | Method |
|---|---|
| 100 | LAN (`lan`) |
| 90 | DHT rendezvous (`dht-rendezvous`) |
| 70 | DHT direct (`dht`) |
| 65 | Gossip direct (`gossip`) |
| 40 | DHT transitive (`dht-transitive`) |
| 35 | Gossip transitive |
| 30 | (default / other) |
| 20 | Cache restore |

At equal rank: IPv6 endpoint preferred over IPv4.
LAN endpoint is sticky — only a newer LAN discovery can replace it.

## Design

- `Get` returns a copy of the peer struct to prevent callers from mutating store state.
- Notification is decoupled from the write lock: subscriber snapshot is taken under read lock, sends happen after lock release.
- `shouldUpdateEndpoint` encodes the ranking and IPv6 preference as a pure function, making the merge policy testable independently of the store.

## Interactions

- Discovery layers (DHT, LAN, gossip) — call `Update` when new peers are found.
- Reconciliation loop — calls `GetActive()` each cycle.
- Health monitor — calls `GetActive()` and `Remove()`.
- Peer cache — calls `GetAll()` for serialisation; calls `Update(…, "cache")` on restore.
- Stale cleanup loop — calls `CleanupStale()` every minute.

## Mapping

> [[pkg/daemon/peerstore.go]]
