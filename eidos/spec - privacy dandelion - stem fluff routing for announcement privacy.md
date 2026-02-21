---
tldr: Dandelion++ routes peer announcements through a random stem of up to 4 hops before broadcasting; relay peers rotate every 10 minutes via a deterministic epoch-seeded shuffle so routing cannot be predicted across epochs.
category: core
---

# Privacy — Dandelion++ stem/fluff routing

## Target

`DandelionRouter` implements the Dandelion++ protocol for peer announcements, obscuring which
node originated a discovery advertisement by routing it through intermediate relays before
public broadcast.

## Behaviour

### Stem phase

An announcement begins in stem phase (`HopCount = 0`).
At each hop, `HandleAnnounce(msg)`:

1. Increment `HopCount` (capped at `MaxStemHops = 4` to prevent uint8 overflow from
   malformed messages).
2. Call `ShouldFluff(hopCount)`:
   - Force fluff if `hopCount >= 4`.
   - Otherwise: 10% probability via `crypto/rand` (not `math/rand` — unpredictable).
   - On `crypto/rand` failure: default to fluff (fail-safe for privacy).
3. **If fluff:** call `onFluff(msg)` — the announcement is broadcast publicly.
4. **If stem:** select relay = `epoch.RelayPeers[hopCount % len(relayPeers)]` and call
   `onStem(msg, relay)`.
   If no relay peers are available: fluff immediately.

### Fluff phase

`onFluff` is set by the caller — it broadcasts the announcement to all reachable peers.
This is where the announcement enters the normal discovery pipeline.

### Epoch — relay peer rotation

Relay peers change every 10 minutes (epoch duration).
`RotateEpoch(allPeers)`:

1. Increment epoch ID.
2. Derive a deterministic per-epoch seed:
   `HMAC-SHA256(epochSeed, epochID_bigendian_8bytes)`
3. Sort all peers lexicographically by WireGuard public key.
4. Deterministic Fisher-Yates shuffle seeded with the first 8 bytes of the HMAC output.
5. Take the first 2 peers as relay peers for this epoch.

`EpochRotationLoop(ctx, getPeers)`: background goroutine that calls `RotateEpoch` every
10 minutes (immediately on start). Stopped by context cancellation.

### Announcement format (`DandelionAnnounce`)

- `OriginPubkey`, `OriginMeshIP`, `OriginEndpoint`, `RoutableNetworks` — origin identity.
- `HopCount uint8` — current hop count (incremented at each relay node).
- `Timestamp int64` — Unix epoch of creation.
- `Nonce []byte` — 16-byte random value per announcement (prevents correlation via message equality).

`CreateAnnounce(pubkey, meshIP, endpoint, routableNetworks)` constructs a new announcement
with `HopCount = 0` and a fresh 16-byte `crypto/rand` nonce.

## Design

- The 10% fluff probability means an announcement traverses on average ~10 hops before
  broadcasting, but the forced-fluff at 4 hops bounds the maximum path length.
  The observer sees the announcement enter the network only at the fluff broadcaster,
  not at the true origin.
- Relay selection is deterministic per `(epochSeed, epochID)` — nodes with the same shared
  secret and epoch ID will derive the same relay set. Epoch IDs are local counters;
  they are not synchronized across nodes, so relay paths are not globally coordinated.
  This is intentional: the epoch prevents an adversary from predicting the stem path across
  epoch boundaries, while the determinism ensures consistent routing within an epoch.
- `ShouldFluff` uses `crypto/rand` (not `math/rand`) to make the fluff decision
  unpredictable to an adversary who can observe the local node's state.
- The nonce in each announcement ensures two identical announcements from the same origin
  cannot be correlated by content (each relay retransmits a distinct message).
- Handlers (`onFluff`, `onStem`) are injected callbacks — `DandelionRouter` is agnostic to
  the transport layer; the daemon's `EpochManager` wires it to the WireGuard mesh.

## Interactions

- `pkg/crypto.DerivedKeys.EpochSeed` — seed for epoch relay derivation.
- `pkg/daemon.EpochManager` — owns the `DandelionRouter`, calls `RotateEpoch` periodically,
  exposes `GetRouter()` for the reconcile loop.
- Reconcile loop — uses router to select relay peers for symmetric-NAT peers.
- `onFluff` callback — wired to the peer store update path for normal announcement delivery.
- `onStem` callback — wired to relay the announcement to the selected peer's mesh endpoint.

## Mapping

> [[pkg/privacy/dandelion.go]]
