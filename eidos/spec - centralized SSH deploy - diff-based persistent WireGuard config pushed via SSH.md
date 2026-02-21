---
tldr: Operator machine holds all keys and pushes per-node WireGuard config over SSH; diffs against live state to minimise disruption; persists as wg-quick systemd service.
category: core
---

# Centralized SSH deployment

## Target

The operator-driven deployment path that computes per-node WireGuard configurations and pushes them to every node over SSH.

## Behaviour

- Deployment is centralized: the machine running `wgmesh deploy` holds all WireGuard private keys and pushes config to every node.
- Before pushing, the system SSHs to each node to collect: actual hostname, FQDN, and public IP.
  If the public IP differs from the SSH host address, the node is marked as behind NAT and given no public endpoint.
- Per-node WireGuard config is computed from the mesh state (policy-filtered if access control is enabled).
- The live WireGuard config is fetched via `wg show <iface> dump` over SSH and diffed against the desired config.
  - If only peers changed: apply peer updates online via `wg set` — no service restart.
  - If the interface config changed, or no existing config is found: write conf file and `systemctl restart wg-quick@<iface>`.
- Config is persisted as `/etc/wireguard/<iface>.conf` (wg-quick format) and enabled as a systemd service — it survives reboots.
- Route synchronisation is always performed independently of WireGuard peer sync.
  Routes for all allowed peers' routable networks are computed and reconciled against the kernel routing table.
- WireGuard installation is idempotent: the system checks `which wg` first; installs via `apt` only if absent.
- The wg-quick config enables IP forwarding via `PostUp = sysctl -w net.ipv4.ip_forward=1`.

## Design

- **Diff → online update or full restart:** the system prefers live `wg set` peer updates to avoid traffic disruption; falls back to full `wg-quick` restart only when the interface itself changes.
- **Route sync is decoupled:** routes can drift independently of WireGuard peers; they are always reconciled on every deploy.
- **Two config representations:** `FullConfig` (ordered slice, for writing/rendering) and `Config` (map by pubkey, for diffing). `FullConfigToConfig` converts between them.
- **Peer equality for diff**: checks PSK, endpoint, keepalive, and AllowedIPs as a set.
- `PersistentKeepalive = 5` on all peers — keeps connections alive through NAT.
- Private keys are written to a temp file on the remote (`/tmp/wg-key-<iface>`) and deleted immediately after `wg set` reads them.

## Interactions

- `pkg/mesh` — orchestrator; calls deploy, provides computed `FullConfig` and route lists per node.
- `pkg/wireguard` — config structs, diff computation, wg-quick rendering, `ApplyDiff`, `ApplyPersistentConfig`.
- `pkg/ssh` — transport: `Run`, `RunQuiet`, `WriteFile`, `RunWithStdin`; also `EnsureWireGuardInstalled`, `DetectPublicIP`, `GetHostname`, `GetCurrentRoutes`, `ApplyRouteDiff`.

## SSH client behaviour

- Auth: SSH agent (via `SSH_AUTH_SOCK`) first, then identity files (`id_rsa`, `id_ed25519`, `id_ecdsa`).
- Always connects as `root`.
- Host key verification disabled (`InsecureIgnoreHostKey`) — assumes a trusted private network.
- Connection timeout: 10 seconds.

## Mapping

> [[pkg/mesh/deploy.go]]
> [[pkg/wireguard/apply.go]]
> [[pkg/wireguard/config.go]]
> [[pkg/wireguard/convert.go]]
> [[pkg/wireguard/keys.go]]
> [[pkg/wireguard/persist.go]]
> [[pkg/ssh/client.go]]
> [[pkg/ssh/routes.go]]
> [[pkg/ssh/wireguard.go]]
