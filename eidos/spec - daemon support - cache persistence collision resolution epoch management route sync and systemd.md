---
tldr: Five support subsystems: peer cache survives restarts; collision resolution re-derives IPs deterministically; epoch manager rotates Dandelion++ relays; route sync tracks relay-aware gateways; systemd integration installs the daemon as a hardened service.
category: core
---

# Daemon support subsystems

## Target

The five supporting mechanisms that complement the daemon core: persistence, IP uniqueness, privacy relay rotation, kernel route management, and OS service integration.

## Behaviour

### Peer cache

- The peer store is serialised to `/var/lib/wgmesh/<iface>-peers.json` every 5 minutes and on clean shutdown.
- On startup, cached entries not older than 24 hours are restored into the peer store via the `"cache"` discovery method.
  This allows the node to reconnect to known peers without waiting for a full DHT/gossip rediscovery cycle.
- Cache restores do not update `LastSeen` — the peer must be re-confirmed by a live discovery source to be treated as active.

### Mesh IP collision resolution

- Mesh IPs are derived deterministically from `(shared secret, WireGuard public key)`. Two peers can independently derive the same IP.
- **Detection:** scan the peer store for duplicate IPs held by different public keys.
- **Resolution:** the peer with the lexicographically lower public key is the winner and keeps its IP. The loser re-derives with a nonce appended to the hash input (`nonce=1`).
- If the **local node** is the loser: immediately reconfigure the WireGuard interface address with the new IP.
- If a **remote peer** is the loser: record the expected new IP (the remote will self-correct on its next cycle).
- Collision check runs at the end of every reconcile cycle.

### Epoch management (Dandelion++)

- `EpochManager` wraps `pkg/privacy.DandelionRouter` and starts a rotation goroutine that periodically re-selects relay peers for Dandelion++ stem routing.
- The epoch seed is the mesh subnet bytes from `pkg/crypto.DerivedKeys`.
- Epoch rotation runs independently of reconciliation; relay peer selection is available to the reconcile loop via `GetRouter()`.

### Route sync

- After each WireGuard peer sync, kernel routes are reconciled for all peers' routable networks.
- **Relay-aware:** if a peer is relay-routed, its network gateway is set to the relay's mesh IP (not the peer's mesh IP).
- Skips peers that are temporarily offline.
- Only runs on Linux (no-op on other platforms).
- Installs an iptables `FORWARD ACCEPT` rule for the WireGuard interface to allow relay traffic to pass through.
- `sysctl net.ipv4.ip_forward=1` is set after each route apply.

### Systemd integration

- Generates a systemd unit file from daemon options. The secret is stored in `/etc/wgmesh/secret.env` (mode 0600) and referenced as `${WGMESH_SECRET}` — it never appears in the process list.
- Unit hardening: `NoNewPrivileges=yes`, `ProtectSystem=full`, `ProtectHome=true`, `ReadWritePaths=/var/lib/wgmesh`.
- `InstallSystemdService`: writes unit + secret env, creates `/var/lib/wgmesh` (required by `ReadWritePaths`), runs `systemctl enable + start`.
- `UninstallSystemdService`: stops, disables, removes unit and secret files.

## Design

- Cache file path: `/var/lib/wgmesh/<iface>-peers.json` (state directory, not config directory).
- Collision nonce: only nonce=1 is tried; the scheme relies on re-derivation producing a non-colliding IP in the vast majority of cases.
- `CommandExecutor` interface (`executor.go`) wraps `os/exec` — injected globally (`cmdExecutor`), replaceable with a mock for testing. All `systemd.go` and `routes.go` shell-outs use this interface.

## Interactions

- `cache.go` ↔ `PeerStore` — `GetAll()` for save, `Update()` for restore.
- `collision.go` ↔ `PeerStore.DetectCollisions()` + `pkg/crypto.DeriveMeshIP`.
- `epoch.go` ↔ `pkg/privacy.DandelionRouter`.
- `routes.go` ↔ `PeerStore` (via reconcile) + relay routes map + `pkg/routes.CalculateDiff`.
- `systemd.go` ↔ `cmdExecutor` (system commands).

## Mapping

> [[pkg/daemon/cache.go]]
> [[pkg/daemon/collision.go]]
> [[pkg/daemon/epoch.go]]
> [[pkg/daemon/routes.go]]
> [[pkg/daemon/systemd.go]]
> [[pkg/daemon/executor.go]]
