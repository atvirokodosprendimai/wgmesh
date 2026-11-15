# Centralized Mode (SSH Deployment)

Use centralized mode to manage WireGuard across your fleet from a single control node via SSH.
Configs are stored in a state file and deployed diff-based — only changes are applied.
Operator notes for changes to deploy behaviour (validation, per-node results, endpoints): [centralized-rollout.md](centralized-rollout.md).

## 1. Initialize a new mesh

```bash
wgmesh -init
```

This creates a `/var/lib/wgmesh/mesh-state.json` file with default settings:
- Interface name: `wg0`
- Mesh network: `10.99.0.0/16`
- Listen port: `51820`

## 2. Add nodes to the mesh

```bash
# Format: hostname:mesh_ip:ssh_host[:ssh_port]
wgmesh -add node1:10.99.0.1:192.168.1.10
wgmesh -add node2:10.99.0.2:203.0.113.50
wgmesh -add node3:10.99.0.3:198.51.100.20:2222
```

- `hostname`: Node identifier (should match the actual hostname)
- `mesh_ip`: IP address within the mesh network
- `ssh_host`: SSH connection address (can be IP or hostname)
- `ssh_port`: SSH port (optional, defaults to 22)

## 3. List nodes

```bash
wgmesh -list
```

Output:
```
Mesh Network: 10.99.0.0/16
Interface: wg0
Listen Port: 51820

Nodes:
  node1 (local):
    Mesh IP: 10.99.0.1
    SSH: 192.168.1.10:22
    Public Key: AbCd...Ef12
    Endpoint: 192.168.1.10:51820

  node2 [NAT]:
    Mesh IP: 10.99.0.2
    SSH: 203.0.113.50:22
    Public Key: GhIj...Kl34
```

## 4. Deploy configuration

```bash
wgmesh -deploy
```

This will:
1. Check the state file and stop before any connection when a value is unsafe (see "Validation")
2. Connect to each node via SSH, in hostname order
3. Install WireGuard if not present
4. Detect public endpoints and NAT status (see "Endpoints and NAT")
5. Generate or update WireGuard configuration
6. Write configuration to `/etc/wireguard/wg0.conf`
7. Enable and start `wg-quick@wg0` systemd service
8. Apply changes using `wg set` commands for online updates (when possible)
9. Configure routing tables with routes to all mesh networks
10. Print one summary of the nodes that failed, with the phase of each

**Configuration persists across reboots** via systemd service.

### Validation

`-deploy` checks the state file before it opens a connection. A value that a shell or a wg-quick file would give a meaning to (class A: a character outside `[A-Za-z0-9._:/,@%+=\[\]-]`, a control character, a space, a leading `-`, an empty interface name, a missing `mesh_ip`) is refused, and the error names the field, the node and the rule. A value that only parses wrongly (class B: a bad CIDR, port, key or endpoint, a duplicate mesh IP) prints a `Warning: state:` line and is refused only in a later release, when `mesh.StrictState` is set (N+1). `-list`, `-remove` and `mesh list` do not validate. Every value sent to a node is also quoted for the shell. The go/no-go for the later release: [centralized-rollout.md](centralized-rollout.md).

### Endpoints and NAT

A node's endpoint is decided in this order, and the deploy prints the reason:
1. A `public_endpoint` you wrote in the state file wins and is not overwritten.
2. When every node's SSH host is a private IP address, each node's endpoint is `<ssh host>:<listen port>`, the local node included.
3. When the node's detected public IP equals its SSH host, the endpoint is `<ssh host>:<listen port>`.
4. Otherwise the node is behind NAT and has no endpoint.

A DNS name used as `ssh_host` is never treated as private: a LAN mesh that you reach by name needs `public_endpoint` on every node that others should dial. In a mixed mesh (some private, some public SSH hosts) a node with a private SSH host has no endpoint unless you set `public_endpoint`. A node with a private SSH host is not asked for its public IP. The detected endpoint is used for that run only; `-deploy` does not write it to the state file.

### When a node fails

A node that cannot be reached or configured does not stop the run. The other nodes are configured, and the run ends with one error that lists each failed node with its phase (`detect`, `connect`, `install`, `apply`, `routes` or `persist`) and the nodes that were configured, and exits 1:

```
Failed to deploy: deploy failed on 1 of 3 nodes
  node2: detect: <error>
configured: node1, node3
```

Fix the node and run `wgmesh -deploy` again: each node is diffed against its live config, so a second run converges. A config file that cannot be written is printed in a `Warnings:` block and does not fail the node until `mesh.PersistFailureFatal` is set (N+2). The two later flips and what to check before each: [centralized-rollout.md](centralized-rollout.md).

## 5. Remove a node

```bash
wgmesh -remove node3
wgmesh -deploy
```

## Advanced Options

### Custom State File

```bash
wgmesh -state /path/to/custom-state.json -list
```

### Encrypted State File

Encrypt the mesh state file to protect private keys. The file will be AES-256-GCM encrypted and base64-encoded, making it safe to store in vaults.

```bash
# Initialize with encryption (asks for password twice)
wgmesh --encrypt -init
Enter encryption password: ********
Confirm password: ********

# All operations require the password when using --encrypt
wgmesh --encrypt --add node1:10.99.0.1:192.168.1.10
Enter encryption password: ********

wgmesh --encrypt --list
Enter encryption password: ********

wgmesh --encrypt --deploy
Enter encryption password: ********
```

**Encrypted file format:**
```
U2FsdGVkX1+Qq1RZNlBXMTJHVzR4TVRrMllXNWpaVzkxZEdWd0FsSnZibk5hY0dWaGRHbHZi...
(base64-encoded encrypted data)
```

**Security features:**
- AES-256-GCM authenticated encryption
- PBKDF2 key derivation (100,000 iterations)
- Random 32-byte salt per encryption
- Base64-encoded output (vault-friendly)

**Store in vault:**
```bash
# HashiCorp Vault
vault kv put secret/wgmesh state=@/var/lib/wgmesh/mesh-state.json

# Retrieve and use
vault kv get -field=state secret/wgmesh > /var/lib/wgmesh/mesh-state.json
wgmesh --encrypt --list
```

### Adding Routes for Networks Behind Nodes

Edit the `/var/lib/wgmesh/mesh-state.json` file and add routable networks to a node:

```json
{
  "nodes": {
    "node1": {
      "hostname": "node1",
      "mesh_ip": "10.99.0.1",
      "routable_networks": ["192.168.10.0/24", "192.168.20.0/24"],
      ...
    }
  }
}
```

After editing, run `wgmesh -deploy` to apply the changes.

**What happens:**
- `node1` gets no mesh routes for its own networks (it is their gateway); a stale `<net> dev wg0` route left by older versions is removed
- All other nodes get routes via node1's mesh IP: `ip route add 192.168.10.0/24 via 10.99.0.1 dev wg0`
- Routes are added to both the live routing table and the persistent config file
- If you remove a network from `routable_networks`, it will be automatically cleaned up from all nodes on the next deploy
- A network a node advertises itself is never routed through the mesh: HA nodes that share a subnet no longer install routes to it via each other
- When a deploy removes a route `<own network> via <peer> dev wg0` (the HA route of releases before the shared-subnet fix), it puts the node's connected route for that network back, whatever the gateway, and prints `Restored connected route: <net> dev wg0`; it needs a local address in that network, otherwise it warns. An older binary does not restore the connected route on later removals
- Only the exact network is matched: an own `/16` does not drop a peer `/24`
- The default route follows the same rule: when two HA exit nodes both advertise `0.0.0.0/0`, each drops the other's default route
- Upgrade note: nodes still on an older binary keep routing the shared subnet via the peer until they are upgraded

## State File Format

The `/var/lib/wgmesh/mesh-state.json` file stores the complete mesh state:

```json
{
  "interface_name": "wg0",
  "network": "10.99.0.0/16",
  "listen_port": 51820,
  "local_hostname": "control-node",
  "nodes": {
    "node1": {
      "hostname": "node1",
      "mesh_ip": "10.99.0.1",
      "public_key": "base64-encoded-public-key",
      "private_key": "base64-encoded-private-key",
      "ssh_host": "192.168.1.10",
      "ssh_port": 22,
      "listen_port": 51820,
      "public_endpoint": "192.168.1.10:51820",
      "behind_nat": false,
      "routable_networks": [],
      "is_local": true
    }
  }
}
```

The `base64-encoded-...` keys are placeholders. A real state file holds 44-character base64 keys, and a placeholder key is a class B finding (a warning at first, a refusal once `StrictState` is set). The `public_endpoint` here was written by the operator, so it is used as it is.

## SSH Key Authentication

The tool attempts authentication in this order:
1. SSH agent (if `SSH_AUTH_SOCK` is set)
2. `~/.ssh/id_rsa`
3. `~/.ssh/id_ed25519`
4. `~/.ssh/id_ecdsa`

Ensure your SSH keys are added to the `authorized_keys` file on target hosts.

**Host keys:** The SSH client verifies each node's host key against `~/.ssh/known_hosts` (override the file with `WGMESH_SSH_KNOWN_HOSTS`). A node that is not in the file is refused with its fingerprint and the `ssh-keyscan` command to add it, and a node whose key changed is refused as a possible man-in-the-middle. A deploy sends each node's WireGuard private key over this connection, so check the fingerprint with the host's owner before adding it. `WGMESH_SSH_INSECURE_HOST_KEY=1` turns verification off for a lab and prints a warning; do not use it in production.

**Upgrading:** versions before this change accepted any host key. After upgrading, the first `wgmesh -deploy` configures every node that is in `known_hosts`, lists each node that is not as failed in phase `detect` and exits 1; add them as described above (or SSH to each once and accept the key after checking its fingerprint) and run `wgmesh -deploy` again. Recording only some of a node's key types is enough: wgmesh asks each node for a key type that `known_hosts` already holds for it.
