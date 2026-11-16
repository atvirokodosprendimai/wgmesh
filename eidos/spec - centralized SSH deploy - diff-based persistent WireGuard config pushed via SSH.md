---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Operator machine holds all keys and pushes per-node WireGuard config over SSH; diffs against live state to minimise disruption; persists as wg-quick systemd service.
category: core
---

# Centralized SSH deployment

## Target

The operator-driven deployment path that computes per-node WireGuard configurations and pushes them to every node over SSH.

## Behaviour

- Deployment is centralized: the machine running `wgmesh deploy` holds all WireGuard private keys and pushes config to every node.
- Before any connection `Deploy` validates the state (see the mesh state spec): a class A finding refuses the run, a class B finding is a warning until `StrictState` is set in a later release.
- Before pushing, the system SSHs to each node that is not the local one to collect: actual hostname, FQDN, and public IP. A node whose SSH host is a private IP address is not asked for its public IP.
- The endpoint of each node is decided in this order, and the deploy prints the reason (`Detected X has endpoint E (reason)`):
  1. a `public_endpoint` already written in the state wins and is not overwritten;
  2. when every node's SSH host is a private IP address, the endpoint is `<ssh host>:<listen port>` for every node, the local one included;
  3. when the detected public IP equals the SSH host, the endpoint is `<ssh host>:<listen port>` (a remote node only: the local node is never probed, so for it only rules 1 and 2 apply);
  4. otherwise the node is behind NAT and has no endpoint.

  A DNS name is never classified private, and an IPv6 host is bracketed. In a mixed mesh rules 3 and 4 apply, so the operator sets `public_endpoint` for a private node that others should dial.
- Per-node WireGuard config is computed from the mesh state (policy-filtered if access control is enabled).
- The live WireGuard config is fetched via `wg show <iface> dump` over SSH and diffed against the desired config.
  - If only peers changed: apply peer updates online via `wg set` — no service restart.
  - If the interface config changed, or no existing config is found: write conf file and `systemctl restart wg-quick@<iface>`.
- Config is persisted as `/etc/wireguard/<iface>.conf` (wg-quick format) and enabled as a systemd service — it survives reboots.
- Route synchronisation is always performed independently of WireGuard peer sync.
  Routes for all allowed peers' routable networks are computed and reconciled against the kernel routing table.
- When a route `<own network> via <peer> dev <iface>` is removed (the HA route of releases before the shared-subnet fix), the node's connected route for that network is restored, whatever the gateway, and the deploy prints `Restored connected route: <net> dev <local interface>` (the interface that holds the local address in that network). It needs a local address in that network; without one the deploy warns instead.
- WireGuard installation is idempotent: the system checks `which wg` first; installs via `apt` only if absent.
- The wg-quick config enables IP forwarding via `PostUp = sysctl -w net.ipv4.ip_forward=1`.
- Every remote command is built with `ssh.Cmd`: the command name is a constant and each argument goes through `ssh.Quote` (the safe set `[A-Za-z0-9._:/,@%+=-]` is sent unchanged, anything else is single-quoted, brackets included). The guard test `TestNoUnquotedRemoteCommand` fails when a command reaches a node another way. The wg-quick text refuses a value with a newline, carriage return or NUL.
- Nodes are deployed in sorted hostname order. A node that fails does not stop the run: each node gets a result with a phase (`detect`, `connect`, `install`, `apply`, `routes`, `persist`), and `Deploy` returns one `DeployError` that lists every failed node with its phase and the nodes that were configured. The exit code stays 1. A repeated deploy converges because each node is diffed against its live config.
- A config file that cannot be written is a warning in a `Warnings:` block at release N, and fails the node in phase `persist` once `PersistFailureFatal` is set (N+2). The wg-quick text is rendered before the first connection: a render refusal is a `persist` failure and the node is not touched.
- Two later releases flip the switches: `mesh.StrictState = true` (N+1) and `mesh.PersistFailureFatal = true` (N+2). Operator note: `docs/centralized-rollout.md`; decision: ADR-0006.

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
- `pkg/ssh` — transport: `Run`, `RunQuiet`, `WriteFile`, `RunWithStdin`; `Quote` and `Cmd` (every remote command is built with them); also `EnsureWireGuardInstalled`, `DetectPublicIP`, `GetHostname`, `GetCurrentRoutes`, `ApplyRouteDiff`, `RestoreConnectedRoute`.

## SSH client behaviour

- Auth: SSH agent (via `SSH_AUTH_SOCK`) first, then identity files (`id_rsa`, `id_ed25519`, `id_ecdsa`).
- Always connects as `root`.
- Host key verification against `~/.ssh/known_hosts` by default (PR #876, `pkg/ssh/hostkey.go`); `WGMESH_SSH_KNOWN_HOSTS` names another file. An unknown or changed key refuses the connection. `WGMESH_SSH_INSECURE_HOST_KEY=1` turns verification off and prints a warning.
- Connection timeout: 10 seconds.

## Mapping

> [[pkg/mesh/deploy.go]]
> [[pkg/mesh/validate.go]]
> [[pkg/mesh/report.go]]
> [[pkg/mesh/endpoint.go]]
> [[pkg/wireguard/apply.go]]
> [[pkg/wireguard/config.go]]
> [[pkg/wireguard/convert.go]]
> [[pkg/wireguard/keys.go]]
> [[pkg/wireguard/persist.go]]
> [[pkg/ssh/client.go]]
> [[pkg/ssh/quote.go]]
> [[pkg/ssh/routes.go]]
> [[pkg/ssh/wireguard.go]]
