# Pull — discovery package overview

**Timestamp:** 2602211143
**Target:** `pkg/discovery/`
**Mode:** multi-pass overview

---

## File inventory

| File | Lines | Concern |
|---|---|---|
| `init.go` | 18 | Factory registration with daemon |
| `lan.go` | 252 | UDP multicast LAN discovery |
| `stun.go` | 332 | RFC 5389 STUN + NAT type detection |
| `gossip.go` | 329 | In-mesh UDP gossip broadcast |
| `exchange.go` | 1147 | Peer exchange — hello/reply, rendezvous, hole-punching |
| `dht.go` | 1422 | BitTorrent Mainline DHT announce/query, rendezvous, hole-punching |
| `registry.go` | 411 | GitHub Issues as encrypted rendezvous point |
| **Total** | **3911** | |

---

## Territory map

### 1. LAN discovery (`lan.go`)

- Sends/receives UDP multicast on `239.192.X.Y:51830` where X.Y encodes the `MulticastID` derived from the mesh subnet.
- Every 5 seconds, broadcasts a signed `LANAnnouncement` (WireGuard public key, mesh IP, mesh IPv6, gossip port).
- Announcements are verified against the peer's WireGuard public key (signature over the message body).
- Delivers discovered peers to the peer store via `DiscoveryCallback` with source `"lan"`.
- Stops gracefully on context cancel.

### 2. STUN / NAT detection (`stun.go`)

- **STUN client**: RFC 5389 Binding Request over UDP; parses XOR-MAPPED-ADDRESS and MAPPED-ADDRESS response attributes. No dependency on external library.
- **NAT type detection** (`DetectNATType`): queries 2–3 STUN servers; compares mapped addresses to classify as:
  - `cone` — different server, same mapped IP:port (full-cone, restricted, port-restricted)
  - `symmetric` — mapped port changes per destination (hard to traverse)
  - `unknown` — STUN failed or inconclusive
- Used by `DHTDiscovery` to decide relay routing eligibility and to populate the peer's `NATType` field in advertisements.

### 3. In-mesh gossip (`gossip.go`)

- `MeshGossip` runs a UDP server on `gossipPort + 1000` bound to the WireGuard interface address.
- Every 10 seconds, serialises all active peers from the peer store into a `GossipMessage` (JSON) and sends it to a random subset of peers.
- Inbound messages: deserialise, call `DiscoveryCallback` for each peer, tag source as `"gossip"` or `"gossip-transitive"` (if the gossip source is itself a peer, not the local node).
- No authentication — gossip is an in-mesh protocol, trusted by virtue of WireGuard encryption.

### 4. Peer exchange protocol (`exchange.go`)

The structured peer advertisement and rendezvous protocol run over TCP/TLS.

**Hello/reply:**
- `PeerExchange` server listens on `gossipPort` (TCP). Connections are authenticated via a shared TLS certificate derived from the mesh secret.
- On connect: send `PeerHello` (all known peers from peer store) → receive `PeerReply` → store peers.
- Bidirectional: both ends send peers to each other.

**Rendezvous:**
- A node can publish itself as a "rendezvous point" via the DHT. Two nodes that cannot reach each other directly both connect to the same rendezvous peer.
- The rendezvous peer relays connection setup (via `RendezvousRequest`/`RendezvousResponse`) allowing the two nodes to learn each other's external addresses.

**Hole-punching:**
- After rendezvous, both nodes perform simultaneous UDP hole-punching: fire UDP packets at each other's external address repeatedly while the other side does the same.
- On success, discovered address is delivered to peer store as `"dht-rendezvous"` source.

### 5. DHT peer discovery (`dht.go`)

BitTorrent Mainline DHT (BEP 5) adapted for mesh peer discovery.

**Announce loop (every 15min):**
- Publishes `(mesh_secret_hash → local endpoint info)` into the DHT.
- Signed with WireGuard private key. Verifiable by WireGuard public key included in the value.
- Persists known DHT nodes to a local file (`~/.config/wgmesh/<iface>-dht-nodes.json`) for faster bootstrap.

**Query loop (every 5min):**
- Looks up the same info hash; collects all peers advertising under it.
- Verifies signatures; delivers to peer store as `"dht"` or `"dht-transitive"` (if endpoint came transitively from another DHT peer).

**IPv6 sync:**
- If the local node has an IPv6 address, announces it separately and collects peers' IPv6 addresses in a secondary DHT lookup.

**STUN refresh:**
- Periodically re-runs STUN to detect external endpoint changes (common behind dynamic NAT).
- On change: re-announces immediately.

**Persistence:**
- DHT routing table nodes are serialised to JSON on shutdown and restored on startup for warm bootstrap.

### 6. DHT rendezvous + hole-punching (`dht.go` + `exchange.go`)

- When a peer cannot be reached directly (symmetric NAT or no endpoint), `DHTDiscovery` looks for a shared rendezvous node.
- Selects an introducer: a peer that both nodes have in common in their peer stores.
- Sends `RendezvousRequest` to the introducer over PeerExchange; the introducer forwards to the target.
- Both sides perform hole-punching; on success the endpoint is promoted to `"dht-rendezvous"` rank.

### 7. GitHub registry (`registry.go`)

Encrypted bootstrap rendezvous channel using GitHub Issues as a store.

- `RendezvousRegistry` posts encrypted peer advertisements as GitHub Issue comments.
- Encryption: envelope encryption using the shared mesh secret (no cleartext keys exposed).
- Poll interval: every 15 minutes; cleans own old comments (TTL ~1h).
- Delivers discovered peers to peer store as `"registry"` source.
- Used as a fallback when DHT cannot bootstrap (firewall, no UDP, first run).

---

## Startup flow

`DHTDiscovery.Start()` orchestrates in this order:

1. STUN: discover external endpoint.
2. Create `MeshGossip` (if `--gossip`).
3. Start `PeerExchange` server.
4. Start `LANDiscovery` (if enabled).
5. Start gossip loop.
6. Init DHT server (bootstrap from persisted nodes or fallback bootstrap peers).
7. Launch goroutines: `announceLoop`, `queryLoop`, `persistLoop`, `transitiveConnectLoop`, `stunRefreshLoop`.

---

## Existing specs

None found for `pkg/discovery/`.

---

## Planned sub-actions

| Sub-action | Files | Concern |
|---|---|---|
| C.1 | `lan.go`, `stun.go` | LAN multicast discovery + STUN/NAT detection |
| C.2 | `gossip.go` | In-mesh gossip broadcast |
| C.3 | `exchange.go` (hello/reply) | Peer exchange protocol — peer advertisement |
| C.4 | `exchange.go` (rendezvous/hole-punch) | Peer exchange rendezvous + hole-punching |
| C.5 | `dht.go` (announce/query/persist) | DHT announce, query, persistence, IPv6 sync |
| C.6 | `dht.go` (rendezvous) + `registry.go` | DHT rendezvous, hole-punching, GitHub registry |
