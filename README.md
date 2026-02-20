<a href="https://viberank.dev/apps/wgmesh" target="_blank" rel="noopener noreferrer"><img src="https://viberank.dev/badge?app=wgmesh&theme=dark" alt="wgmesh on VibeRank" /></a>
<a href="https://www.producthunt.com/products/wgmesh?embed=true&amp;utm_source=badge-featured&amp;utm_medium=badge&amp;utm_campaign=badge-wgmesh" target="_blank" rel="noopener noreferrer"><img alt="wgmesh - Decentralized WireGuard mesh builder with DHT discovery | Product Hunt" width="250" height="54" src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1081094&amp;theme=light&amp;t=1771444856938"></a>
[![Chimney Deploy](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml/badge.svg)](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml)
# WireGuard Mesh Builder

A Go-based tool for building and managing WireGuard mesh networks with support for NAT traversal, automatic endpoint detection, and incremental configuration updates.

## Features

- **Automatic Mesh Network Creation**: Builds full mesh topology where every node can communicate with every other node
- **Access Control**: Group-based network segmentation with policy-based access rules
- **NAT Detection**: Automatically detects nodes behind NAT and configures persistent keepalive
- **SSH-Based Deployment**: Installs and configures WireGuard on remote Ubuntu hosts via SSH
- **Incremental Updates**: Uses `wg set` commands for online configuration changes without restarting interfaces
- **Key Management**: Generates and stores WireGuard key pairs (centralized mode: in the mesh state file specified via the `-state` flag, default `/var/lib/wgmesh/mesh-state.json`; decentralized mode: in `/var/lib/wgmesh/`)
- **Routing Table Management**: Automatically configures routes for networks behind mesh nodes on all nodes
- **Diff-Based Deployment**: Only applies configuration changes, minimizing disruption
- **Persistent Configuration**: Uses systemd and wg-quick for automatic startup after reboot
- **Persistent State**: Stores mesh configuration in JSON format

## Prerequisites

- Go 1.23 or later
- WireGuard tools (`wg` command) on the machine running wgmesh
- SSH access to all nodes (root or sudo privileges required)
- Ubuntu-based target systems (tested on Ubuntu 20.04+)

## Installation

### Pre-built Binaries

Download pre-built binaries for your platform from the [releases page](https://github.com/atvirokodosprendimai/wgmesh/releases).

Available architectures:
- **Linux amd64** (x86-64)
- **Linux arm64** (ARM 64-bit)
- **Linux armv7** (ARM 32-bit)

```bash
# Example for Linux amd64
wget https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/wgmesh-linux-amd64
chmod +x wgmesh-linux-amd64
sudo mv wgmesh-linux-amd64 /usr/local/bin/wgmesh
```

### From Source

```bash
git clone https://github.com/atvirokodosprendimai/wgmesh.git
cd wgmesh
go build -o wgmesh
```

### Using Docker

Docker images are automatically built for multiple architectures (amd64, arm64, arm/v7) and are available from GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/atvirokodosprendimai/wgmesh:latest

# Or pull a specific version
docker pull ghcr.io/atvirokodosprendimai/wgmesh:v1.0.0

# Run wgmesh in a container
docker run --rm ghcr.io/atvirokodosprendimai/wgmesh:latest --help

# Show version information
docker run --rm ghcr.io/atvirokodosprendimai/wgmesh:latest --version

# Run with state file mounted
docker run --rm -v $(pwd)/data:/data ghcr.io/atvirokodosprendimai/wgmesh:latest -state /data/mesh-state.json -list
```

**Note**: For full WireGuard functionality, the container needs privileged access and network host mode:

```bash
docker run --rm --privileged --network host \
  -v $(pwd)/wgmesh-state:/var/lib/wgmesh \
  ghcr.io/atvirokodosprendimai/wgmesh:latest join \
  --secret "wgmesh://v1/<your-secret>"
```

### Using Docker Compose

For easier deployment and management, use Docker Compose:

```bash
# Copy environment template
cp .env.example .env

# Edit .env and set your MESH_SECRET
nano .env

# Start the mesh node
docker-compose up -d

# View logs
docker-compose logs -f wgmesh-node
```

See [DOCKER-COMPOSE.md](DOCKER-COMPOSE.md) for detailed documentation and advanced configurations.

## Version Information

To display the current version of wgmesh:

```bash
wgmesh version          # Using the version subcommand
wgmesh --version        # Using the long version flag
wgmesh -v              # Using the short version flag
```

Both the `version` subcommand and the `--version`/`-v` flags display version information. The version flags take priority over all other flags and subcommands, so you can use them even with other arguments:

```bash
wgmesh --version --help    # Shows version, ignores --help
wgmesh -v join             # Shows version, ignores subcommand
```

## Quick Start

### Decentralized Mode (Secret-Based Discovery)

Use this mode when you want nodes to self-discover and peer automatically via DHT.

```bash
# 1) Generate a mesh secret (run once)
./wgmesh init --secret

# 2) Join on each node using the same secret
./wgmesh join --secret "wgmesh://v1/<your-secret>"

# 3) Check local derived mesh parameters
./wgmesh status --secret "wgmesh://v1/<your-secret>"
```

Common `join` options:

```bash
./wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug \
  --gossip
```

**Query Running Daemon:**

Once the daemon is running, you can query it for peer information:

```bash
# List all active peers
./wgmesh peers list

# Show peer counts
./wgmesh peers count

# Get specific peer details
./wgmesh peers get <pubkey>
```

The RPC socket is automatically created at:
- `/var/run/wgmesh.sock` (if running as root)
- `$XDG_RUNTIME_DIR/wgmesh.sock` (if running as non-root)
- `/tmp/wgmesh.sock` (fallback)

You can override the socket path with `--socket-path` flag on `join` or `WGMESH_SOCKET` environment variable.

You can also test direct encrypted peer exchange between two nodes:

```bash
./wgmesh test-peer --secret "wgmesh://v1/<your-secret>" --peer <ip:port>
```

### Centralized Mode (SSH Deployment)

### 1. Initialize a new mesh

```bash
./wgmesh -init
```

This creates a `/var/lib/wgmesh/mesh-state.json` file with default settings:
- Interface name: `wg0`
- Mesh network: `10.99.0.0/16`
- Listen port: `51820`

### 2. Add nodes to the mesh

```bash
# Format: hostname:mesh_ip:ssh_host[:ssh_port]
./wgmesh -add node1:10.99.0.1:192.168.1.10
./wgmesh -add node2:10.99.0.2:203.0.113.50
./wgmesh -add node3:10.99.0.3:198.51.100.20:2222
```

- `hostname`: Node identifier (should match the actual hostname)
- `mesh_ip`: IP address within the mesh network
- `ssh_host`: SSH connection address (can be IP or hostname)
- `ssh_port`: SSH port (optional, defaults to 22)

### 3. List nodes

```bash
./wgmesh -list
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

### 4. Deploy configuration

```bash
./wgmesh -deploy
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

### 5. Remove a node

```bash
./wgmesh -remove node3
./wgmesh -deploy
```

## Advanced Usage

### Custom State File

```bash
./wgmesh -state /path/to/custom-state.json -list
```

### Encrypted State File

Encrypt the mesh state file to protect private keys. The file will be AES-256-GCM encrypted and base64-encoded, making it safe to store in vaults.

```bash
# Initialize with encryption (asks for password twice)
./wgmesh --encrypt -init
Enter encryption password: ********
Confirm password: ********

# All operations require the password when using --encrypt
./wgmesh --encrypt --add node1:10.99.0.1:192.168.1.10
Enter encryption password: ********

./wgmesh --encrypt --list
Enter encryption password: ********

./wgmesh --encrypt --deploy
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
./wgmesh --encrypt --list
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

After editing, run `./wgmesh -deploy` to apply the changes.

**What happens:**
- `node1` gets direct routes: `ip route add 192.168.10.0/24 dev wg0` and `ip route add 192.168.20.0/24 dev wg0`
- All other nodes get routes via node1's mesh IP: `ip route add 192.168.10.0/24 via 10.99.0.1 dev wg0`
- Routes are added to both the live routing table and the persistent config file
- If you remove a network from `routable_networks`, it will be automatically cleaned up from all nodes on the next deploy

### Access Control and Network Segmentation

wgmesh supports group-based access control in centralized mode, allowing you to segment your mesh network and control which nodes can communicate with each other.

#### Overview

Without access control, every node in the mesh can communicate with every other node (full mesh topology). With access control enabled:
- Nodes are organized into **groups**
- **Policies** define which groups can communicate
- WireGuard `AllowedIPs` filtering enforces the policies
- Nodes can only reach peers that have at least one policy connecting their groups

#### Defining Groups

Groups are defined in the `/var/lib/wgmesh/mesh-state.json` file:

```json
{
  "groups": {
    "production": {
      "description": "Production environment nodes",
      "members": ["web1", "web2", "app1", "db1"]
    },
    "staging": {
      "description": "Staging environment",
      "members": ["web3", "app2", "db2"]
    },
    "database": {
      "description": "Database servers",
      "members": ["db1", "db2"]
    }
  }
}
```

- `description`: Optional human-readable description
- `members`: List of node hostnames that belong to this group

#### Defining Access Policies

Policies define communication rules between groups:

```json
{
  "access_policies": [
    {
      "name": "prod-to-db",
      "description": "Allow production nodes to access databases",
      "from_groups": ["production"],
      "to_groups": ["database"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "prod-internal",
      "description": "Production nodes can talk to each other",
      "from_groups": ["production"],
      "to_groups": ["production"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "staging-isolated",
      "description": "Staging is isolated from production",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ]
}
```

Policy fields:
- `name`: Unique policy identifier
- `description`: Optional description
- `from_groups`: Groups initiating the connection
- `to_groups`: Groups being accessed
- `allow_mesh_ips`: Allow access to mesh IPs (direct node-to-node communication)
- `allow_routable_networks`: Allow access to networks behind target nodes

#### How Policy Evaluation Works

For each node, wgmesh evaluates policies to determine which peers to configure:

1. **Find node's groups**: Collect all groups where this node is a member
2. **Determine relevant policies**:
   - **Outbound policies**: Policies where the node's groups appear in `from_groups`
   - **Inbound policies**: Policies where the node's groups appear in `to_groups`
3. **Build peer list**: For every policy that connects the node's groups to other groups, add all members of those other groups as WireGuard peers
4. **Configure AllowedIPs**:
   - Always include mesh IP (`/32`) for handshakes when peer is configured
   - Include mesh IP as destination if `allow_mesh_ips: true` in outbound policy
   - Include `routable_networks` if `allow_routable_networks: true` in outbound policy
5. **Deny-by-default**: If no policies connect the node's groups to another node's groups, no peer configuration is created

**Important**: Policies are evaluated bidirectionally for peer configuration. If any policy relates two nodes' groups (in either direction), both nodes will have each other configured as peers. However, the `AllowedIPs` settings are directional based on outbound policies.

#### Example: Three-Tier Architecture

**Scenario**: Production web and app servers need to reach databases, but staging must be isolated.

```json
{
  "groups": {
    "web": {
      "members": ["web1", "web2"]
    },
    "app": {
      "members": ["app1"]
    },
    "db": {
      "members": ["db1"]
    },
    "staging": {
      "members": ["web3", "app2", "db2"]
    }
  },
  "access_policies": [
    {
      "name": "web-to-app",
      "from_groups": ["web"],
      "to_groups": ["app"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    },
    {
      "name": "app-to-db",
      "from_groups": ["app"],
      "to_groups": ["db"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "db-to-app",
      "from_groups": ["db"],
      "to_groups": ["app"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    },
    {
      "name": "staging-isolated",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ]
}
```

**Result**:
- `web1` can reach `app1` but not `db1` directly (must go through `app1`)
- `app1` can reach `web1`, `web2`, and `db1`
- `db1` can reach `app1` (for responses) but not `web1` or `web2`
- `web3`, `app2`, `db2` can only reach each other (isolated from production)
- Staging cannot reach production and vice versa

#### Example: Hub-and-Spoke

**Scenario**: Field offices should only communicate with HQ, not each other.

```json
{
  "groups": {
    "hq": {
      "members": ["hq1", "hq2"]
    },
    "office": {
      "members": ["office1", "office2", "office3"]
    }
  },
  "access_policies": [
    {
      "name": "office-to-hq",
      "from_groups": ["office"],
      "to_groups": ["hq"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "hq-to-office",
      "from_groups": ["hq"],
      "to_groups": ["office"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    }
  ]
}
```

**Result**:
- `office1` can reach `hq1` and `hq2` (and their routable networks)
- `office2` cannot reach `office1` or `office3`
- `hq1` can reach all offices (for responses to connections initiated from offices)
- Offices are isolated from each other for security

#### Viewing Access Control Configuration

Use the `-list` command to see groups, policies, and memberships:

```bash
./wgmesh -list
```

Output:
```
Mesh Network: 10.99.0.0/16
Interface: wg0
Listen Port: 51820

Access Control: Enabled

Groups:
  production (2 members):
    - web1
    - web2
  
  staging (1 member):
    - web3

Policies:
  prod-internal:
    From: production
    To: production
    Mesh IPs: Yes
    Routable Networks: Yes

Nodes:
  web1:
    Mesh IP: 10.99.0.1
    Groups: [production]
    SSH: 192.168.1.10:22

  web2:
    Mesh IP: 10.99.0.2
    Groups: [production]
    SSH: 192.168.1.11:22

  web3:
    Mesh IP: 10.99.0.3
    Groups: [staging]
    SSH: 192.168.1.12:22
```

#### Validation and Warnings

When you run `./wgmesh -deploy`, wgmesh validates the access control configuration:

**Validation checks**:
- All group members must exist as nodes
- All group names in policies must exist
- Policies must have at least one `from_group` and one `to_group`
- No duplicate policy names
- No duplicate members within a group

**Warnings**:
- Groups defined without policies: "Groups exist but no access policies defined. Nodes in groups will have no peers."
- Nodes not in any group: "Node X is not a member of any group (when groups are defined)."

#### Backward Compatibility

**No groups or policies defined**: Full mesh topology (all nodes connect to all nodes)

**Groups defined but no policies**: Deny-by-default behavior. Nodes in groups will have no peers configured.

**Backward compatible**: Existing meshes without groups continue to work exactly as before.

#### Best Practices

1. **Always use policies with groups**: If you define groups, define at least one policy that references them
2. **Be explicit with policy direction**: Consider both `allow_mesh_ips` and `allow_routable_networks` flags
3. **Test isolated networks**: Verify that nodes that shouldn't communicate truly cannot reach each other
4. **Document your architecture**: Add clear descriptions to groups and policies
5. **Use descriptive names**: Choose group and policy names that reflect their purpose
6. **Plan for bidirectional access**: Remember that peer configuration is symmetric, but `AllowedIPs` are directional

#### Common Patterns

**Allow all access within a group**:
```json
{
  "name": "group-internal",
  "from_groups": ["groupname"],
  "to_groups": ["groupname"],
  "allow_mesh_ips": true,
  "allow_routable_networks": true
}
```

**Allow mesh access but not routable networks**:
```json
{
  "name": "limited-access",
  "from_groups": ["client"],
  "to_groups": ["server"],
  "allow_mesh_ips": true,
  "allow_routable_networks": false
}
```

**Read-only access (return traffic only)**:
```json
{
  "name": "read-only-response",
  "from_groups": ["server"],
  "to_groups": ["client"],
  "allow_mesh_ips": true,
  "allow_routable_networks": false
}
```

This configures the peer (for WireGuard handshakes) but doesn't allow the server to initiate new connections to the client's routable networks.

## How It Works

### Mesh Topology

Every node becomes a peer to every other node. For a 4-node mesh:

```
node1 <----> node2
  ^            ^
  |            |
  v            v
node3 <----> node4
```

### NAT Traversal

- Nodes with public IPs are configured as endpoints for other nodes
- Nodes behind NAT use persistent keepalive to maintain connections
- The tool detects NAT by comparing SSH host with detected public IP

### Persistence Across Reboots

The tool ensures configuration survives server reboots by:
1. Writing WireGuard configuration to `/etc/wireguard/wg0.conf` (wg-quick format)
2. Enabling the `wg-quick@wg0.service` systemd unit
3. Including `PostUp` commands in the config to:
   - Add routes for networks behind other mesh nodes (e.g., `ip route add 192.168.10.0/24 via 10.99.0.2`)
   - Enable IP forwarding (`sysctl -w net.ipv4.ip_forward=1`)
4. Including `PreDown` commands to clean up routes on shutdown

When the server reboots, systemd automatically:
- Brings up the WireGuard interface
- Restores all peer connections
- Re-applies all routing table entries

### Route Management and Cleanup

The tool intelligently manages routing tables:

1. **Reading Current Routes**: Uses `ip route show dev wg0` to get existing routes
2. **Calculating Diff**: Compares current vs desired routes
3. **Removing Stale Routes**: Automatically removes routes that are no longer in the mesh state
4. **Adding New Routes**: Adds routes for newly configured networks
5. **Persistence**: All routes are embedded in the config file via `PostUp` commands

**Example scenario:**
- You add `"routable_networks": ["192.168.10.0/24"]` to node1
- Deploy: All nodes get route `192.168.10.0/24 via 10.99.0.1`
- You remove that network from node1's config
- Deploy: All nodes automatically remove the stale route

### Online Updates

When deploying changes, the tool:
1. Reads current WireGuard configuration using `wg show dump`
2. Reads current routing table using `ip route show`
3. Calculates differences between current and desired state for both peers and routes
4. Updates the persistent configuration file
5. Applies changes using `wg set` commands for online updates:
   - Add new peers
   - Update existing peers
   - Remove old peers
6. Applies route changes using `ip route` commands:
   - Remove stale routes
   - Add new routes
7. Only restarts the interface if fundamental changes are required (e.g., IP address change)

### SSH Key Authentication

The tool attempts authentication in this order:
1. SSH agent (if `SSH_AUTH_SOCK` is set)
2. `~/.ssh/id_rsa`
3. `~/.ssh/id_ed25519`
4. `~/.ssh/id_ecdsa`

Ensure your SSH keys are added to the `authorized_keys` file on target hosts.

## Configuration File

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

## Security Considerations

- **Private keys in state file**: WireGuard private keys storage varies by mode:
  - **Centralized mode**: Keys stored in `/var/lib/wgmesh/mesh-state.json`
    - Without encryption: Use file permissions (`chmod 600`) and secure storage
    - With `--encrypt`: State file is AES-256-GCM encrypted and base64-encoded
    - **Recommended**: Always use `--encrypt` flag for production deployments
  - **Decentralized mode**: Each node stores its own keypair in `/var/lib/wgmesh/{interface}.json` (e.g., `/var/lib/wgmesh/wg0.json`)
    - File is created with `0600` permissions for security
    - Keys persist across daemon restarts
- **Password storage**: Never store encryption passwords in scripts or environment variables
- The tool uses `InsecureIgnoreHostKey` for SSH - consider implementing proper host key verification for production
- WireGuard traffic is encrypted end-to-end
- Root SSH access is required on target hosts - ensure SSH keys are properly secured

## Troubleshooting

### Connection Issues

```bash
# Test SSH connectivity
ssh root@node-address

# Check WireGuard status on a node
ssh root@node-address wg show
```

### Check Persistence

```bash
# Check if systemd service is enabled and running
ssh root@node-address systemctl status wg-quick@wg0

# View the persistent configuration file
ssh root@node-address cat /etc/wireguard/wg0.conf

# Check if service starts on boot
ssh root@node-address systemctl is-enabled wg-quick@wg0
```

### Check Interface and Routes

```bash
# Check interface status
ssh root@node-address ip addr show wg0

# Check routing table
ssh root@node-address ip route

# Test connectivity through mesh
ssh root@node-address ping -c 3 10.99.0.2
```

### View WireGuard Logs

```bash
# View systemd service logs
ssh root@node-address journalctl -u wg-quick@wg0 -n 50

# Follow logs in real-time
ssh root@node-address journalctl -u wg-quick@wg0 -f
```

### Test Reboot Persistence

```bash
# Reboot a node
ssh root@node-address reboot

# Wait for reboot, then check if WireGuard came back up
sleep 30
ssh root@node-address wg show
ssh root@node-address ip route | grep 192.168
```

### Rebuild Configuration

If something goes wrong, you can force a fresh configuration:
```bash
# On each node, stop and disable the service
ssh root@node-address systemctl stop wg-quick@wg0
ssh root@node-address systemctl disable wg-quick@wg0

# Then redeploy
./wgmesh -deploy
```

## Architecture

```
wgmeshbuilder/
├── main.go                      # CLI interface
├── pkg/
│   ├── mesh/
│   │   ├── types.go            # Data structures
│   │   ├── mesh.go             # Mesh management (add/remove/list)
│   │   └── deploy.go           # Deployment logic
│   ├── wireguard/
│   │   ├── keys.go             # Key generation
│   │   ├── config.go           # Config parsing and diffing
│   │   ├── apply.go            # Configuration application
│   │   └── convert.go          # Type conversions
│   └── ssh/
│       ├── client.go           # SSH connection management
│       └── wireguard.go        # Remote WireGuard operations

**State files (system-level, not in project directory):**
```
/var/lib/wgmesh/
└── mesh-state.json             # Mesh state (created on init)
```
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see LICENSE file for details

## Roadmap

- [ ] Support for multiple mesh networks
- [ ] Web UI for mesh management
- [ ] Monitoring and health checks
- [ ] Support for more Linux distributions
- [ ] IPv6 support
- [ ] Automatic key rotation
- [ ] Integration with service discovery systems
