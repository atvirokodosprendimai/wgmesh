# Specification: Issue #492

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

wgmesh has a short Quick Start block in README.md and installation instructions, but no single comprehensive guide that takes a brand-new user from "never heard of wgmesh" through running a real two-node mesh. The acceptance criteria require:

- Installation methods for Homebrew, direct binary, and Docker
- A first-mesh creation walkthrough (not just 3 raw commands)
- Coverage of the basic commands (`join`, `status`, `peers`)
- Troubleshooting common issues
- Published in README or a separate `docs/` file

The existing `docs/troubleshooting.md` is incomplete — it covers only centralized-mode SSH commands and says nothing about decentralized-mode daemon issues. `docs/FAQ.md` covers interface naming and secret format but has no setup walkthrough.

The deliverable is a new file `docs/quickstart.md` plus small linking changes in README.md.

## Implementation Tasks

### Task 1: Create `docs/quickstart.md`

Create the file `docs/quickstart.md` with exactly the following content:

```markdown
# Quickstart Guide

This guide takes you from installation to a running two-node WireGuard mesh in under 10 minutes.

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Linux kernel ≥ 5.6, or macOS with wireguard-go | WireGuard is built into Linux 5.6+; on macOS install via Homebrew |
| `wireguard-tools` (`wg` command) | Required on Linux: `apt install wireguard-tools` / `yum install wireguard-tools` |
| Root or `sudo` access | Required to create WireGuard network interfaces |
| Two or more hosts reachable over the internet | Can be VMs, VPS, containers, or physical machines; NAT is fine |

## Step 1 — Install wgmesh

Choose the method that fits your environment. All methods install the same `wgmesh` binary.

### Homebrew (macOS and Linux)

```bash
brew install atvirokodosprendimai/tap/wgmesh
```

Verify:

```bash
wgmesh version
```

### Pre-built binary (Linux)

```bash
# Linux amd64
curl -L -o /tmp/wgmesh \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64
sudo install -m 0755 /tmp/wgmesh /usr/local/bin/wgmesh
wgmesh version
```

For arm64 replace `amd64` with `arm64`; for armv7 use `armv7`.

Full list of architectures and checksums on the [releases page](https://github.com/atvirokodosprendimai/wgmesh/releases).

### Debian / Ubuntu package

```bash
# Download the .deb for your architecture (example: amd64)
curl -L -o /tmp/wgmesh.deb \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64.deb
sudo apt install /tmp/wgmesh.deb
wgmesh version
```

### RPM (Fedora / RHEL / AlmaLinux)

```bash
sudo rpm -i \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64.rpm
wgmesh version
```

### Docker

No host installation required — run wgmesh inside a container. The container needs `--privileged` and `--network host` so it can manage WireGuard interfaces on the host.

```bash
docker pull ghcr.io/atvirokodosprendimai/wgmesh:latest

# Run the daemon (replace <your-secret> with the secret from Step 2)
docker run -d \
  --name wgmesh \
  --privileged \
  --network host \
  --restart unless-stopped \
  -v wgmesh-state:/var/lib/wgmesh \
  ghcr.io/atvirokodosprendimai/wgmesh:latest join \
  --secret "wgmesh://v1/<your-secret>"
```

> **Note:** The `--privileged` flag is required because wgmesh creates WireGuard kernel interfaces. Running without it produces `RTNETLINK answers: Operation not permitted`.

### From source (requires Go 1.23+)

```bash
git clone https://github.com/atvirokodosprendimai/wgmesh.git
cd wgmesh
go build -o wgmesh .
sudo install -m 0755 wgmesh /usr/local/bin/wgmesh
wgmesh version
```

---

## Step 2 — Generate a mesh secret

Run this **once** on any machine. You will copy the printed secret to every node.

```bash
wgmesh init --secret
```

Example output:

```
wgmesh://v1/dGhpcyBpcyBhIHRlc3QgZm9yIHFzZWN1cmU
```

Store this secret somewhere safe (password manager, environment file). **Anyone with this secret can join your mesh.**

> **Using your own passphrase:** Any string of 16+ characters works as a secret — for example, `MESH_SECRET=myfavoritepizza99`. Generated secrets have 256 bits of entropy from `crypto/rand`; short passphrases are significantly weaker. See [FAQ](FAQ.md#how-do-mesh-secrets-work) for details.

---

## Step 3 — Join the mesh on each node

Run the following command **as root** (or with `sudo`) on every node that should join the mesh. Use the **same secret** on every node.

```bash
sudo wgmesh join --secret "wgmesh://v1/<your-secret>"
```

The daemon starts in the foreground. To run it in the background as a systemd service:

```bash
# Install and start the systemd service (run once per node)
sudo wgmesh install-service --secret "wgmesh://v1/<your-secret>"
sudo systemctl enable --now wgmesh
sudo systemctl status wgmesh
```

### What happens after `join`

1. wgmesh derives a deterministic mesh subnet and WireGuard PSK from your secret.
2. A WireGuard keypair is generated for this node and stored in `/var/lib/wgmesh/wg0.json`.
3. The daemon announces itself on three discovery channels simultaneously:
   - **DHT (BitTorrent Mainline)** — finds peers across the internet
   - **LAN multicast** — instantly finds peers on the same local network
   - **GitHub Issues registry** — bootstraps cold-start discovery
4. As peers are found, WireGuard configuration is applied live (no interface restarts).
5. NAT traversal (UDP hole-punching) is attempted for peers behind NAT.

First peer discovery typically takes **5–30 seconds** for DHT, or **< 1 second** for LAN peers.

---

## Step 4 — Verify the mesh

### Check connected peers

```bash
wgmesh peers list
```

Example output:

```
PUBKEY                                          MESH IP         ENDPOINT              LAST SEEN
AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abc=       10.47.23.1      203.0.113.10:51820    2s ago
XyZaBcDeFgHiJkLmNoPqRsTuVwXy0123456789=        10.47.23.2      (relayed)             8s ago
```

The `MESH IP` column shows each peer's deterministic mesh address. The `ENDPOINT` column shows the direct WireGuard endpoint, or `(relayed)` if no direct path exists yet.

### Show peer count

```bash
wgmesh peers count
```

### Get details for a specific peer

```bash
wgmesh peers get <pubkey>
```

### Check daemon status

```bash
wgmesh status --secret "wgmesh://v1/<your-secret>"
```

This prints the derived mesh parameters (subnet, gossip port, rendezvous ID) for verification.

### Test end-to-end connectivity

Once at least one peer appears in `peers list`, ping it using its mesh IP:

```bash
ping 10.47.23.1
```

---

## Common options

| Flag | Description | Default |
|------|-------------|---------|
| `--secret` | Mesh secret (required) | — |
| `--interface` | WireGuard interface name | `wg0` (Linux), `utun20` (macOS) |
| `--listen-port` | WireGuard listen port | `51820` |
| `--advertise-routes` | Comma-separated CIDRs to advertise | — |
| `--log-level` | `debug`, `info`, `warn`, `error` | `info` |
| `--gossip` | Enable in-mesh gossip discovery | disabled |
| `--socket-path` | Override RPC socket path | `/var/run/wgmesh.sock` |

Example with extra options:

```bash
sudo wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --interface wg1 \
  --listen-port 51821 \
  --advertise-routes "192.168.10.0/24" \
  --log-level debug \
  --gossip
```

---

## Troubleshooting

### No peers appear after 2 minutes

1. **Check internet connectivity.** DHT requires outbound UDP. Confirm with:
   ```bash
   nc -u -z -w 3 router.bittorrent.com 6881 && echo "UDP OK"
   ```
2. **Check firewall rules.** Ensure UDP port `51820` (or `--listen-port`) is open inbound:
   ```bash
   sudo ufw allow 51820/udp     # Ubuntu
   sudo firewall-cmd --add-port=51820/udp --permanent  # Fedora/RHEL
   ```
3. **Run with debug logging** to see discovery events:
   ```bash
   sudo wgmesh join --secret "..." --log-level debug
   ```
   Look for lines containing `[dht]`, `[lan]`, or `[registry]`.

### `RTNETLINK answers: Operation not permitted`

You are not running as root. Use `sudo wgmesh join ...` or ensure the binary has the `CAP_NET_ADMIN` capability:
```bash
sudo setcap cap_net_admin+ep /usr/local/bin/wgmesh
```

### `device or resource busy` when creating interface

Another process already owns the `wg0` interface. Check with `ip link show wg0`. Either stop the existing process or use a different interface name with `--interface wg1`.

### Peers connect but cannot ping each other

1. Verify both nodes show each other in `wgmesh peers list`.
2. Check WireGuard state on the local node:
   ```bash
   sudo wg show wg0
   ```
   Confirm the peer's `allowed-ips` includes the remote mesh IP and the `latest-handshake` timestamp is recent.
3. Check routing:
   ```bash
   ip route get <remote-mesh-ip>
   ```
   The output should route through `wg0`.
4. Check if a host firewall on the remote node is blocking ICMP:
   ```bash
   # On the remote node
   sudo iptables -L INPUT -n | grep DROP
   ```

### Daemon crashes on startup

Check the systemd journal for the error:
```bash
sudo journalctl -u wgmesh -n 50 --no-pager
```

Common causes:
- **`/var/lib/wgmesh` not writable** — fix with `sudo chown root:root /var/lib/wgmesh && sudo chmod 750 /var/lib/wgmesh`
- **Corrupted state file** — delete `/var/lib/wgmesh/wg0.json` and restart (new keypair will be generated)
- **Port already in use** — use `--listen-port` to pick a different port

### macOS: interface fails to create

On macOS, wgmesh uses `wireguard-go` which only supports `utunN`-style interface names. If you specified a custom `--interface`, change it to `utun20` (or any `utun` + number):
```bash
wgmesh join --secret "..." --interface utun20
```

Install `wireguard-go` if not already present:
```bash
brew install wireguard-go wireguard-tools
```

### Reset a node completely

Stop the daemon, remove state, and rejoin:
```bash
sudo systemctl stop wgmesh
sudo ip link delete wg0            # remove the WireGuard interface
sudo rm /var/lib/wgmesh/wg0.json   # remove the keypair (new one generated on next join)
sudo wgmesh join --secret "..."
```

---

## Next steps

- **Centralized mode** (SSH-based fleet management): see [docs/centralized-mode.md](centralized-mode.md)
- **Access control** (group-based segmentation): see [docs/access-control.md](access-control.md)
- **Troubleshooting centralized deployments**: see [docs/troubleshooting.md](troubleshooting.md)
- **FAQ** (interface naming, secrets, subnet customisation): see [docs/FAQ.md](FAQ.md)
```

### Task 2: Update `docs/troubleshooting.md`

The existing `docs/troubleshooting.md` only covers centralized SSH mode. Add a header and a decentralized-mode section at the top of the file so that readers of the quickstart guide who follow the `troubleshooting.md` link also find useful content.

Open `docs/troubleshooting.md`. Replace the first line:

```markdown
# Troubleshooting
```

with:

```markdown
# Troubleshooting

> For decentralized mode (`wgmesh join`) issues, see the [Quickstart Guide troubleshooting section](quickstart.md#troubleshooting).

## Centralized Mode (SSH Deployment)
```

Leave all remaining content unchanged.

### Task 3: Update `README.md` — add quickstart link

In `README.md`, locate the existing `## Troubleshooting` section (near the end):

```markdown
## Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md) for runbooks covering connection issues, persistence checks, log viewing, and configuration rebuilds.
```

Replace it with:

```markdown
## Troubleshooting

See the [Quickstart Guide troubleshooting section](docs/quickstart.md#troubleshooting) for decentralized-mode issues (daemon, peers, NAT, interface errors).

See [docs/troubleshooting.md](docs/troubleshooting.md) for centralized-mode SSH runbooks (persistence checks, log viewing, configuration rebuilds).
```

Also locate the `## Installation` section in `README.md`. After the subsection:

```markdown
See [DOCKER.md](DOCKER.md) and [DOCKER-COMPOSE.md](DOCKER-COMPOSE.md) for detailed Docker deployment guides.
```

Add a new line:

```markdown
For a step-by-step first-mesh walkthrough covering all installation methods, see [docs/quickstart.md](docs/quickstart.md).
```

## Affected Files

- **New:** `docs/quickstart.md` — comprehensive installation and usage guide
- **Modified:** `docs/troubleshooting.md` — add pointer to quickstart for decentralized mode, rename existing section to "Centralized Mode"
- **Modified:** `README.md` — add quickstart link in Installation section and update Troubleshooting section

No code files are changed. No Go packages are touched. No new dependencies.

## Test Strategy

No automated tests required for documentation. Verify manually:

1. `docs/quickstart.md` renders without broken Markdown in GitHub preview (all code fences closed, all tables aligned).
2. Every relative link in `docs/quickstart.md` resolves to an existing file:
   - `FAQ.md` → `docs/FAQ.md` ✓
   - `centralized-mode.md` → `docs/centralized-mode.md` ✓
   - `access-control.md` → `docs/access-control.md` ✓
   - `troubleshooting.md` → `docs/troubleshooting.md` ✓
3. The link `docs/quickstart.md` in `README.md` resolves correctly.
4. The link `quickstart.md#troubleshooting` in `docs/troubleshooting.md` resolves to the correct anchor in `docs/quickstart.md`.
5. All `docs/troubleshooting.md` content under "Centralized Mode (SSH Deployment)" is preserved unchanged.

## Estimated Complexity
low

**Reasoning:** Pure documentation. One new Markdown file (~170 lines) and targeted edits to two existing files (< 10 lines changed total). No code changes, no dependency updates, no build pipeline changes. Estimated effort: 45–60 minutes.
