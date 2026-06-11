# wgmesh Pilot Evaluation Guide

This guide walks network administrators through a structured 30-day evaluation of wgmesh
using the built-in pilot framework. By the end of this evaluation you will have measurable
data on mesh stability, peer discovery, NAT traversal, and operational readiness.

---

## Prerequisites

Before starting the pilot, ensure your environment meets these requirements:

| Requirement | Details |
|-------------|---------|
| Linux kernel ≥ 5.6 | WireGuard is built into Linux 5.6+ |
| `wireguard-tools` installed | `apt install wireguard-tools` or equivalent |
| Root access or `CAP_NET_ADMIN` | Required to create WireGuard interfaces |
| Outbound UDP not blocked | At least one node must be reachable over UDP |
| 2–5 test nodes | Mix of public IP and NAT-behind nodes recommended |
| `/etc/wgmesh/` writable | Stores pilot configuration and peer cache |
| Systemd (optional) | For persistent service via `install-service` |

See [evaluation-checklist.md](evaluation-checklist.md) for a detailed pre-evaluation checklist.

---

## Quick Start

### 1. Initialize the Pilot (Day 0)

On your primary evaluation node:

```bash
wgmesh pilot init \
  --org "Your Organization" \
  --contact admin@yourorg.com \
  --nodes 5 \
  --mode decentralized \
  --duration 30
```

This creates `/etc/wgmesh/pilot.yaml` with default milestones and metrics targets.

### 2. Generate a Mesh Secret

```bash
wgmesh init --secret
```

Copy the printed `wgmesh://v1/...` URI.

### 3. Start the Pilot

```bash
wgmesh pilot start
```

This starts the 30-day evaluation clock and begins metrics collection.

### 4. Deploy to Pilot Nodes

On each node, join the mesh:

```bash
wgmesh join --secret "wgmesh://v1/..."
```

For persistent operation:

```bash
wgmesh install-service --secret "wgmesh://v1/..."
```

### 5. Monitor Progress

```bash
wgmesh pilot status       # Current phase, milestones, days elapsed
wgmesh pilot validate     # Run health checks
wgmesh pilot report       # Generate evaluation report
```

---

## Four-Phase Milestone Structure

The pilot progresses through four phases, each with specific validation criteria.

### Phase 1: Baseline Setup (Days 1–3)

**Goal:** Successful deployment and basic connectivity.

**Tasks:**
- Install wgmesh on all pilot nodes
- Configure mesh secret and join mesh
- Verify peer discovery across all nodes
- Run `wgmesh pilot status` to confirm Phase 1 progress

**Validation:**
- [ ] All peers visible in `wgmesh peers list`
- [ ] `ping <mesh-ip>` succeeds between all peer pairs
- [ ] No interface churn (WireGuard restart loops)
- [ ] `wgmesh pilot validate` reports no errors

**Commands to verify:**
```bash
wgmesh peers list                    # Should show all pilot nodes
wg show                              # Check latest handshake times
ping <mesh-ip-of-another-node>       # Verify connectivity
wgmesh pilot validate                # Run health checks
```

**Mark milestone complete:**
```bash
# Milestones are tracked automatically; you can also manually mark:
# (via the pilot status reporting — all milestones appear in reports)
```

---

### Phase 2: Mesh Stability (Days 4–7)

**Goal:** Verify mesh stability under normal operations.

**Tasks:**
- Run continuous connectivity tests (24h soak)
- Verify route propagation after network changes
- Test graceful node restart and reconnection
- Log key metrics: connection uptime, discovery success rate

**Validation:**
- [ ] ≥99.9% connectivity uptime between all nodes
- [ ] All routes propagate within 30 seconds of topology change
- [ ] Zero daemon crashes or WireGuard interface crashes
- [ ] NAT type detection completed for all nodes

**Commands to verify:**
```bash
wgmesh pilot status                  # Check milestone progress
wgmesh peers count                   # Verify all peers still active
journalctl -u wgmesh --since "24 hours ago" | grep -i error
wgmesh pilot report                  # Generate progress report
```

**Simulating a node restart:**
```bash
# On the test node:
sudo systemctl restart wgmesh        # Or: Ctrl-C and re-run join

# Wait 60 seconds, then from another node:
wgmesh peers list                    # Node should reappear
```

---

### Phase 3: Production Traffic Simulation (Days 8–14)

**Goal:** Validate under realistic workload.

**Tasks:**
- Route application traffic through mesh
- Measure throughput and latency
- Test with intermittent network failures (simulated outages)
- Exercise all discovery layers (registry, LAN, DHT, gossip)

**Validation:**
- [ ] Throughput ≥80% of native WireGuard baseline
- [ ] Latency overhead <20ms compared to native WireGuard
- [ ] Successful recovery from simulated network partitions
- [ ] All discovery layers successfully used

**Throughput testing:**
```bash
# On node A (server):
iperf3 -s -B <mesh-ip-a>

# On node B (client):
iperf3 -c <mesh-ip-a> -t 30         # 30-second throughput test
```

**Latency testing:**
```bash
# Native WireGuard latency:
ping -c 100 <mesh-ip> | tail -1

# Compare with direct IP latency to assess overhead
```

**Simulating network failure:**
```bash
# Temporarily block traffic on one node:
sudo iptables -A OUTPUT -d <other-node-ip> -j DROP

# Wait 30 seconds, then restore:
sudo iptables -D OUTPUT -d <other-node-ip> -j DROP

# Verify reconnection:
wgmesh peers list
```

---

### Phase 4: Advanced Scenarios & NAT Traversal (Days 15–30)

**Goal:** Stress-test edge cases and operational workflows.

**Tasks:**
- Deploy nodes behind diverse NAT types (Full Cone, Symmetric, etc.)
- Test relay fallback when direct connection fails
- Verify secret rotation workflow
- Exercise operational procedures: daemon restart, config reload, node add/remove

**Validation:**
- [ ] Successful hole-punching across all NAT type combinations
- [ ] Relay fallback engages within 60 seconds of direct path failure
- [ ] Zero secret leaks or key derivation failures
- [ ] Clean node addition/removal with no orphaned WireGuard configs

**Adding a new node mid-pilot:**
```bash
# On the new node:
wgmesh join --secret "wgmesh://v1/..."

# From existing nodes:
wgmesh peers list                    # New node should appear within 60s
```

**Removing a node:**
```bash
# On the node being removed:
sudo wgmesh uninstall-service        # If running as service
sudo systemctl stop wgmesh           # Stop the daemon

# Other nodes will mark it as stale after timeout
```

**Secret rotation:**
```bash
wgmesh rotate-secret --current "wgmesh://v1/old-secret"
# Follow printed instructions to deploy new secret
```

---

## Reports and Metrics

### Generating Reports

```bash
# Console report (default)
wgmesh pilot report

# JSON export for automated analysis
wgmesh pilot report --format json --output pilot-report.json

# HTML report for executive summary
wgmesh pilot report --format html --output pilot-report.html
```

### Default Metrics Targets

| Metric | Target | Description |
|--------|--------|-------------|
| Mesh Connectivity | ≥99.9% | Uptime between all peer pairs |
| Peer Discovery Time | ≤60s | Time for all nodes to discover each other |
| Route Propagation | ≤30s | Time for route changes to propagate |
| Throughput | ≥80 Mbps | Compared to native WireGuard |
| Latency Overhead | ≤20ms | Additional latency vs native WireGuard |

### Report Sections

Each report contains:

1. **Milestone Status** — Progress through the four phases
2. **Key Metrics** — Connectivity, discovery, route propagation, restart counts
3. **Discovery Layer Distribution** — Usage of Registry, LAN, DHT, and Gossip layers
4. **NAT Types Detected** — Breakdown of NAT types across pilot nodes
5. **Issues / Warnings** — Any metrics below targets or operational issues
6. **Next Steps** — Guidance on what to do next

---

## Completing the Pilot

### Final Evaluation

```bash
wgmesh pilot complete
```

This produces a final report with:

- **Overall Rating**: Excellent / Good / Fair / Poor
- **Recommendation**: Production readiness assessment
- **Milestone Summary**: Which milestones were completed
- **Metrics Summary**: Final metrics across the evaluation period

### Rating Criteria

| Rating | Score | Meaning |
|--------|-------|---------|
| Excellent | ≥90 | Ready for production deployment |
| Good | 70–89 | Suitable for production with monitoring |
| Fair | 50–69 | Requires investigation before production |
| Poor | <50 | Not recommended for production |

### Saving Results

```bash
# Save final report as JSON
wgmesh pilot complete --output pilot-final.json

# The console output shows the human-readable summary
```

---

## Troubleshooting

### Peers Not Discovered

**Symptoms:** `wgmesh peers list` shows no peers after 5+ minutes.

**Check:**
```bash
# Verify both nodes use the same secret
wgmesh status --secret "wgmesh://v1/..."   # Compare Network ID

# Check UDP connectivity
wgmesh test-peer --secret "wgmesh://v1/..." --peer <ip:port>

# Check firewall rules
sudo iptables -L -n | grep -i drop
```

**Common causes:**
- Different secrets on different nodes (Network IDs won't match)
- Firewall blocking outbound UDP
- No internet access for DHT bootstrap

### High Latency

**Symptoms:** Latency overhead exceeds 20ms target.

**Check:**
```bash
# Compare native vs mesh latency
ping -c 10 <direct-ip>     # Direct latency
ping -c 10 <mesh-ip>       # Mesh latency

# Check if relay is being used
wgmesh peers list          # Look for "(relayed)" in endpoint
```

**Common causes:**
- Relay path instead of direct (Symmetric NAT)
- High baseline latency between regions

### Daemon Crashes

**Symptoms:** Daemon exits unexpectedly, shown in pilot report as restart count.

**Check:**
```bash
journalctl -u wgmesh --since "1 hour ago" | grep -i "panic\|fatal\|error"
```

**Action:** File an issue at https://github.com/atvirokodosprendimai/wgmesh/issues
with the pilot report and relevant log output.

### Connectivity Drops

**Symptoms:** Pilot report shows mesh connectivity below 99.9%.

**Check:**
```bash
# Monitor connectivity in real-time
watch -n 5 'wg show wg0 | grep latest-handshake'

# Check for WireGuard interface issues
sudo wg show wg0
```

---

## FAQ

**Q: Can I run the pilot with fewer than 5 nodes?**
A: Yes. Set `--nodes` to your actual count. The minimum is 1, but 2–3 is recommended
   for meaningful evaluation.

**Q: Can I shorten the pilot duration?**
A: Yes. The minimum is 7 days (`--duration 7`), but the full 30-day evaluation
   provides the most comprehensive results.

**Q: Do I need a separate node for pilot management?**
A: No. The pilot commands run on any node with the pilot configuration. Typically
   you run them on your primary evaluation node.

**Q: Can I run multiple pilots simultaneously?**
A: No. The current implementation supports one pilot at a time per configuration
   file. Use separate configuration paths if needed.

**Q: What happens if I restart the daemon?**
A: Daemon restarts are tracked in pilot metrics. A few restarts during evaluation
   are acceptable; excessive restarts will lower the overall rating.

**Q: How do I reset and start over?**
A: Remove the pilot configuration and reinitialize:
```bash
sudo rm /etc/wgmesh/pilot.yaml
wgmesh pilot init --org "..." --contact "..."
wgmesh pilot start
```
