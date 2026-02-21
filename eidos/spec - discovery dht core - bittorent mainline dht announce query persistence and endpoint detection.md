---
tldr: BitTorrent Mainline DHT announces local endpoint under a mesh-secret-derived hourly-rotating infohash; queries return peer addresses that are contacted via peer exchange; routing table persists between restarts; STUN refresh and IPv6 interface scan keep the local endpoint current.
category: core
---

# Discovery — DHT announce, query, persistence, and endpoint detection

## Target

The BitTorrent Mainline DHT layer of `DHTDiscovery`: server lifecycle, announce/query loops,
persistence of DHT routing table nodes, STUN refresh, and IPv6 endpoint selection.

## Behaviour

### DHT server

- Uses `github.com/anacrolix/dht/v2` (BEP 5 Mainline DHT implementation).
- Binds to `gossipPort + 1` — separate from the exchange/gossip port to prevent read-deadline
  interference between the two servers.
- Bootstrap: contacts well-known BitTorrent DHT bootstrap nodes on first run.
  Waits up to 10 seconds for at least one routing table node to appear; continues anyway on timeout.
- On startup, loads previously persisted DHT nodes before the bootstrap lookup for warm start.
- DHT routing table nodes are persisted to disk every 2 minutes and on clean shutdown.
  File: `/var/lib/wgmesh/<iface>-<network_id_hex8>-dht.nodes`
  where `network_id_hex8` = first 8 bytes of `NetworkID` as hex.

### Announce loop (every 15 minutes)

- Derives the current and previous network IDs from the shared secret via
  `crypto.GetCurrentAndPreviousNetworkIDs` — IDs rotate on the hour.
- Announces `(networkID, exchangePort)` into the DHT using BEP 5 `announce_peer`.
  During the transition minute: announces under both current and previous IDs for continuity.
- Announces on the first cycle immediately at startup.

### Query loop (30s initially, 60s once mesh is stable)

- Queries the same network IDs via BEP 5 `get_peers`.
  Also queries previous hour's ID during transitions.
- Each returned peer address is dispatched to `contactPeer`:
  - Deduplicates: skips addresses contacted within the past 60 seconds.
  - Skips own external address.
  - Calls `ExchangeWithPeer` (via `PeerExchange`) — on reply, peer is stored as `"dht"`.
- Query interval slows from 30s to 60s once the peer store holds ≥3 peers.

### Goodbye broadcast

- On `Stop()`: sends `GOODBYE` to all known peers' control endpoints before shutting down,
  allowing peers to remove this node immediately rather than waiting for dead timeout.

### External endpoint detection

- On startup: tries IPv6 first, then STUN-over-IPv4.
- **IPv6 detection**: scans all non-loopback network interfaces for global unicast public IPv6
  addresses (excluding ULA, link-local, documentation, Yggdrasil-style 200::/7, multicast,
  loopback). Scores candidates:
  - Base score: 10
  - +20 for `2000::/3` global unicast prefix
  - -5 penalty for EUI-64 addresses (bytes 11–12 are `ff:fe` — embeds MAC, less stable)
  - Deterministic tiebreak: lexicographic
  - IPv6 is preferred: implies no NAT, so `NATType = "unknown"` is set (not `cone` or `symmetric`)
- **STUN (IPv4 fallback)**: `DetectNATType` with two servers; updates `localNode.WGEndpoint`
  (STUN-derived external IP + WG listen port) and `localNode.NATType`.

### STUN refresh loop (every 60 seconds)

- Re-runs endpoint detection: tries IPv6 first, falls back to STUN.
- Logs endpoint and NAT type changes for observability.
- On endpoint change: next announce cycle will publish the new address.

## Design

- The DHT infohash is derived from the shared secret (not public key) — only nodes with the
  correct secret can find each other in the global DHT keyspace.
- Hourly rotation of the network ID provides forward secrecy for discovery metadata:
  an observer can only correlate discoveries within a one-hour window.
  The two-ID overlap ensures no connectivity gap during rotation.
- EUI-64 penalty avoids advertising a MAC-address-embedding endpoint when a stable privacy
  extension address is also available.
- Query interval adaptive slow-down reduces DHT traffic in a stable mesh.

## Interactions

- `github.com/anacrolix/dht/v2` — DHT server, announce, get_peers.
- `pkg/crypto.GetCurrentAndPreviousNetworkIDs` — hourly-rotating infohash derivation.
- `PeerExchange.ExchangeWithPeer` — contact each DHT-discovered peer address.
- `pkg/daemon.PeerStore.Update(peer, "dht")` — store discovered peers.
- `DetectNATType`, `DiscoverExternalEndpoint` — STUN queries.
- `pkg/daemon.LocalNode.SetEndpoint`, `.NATType` — update local state.

## Mapping

> [[pkg/discovery/dht.go]]
