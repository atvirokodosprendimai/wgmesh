---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Thread-safe in-memory registry of discovered peers; merge semantics prefer higher-trust endpoints; subscribers receive new/updated events without blocking the store.
category: core
---

# Peer store

## Target

The runtime registry of all discovered mesh peers, shared between discovery, reconciliation, and health subsystems.

## Behaviour

- **Entry point (ADR-0002):** every production claim reaches the store through `Offer(Candidate) Decision`. A `Candidate` is the claimed `PeerInfo`, a typed `Source` (`lan`, `dht`, `dht-rendezvous`, `dht-ipv6-sync`, `gossip`, `cache`) and a typed `Hand`: `FirstHand` (the sender describes itself), `SecondHand` (a `KnownPeers` mention) or `Restored` (the on-disk cache). Trust is read from `Hand`, never from the method string. The decision (`Decide`) is made under the store lock and returns `Admit`, `AdmitPartial` (route entries listed in `Dropped`) or `Refuse`, with a `Reason` and the `Warnings` of rules that would act once enforced. `TestNoDirectPeerStoreMutation` (`pkg/node/admit_guard_test.go`) fails when a production file outside `pkg/node` calls `Update`, `SetPeerDirectly` or `Remove` on a receiver named `peerStore`, `PeerStore`, `ps` or `store`.
- **Update:** `Update(info, discoveryMethod)` stays exported for tests and fixtures and forwards to `Offer`; the merge rules below are the merge step of `Offer`.
- Peers are keyed by WireGuard public key. All operations are thread-safe.
- The merge step folds an accepted candidate into the existing record:
  - Endpoint: updated only if the new source has higher or equal rank (see Endpoint ranking below).
  - RoutableNetworks, MeshIP, MeshIPv6, Hostname: last non-empty value wins.
  - Introducer flag: always overwritten by the latest announcement (a node can stop being an introducer).
  - NATType: last non-empty value wins.
  - DiscoveredVia: accumulates all methods used to find this peer (no duplicates).
  - LastSeen: refreshed on direct discovery; not refreshed for cache restores or transitive methods.
- New peer insertions are rejected when the store holds 1000 peers (flood protection). Updates to existing peers are always allowed through.
- **Withdraw and the tombstone:** `Withdraw(key, cause)` removes a peer and leaves a tombstone for `TombstoneTTL` (5 minutes, `pkg/node/store.go`). While the tombstone lasts, `SecondHand` and `Restored` offers for that key are refused (a mention cannot bring a withdrawn peer back); the peer's own `FirstHand` offer clears it at once. Causes: `goodbye` (GOODBYE message), `evicted` (health eviction), `stale` (stale cleanup, written only for a peer that was heard first-hand). A key that is not in the store leaves no tombstone. The tombstone set is in memory (at most `DefaultMaxPeers` keys), so a restart forgets it; a first-hand HELLO after a GOODBYE still re-admits the peer (known limit, `docs/adr/BACKLOG.md`).
- **Claim rules in `Decide`:** a first-hand claim for a mesh IP held by a bound claim of another key, and route entries inside the mesh network or equal to a member's mesh IP, are counted as warnings in release N and refused or dropped only when `EnforceIPBinding` or `EnforceRouteBinding` is set (both `false`); the `accept-routes` list of the operator (`Policy.AcceptRoutes`) judges first-hand and restored routes; a network it would drop is counted as `route-not-approved` in N and dropped only when `EnforceRouteBinding` is set (N+2). See `docs/admit-rollout.md`.
- **Dead timeout:** 5 minutes without an update → peer considered dead; excluded from `GetActive()`.
- **Remove timeout:** 10 minutes without an update → removed from store by the stale cleanup loop.
- **Liveness (ADR-0004):** `Assess(Evidence, now)` judges a peer by the newest of four times: `LastSeen` (a claim heard first-hand), `LastDataPlane` (measured by this node: a WireGuard handshake or a successful mesh probe, recorded through `ObserveDataPlane`; it never moves backwards and never touches `LastSeen` or a claim field), `RestoredAt` (set when the cache restores the entry) and the store-wide wake time (`NoteResume`, stamped into every copy the store returns). Younger than `PeerDeadTimeout` is `Alive`, up to `PeerRemoveTimeout` is `Suspect`, older or no evidence is `Dead`. `PeerInfo.Alive(now)` is that verdict; `PeerInfo.LastContact()` is the newest of `LastSeen` and `LastDataPlane` only (measured contact, written to the cache file as `last_seen`).
- **Switch:** `node.EnforceLiveness` (an atomic, `false` at release N) makes the verdict act in `GetActive`, `CleanupStale` and `IsDead`; while it is `false` the legacy `LastSeen` age decides and each case where the legacy test drops a peer that `Assess` keeps is reported to the hook of `SetLivenessDisagreeHook` (sites `get-active`, `cleanup`, `is-dead`; `wgmesh_liveness_disagree_total{site}`). `GetProbeable()` returns the `Alive` and `Suspect` peers and always judges by `Assess`. See `docs/liveness-rollout.md`.
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
| 10 | Guessed endpoint (sender IP and the default WireGuard port, nothing advertised) |

A guessed endpoint (`PeerInfo.EndpointGuessed`) is replaced by the first advertisement from any source, is not announced in `KnownPeers` and is not written to the peer cache.

At equal rank: IPv6 endpoint preferred over IPv4.
LAN endpoint is sticky — only a newer LAN discovery can replace it.

## Design

- `Get` returns a copy of the peer struct to prevent callers from mutating store state.
- Notification is decoupled from the write lock: subscriber snapshot is taken under read lock, sends happen after lock release.
- `shouldUpdateEndpoint` encodes the ranking and IPv6 preference as a pure function, making the merge policy testable independently of the store.

## Interactions

- Discovery layers (DHT, LAN, gossip, exchange) and the peer cache — call `Offer`; GOODBYE and health eviction call `Withdraw`.
- Reconciliation loop — calls `GetActive()` each cycle.
- Health monitor — calls `GetActive()` and `Withdraw()`.
- Mesh probe and reconciliation — call `ObserveDataPlane` (a handshake or a pong); the mesh probe loop reads `GetProbeable()`; the resume loop calls `NoteResume`.
- Peer cache — calls `GetAll()` for serialisation; offers entries as `Restored` on restore.
- Stale cleanup loop — calls `CleanupStale()` every minute.
- Daemon — `SetPolicy(Policy)` before discovery starts; `Policy()` returns it to the collision code.

## Mapping

> [[pkg/daemon/peerstore.go]]
