# Centralized Mode (SSH Deployment)

Use centralized mode to manage WireGuard across your fleet from a single control node via SSH.
Configs are stored in a state file and deployed diff-based — only changes are applied.

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
1. Connect to each node via SSH
2. Install WireGuard if not present
3. Detect public endpoints and NAT status
4. Generate or update WireGuard configuration
5. Write configuration to `/etc/wireguard/wg0.conf`
6. Enable and start `wg-quick@wg0` systemd service
7. Apply changes using `wg set` commands for online updates (when possible)
8. Configure routing tables with routes to all mesh networks

**Configuration persists across reboots** via systemd service.

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
- `node1` gets direct routes: `ip route add 192.168.10.0/24 dev wg0` and `ip route add 192.168.20.0/24 dev wg0`
- All other nodes get routes via node1's mesh IP: `ip route add 192.168.10.0/24 via 10.99.0.1 dev wg0`
- Routes are added to both the live routing table and the persistent config file
- If you remove a network from `routable_networks`, it will be automatically cleaned up from all nodes on the next deploy

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

## SSH Key Authentication

The tool attempts authentication in this order:
1. SSH agent (if `SSH_AUTH_SOCK` is set)
2. `~/.ssh/id_rsa`
3. `~/.ssh/id_ed25519`
4. `~/.ssh/id_ecdsa`

Ensure your SSH keys are added to the `authorized_keys` file on target hosts.

**Security note:** The SSH client currently uses `InsecureIgnoreHostKey`, which skips host key verification. This means the tool does not verify the identity of remote hosts and is vulnerable to man-in-the-middle attacks. For production deployments, consider implementing proper host key verification using `known_hosts` files or pinned host keys.
