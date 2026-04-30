# Use Case: Managed Fleet (Centralized SSH)

**Time to first value:** ~20 minutes  
**Mode:** Centralized (`wgmesh -deploy`)  
**Nodes:** Any number of Linux servers accessible via SSH from a single control host

---

## Problem

An operations team manages a fleet of servers (web hosts, database nodes, cache servers) and
needs to maintain WireGuard tunnels between them. The team requires:

- A single source of truth for the mesh topology.
- Atomic, diff-based config updates (no interface restarts in production).
- The ability to add/remove nodes from a central control machine without touching individual servers.
- Optionally: an encrypted topology file for storing sensitive key material at rest.

**Current painful alternatives:**

| Alternative | Pain point |
|-------------|------------|
| Ansible / Terraform WireGuard roles | Require re-running the full playbook on every topology change; restart-based config updates cause brief connectivity loss |
| Manual `wg set` scripts | Error-prone; no state tracking; hard to audit what changed |
| Decentralized mode | Nodes self-manage — no central operator control over who joins or what topology looks like |

**wgmesh advantage:** The operator edits one state file and runs `wgmesh -deploy`. Only the
changed peers and routes are updated on each node via `wg set` — no interface restart, no
connectivity disruption.

---

## Setup

**Prerequisites:**
- A **control node** with SSH access to all fleet nodes. The control node does not need to be in
  the mesh itself.
- Three **fleet nodes**: `node1` (10.99.0.1), `node2` (10.99.0.2), `node3` (10.99.0.3).
- SSH key authentication already configured from the control node to all fleet nodes.
- wgmesh installed on the **control node only** (fleet nodes need only `wireguard-tools`).

### Step 1 — Install wgmesh on the control node

```bash
curl -L -o /tmp/wgmesh \
  https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh_linux_amd64
sudo install -m 0755 /tmp/wgmesh /usr/local/bin/wgmesh
wgmesh version
```

### Step 2 — Initialize the mesh state

```bash
wgmesh -init
```

This creates `/var/lib/wgmesh/mesh-state.json` with default settings:
- Interface: `wg0`
- Mesh network: `10.99.0.0/16`
- Listen port: `51820`

### Step 3 — Add fleet nodes

```bash
# Format: hostname:mesh_ip:ssh_host[:ssh_port]
wgmesh -add node1:10.99.0.1:192.168.1.10
wgmesh -add node2:10.99.0.2:203.0.113.50
wgmesh -add node3:10.99.0.3:198.51.100.20
```

Verify:

```bash
wgmesh -list
```

Expected output:

```
Mesh Network: 10.99.0.0/16
Interface: wg0
Listen Port: 51820

Nodes:
  node1 (local):
    Mesh IP: 10.99.0.1
    SSH: 192.168.1.10:22
  node2 [NAT]:
    Mesh IP: 10.99.0.2
    SSH: 203.0.113.50:22
  node3:
    Mesh IP: 10.99.0.3
    SSH: 198.51.100.20:22
```

### Step 4 — Deploy the mesh

```bash
wgmesh -deploy
```

This command:
1. SSHes into each node.
2. Reads the current WireGuard state (`wg show dump`).
3. Calculates a diff vs. the desired state.
4. Applies only the changes with `wg set` (no restart).
5. Adds/removes routes with `ip route`.

### Step 5 — Add a node and redeploy

```bash
wgmesh -add node4:10.99.0.4:203.0.113.75
wgmesh -deploy
```

Only `node4` and the `allowed-ips` diff on existing nodes are updated. Existing tunnels remain
uninterrupted.

### Step 6 — Remove a node and redeploy

```bash
wgmesh -remove node2
wgmesh -deploy
```

`node2`'s WireGuard interface is torn down; the peer entry is removed from all remaining nodes.

### Step 7 — (Optional) Encrypt the state file

```bash
wgmesh -encrypt
# Enter passphrase: ••••••••
# State file re-written with AES-256-GCM + PBKDF2
```

Subsequent `-deploy` calls will prompt for the passphrase.

---

## Outcomes

| Metric | Ansible WireGuard role | wgmesh centralized |
|--------|----------------------|-------------------|
| Config update causes restart | Yes (interface down/up) | No (live `wg set`) |
| Adding a node | Edit vars + run full playbook | `wgmesh -add` + `-deploy` |
| Time to deploy 10-node change | ~3 min (full Ansible run) | ~30 sec |
| State file format | YAML (human-readable) | JSON (human-readable), optionally encrypted |
| Key generation | External (vault or manual) | Automatic per node |

**Deployment speed:** 10-node mesh initial setup in under 2 minutes from `wgmesh -init` to all
tunnels established.  
**Update latency:** Diff-based `wg set` completes in < 100 ms per node over LAN.

---

## Cleanup

```bash
# Tear down the mesh on all nodes
wgmesh -deploy --teardown     # removes WireGuard interfaces and configs on all nodes

# Or manually on each fleet node:
sudo wg-quick down wg0
sudo ip link delete wg0
```
