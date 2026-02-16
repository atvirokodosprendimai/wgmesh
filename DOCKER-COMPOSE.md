# Docker Compose Setup

This document describes how to use Docker Compose to deploy wgmesh in decentralized mode.

## Overview

The `docker-compose.yml` file provides a simple way to deploy wgmesh nodes that automatically join a mesh network using a shared secret. This is ideal for:

- Quick local testing with multiple nodes
- Production deployments with consistent configuration
- Easy management of multiple mesh nodes

## Prerequisites

- Docker and Docker Compose installed
- A mesh secret (generated using `wgmesh init --secret`)
- Linux host with WireGuard kernel module support

## Quick Start

### 1. Generate a Mesh Secret

First, generate a mesh secret using the wgmesh CLI or Docker:

```bash
# Using local binary
./wgmesh init --secret

# Or using Docker
docker run --rm ghcr.io/atvirokodosprendimai/wgmesh:latest init --secret
```

This will output a secret in the format: `wgmesh://v1/<base64-encoded-secret>`

### 2. Configure Environment

Copy the example environment file and add your mesh secret:

```bash
cp .env.example .env
```

Edit `.env` and set your mesh secret:

```
MESH_SECRET=wgmesh://v1/your-actual-secret-here
LOG_LEVEL=info
```

**Important**: Never commit `.env` to version control as it contains sensitive information!

### 3. Start the Mesh Node

```bash
# Start a single node
docker-compose up -d wgmesh-node

# View logs
docker-compose logs -f wgmesh-node

# Check status
docker-compose exec wgmesh-node sh -c 'wgmesh status --secret "$MESH_SECRET"'
```

## Configuration Options

### Basic Configuration

The default configuration in `docker-compose.yml` includes:

- **Image**: `ghcr.io/atvirokodosprendimai/wgmesh:latest`
- **Network Mode**: `host` (required for WireGuard)
- **Capabilities**: `NET_ADMIN` and `SYS_MODULE` (for WireGuard interface management)
- **Persistent Storage**: `./data/node1:/data` volume mount
- **Interface**: `wg0` on port `51820`

### Advanced Options

You can customize the node configuration by modifying the `command` section:

```yaml
command: >
  join
  --secret "${MESH_SECRET}"
  --interface wg0
  --listen-port 51820
  --log-level debug
  --privacy
  --gossip
  --advertise-routes "192.168.1.0/24,10.0.0.0/16"
```

Available options:

- `--interface`: WireGuard interface name (default: `wg0`)
- `--listen-port`: UDP port for WireGuard (default: `51820`)
- `--log-level`: Logging verbosity: `debug`, `info`, `warn`, `error` (default: `info`)
- `--privacy`: Enable Dandelion++ privacy mode for peer announcements
- `--gossip`: Enable in-mesh gossip protocol
- `--advertise-routes`: Comma-separated list of CIDR routes to advertise (e.g., `192.168.1.0/24,10.0.0.0/16`)

## Running Multiple Nodes

### Local Testing with Multiple Nodes

For local testing, you can run multiple nodes on the same host by using different interfaces and ports:

```bash
# Uncomment the additional nodes in docker-compose.yml, then:
docker-compose up -d wgmesh-node wgmesh-node2 wgmesh-node3

# Each node uses a different interface (wg0, wg1, wg2) and port (51820, 51821, 51822)
```

### Production Multi-Node Setup

In production, run one instance per host:

```bash
# On host 1
docker-compose up -d wgmesh-node

# On host 2
docker-compose up -d wgmesh-node

# On host 3
docker-compose up -d wgmesh-node
```

All nodes with the same `MESH_SECRET` will automatically discover each other and form a mesh.

## Networking Requirements

### Required Capabilities

The container needs the following capabilities to manage WireGuard:

- `NET_ADMIN`: Create and configure network interfaces
- `SYS_MODULE`: Load kernel modules (if WireGuard is not built-in)

### Firewall Configuration

Ensure the WireGuard port is accessible:

```bash
# Ubuntu/Debian with ufw
sudo ufw allow 51820/udp

# CentOS/RHEL with firewalld
sudo firewall-cmd --permanent --add-port=51820/udp
sudo firewall-cmd --reload
```

### Port Forwarding (NAT)

If running behind NAT, forward UDP port 51820 (or your custom port) to the host.

## Management Commands

### Check Node Status

```bash
# Show mesh status
docker-compose exec wgmesh-node sh -c 'wgmesh status --secret "$MESH_SECRET"'

# View WireGuard interface
docker-compose exec wgmesh-node wg show wg0

# Check logs
docker-compose logs -f wgmesh-node
```

### Restart Node

```bash
docker-compose restart wgmesh-node
```

### Stop and Remove

```bash
# Stop containers
docker-compose down

# Remove persistent data (WARNING: this deletes peer state)
rm -rf ./data
```

## Security Considerations

### Mesh Secret Protection

- **Never commit** `.env` or mesh secrets to version control
- Use `.env.example` as a template
- Rotate secrets periodically using `wgmesh rotate-secret --current CURRENT_SECRET` (optionally with `--new` and `--grace`)
- Share secrets securely (encrypted channels only)

### Container Security

- Containers run as `root` by default (required for WireGuard operations)
- Uses specific capabilities (`NET_ADMIN`, `SYS_MODULE`) following the principle of least privilege
- Alternatively, you can use `privileged: true` for simpler configuration (less secure)
- Host network mode is required for WireGuard functionality
- Persistent data in `./data/` contains private keys - protect accordingly

### Network Isolation

- WireGuard provides end-to-end encryption between mesh nodes
- Enable `--privacy` mode for enhanced announcement privacy (Dandelion++ relay)
- Use `--gossip` for in-mesh peer discovery without external registries

## Troubleshooting

### Container Won't Start

**Check kernel module:**
```bash
# Verify WireGuard module is available
lsmod | grep wireguard

# Load module if missing
sudo modprobe wireguard
```

**Check permissions:**
```bash
# Ensure user can run containers with capabilities
docker run --rm --cap-add=NET_ADMIN alpine:latest echo "OK"
```

### Nodes Can't Connect

**Verify mesh secret:**
```bash
# Ensure all nodes use the same secret
docker-compose exec wgmesh-node sh -c 'echo $MESH_SECRET'
```

**Check firewall:**
```bash
# Test UDP port connectivity
sudo nc -uvz <peer-ip> 51820
```

**Review logs:**
```bash
# Enable debug logging
# Set LOG_LEVEL=debug in .env, then restart
docker-compose logs -f wgmesh-node
```

### State Corruption

**Reset node state:**
```bash
# Stop container
docker-compose down

# Clear persistent data
rm -rf ./data/node1

# Restart
docker-compose up -d wgmesh-node
```

## Architecture Notes

### State Directory: /var/lib/wgmesh

wgmesh stores persistent state in `/var/lib/wgmesh/` inside the container. To preserve this state across container restarts and upgrades, this directory is mounted as a volume from the host system.

The `/var/lib/wgmesh/` directory contains:

| File | Purpose |
|------|---------|
| `{interface}.json` | WireGuard keypair (public + private keys) - **CRITICAL for node identity** |
| `{interface}-peers.json` | Cached peer data with 24-hour expiration |
| `{interface}-dht.nodes` | DHT bootstrap nodes cache |

### Why Persistence Matters

Without volume persistence, deploying a new container image would:

1. **Generate new WireGuard keys** - Node gets a new identity and mesh IP
2. **Lose peer connections** - Other nodes still have the old public key
3. **Lose DHT state** - Bootstrap nodes must be rediscovered from scratch
4. **Disrupt the mesh** - Connections break until all nodes rediscover the new identity

The volume mount ensures your node maintains its identity across:
- Container restarts (`docker-compose restart`)
- Container re-creations (`docker-compose up -d --force-recreate`)
- Image upgrades (`docker-compose pull && docker-compose up -d`)
- Host reboots

### Example State Files

After starting a node with interface `wg0`, your host data directory will contain:

```bash
./data/node1/
├── wg0.json           # WireGuard keypair (node identity)
├── wg0-peers.json     # Discovered peers cache
└── wg0-dht.nodes      # DHT bootstrap nodes
```

**⚠️ WARNING**: Never delete `{interface}.json` unless you want the node to generate a new identity. This file contains the private key and cannot be recovered.

### Centralized vs Decentralized Modes

**Decentralized mode** (default in docker-compose.yml):
- Uses hardcoded state directory: `/var/lib/wgmesh/`
- Volume mount required for persistence
- Self-discovery using shared secret

**Centralized mode** (operator-managed):
- Uses `--state` flag to specify state file location
- Can use any path (e.g., `/data/state.json`)
- Operator manages peer configuration directly

The docker-compose configuration uses decentralized mode, requiring the `/var/lib/wgmesh` volume mount.

### Host Network Mode

Docker Compose uses `network_mode: host` because:

- WireGuard creates kernel interfaces that must be accessible on the host
- NAT traversal and peer discovery require direct network access
- Bridge networking doesn't support the required functionality

## Examples

### Single Node with Route Advertisement

```yaml
wgmesh-node:
  image: ghcr.io/atvirokodosprendimai/wgmesh:latest
  container_name: wgmesh-gateway
  network_mode: host
  restart: unless-stopped
  volumes:
    - ./data/gateway:/var/lib/wgmesh
  command: >
    join
    --secret "${MESH_SECRET}"
    --interface wg0
    --listen-port 51820
    --advertise-routes "192.168.1.0/24,10.0.0.0/16"
  cap_add:
    - NET_ADMIN
    - SYS_MODULE
```

### Privacy-Enabled Node

```yaml
wgmesh-node:
  image: ghcr.io/atvirokodosprendimai/wgmesh:latest
  container_name: wgmesh-private
  network_mode: host
  restart: unless-stopped
  volumes:
    - ./data/private:/var/lib/wgmesh
  command: >
    join
    --secret "${MESH_SECRET}"
    --interface wg0
    --listen-port 51820
    --privacy
    --gossip
    --log-level debug
  cap_add:
    - NET_ADMIN
    - SYS_MODULE
```

## References

- [Docker Documentation](DOCKER.md) - General Docker build and deployment
- [README.md](README.md) - Main wgmesh documentation
- [WireGuard Documentation](https://www.wireguard.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
