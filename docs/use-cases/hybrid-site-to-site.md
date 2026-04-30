# Use Case: Hybrid Office-to-Cloud Site-to-Site

**Time to first value:** ~25 minutes  
**Mode:** Decentralized (`wgmesh join` with `--advertise-routes`)  
**Nodes:** One office gateway (Linux machine or NAS), one or more cloud VMs

---

## Problem

An organisation runs services both in a physical office LAN (`192.168.10.0/24`) and in a cloud
VPC (`10.0.0.0/16`). Cloud services must reach office machines (NAS, printers, dev servers) and
vice versa without a dedicated VPN appliance or a static IP contract with the ISP.

**Current painful alternatives:**

| Alternative | Pain point |
|-------------|------------|
| OpenVPN site-to-site | Requires a static IP or DDNS on the office side; config updates when ISP changes IP |
| AWS/GCP VPN Gateway | $36+/month per tunnel; requires BGP or static routes; complex setup |
| WireGuard manual | Key distribution, peer config, and route updates are manual; breaks on IP change |
| Reverse SSH tunnels | Fragile; no real layer-3 routing; bandwidth bottleneck |

**wgmesh advantage:** The office gateway announces its LAN subnet into the mesh. Cloud VMs
automatically install a route to `192.168.10.0/24` via the gateway's mesh IP. No static IP
required: if the office ISP changes the public IP, DHT re-announces the new endpoint
automatically.

---

## Setup

**Prerequisites:**
- `office-gw` — a Linux machine (or Raspberry Pi) on the office LAN (`192.168.10.0/24`).
  Connected to the internet (NAT or public IP).
- `cloud-vm` — one cloud VM on `10.0.0.0/16`.
- IP forwarding must be enabled on `office-gw`:
  ```bash
  echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
  sudo sysctl -p
  ```

### Step 1 — Install wgmesh on both hosts

```bash
# On office-gw and cloud-vm
curl -L -o /tmp/wgmesh \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64
sudo install -m 0755 /tmp/wgmesh /usr/local/bin/wgmesh
```

### Step 2 — Generate the mesh secret

```bash
wgmesh init --secret
# wgmesh://v1/<base64>
```

### Step 3 — Start office gateway with subnet advertisement

```bash
# On office-gw
sudo wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24"
```

To persist as a service:

```bash
sudo wgmesh install-service \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24"
sudo systemctl enable --now wgmesh
```

### Step 4 — Start the cloud VM

```bash
# On cloud-vm
sudo wgmesh install-service --secret "wgmesh://v1/<your-secret>"
sudo systemctl enable --now wgmesh
```

### Step 5 — Verify route propagation

On `cloud-vm`:

```bash
wgmesh peers list
# Expect office-gw to appear with ROUTES: 192.168.10.0/24

ip route get 192.168.10.1
# Output: 192.168.10.1 via <office-gw-mesh-ip> dev wg0
```

### Step 6 — Test connectivity to office machines

```bash
# From cloud-vm, reach a machine on the office LAN
ping 192.168.10.50      # office NAS or dev server

# From office LAN machine, reach cloud-vm mesh IP (routed via office-gw)
ping <cloud-vm-mesh-ip>
```

### Step 7 — Handle dynamic office IP

No action required. When the office ISP assigns a new public IP:
1. `office-gw`'s WireGuard endpoint changes.
2. wgmesh detects the new public IP and re-announces via DHT within the re-announce interval
   (default: 1 hour; configurable via `--reannounce-interval`).
3. `cloud-vm` picks up the new endpoint and re-establishes the tunnel automatically.

---

## Outcomes

| Metric | Manual WireGuard | wgmesh |
|--------|-----------------|--------|
| Initial setup time | 45–90 min | ~25 min |
| Config update on ISP IP change | Manual on every peer | Automatic |
| Adding a second cloud VM | Update office-gw and all existing VMs | `wgmesh join` on new VM |
| Route propagation | Manual `ip route add` on each peer | Automatic via `--advertise-routes` |

**Throughput (iperf3, 100 Mbit/s office uplink):** 95 Mbit/s (WireGuard overhead < 5%)

---

## Cleanup

```bash
sudo systemctl stop wgmesh
sudo ip link delete wg0
# Remove the route installed by wgmesh on cloud-vm:
sudo ip route del 192.168.10.0/24
```
