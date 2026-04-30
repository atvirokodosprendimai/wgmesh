# Use Case: Remote Development Team VPN

**Time to first value:** ~15 minutes  
**Mode:** Decentralized (`wgmesh join`)  
**Nodes:** Developer laptops + one always-on server (VPS or office machine)

---

## Problem

A small engineering team (3–10 people) works remotely. Internal services — a staging database,
a private API server, a code-review tool — live on a VPS or behind a NAT router in the office.
Every developer needs persistent access regardless of their home network ISP, hotel Wi-Fi, or
mobile tethering.

**Current painful alternatives:**

| Alternative | Pain point |
|-------------|------------|
| OpenVPN / WireGuard (manual) | Operator must exchange keys and update every config when someone joins or leaves |
| Tailscale | Requires a Tailscale account; SaaS control plane is a trust dependency; free tier limits |
| SSH port-forwarding | Not a real network layer; each service needs a separate tunnel; breaks on reconnect |
| VPN appliance (Cisco, etc.) | Requires static IP, complex licensing, hardware in office |

**wgmesh advantage:** Any developer can join or leave the mesh by receiving or revoking the shared
secret. The operator never touches individual configs. The always-on VPS provides a stable DHT
bootstrap peer; laptops behind NAT hole-punch through automatically.

---

## Setup

**Prerequisites:** Three hosts: one VPS with a public IP (`vps`), two developer laptops (`dev1`,
`dev2`) on different home networks behind NAT. All have Linux (kernel ≥ 5.6) or macOS with
`wireguard-go`.

### Step 1 — Install wgmesh on all three hosts

```bash
# Linux (amd64) — run on vps, dev1, dev2
curl -L -o /tmp/wgmesh \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64
sudo install -m 0755 /tmp/wgmesh /usr/local/bin/wgmesh
wgmesh version
```

macOS developers: `brew install atvirokodosprendimai/tap/wgmesh`

### Step 2 — Generate the team secret (run once, on any machine)

```bash
wgmesh init --secret
# Output: wgmesh://v1/<base64>
```

Store this secret in your team password manager. Share it with every developer who should be in
the mesh. **Rotating the secret is the mechanism for revoking access.**

### Step 3 — Start the daemon on the VPS (as a systemd service)

```bash
# On vps
sudo wgmesh install-service --secret "wgmesh://v1/<your-secret>"
sudo systemctl enable --now wgmesh
sudo systemctl status wgmesh
```

The VPS becomes a stable DHT bootstrap anchor for the team.

### Step 4 — Join from each developer laptop

```bash
# On dev1 and dev2 (repeat for every new developer)
sudo wgmesh join --secret "wgmesh://v1/<your-secret>"
```

Developers on macOS:

```bash
sudo wgmesh join --secret "wgmesh://v1/<your-secret>" --interface utun20
```

### Step 5 — Verify the mesh

On any node:

```bash
wgmesh peers list
```

Expected output (~20 seconds after starting):

```
PUBKEY                                          MESH IP         ENDPOINT              LAST SEEN
AbCdEfGhIjKl...=                               10.47.23.1      203.0.113.10:51820    3s ago
XyZaBcDeFgHi...=                               10.47.23.2      (relayed)             7s ago
```

Ping across the mesh:

```bash
ping 10.47.23.1   # replace with the mesh IP shown in peers list
```

### Step 6 — Expose an internal service

On `vps`, expose a private staging API on the mesh IP only:

```bash
# Example: PostgreSQL listening on the mesh interface
sudo -u postgres psql -c "LISTEN *"  # already listening on all; restrict in pg_hba.conf:
# host mydb myuser 10.47.0.0/16 scram-sha-256
```

Developers connect using the VPS mesh IP:

```bash
psql -h 10.47.23.1 -U myuser mydb
```

---

## Outcomes

| Metric | Before wgmesh | After wgmesh |
|--------|--------------|--------------|
| Time to onboard a new developer | 15–30 min (key exchange, config push) | < 2 min (share secret) |
| Connectivity after IP change | Manual config update required | Automatic (DHT re-announces) |
| Access revocation | Update every peer config | Rotate secret and restart all daemons |
| Operator toil per join/leave event | ~10 min per event | 0 min |

**First ping latency (same region):** < 5 ms over the WireGuard tunnel.  
**NAT traversal success rate:** ~85% for typical home/office NAT; remaining 15% fall back to relay path with < 50 ms overhead.

---

## Cleanup

```bash
# Stop daemon on each node
sudo systemctl stop wgmesh

# Remove interface and state (optional)
sudo ip link delete wg0
sudo rm /var/lib/wgmesh/wg0.json
```
