# 30-Day wgmesh Pilot Evaluation Guide

This guide helps network administrators evaluate wgmesh in their environment through a structured 30-day pilot program. By the end, you'll have confidence in whether wgmesh fits your use case, backed by quantitative health checks and milestone tracking.

## Quick Start

```bash
# 1. Initialize pilot tracking
wgmesh pilot init --mode decentralized --use-case remote-team --nodes 3

# 2. Deploy wgmesh on all nodes
wgmesh join --secret "wgmesh://v1/<your-secret>"

# 3. Check pilot status anytime
wgmesh pilot status

# 4. Run health checks
wgmesh pilot validate

# 5. Mark milestones as you complete them
wgmesh pilot milestone mesh-bootstrap

# 6. Generate a report for stakeholders
wgmesh pilot report --format markdown > pilot-report.md
```

## Evaluation Framework

The pilot is organized into four weekly phases, each with specific milestones and success criteria.

### Week 1: Deployment & Connectivity (Days 1–7)

**Goal**: Get the mesh running and verify basic connectivity.

| Milestone | Description | Success Criteria |
|---|---|---|
| `mesh-bootstrap` | Deploy wgmesh on all nodes | All nodes joined, no errors |
| `all-peers-connected` | Verify all peers can reach each other | `wg show` lists all peers with recent handshakes |
| `basic-throughput-test` | Measure baseline throughput | iperf3 between nodes shows expected bandwidth |

**Recommended tests**:
- Deploy on minimum 2 nodes (3+ recommended)
- Verify mesh IP connectivity: `ping <mesh-ip>`
- Run throughput test: `iperf3 -c <mesh-ip>`
- Test from multiple locations (LAN, WAN, different subnets)

### Week 2: Operational Testing (Days 8–14)

**Goal**: Verify NAT traversal, relay fallback, and node lifecycle operations.

| Milestone | Description | Success Criteria |
|---|---|---|
| `nat-traversal-verified` | Confirm NAT traversal works | Nodes behind NAT connect without manual port forwarding |
| `relay-fallback-tested` | Test relay path for unreachable peers | Traffic flows via relay when direct path is blocked |
| `node-addition-removal` | Add and remove nodes dynamically | New nodes discover mesh; removed nodes are cleaned up |

**Recommended tests**:
- Join from behind different NAT types (home router, CGNAT, corporate firewall)
- Block direct UDP traffic and verify relay fallback
- Add a new node and verify it discovers all peers
- Remove a node and verify peers update their configs

### Week 3: Resilience & Recovery (Days 15–21)

**Goal**: Verify the mesh recovers from failures and supports secret rotation.

| Milestone | Description | Success Criteria |
|---|---|---|
| `daemon-restart-recovery` | Restart daemons and verify recovery | All peers reconnect within 60 seconds |
| `network-interruption-recovery` | Simulate network outages | Mesh recovers when connectivity returns |
| `key-rotation-tested` | Test secret rotation | New secret accepted; old peers update gracefully |

**Recommended tests**:
- Restart the wgmesh daemon on one node: `systemctl restart wgmesh`
- Disconnect a node's network for 5 minutes, then reconnect
- Test secret rotation: `wgmesh rotate-secret --current <old> --new <new>`
- Verify peer cache restoration after reboot

### Week 4: Production Readiness (Days 22–30)

**Goal**: Validate production deployment patterns and monitoring integration.

| Milestone | Description | Success Criteria |
|---|---|---|
| `systemd-integration` | Install and verify systemd service | Service starts on boot, survives reboots |
| `metrics-collection` | Set up Prometheus metrics | Dashboard shows peer count, latency, NAT type |
| `policy-configuration` | Configure access control policies | Policies restrict traffic as expected |

**Recommended tests**:
- Install service: `wgmesh install-service --secret <SECRET>`
- Reboot node and verify service auto-starts
- Enable metrics: `wgmesh join --secret <SECRET> --metrics :9090`
- Configure Prometheus scrape config
- If using centralized mode, define groups and policies

## Use Case-Specific Guidance

### Hybrid Site-to-Site

Focus on route propagation and failover:
- Advertise subnets: `--advertise-routes "192.168.1.0/24"`
- Test failover by disconnecting the primary site link
- Verify routed networks appear on all peers

### Multi-Cloud

Focus on inter-cloud connectivity and latency:
- Deploy nodes in at least 2 cloud providers
- Measure cross-cloud latency with `wgmesh pilot validate`
- Monitor for egress cost optimization (prefer direct paths)

### Remote Team

Focus on access control and onboarding:
- Configure access policies for different teams
- Time how long it takes to onboard a new team member
- Verify policy enforcement blocks unauthorized access

### Managed Fleet

Focus on service registration and monitoring:
- Register services with `wgmesh service add`
- Verify Lighthouse integration if applicable
- Test fleet-wide config rollout via centralized mode

## Health Checks

Run `wgmesh pilot validate` to execute automated health checks:

| Check | Description | Pass Criteria |
|---|---|---|
| Interface exists | WireGuard interface is present | `wg show` responds |
| Peers connected | All peers reachable | Active peer count > 0 |
| MTU consistency | Interface MTU is reasonable | Local MTU check passes |
| NAT type detected | NAT detection completed | Daemon status available |
| Routes present | Route tables populated | Peers advertising routes |
| Daemon responding | RPC socket responsive | `daemon.status` returns data |
| Clock skew tolerance | System clock reasonable | Year is within 2024–2030 |
| Interface persistence | Interface survives restart | Interface present after daemon running |

**Exit codes**: 0 = all checks pass or warn, 1 = any check fails.

## Generating Reports

```bash
# Markdown report (for documentation)
wgmesh pilot report --format markdown > pilot-report.md

# JSON report (for tooling integration)
wgmesh pilot report --format json > pilot-report.json
```

Reports include:
- Deployment summary (nodes, platforms, topology)
- Milestone completion timeline
- Health check pass/fail history
- Issues encountered and resolutions
- Overall progress assessment

## Pilot State

Pilot metadata is stored in `~/.wgmesh/pilot.json`. This file is:
- JSON-formatted and human-readable
- Safe to version-control (contains no secrets)
- Portable between machines if needed

Override the default path with `--state`:
```bash
wgmesh pilot init --state /path/to/pilot.json
```
