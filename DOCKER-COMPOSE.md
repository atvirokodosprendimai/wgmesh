# Docker Compose Setup

This document describes how to use Docker Compose to deploy wgmesh in decentralized mode.

## Overview

The `docker-compose.yml` file provides a simple way to deploy wgmesh nodes that automatically join a mesh network using a shared secret. This is ideal for:

- Quick local testing with multiple nodes
- Production deployments with consistent configuration
- Easy management of multiple mesh nodes

## Prerequisites

- Docker and Docker Compose installed
- A mesh secret (generated using `wgmesh init`)
- Linux host with WireGuard kernel module support

## Quick Start

### 1. Generate a Mesh Secret

First, generate a mesh secret using the wgmesh CLI or Docker:

```bash
# Using local binary
./wgmesh init

# Or using Docker
docker run --rm ghcr.io/atvirokodosprendimai/wgmesh:latest init
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
docker-compose exec wgmesh-node wgmesh status
```

## Configuration Options

### Basic Configuration

The default configuration in `docker-compose.yml` includes:

- **Image**: `ghcr.io/atvirokodosprendimai/wgmesh:latest`
- **Network Mode**: `host` (required for WireGuard)
- **Privileges**: `privileged: true` with `NET_ADMIN` and `SYS_MODULE` capabilities
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
docker-compose exec wgmesh-node wgmesh status

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
- Rotate secrets periodically using `wgmesh rotate-secret`
- Share secrets securely (encrypted channels only)

### Container Security

- Containers run as `root` by default (required for WireGuard operations)
- Use `privileged: true` or specific capabilities (`NET_ADMIN`, `SYS_MODULE`)
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
# Ensure user can run privileged containers
docker run --rm --privileged alpine:latest echo "OK"
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

### Host Network Mode

Docker Compose uses `network_mode: host` because:

- WireGuard creates kernel interfaces that must be accessible on the host
- NAT traversal and peer discovery require direct network access
- Bridge networking doesn't support the required functionality

### Persistent State

The `./data/` directory stores:

- WireGuard private key
- Peer database
- Discovery state
- Gossip protocol data

This data must persist across container restarts for stable mesh operation.

## Examples

### Single Node with Route Advertisement

```yaml
wgmesh-node:
  image: ghcr.io/atvirokodosprendimai/wgmesh:latest
  container_name: wgmesh-gateway
  privileged: true
  network_mode: host
  restart: unless-stopped
  volumes:
    - ./data/gateway:/data
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
  privileged: true
  network_mode: host
  restart: unless-stopped
  volumes:
    - ./data/private:/data
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
