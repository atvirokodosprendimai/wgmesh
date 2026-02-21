# Pull — mesh state, access control, and centralized SSH deploy

**Source packages:** `pkg/mesh/`, `pkg/wireguard/`, `pkg/ssh/`
**Date:** 2026-02-21

---

## Collected Material

### pkg/mesh — state, node lifecycle, access control

**Types (types.go)**
- `Mesh`: top-level struct — interface name (`wg0`), network CIDR (`10.99.0.0/16`), listen port (51820), nodes map, local hostname, optional groups + access policies.
- `Node`: hostname, mesh IP, WireGuard key pair, SSH host+port, optional public endpoint, listen port, NAT flag, routable networks, is-local flag, actual hostname, FQDN.
- `Group`: named set of hostnames.
- `AccessPolicy`: name, from_groups, to_groups, `allow_mesh_ips`, `allow_routable_networks`.
- `PeerAccess`: computed per-node access result — `AllowMeshIP`, `AllowRoutableNetworks`.

**State file (mesh.go)**
- `Initialize`: creates default Mesh, saves to file.
- `Load/Save`: JSON marshal/unmarshal with optional symmetric encryption via `pkg/crypto`.
- State file written with mode `0600` under a `0700` directory.
- `AddNode`: parses `hostname:mesh_ip:ssh_host[:port]` spec, generates WireGuard keypair, sets IsLocal if hostname matches local OS hostname.
- `RemoveNode`: deletes by hostname.

**Policy engine (policy.go)**
- `ValidateGroups/ValidatePolicies`: deduplication checks, referential integrity (all referenced groups must exist), ensures every policy matches at least one node.
- `GetNodeGroups(hostname)`: all groups containing the hostname.
- `GetAllowedPeers(hostname)` → `map[string]*PeerAccess`:
  - Node with no groups → empty map (deny by default).
  - For each policy, checks if node is in `from_groups` (outbound) OR `to_groups` (inbound).
  - Collects members of the "other side" of each matching policy as peers.
  - Merges: `AllowMeshIP` set by any outbound policy with `allow_mesh_ips=true`.
  - Note: routable network access only accumulates on outbound direction.
  - Self is always excluded.
  - Missing nodes (group member not in Nodes map) silently skipped.

**Deploy orchestration (deploy.go)**
- `Deploy()`:
  1. Validate access control config if enabled.
  2. `detectEndpoints()`: SSH to each non-local node, collect actual hostname, FQDN, public IP; set `BehindNAT` if public IP differs from SSH host, otherwise set `PublicEndpoint`.
  3. For each node: open SSH client, ensure WireGuard installed, compute per-node config + routes, fetch current config via `wg show dump`, compute diff, apply (persistent), sync routes, write conf file.
- `generateConfigForNode`: produces `FullConfig` — uses `GetAllowedPeers` when AC enabled, full mesh otherwise. Mesh IP always in AllowedIPs as `/32`; routable networks added only if policy permits. `PersistentKeepalive = 5`.
- `collectAllRoutesForNode`: own networks (no gateway) + peer networks via mesh IP gateway (policy-filtered or all).
- `syncRoutesForNode`: `GetCurrentRoutes` → `CalculateRouteDiff` → `ApplyRouteDiff`. Falls back to add-all if current routes can't be read.

---

### pkg/wireguard — config structs, diff, apply, persistence

**Two config representations:**
- `Config` / `Interface` / `Peer` (map-keyed by pubkey) — used for live state parsing from `wg show dump`.
- `FullConfig` / `WGInterface` / `WGPeer` (slice) — used for centralized deployment.
- `FullConfigToConfig`: converts FullConfig → Config for diffing.

**Config lifecycle (config.go + apply.go + persist.go):**
- `GetCurrentConfig`: runs `wg show <iface> dump` via SSH, parses tab-separated output.
- `CalculateDiff(current, desired *Config) *ConfigDiff`: identifies added/removed/modified peers and interface changes. Peer equality checks PSK, endpoint, keepalive, and AllowedIPs (set comparison).
- `ApplyDiff`: online peer updates via `wg set` over SSH. Interface changes are rejected (require full reconfig).
- `ApplyPersistentConfig`: writes wg-quick conf to `/etc/wireguard/<iface>.conf`, `systemctl enable + restart wg-quick@<iface>`.
- `UpdatePersistentConfig`: if only peers changed → write conf + `ApplyDiff` (no restart); if interface changed → full `ApplyPersistentConfig`.
- `RemovePersistentConfig`: stop + disable service, delete conf file.
- `GenerateWgQuickConfig`: renders `[Interface]` + `[Peer]` sections. Embeds `PostUp/PreDown` for extra routes. Enables IP forwarding via `PostUp = sysctl -w net.ipv4.ip_forward=1`.

**Keys (keys.go):** `GenerateKeyPair()` — shells out to `wg genkey` + `wg pubkey`.

**Local WG operations (apply.go):**
- `SetPeer`, `RemovePeer`, `GetPeers`: manipulate local interface via `wg set`/`wg show`.
- `GetLatestHandshakes`, `GetPeerTransfers`: read live stats from local `wg show` — used by daemon.
- PSK passed via `/dev/stdin` to avoid shell exposure.

---

### pkg/ssh — transport layer

**Client (client.go):**
- Auth order: SSH agent (via `SSH_AUTH_SOCK`) → identity files (id_rsa, id_ed25519, id_ecdsa).
- Always connects as `root`.
- `HostKeyCallback: ssh.InsecureIgnoreHostKey()` — no known_hosts check.
- Timeout: 10 seconds.
- `Run(cmd)` → `CombinedOutput`. `RunQuiet(cmd)` → discards output. `RunWithStdin(cmd, stdin)` → pipes stdin.
- `WriteFile(path, content, mode)` → `cat > path && chmod mode path` via SSH.

**WireGuard helpers (wireguard.go):**
- `EnsureWireGuardInstalled`: checks `which wg`, installs via `apt` if missing. Idempotent.
- `DetectPublicIP`: `curl -s -4 ifconfig.me || curl -s -4 icanhazip.com`.
- `GetHostname`, `GetFQDN`: `hostname` / `hostname -f`.

**Route management (routes.go):**
- `GetCurrentRoutes`: `ip route show dev <iface>`, parses network + optional `via` gateway.
- `CalculateRouteDiff`: delegates to `pkg/routes`.
- `ApplyRouteDiff`: removes stale, adds new, always enables IP forwarding.

---

## Intent Sketch

### Mesh state and access control

- The mesh is a JSON file (optionally encrypted) that is the single source of truth for all nodes, their WireGuard keys, and their network topology.
- Each node is identified by hostname and carries its WireGuard keypair, mesh IP, SSH access coordinates, NAT status, and optional routable networks.
- When no groups or policies are defined, all nodes are full-mesh peers — the default allows any node to reach any other node.
- Groups are named sets of hostnames; access policies define directional reachability between groups.
- A node with no group membership gets no peers (deny by default when AC is enabled).
- Policy evaluation considers both directions: a node is a peer of another if either an outbound or inbound policy involves both their groups.
- Routable network reachability is controlled per-policy, separate from mesh IP reachability.

### Centralized SSH deployment

- Deployment is operator-driven: the machine running `wgmesh deploy` holds all private keys and pushes configuration to every node over SSH.
- Before pushing, the system detects each node's public endpoint vs NAT by comparing the SSH host address against the node's public IP.
- Per-node WireGuard config is computed from mesh state (applying access policy if enabled), then reconciled against the live config via `wg show dump` diff to minimise disruption.
- If only peers changed, updates are applied online without restarting the service; interface changes require a full service restart.
- Configuration is persisted as a wg-quick file (`/etc/wireguard/wg0.conf`) enabled as a systemd service, so it survives reboots.
- Route synchronisation is always performed independently of WireGuard peer sync — routes can drift separately.
- The SSH transport layer authenticates via SSH agent or standard identity files; it always connects as root and skips host key verification.

---

## Patterns

- **Dual config representations**: `FullConfig` (slice, for writing) vs `Config` (map by pubkey, for diffing) — `convert.go` bridges them.
- **Diff → online update or full restart**: the system prefers online peer updates to avoid service disruption; falls back to full restart only when needed.
- **Idempotent helpers**: `EnsureWireGuardInstalled`, route add with `|| ip route replace` fallback.
- **Access control is opt-in**: the absence of groups/policies means full mesh — existing deployments need no changes to adopt the feature.

---

## Dependencies

- `pkg/crypto` — optional symmetric encryption for the state file.
- `pkg/routes` — normalise and diff kernel routes.
- `pkg/ssh` ← `pkg/wireguard` (SSH client passed to all remote operations).
- `pkg/mesh` ← `pkg/wireguard` + `pkg/ssh` (orchestrates both).
