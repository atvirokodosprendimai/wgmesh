---
tldr: PeerExchange owns the gossipPort UDP socket; runs HELLO/REPLY peer advertisement with peer-as-STUN reflection; acts as introducer for synchronized rendezvous; hole-punches by retrying HELLO at 100ms until reply arrives.
category: core
---

# Discovery — peer exchange protocol

## Target

The `PeerExchange` server: a single UDP socket shared by all exchange message types (HELLO, REPLY,
ANNOUNCE, RENDEZVOUS_OFFER, RENDEZVOUS_START, GOODBYE) plus the DHT layer.
Handles both direct peer advertisement and introducer-mediated rendezvous.

## Behaviour

### Transport

- Listens on `gossipPort` over **UDP** (same port number as PeerExchange's own control protocol
  and as DHT — the DHT layer reuses the same `UDPConn`).
- All messages are envelope-encrypted with the mesh gossip key.
  Non-decodable packets (wrong key, DHT protocol messages) are logged and discarded.
- Inbound rate-limited per source IP before per-message processing.
- Each inbound message dispatched to its handler in a new goroutine.

### Hello / Reply — direct peer advertisement

`ExchangeWithPeer(addr)` initiates an exchange:
1. Sends `HELLO` (local peer info + full known-peers list) to the target address.
2. If hole-punching is not disabled: retransmits HELLO every 100ms (acting as a simultaneous
   open attempt) until a REPLY arrives or 4 seconds elapse.
   If punching is disabled: send once, wait 4 seconds.
3. On reply received: return the peer info to the caller.

On receiving a HELLO:
1. Update peer store with sender info (`"dht"` source).
2. Store transitive peers from `KnownPeers` (`"dht-transitive"`).
3. Send REPLY — includes own peer info, known-peers list, and `ObservedEndpoint` (the sender's
   IP:port as seen by this node = peer-as-STUN reflector).

On receiving a REPLY:
1. Extract `ObservedEndpoint` and apply it to `localNode.WGEndpoint`
   — only the IP portion is taken; the WireGuard listen port is used for the port component.
   Won't overwrite an existing global IPv6 endpoint with an IPv4 reflection.
   Won't apply private/loopback addresses.
2. Update peer store with sender info (`"dht"`).
3. Route to the pending reply channel if a concurrent `ExchangeWithPeer` is waiting.

### Goodbye

- `SendGoodbye(addr)` sends a signed shutdown notification with a current timestamp.
- Receivers validate the timestamp (within ±60 seconds of now) to prevent replay attacks.
- On valid GOODBYE: immediately remove the peer from the store.

### Introducer rendezvous (for symmetric NAT traversal)

When two nodes cannot reach each other directly (e.g. both behind symmetric NAT), a third
node (introducer) coordinates synchronized hole-punching.

**Requesting rendezvous (`RequestRendezvous`):**
- Node A sends a `RENDEZVOUS_OFFER` to the introducer, naming its target peer B and providing
  its candidate endpoints.
- `PairID` = deterministic hash of (A pubkey, B pubkey) — symmetric, order-independent.

**Introducer processing (`handleRendezvousOffer`):**
- Accumulates both peers' offers for a pair ID (session TTL 20s).
- If B's offer hasn't arrived yet but B is in the peer store, synthesizes B's offer from
  its stored endpoint (control port = gossipPort).
- Once both offers are present: computes `startAt = now + 1800ms` and sends `RENDEZVOUS_START`
  to both participants simultaneously — each message carries the other peer's candidates.
- Port spread: each candidate address expanded ±2 ports (5 candidates total per base candidate)
  to handle port-sequencing NATs.
- Start cooldown: 8 seconds between starts for the same pair.
- Only nodes with `Introducer = true` act as rendezvous relays.

**Executing hole-punch (`handleRendezvousStart` → `runRendezvousPunch`):**
- On receiving `RENDEZVOUS_START`, validate pair ID matches local node pair.
- Sleep until `startAt` (synchronized with other side).
- For each candidate: call `ExchangeWithPeer(candidate)` — simultaneous retransmission from
  both sides creates the necessary firewall state entries.
- Wait up to 10 seconds for a WireGuard handshake above the pre-punch baseline.
- On success: update peer store as `"dht-rendezvous"` (high-priority endpoint rank).
- Punch cooldown: 6 seconds per pair.

### ANNOUNCE dispatch

`ANNOUNCE` messages (used by gossip in exchange-integrated mode) are forwarded to the
`announceHandler` callback if set; not stored by PeerExchange itself.

### Socket multiplexing

`PeerExchange.UDPConn()` exposes the UDP connection so DHT can share the same port/socket.
DHT messages will fail decryption and are silently discarded by PeerExchange.

## Design

- HELLO retransmission every 100ms is simultaneously hole-punching: both sides send to each
  other's external address at the same time, which opens the NAT pinhole on both sides.
  No separate punch mechanism is needed for direct (non-rendezvous) connections.
- Peer-as-STUN reflector: the REPLY `ObservedEndpoint` field lets the HELLO sender learn its
  external IP without dedicated STUN infrastructure once at least one peer has been contacted.
  IP only — WG port is preserved from local config to avoid NAT port-translation confusion.
- `PairID` is order-independent (FNV hash of sorted public keys) so A→B and B→A offers map
  to the same session slot on the introducer.
- Intro synthesis: if B hasn't sent its own offer, the introducer constructs one from the peer
  store — this unblocks one-sided rendezvous (A retries, B hasn't started yet).

## Interactions

- `pkg/crypto.SealEnvelope` / `OpenEnvelopeRaw` — encrypt/decrypt all messages.
- `pkg/daemon.PeerStore.Update` — direct (`"dht"`), transitive (`"dht-transitive"`),
  rendezvous (`"dht-rendezvous"`).
- `pkg/daemon.PeerStore.Remove` — goodbye handling.
- `pkg/wireguard.GetLatestHandshakes` — wait for handshake after hole-punch.
- `MeshGossip.SetAnnounceHandler` / `gossip.HandleAnnounceFrom` — announce integration.
- `DHTDiscovery` — shares the UDP socket and calls `ExchangeWithPeer` for transitive connects.

## Mapping

> [[pkg/discovery/exchange.go]]
