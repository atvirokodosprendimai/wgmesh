# Pull — daemon package overview (multi-pass)

**Source:** `pkg/daemon/`
**Date:** 2026-02-21
**Mode:** overview (subsection pulls planned separately)

---

## Territory Map

The daemon package implements the **decentralized mode** of wgmesh: a long-running process that discovers peers, maintains WireGuard configuration, monitors connectivity, and manages the node's lifecycle without an operator present.

### Files

| File | Lines | Concern |
|---|---|---|
| `daemon.go` | 1700 | Core lifecycle, reconciliation loop, relay routing, health + probe |
| `peerstore.go` | 374 | Thread-safe peer registry with subscription/notification |
| `helpers.go` | 384 | Cross-platform WireGuard interface management (Linux + macOS) |
| `config.go` | 200 | Daemon config, secret URI parsing, hot-reload file |
| `systemd.go` | 208 | Systemd unit generation, install/uninstall, secret env file |
| `cache.go` | 158 | Peer cache persistence to `/var/lib/wgmesh/` |
| `routes.go` | 137 | Kernel route sync for routable networks, relay-aware gateways |
| `collision.go` | 136 | Deterministic mesh IP collision detection + nonce re-derivation |
| `epoch.go` | 47 | EpochManager wrapping `pkg/privacy.DandelionRouter` |
| `executor.go` | 92 | `CommandExecutor` interface for testable `os/exec` calls |

---

## Major Subsections

### 1 — Core lifecycle
`Daemon` struct, `NewDaemon`, `Run`, `Shutdown`, `initLocalNode`, `setupWireGuard`, `teardownWireGuard`, signal handling (SIGINT/SIGTERM/SIGHUP), `handleSIGHUP`.

The daemon derives its identity (WireGuard keypair + mesh IP) from a shared secret via `pkg/crypto`.
Local node state is persisted to `/var/lib/wgmesh/<iface>.json`.
Startup: create WG interface → configure keys + address → start goroutines → wait for signal.
SIGINT/SIGTERM: graceful shutdown via context cancellation + WaitGroup.
SIGHUP: hot-reload of `advertise-routes` and `log-level` from `/var/lib/wgmesh/<iface>.reload` under `configMu` write lock.

### 2 — Reconciliation loop
`reconcileLoop` (every 5s), `reconcile`, `buildDesiredPeerConfigs`, `applyDesiredPeerConfigs`.

Reads active peers from `PeerStore` → computes desired WG peer configs (AllowedIPs = mesh /32 + routable networks) → diffs against live WG peers (`wg show` + `wg set`) → applies adds/removes/updates.
Relay routing: if a peer is behind symmetric NAT or WG handshake is stale, route traffic via an introducer relay node (`wg set ... allowed-ips` via relay's mesh IP, kernel route via relay).
`shouldRelayPeer` and `selectRelayForPeer` encode the relay selection logic.

### 3 — Peer store
`PeerStore`, `PeerInfo`, endpoint ranking, pub/sub.

In-memory map keyed by WireGuard public key. Thread-safe (RWMutex).
Merge semantics: newest info wins; endpoint updates follow a priority ranking (LAN > DHT rendezvous > DHT > gossip > cache). IPv6 preferred at equal rank.
Subscribers receive `PeerEvent` (new/updated) on a buffered channel; notifications sent outside the lock to prevent deadlock.
Peer lifecycle: dead after 5 min without update; removed from store after 10 min (stale cleanup).
Cap: 1000 peers (flood attack protection). New peers rejected at cap; existing updates always allowed.
`DiscoveredVia`: accumulates discovery methods used to find each peer.

### 4 — Health monitoring & mesh probing
`healthMonitorLoop` (every 20s), `meshProbeLoop` (every 1s), `checkPeerHealth`, `probePeer`.

Two independent health signals:
- **WG handshake + transfer check**: reads `wg show latest-handshakes` and `wg show transfer`; marks peer stale if no handshake for >150s AND no transfer increase. Stale → reconnect attempt; stale twice → evict from pool.
- **TCP mesh probe**: persistent TCP connections over the WireGuard mesh IP (port = WG listen port + 2000), bound to the WG interface via `SO_BINDTODEVICE`. Probe failures up to 8 consecutive → mark peer as temporarily offline for 30s.

### 5 — IP collision resolution
`collision.go`: `DetectCollisions`, `DeterministicWinner`, `ResolveCollision`, `DeriveMeshIPWithNonce`.

Mesh IPs are derived deterministically from shared secret + public key. Two peers may derive the same IP.
Winner = lower public key (lexicographic). Loser re-derives with nonce=1 appended to the hash input.
Collision check is run against the peer store; if local node loses, it reconfigures its own WG interface address immediately.

### 6 — Epoch management / Dandelion++
`epoch.go`: thin wrapper over `pkg/privacy.DandelionRouter`.
`EpochManager` starts a rotation goroutine that periodically re-selects relay peers for privacy (Dandelion++ stem routing). The rotation epoch is derived from the shared secret's mesh subnet bytes.

### 7 — Peer cache
`cache.go`: persists peer store to `/var/lib/wgmesh/<iface>-peers.json` every 5 minutes and on shutdown.
On startup, cached peers (not expired after 24h) are restored into the peer store via the "cache" discovery method.
Allows reconnect after restart without waiting for full DHT/gossip rediscovery.

### 8 — Config & hot reload
`config.go`: `Config` struct derived from `DaemonOpts` + `pkg/crypto.DeriveKeys`.
Secret accepted as raw string or `wgmesh://v1/<secret>` URI.
`LoadReloadFile` parses `/var/lib/wgmesh/<iface>.reload` (KEY=VALUE format) for live updates: `advertise-routes` and `log-level`.

### 9 — WireGuard interface helpers
`helpers.go`: cross-platform (Linux/macOS) WireGuard interface lifecycle: `createInterface`, `configureInterface`, `setInterfaceAddress`, `setInterfaceUp/Down`, `deleteInterface`, `resetInterface`.
Linux: `ip link` + `ip addr` + `wg set`. macOS: `wireguard-go <utun>` + `ifconfig` + `route`.
Also: `loadLocalNode`/`saveLocalNode` (JSON state file), `isPortInUse`/`findAvailablePort`.

### 10 — Route management
`routes.go`: `syncPeerRoutes` — computes desired kernel routes from active peers + relay table, diffs against live `ip route show dev <iface>`, applies adds/removes.
Relay-aware: if peer has a relay route, gateway = relay's mesh IP (not peer's mesh IP directly).
Also installs iptables FORWARD rule for relay traffic: `iptables -A FORWARD -i <iface> -o <iface> -j ACCEPT`.

### 11 — Systemd integration
`systemd.go`: generates systemd unit file from config, writes `/etc/wgmesh/secret.env` with mode 0600 (secret kept out of process list), installs/enables/starts `wgmesh.service`. Security hardening: `NoNewPrivileges`, `ProtectSystem=full`, `ProtectHome`.

---

## How the Subsections Relate

```
    Config + Secret
         |
    initLocalNode  → /var/lib/wgmesh/<iface>.json
         |
    setupWireGuard → helpers.go (cross-platform interface ops)
         |
    goroutines:
      ├── reconcileLoop (5s) ←── PeerStore ←── Discovery (DHT/LAN/gossip)
      │         │
      │         └── buildDesiredPeerConfigs ←── EpochManager (relay selection)
      │                   │
      │                   └── applyDesiredPeerConfigs → wg set (wireguard.SetPeer)
      ├── syncPeerRoutes ←── PeerStore + relay table → ip route replace
      ├── healthMonitorLoop (20s) → checkPeerHealth → wg show handshakes/transfers
      ├── meshProbeLoop (1s) → TCP probe over WG mesh IP → temporaryOffline map
      ├── staleCleanupLoop (1m) → PeerStore.CleanupStale
      ├── cacheSaver (5m) → /var/lib/wgmesh/<iface>-peers.json
      └── statusLoop (30s) → log active peers
```

---

## Intent Sketch (overview level)

- The daemon is the decentralized runtime: it joins the mesh autonomously, discovers peers via pluggable discovery layers, and keeps WireGuard configuration in sync without operator involvement.
- Identity is derived from a shared secret — no pre-shared key exchange, no central server.
- The reconciliation loop is the heartbeat: it reads the peer store and makes WireGuard match it every 5 seconds.
- Two health signals work independently: WireGuard handshake staleness and TCP probe reachability.
  When a peer is unreachable, the daemon routes via an introducer relay.
- Peer state survives restarts via a local cache; full rediscovery is not required on every boot.
- Hot-reload (SIGHUP) allows changing advertised routes and log level without restarting the process.

---

## Existing Specs

None in `eidos/` cover this package.
Older `specs/issue-181-spec.md` touches daemon status output formatting (a feature spec, not a structural one).
