---
tldr: Peer store events trigger rendezvous attempts for unreachable peers; introducers are selected by explicit flag or auto-detection, using synchronized time windows to avoid collision; GitHub Issues provides an encrypted fallback bootstrap channel.
category: core
---

# Discovery — DHT rendezvous, transitive connect, and GitHub registry

## Target

Two mechanisms that fill the gap when DHT query-based discovery is insufficient:
(1) the event-driven transitive connect loop in `DHTDiscovery` that reacts to peer store events
and orchestrates rendezvous through introducers; and
(2) `RendezvousRegistry` — a GitHub-Issues-backed bootstrap channel for initial discovery.

## Behaviour

### Transitive connect loop

`transitiveConnectLoop` runs as a goroutine for the lifetime of `DHTDiscovery`.

- **Event-driven**: subscribes to peer store `PeerEvent` (new / updated); reacts immediately
  when a peer appears or changes state.
- **Stale handshake check**: also polls every 10 seconds for peers with stale WireGuard
  handshakes that may need a rendezvous refresh.

On each trigger, calls `tryRendezvousForPeer(peer)`:

1. **Skip if fresh**: peer has a WireGuard handshake within the past 2 minutes → no action.
2. **IPv6 path**: if peer endpoint is IPv6 and local node has an IPv6 route, use
   `shouldAttemptIPv6Sync` (8s window / 2s phase) to schedule a direct exchange attempt.
   Both peers fire in the same 2-second slot every 8 seconds (FNV-hash-derived offset),
   reducing simultaneous-open races.
3. **Select introducers** (`selectRendezvousIntroducers`): up to 3, from active peers that:
   - Have a reachable public control endpoint (gossipPort on their WireGuard endpoint IP)
   - Have been reached via DHT (`DiscoveredVia` contains a `dht*` method)
   - Are either explicitly flagged as `Introducer = true` OR auto-detected:
     auto-detection requires: control endpoint known, WireGuard handshake within 2 minutes.
   - Explicit introducers are sorted first; within a tier, sorted lexicographically for stability.
   - Starting index: `FNV(local_pubkey, remote_pubkey) % len(candidates)` — deterministic,
     distributes load across multiple available introducers.
4. **If introducers available**: send `RequestRendezvous` to each selected introducer.
   The introducer relays the rendezvous to the target (see peer exchange spec).
5. **If no introducer, direct route available**: call `ExchangeWithPeer` directly + wait up
   to 10 seconds for a WireGuard handshake above the pre-punch baseline.

### Rendezvous timing window (`shouldAttemptRendezvous`)

To avoid both ends triggering rendezvous simultaneously with uncoordinated intervals:

- Window: 20 seconds, phase: 4 seconds.
- Offset: `FNV(pair) % 20` — each pair has a unique time slot in the 20-second cycle.
- A rendezvous attempt is allowed only when `(now.Unix() % 20) - offset < 4`.
- Result: both nodes fire within the same 4-second window every 20 seconds.

### Exponential backoff

Per-peer backoff state tracks consecutive failures:
- First failure: 3-second backoff.
- Each subsequent failure: doubles, capped at 30 seconds.
- Success (WG handshake established): backoff cleared.
- `canAttemptRendezvous(pubKey)` is checked before any attempt.

---

### GitHub Registry (`RendezvousRegistry`)

A bootstrap channel for peers that have never heard of each other (no DHT, no LAN, first run).

**Storage**: GitHub Issues in a well-known public repo (`wgmesh-registry/public`).

**Issue identity**: title = `wgmesh-<RendezvousID hex>` derived from the mesh secret.
Each mesh network gets a unique title; no title collision between different meshes.

**Content**: issue body contains human-readable text plus
`<!-- PEERS: <encrypted_blob> :PEERS -->` markers.
The blob is `crypto.SealEnvelope(MessageTypeAnnounce, announcement, gossipKey)` —
a standard `PeerAnnouncement` with the local node as the main entry and up to 49 other peers
in `KnownPeers`. Max 50 peers total per issue.

**On `FindOrCreate(myInfo)`**:
1. Search GitHub Issues API for the title (unauthenticated — read-only rate limit applies).
2. Decrypt and return discovered peers from the issue body.
3. If no issue found + `GITHUB_TOKEN` set: create a new issue with own info.
4. If issue found + `GITHUB_TOKEN` set: PATCH the issue body, merging own info with
   existing peers (deduplication by WireGuard public key).
5. Read-only mode: if `GITHUB_TOKEN` is unset, search only.

**Auth**: `GITHUB_TOKEN` is required for create/update; search is unauthenticated.
**Encryption**: gossip key encrypts the peer list — an observer cannot read peer info
without the shared mesh secret, even though the GitHub issue is in a public repo.

## Design

- Event subscription ensures rendezvous reacts within milliseconds of peer discovery,
  rather than waiting for the next fixed poll interval.
- The 10-second stale handshake check is a safety net for peers whose events were missed
  or whose WireGuard tunnel degraded after initial connection.
- Auto-detected introducers (vs. explicit `Introducer` flag) allow any reachable, recently
  handshaked peer to relay — no manual configuration required for fully-connected meshes.
- The pair-hash window sync ensures that even if both ends simultaneously discover each other
  as unreachable, they converge on the same rendezvous slot rather than repeatedly missing.
- GitHub Issues provides persistence across restarts without infrastructure: the encrypted blob
  survives as long as the issue is open, acting as a shared bulletin board.

## Interactions

- `PeerExchange.RequestRendezvous` — send rendezvous offer to introducer.
- `PeerExchange.ExchangeWithPeer` — direct punch attempt.
- `pkg/daemon.PeerStore.Subscribe` / `Unsubscribe` — peer event subscription.
- `pkg/daemon.PeerStore.GetActive` — stale handshake check source.
- `pkg/wireguard.GetLatestHandshakes` — WG handshake timestamp queries.
- `pkg/daemon.PeerStore.Update(peer, "dht-rendezvous")` — store successfully punched peers.
- `pkg/crypto.DerivedKeys.RendezvousID` — GitHub issue title derivation.
- `pkg/crypto.SealEnvelope` / `OpenEnvelope` — registry encryption/decryption.

## Mapping

> [[pkg/discovery/dht.go]]
> [[pkg/discovery/registry.go]]
