---
tldr: main.go is a dual-mode dispatcher — decentralized subcommands (join, init, status, qr, install-service, rotate-secret, peers) coexist with legacy centralized flags (-add, -deploy, -init); join wires the RPC server as a thin adapter to break the daemon→rpc circular dependency.
category: core
---

# CLI entry point — dual-mode dispatch with daemon wiring and RPC server

## Target

`main.go` is the single binary entry point for wgmesh.
It dispatches to either decentralized (daemon-based) subcommands or legacy centralized (SSH-deploy) flag-based mode,
and wires together the daemon, discovery, and RPC layers.

## Behaviour

### Dispatch order

1. **Version flags** (`--version`, `-v`) — checked before any flag parsing; prints `wgmesh <version>` and exits.
2. **Subcommand routing** — if `os.Args[1]` matches a known subcommand name, dispatch and return:
   `version`, `join`, `init`, `status`, `test-peer`, `qr`, `install-service`, `uninstall-service`, `rotate-secret`, `mesh`, `peers`.
3. **Centralized flag mode** — falls through to `flag.Parse()` if no subcommand matched.

### Decentralized subcommands

#### `init --secret`
Generates a new mesh secret via `daemon.GenerateSecret()` and prints it as a `wgmesh://v1/…` URI.
No flags other than `--secret` (presence triggers generation).

#### `join --secret <SECRET>` (primary operation)

Flags: `--secret` (required), `--advertise-routes` (comma-separated CIDRs), `--listen-port` (default 51820), `--interface`, `--log-level` (default `info`), `--privacy`, `--gossip`, `--socket-path`, `--no-lan-discovery`, `--no-ipv6`, `--force-relay`, `--no-punching`, `--introducer`, `--pprof`.

Startup sequence:
1. `daemon.NewConfig(DaemonOpts{…})` — derives keys, resolves interface name.
2. `daemon.ConfigureLogging(cfg.LogLevel)` — must be called in main before daemon creation (not inside library).
3. `daemon.NewDaemon(cfg)` — creates daemon.
4. Optional: start pprof HTTP server (`net/http/pprof` imported via blank import).
5. `createRPCServer(d, socketPath)` — wires RPC callbacks (see below); attaches to daemon via `d.SetRPCServer(rpcServer)`.
6. `d.RunWithDHTDiscovery()` — blocks until stopped.

Discovery registration: `pkg/discovery` is imported blank (`_ "…/pkg/discovery"`) so its `init()` registers the DHT factory before `RunWithDHTDiscovery` is called.

#### `status --secret <SECRET>`
Derives keys from secret (no running daemon required) and prints network parameters:
interface, network ID (first 8 bytes, hex), mesh subnet, IPv6 prefix, gossip port, rendezvous ID.
Also calls `daemon.ServiceStatus()` to show systemd unit state if available.

#### `qr --secret <SECRET>`
Formats the secret as a `wgmesh://v1/…` URI if not already, then renders it inside a Unicode block-character border.
Note: this is a text display, not a scannable QR code (the library for real QR encoding is not yet wired).

#### `install-service --secret <SECRET>`
Accepts the same feature flags as `join`.
Builds a `daemon.SystemdServiceConfig` and calls `daemon.InstallSystemdService(cfg)`.

#### `uninstall-service`
Calls `daemon.UninstallSystemdService()`. No flags.

#### `rotate-secret --current <OLD> [--new <NEW>] [--grace <DURATION>]`
Derives `MembershipKey` from the current secret and creates a signed rotation announcement via `crypto.GenerateRotationAnnouncement`.
If `--new` is omitted, generates a fresh secret automatically.
Grace period defaults to 24h.
**Current limitation**: the announcement is generated but not broadcast (`_ = announcement`) — the command only prints the new URI and instructions. Full rotation requires a running mesh.

#### `test-peer --secret <SECRET> --peer <IP:PORT>`
Diagnostic connectivity probe. Opens a UDP socket (random port if `--port 0`), sends an AES-GCM encrypted HELLO to the target, waits 10s for a REPLY, decrypts it with the gossip key, and prints the peer's public key and mesh IP on success.

### Query subcommands (daemon must be running)

**`peers list`**: calls `peers.list` via RPC; formats output as a table with columns: PUBLIC KEY (40 chars, truncated), MESH IP, ENDPOINT, LAST SEEN (relative: `Xs`, `Xm`, `Xh`, `Xd`), DISCOVERED VIA.

**`peers count`**: calls `peers.count`; prints active/total/dead counts.

**`peers get <pubkey>`**: calls `peers.get`; prints full peer detail including routes.

Socket path: `$WGMESH_SOCKET` env var if set, else `rpc.GetSocketPath()`.

**`mesh list [--state <file>] [--encrypt]`**: loads centralized mesh state file, calls `m.ListSimple()`.

### Centralized flag mode (legacy)

Parsed via `flag.Parse()` after subcommand check fails.

Flags: `-state` (default `/var/lib/wgmesh/mesh-state.json`), `-add <hostname:ip:ssh_host[:ssh_port]>`, `-remove <hostname>`, `-list`, `-list-simple`, `-deploy`, `-init`, `-encrypt`.

`-init -encrypt` combination: prompts for password twice (confirmation). Other `-encrypt` uses: prompt once.

### RPC server wiring (`createRPCServer`)

`main` creates the `rpc.Server` directly and injects callbacks that translate between `daemon`'s internal peer types and `rpc.PeerData`/`rpc.StatusData`.
This breaks a circular dependency: `daemon` does not import `rpc`; `main` imports both and bridges them.

Callbacks wired:
- `GetPeers` → `d.GetRPCPeers()` mapped to `[]*rpc.PeerData`
- `GetPeer` → `d.GetRPCPeer(pubKey)` mapped to `*rpc.PeerData`
- `GetPeerCounts` → `d.GetRPCPeerCounts` (direct assignment, types match)
- `GetStatus` → `d.GetRPCStatus()` mapped to `*rpc.StatusData`

## Design

- **Subcommand-first, flags-second**: the `os.Args[1]` switch runs before `flag.Parse()`, so subcommand names cannot collide with flag names. The centralized flag mode is preserved as the fallback for backward compatibility.
- **RPC as a thin adapter in main**: daemon exposes `GetRPC*` methods that return its own internal struct types; main performs the field-by-field copy to `rpc.*` types. This keeps `pkg/daemon` and `pkg/rpc` independent packages with no import cycle.
- **Logging configured before daemon construction**: `daemon.ConfigureLogging` must be called in main, not inside library code, because the log level is an operator concern and library code shouldn't configure global state from within `New*` constructors.
- **Blank import for discovery registration**: `pkg/discovery`'s `init()` registers the DHT factory with the daemon's discovery registry. The blank import at the binary boundary is intentional — it's the only place that should decide which backends are available.
- **pprof via blank import**: `_ "net/http/pprof"` registers pprof handlers on the default mux; `--pprof` starts an HTTP listener. Available in production builds for live profiling without recompilation.
- **`rotate-secret` is a preview**: the command correctly derives keys and creates a cryptographically valid rotation announcement, but distribution requires broadcasting it to the mesh. Until that wiring exists, the operator must manually restart each node with the new secret.

## Interactions

- `pkg/daemon` — `NewConfig`, `NewDaemon`, `RunWithDHTDiscovery`, `GenerateSecret`, `FormatSecretURI`, `ConfigureLogging`, `ServiceStatus`, `InstallSystemdService`, `UninstallSystemdService`, `SystemdServiceConfig`, `GetRPC*` methods.
- `pkg/rpc` — `NewServer`, `NewClient`, `GetSocketPath`, `ServerConfig`, `PeerData`, `StatusData`.
- `pkg/mesh` — `Initialize`, `Load`, centralized mesh operations.
- `pkg/crypto` — `DeriveKeys`, `GenerateRotationAnnouncement`, `ReadPassword`, `SealEnvelope`, `OpenEnvelope`, `CreateAnnouncement`.
- `pkg/discovery` — blank import triggers DHT factory registration.

## Mapping

> [[main.go]]
