---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Two independent health signals — WireGuard handshake staleness and TCP mesh probes — detect unreachable peers and either reconnect or evict them with a 30-second cooldown.
category: core
---

# Daemon health monitoring

## Target

The runtime health monitoring that detects unresponsive peers and removes them
from the active pool until they become reachable again.

## Behaviour

Two independent signals run on separate intervals:

### Signal 1 — WireGuard handshake & transfer monitor (every 20s)

- Reads live WireGuard handshake timestamps and cumulative transfer bytes per peer.
- A peer is **stale** when: last handshake is >150s ago AND transfer counters have not increased since the previous check.
  - Transfer growth means the WireGuard data plane is moving bytes even without a new handshake — not stale.
- On first stale detection: attempt reconnect (re-issue `wg set` with current endpoint to trigger key re-exchange). Clears the optimistic signature cache so the next reconcile will re-apply the peer.
- On second consecutive stale detection: evict the peer.
- `checkPeerHealth` reads the handshakes and transfers from WireGuard and passes them to `evaluatePeerHealth`, which takes the decisions of one cycle.
- A relay-routed peer is not judged: it has no handshake of its own, a reconnect would give it its mesh IP and take the address from the relay, and an eviction would drop a peer the relay serves. Its failure counters are cleared; `attemptPeerReconnect` skips it too. This holds whatever `daemon.EnforceRelayProbe` is.
- Inside `ResumeGrace` (60 s) after a wake the handshake age includes the time asleep: the peer is not treated as stale, so neither the reconnect nor the eviction happens. The grace blocks only while `node.EnforceLiveness` is set; otherwise the case is counted as `resume-grace`.

### Signal 2 — TCP mesh probe (every 1s)

- Sends `ping\n` over a persistent TCP connection to the peer's mesh IP at `gossipPort + 2000`.
  Listens on the same port for incoming probes. Probes are bound to the WireGuard interface via `SO_BINDTODEVICE` (Linux) — traffic must pass through the mesh tunnel.
- Sessions are persistent (reused across probe cycles); reconnected lazily on failure.
- A probe is not enforced for brand-new peers (< 45s old) unless they are relay-routed or WireGuard has handshake data for them.
- 8 consecutive probe failures → evict the peer.
- If WireGuard reports a recent handshake, probe failures are cleared and the probe session is closed (no need to probe a working peer).
- The probe loop visits `GetProbeable()` (Alive and Suspect peers); a Suspect peer is visited only while it is configured on the interface or relay-routed. A successful probe is recorded as `LastDataPlane` (`ObserveDataPlane`). The 8-failure eviction is skipped inside the wake grace (failures restart from 0) when `EnforceLiveness` is set.

### Eviction

When a peer is evicted:
1. Marked temporarily offline for 30 seconds — excluded from reconcile, route sync, and probe cycles.
2. Removed from peer store.
3. Removed from WireGuard (`wg set peer … remove`).
4. Cleared from relay routes, applied-config cache, health failure counters, probe session.

After 30 seconds the temporary-offline entry expires and the peer can be re-discovered and re-added by any discovery layer.

## Design

- Both health signals are **independent** — either can trigger eviction regardless of the other.
- The probe uses IPv6 mesh address when available (preferred over IPv4 at equal rank).
- `isTemporarilyOffline` is checked with an inline expiry cleanup — no background sweep needed.
- WG handshake takes precedence over probe: a live handshake clears probe failures immediately, because the WireGuard data plane is the ground truth.

## Interactions

- `wireguard.GetLatestHandshakes`, `wireguard.GetPeerTransfers` — WG health signal data.
- `wireguard.SetPeer`, `wireguard.RemovePeer` — reconnect and eviction.
- `PeerStore.GetActive`, `PeerStore.Remove` — read active set, remove evicted peers.
- Reconciliation loop — respects `isTemporarilyOffline` in `planPeers` (which records the handshakes as `LastDataPlane`, takes `GetActive` and builds the desired set in `buildDesiredPeerConfigsWithHandshakes`) and `syncPeerRoutes`.

## Mapping

> [[pkg/daemon/daemon.go]]
