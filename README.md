<a href="https://viberank.dev/apps/wgmesh" target="_blank" rel="noopener noreferrer"><img src="https://viberank.dev/badge?app=wgmesh&theme=dark" alt="wgmesh on VibeRank" /></a>
<a href="https://www.producthunt.com/products/wgmesh?embed=true&amp;utm_source=badge-featured&amp;utm_medium=badge&amp;utm_campaign=badge-wgmesh" target="_blank" rel="noopener noreferrer"><img alt="wgmesh - Decentralized WireGuard mesh builder with DHT discovery | Product Hunt" width="250" height="54" src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1081094&amp;theme=light&amp;t=1771444856938"></a>
[![Chimney Deploy](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml/badge.svg)](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml)
# wgmesh — Share a Secret, Build a Mesh

**Build encrypted mesh networks in minutes, not hours.** Generate a shared secret, run `wgmesh join` on each node, and let DHT discovery wire everything together — NAT traversal, endpoint detection, and route management included.

## Motivation

Setting up WireGuard between two machines is simple. Setting it up between *ten* is a nightmare of key exchanges, endpoint tracking, and config file juggling. Every time you add or remove a node, every other node's config needs updating.

Existing tools either require a coordination server you have to host and trust (Tailscale, Netmaker), or need manual key distribution (innernet). wgmesh takes a different approach:

- **Decentralized mode** — nodes discover each other automatically via DHT using a shared secret. No coordination server, no manual config. Just `wgmesh join` and you're in the mesh.
- **Centralized mode** — SSH into your fleet, deploy WireGuard configs, and manage the topology from a single state file. Diff-based updates mean minimal disruption.

Both modes handle NAT traversal, route propagation, and persistence across reboots out of the box.

## Quick Start

```bash
# Generate a secret (once)
wgmesh init --secret

# On every node — same secret, automatic discovery
wgmesh join --secret "wgmesh://v1/<your-secret>"

# Check status
wgmesh status --secret "wgmesh://v1/<your-secret>"
```

That's it. Nodes find each other via DHT, exchange keys, and build the mesh.

## How It Works

### Mesh Topology

Every node becomes a peer to every other node:

```
node1 <----> node2
  ^            ^
  |            |
  v            v
node3 <----> node4
```

### NAT Traversal

Nodes with public IPs are configured as endpoints for other nodes. Nodes behind NAT use persistent keepalive to maintain connections. NAT status is detected automatically by comparing the SSH host with the detected public IP.

### Online Updates

Deploying changes reads the current WireGuard state via `wg show dump`, calculates a diff against the desired state, and applies changes with `wg set` — no interface restart needed. Routes are managed the same way: stale routes are removed and new ones added in-place.

### State Persistence

Mesh state is persisted in `/var/lib/wgmesh/`. In centralized mode, the state file (`mesh-state.json`) holds the full topology including keys and node metadata. In decentralized mode, each node stores its keypair in `/var/lib/wgmesh/{interface}.json`. WireGuard configuration persists across reboots via systemd (`wg-quick@wg0.service`).

## Usage

### Decentralized Mode (Secret-Based Discovery)

Nodes self-discover and peer automatically via DHT.

```bash
# 1) Generate a mesh secret (run once)
wgmesh init --secret

# 2) Join on each node using the same secret
wgmesh join --secret "wgmesh://v1/<your-secret>"

# 3) Check local derived mesh parameters
wgmesh status --secret "wgmesh://v1/<your-secret>"
```

Common `join` options:

```bash
wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug \
  --gossip
```

### Centralized Mode (SSH Deployment)

Manage WireGuard across your fleet from a single control node via SSH:

```bash
wgmesh -init                                        # Create mesh state
wgmesh -add node1:10.99.0.1:192.168.1.10           # Add nodes
wgmesh -add node2:10.99.0.2:203.0.113.50
wgmesh -deploy                                      # Push configs via SSH
```

See [docs/centralized-mode.md](docs/centralized-mode.md) for the full reference: encrypted state files, custom state paths, routable networks, and vault integration.

### Access Control

Centralized mode supports group-based network segmentation. Assign nodes to groups, define policies that control which groups can communicate, and wgmesh enforces the rules via WireGuard `AllowedIPs` filtering. Without access control, all nodes form a full mesh.

See [docs/access-control.md](docs/access-control.md) for the full reference with examples (three-tier architecture, hub-and-spoke, etc.).

### Querying the Daemon

Once the daemon is running (decentralized mode), query it for peer information:

```bash
# List all active peers
wgmesh peers list

# Show peer counts
wgmesh peers count

# Get specific peer details
wgmesh peers get <pubkey>
```

The RPC socket is automatically created at:
- `/var/run/wgmesh.sock` (if running as root)
- `$XDG_RUNTIME_DIR/wgmesh.sock` (if running as non-root)
- `/tmp/wgmesh.sock` (fallback)

Override with `--socket-path` flag on `join` or `WGMESH_SOCKET` environment variable.

### Testing Connectivity

Use `test-peer` to verify direct UDP connectivity to another wgmesh node. Start `wgmesh join` on the remote peer, note its exchange port, then run:

```bash
wgmesh test-peer --secret "wgmesh://v1/<your-secret>" --peer <PEER_IP>:<EXCHANGE_PORT>
```

### Metrics

wgmesh exposes a Prometheus-compatible `/metrics` endpoint. Enable it with the `--metrics` flag on `join`:

```bash
wgmesh join --secret "wgmesh://v1/<your-secret>" --metrics :9090
```

Then scrape `http://<host>:9090/metrics`.

#### Available metrics

| Metric | Type | Description |
|---|---|---|
| `wgmesh_active_peers` | Gauge | Current active peers in the mesh |
| `wgmesh_relayed_peers` | Gauge | Peers routed via relay (not direct) |
| `wgmesh_nat_type{type}` | Gauge | Local NAT type — `type` is `cone`, `symmetric`, or `unknown`; value is 1 for the current type |
| `wgmesh_discovery_events_total{layer}` | Counter | Peer-discovery events by layer — `layer` is `dht`, `lan`, `gossip`, or `registry` |
| `wgmesh_nat_traversal_attempts_total{method}` | Counter | NAT traversal attempts by method |
| `wgmesh_nat_traversal_successes_total{method}` | Counter | Successful NAT traversal exchanges by method |
| `wgmesh_probe_rtt_seconds{peer_key}` | Histogram | Mesh probe round-trip time per peer (first 8 chars of pubkey) |
| `wgmesh_reconcile_duration_seconds` | Histogram | Time spent in the reconcile loop |
| `go_goroutines` | Gauge | Number of active goroutines (Go runtime) |
| `go_memstats_alloc_bytes` | Gauge | Allocated heap bytes (Go runtime) |
| `process_resident_memory_bytes` | Gauge | Resident memory (OS process) |

#### Example Prometheus scrape config

```yaml
scrape_configs:
  - job_name: wgmesh
    static_configs:
      - targets: ['<node1>:9090', '<node2>:9090']
```

## Installation

### Pre-built Binaries

Download pre-built binaries for your platform from the [releases page](https://github.com/atvirokodosprendimai/wgmesh/releases).

Available architectures: Linux amd64, arm64, armv7.

```bash
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

Requires Go 1.23+ and WireGuard tools (`wg` command).

### Docker

```bash
docker pull ghcr.io/atvirokodosprendimai/wgmesh:latest

# For full WireGuard functionality
docker run --rm --privileged --network host \
  -v $(pwd)/wgmesh-state:/var/lib/wgmesh \
  ghcr.io/atvirokodosprendimai/wgmesh:latest join \
  --secret "wgmesh://v1/<your-secret>"
```

See [DOCKER.md](DOCKER.md) and [DOCKER-COMPOSE.md](DOCKER-COMPOSE.md) for detailed Docker deployment guides.

For a step-by-step first-mesh walkthrough covering all installation methods, see [docs/quickstart.md](docs/quickstart.md).

### Verify Installation

```bash
wgmesh version
```

## Security Considerations

- **Centralized mode**: Keys stored in `mesh-state.json` — use `--encrypt` for AES-256-GCM encryption. See [ENCRYPTION.md](ENCRYPTION.md).
- **Decentralized mode**: Each node stores its keypair in `/var/lib/wgmesh/{interface}.json` with `0600` permissions.
- WireGuard traffic is encrypted end-to-end.
- **SSH authentication**: The tool tries the SSH agent first (`SSH_AUTH_SOCK`), then `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`, and `~/.ssh/id_ecdsa`.
- The tool currently uses `InsecureIgnoreHostKey` for SSH — consider implementing proper host key verification for production.

## Architecture

```
wgmesh/
├── main.go                       # CLI entry point (dual-mode dispatch)
├── cmd/
│   ├── chimney/                  # Dashboard server with GitHub API proxy
│   └── lighthouse/               # CDN control plane with xDS
├── pkg/
│   ├── crypto/                   # Secret-derived keys, envelope encryption
│   ├── daemon/                   # Lifecycle, reconciliation, health checks
│   ├── discovery/                # DHT, gossip, LAN/STUN, peer exchange
│   ├── mesh/                     # Mesh state, add/remove/list/deploy
│   ├── wireguard/                # Key gen, config parsing, diffing, apply
│   ├── ssh/                      # SSH client, remote WireGuard operations
│   ├── rpc/                      # Unix socket JSON-RPC server/client
│   ├── privacy/                  # Dandelion stem-fluff routing
│   ├── routes/                   # Route management
│   ├── ratelimit/                # Rate limiting
│   ├── proxy/                    # Proxy utilities
│   └── lighthouse/               # Lighthouse client library
```

**State files** (system-level, not in project directory):
```
/var/lib/wgmesh/
└── mesh-state.json               # Mesh state (created on init)
```

## Troubleshooting

See the [Quickstart Guide troubleshooting section](docs/quickstart.md#troubleshooting) for decentralized-mode issues (daemon, peers, NAT, interface errors).

See [docs/troubleshooting.md](docs/troubleshooting.md) for centralized-mode SSH runbooks (persistence checks, log viewing, configuration rebuilds).

- [Team Dogfooding & Stability Metrics](docs/dogfooding/README.md)

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Found a bug or have an idea? [Open an issue](https://github.com/atvirokodosprendimai/wgmesh/issues) — even small improvements are appreciated.

## License

MIT License - see LICENSE file for details
