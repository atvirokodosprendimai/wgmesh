---
tldr: Daemon derives WireGuard identity and mesh IP from a shared secret; manages the interface lifecycle from startup through graceful shutdown; supports SIGHUP hot-reload of routes and log level without restart.
category: core
---

# Daemon lifecycle

## Target

The startup, runtime, and shutdown sequence of the decentralized mesh daemon.

## Behaviour

- The daemon's identity (WireGuard keypair + mesh IP) is derived deterministically from a shared secret via `pkg/crypto`. No pre-shared key exchange is needed — any node with the same secret derives a compatible identity.
- The WireGuard keypair is persisted to `/var/lib/wgmesh/<iface>.json` (mode 0600). On restart, the same keypair is reused so the mesh IP and public key remain stable.
- If the configured listen port is already in use, the daemon automatically selects the next available UDP port and logs the substitution.
- Startup sequence: derive identity → create/reset WireGuard interface → configure key + port → assign mesh IP (IPv4 `/16` + optional IPv6 `/64`) → bring up → start goroutines.
- Shutdown on SIGINT/SIGTERM: cancel context → goroutines drain via WaitGroup → teardown WireGuard interface (down + delete).
- SIGHUP triggers hot-reload: reads `/var/lib/wgmesh/<iface>.reload` (KEY=VALUE format) and applies changes without restarting WireGuard or DHT. If no reload file exists, the signal is a no-op (warning logged).
  - Reloadable: `advertise-routes` (comma-separated CIDRs), `log-level`.
  - Not reloadable: secret, interface name, listen port, privacy/gossip flags.
  - After a successful reload, an immediate reconcile is triggered.
- The discovery layer is pluggable and injected after construction via `SetDHTDiscovery`; the daemon runs without it if none is provided.
- The RPC server is injected via `SetRPCServer`; it is started/stopped by the daemon's lifecycle.

## Design

- `configMu` (RWMutex) guards hot-reloadable fields (`advertise-routes`, `log-level`). All callers reading these at runtime must hold at least a read lock; SIGHUP reload holds the write lock.
- `LocalNode.wgEndpoint` is guarded by its own `endpointMu` — discovery goroutines may update it concurrently.
- The WireGuard interface is idempotent on startup: if it already exists, it is reset (addresses flushed, peers cleared) rather than deleted and recreated.
  - {>> avoids a brief interface-down gap and preserves the port binding on partial restarts}
- Cross-platform interface management: Linux uses `ip link` + `wg set`; macOS uses `wireguard-go` (daemon started asynchronously) + `ifconfig`/`route`.
- Private key is passed to `wg set` via `/dev/stdin`, never as a CLI argument.

## Interactions

- `pkg/crypto` — `DeriveKeys` (from secret → all mesh parameters), `DeriveMeshIP`, `DeriveMeshIPv6`.
- `pkg/wireguard` — `GenerateKeyPair`, `GetLatestHandshakes`, `GetPeerTransfers`.
- `pkg/privacy` — `DandelionRouter` (started by `EpochManager` after startup).
- `DiscoveryLayer` interface — injected; `Start`/`Stop` called by daemon.
- `RPCServer` interface — injected; lifecycle tied to daemon.

## Mapping

> [[pkg/daemon/daemon.go]]
> [[pkg/daemon/helpers.go]]
> [[pkg/daemon/config.go]]
