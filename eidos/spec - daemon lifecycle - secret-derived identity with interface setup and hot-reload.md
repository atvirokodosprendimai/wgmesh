---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Daemon derives WireGuard identity and mesh IP from a shared secret; manages the interface lifecycle from startup through graceful shutdown; supports SIGHUP hot-reload of routes and log level without restart.
category: core
---

# Daemon lifecycle

## Target

The startup, runtime, and shutdown sequence of the decentralized mesh daemon.

## Behaviour

- The daemon's identity (WireGuard keypair + mesh IP) is derived deterministically from a shared secret via `pkg/crypto`. No pre-shared key exchange is needed — any node with the same secret derives a compatible identity.
- The WireGuard keypair is persisted to `/var/lib/wgmesh/<iface>.json` (mode 0600, written by `atomicfile.WriteFile`). On restart, the same keypair is reused so the mesh IP and public key remain stable.
- **Corrupt identity (ADR-0005).** `loadLocalNode` returns `ErrStateMissing` for an absent file and `ErrStateCorrupt` for a file that is not JSON, is empty, is `{}` or lacks the public or private key; any other read error (permission, a directory) stops the daemon. At release N a corrupt file is moved to `<iface>.json.corrupt-<unix>` (`-1`, `-2` ... when the name is taken; mode 0600; never deleted or overwritten), the log names both paths and says how to restore it (stop the daemon, rename the file back), and a new identity (new key, new mesh IP) is created. The package variable `RefuseCorruptState` is `false` at N; a later release sets it to `true`, and then `initLocalNode` refuses to start and leaves the file in place (BACKLOG, ADR-0005 N+1). It is not an operator switch.
- **Interface lock (ADR-0005).** `wgmesh join` takes an exclusive `flock` on `<state dir>/<iface>.lock` (`pkg/flock`, the default state directory `/var/lib/wgmesh` whatever `--state-dir` says) before it reads the account file, records a referral or creates the daemon or the RPC server. A second `join` for the interface exits 1 with a message that names the lock file and the pid of the holder. The lock file holds the pid, is never removed, and is released when the process ends. It excludes only a running daemon of a release that takes the lock: an older daemon holds none.
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
- `LocalNode.MeshIP`, `MeshIPv6`, `NATType` and the introducer flag are unexported and guarded by one mutex (`stateMu`), reached through `GetMeshIP/SetMeshIP`, `GetNATType/SetNATType` and the other accessors, `Identity()`, `Snapshot()` and `NewLocalNode`; the routes have their own mutex. `reconcile()` is serialized by `reconcileMu` (the reconcile loop and the SIGHUP handler both call it); lock order is `reconcileMu`, then `configMu`.
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
- `pkg/atomicfile` — `WriteFile` for the identity file; `pkg/flock` — `TryLock` for the interface lock.

## Mapping

> [[pkg/daemon/daemon.go]]
> [[pkg/daemon/helpers.go]]
> [[pkg/daemon/config.go]]
> [[pkg/atomicfile/atomicfile.go]]
> [[pkg/flock/flock.go]]
