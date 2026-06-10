# Pilot Milestone Checklist

A detailed checklist for tracking progress through the 30-day wgmesh pilot evaluation.

## Week 1: Deployment & Connectivity (Days 1–7)

### `mesh-bootstrap` — Deploy wgmesh on all nodes
- [ ] Generate mesh secret: `wgmesh init --secret`
- [ ] Install wgmesh on all target nodes
- [ ] Join mesh on each node: `wgmesh join --secret <URI>`
- [ ] Verify no errors in daemon logs
- [ ] Confirm WireGuard interface created: `wg show`
- [ ] Verify mesh IP assigned to each node

### `all-peers-connected` — Verify all peers can reach each other
- [ ] Check peer list: `wgmesh peers list`
- [ ] Verify all expected peers appear
- [ ] Ping each peer's mesh IP: `ping <mesh-ip>`
- [ ] Confirm recent handshakes in `wg show`
- [ ] Test bidirectional connectivity between all peer pairs

### `basic-throughput-test` — Measure baseline throughput
- [ ] Install iperf3 on at least two nodes
- [ ] Run server: `iperf3 -s` on one node
- [ ] Run client: `iperf3 -c <mesh-ip>` on another node
- [ ] Record baseline throughput (Mbps)
- [ ] Test both directions (swap client/server)
- [ ] Test with different packet sizes if relevant

---

## Week 2: Operational Testing (Days 8–14)

### `nat-traversal-verified` — Confirm NAT traversal works
- [ ] Deploy at least one node behind NAT (home router or CGNAT)
- [ ] Verify node discovers peers without manual port forwarding
- [ ] Check NAT type detection: look in daemon logs
- [ ] Test connectivity from NAT'd node to all peers
- [ ] Verify persistent keepalive maintains connection

### `relay-fallback-tested` — Test relay path for unreachable peers
- [ ] Identify relay candidates (nodes with public IPs or introducer role)
- [ ] Block direct UDP between two nodes (firewall rule)
- [ ] Verify traffic flows via relay peer
- [ ] Remove firewall block and verify return to direct path
- [ ] Measure latency difference: direct vs relay

### `node-addition-removal` — Add and remove nodes dynamically
- [ ] Join a new node to the mesh
- [ ] Verify new node discovers all existing peers
- [ ] Verify all existing peers discover the new node
- [ ] Remove a node (stop daemon)
- [ ] Verify other peers detect the departure
- [ ] Re-add the node and verify full mesh restored

---

## Week 3: Resilience & Recovery (Days 15–21)

### `daemon-restart-recovery` — Restart daemons and verify recovery
- [ ] Restart daemon on one node: `systemctl restart wgmesh`
- [ ] Verify all peers reconnect within 60 seconds
- [ ] Check peer store restored from cache
- [ ] Verify mesh IPs unchanged after restart
- [ ] Test connectivity immediately after restart

### `network-interruption-recovery` — Simulate network outages
- [ ] Disconnect one node's network for 5 minutes
- [ ] Verify other peers detect the outage (peer marked dead)
- [ ] Reconnect network
- [ ] Verify automatic recovery and reconnection
- [ ] Check no persistent state corruption
- [ ] Test longer outage (30 minutes) and verify recovery

### `key-rotation-tested` — Test secret rotation
- [ ] Initiate rotation: `wgmesh rotate-secret --current <old>`
- [ ] Verify grace period is active (both secrets accepted)
- [ ] Update one node with new secret
- [ ] Verify updated node still connects to old-secret nodes
- [ ] Update all remaining nodes
- [ ] Confirm old secret no longer accepted after grace period

---

## Week 4: Production Readiness (Days 22–30)

### `systemd-integration` — Install and verify systemd service
- [ ] Install service: `wgmesh install-service --secret <URI>`
- [ ] Verify service is running: `systemctl status wgmesh`
- [ ] Enable auto-start: confirm `enabled` in systemctl output
- [ ] Reboot node and verify service starts automatically
- [ ] Check logs: `journalctl -u wgmesh -f`
- [ ] Verify mesh connectivity after reboot

### `metrics-collection` — Set up Prometheus metrics
- [ ] Enable metrics: `--metrics :9090` in join/install-service
- [ ] Verify `/metrics` endpoint responds
- [ ] Configure Prometheus scrape target
- [ ] Build Grafana dashboard or review key metrics
- [ ] Set up alerts for peer count drops or high latency
- [ ] Record baseline metric values for reference

### `policy-configuration` — Configure access control policies
- [ ] Define node groups (if using centralized mode)
- [ ] Create access policies between groups
- [ ] Deploy policy configuration
- [ ] Verify restricted traffic is blocked
- [ ] Verify allowed traffic flows normally
- [ ] Test policy changes with `wgmesh -deploy`

---

## Final Checklist

Before concluding the pilot, verify:

- [ ] All Week 1–4 milestones completed or intentionally skipped
- [ ] Health check report shows no failures: `wgmesh pilot validate`
- [ ] No unresolved issues in pilot tracking
- [ ] Stakeholder report generated: `wgmesh pilot report --format markdown`
- [ ] Production deployment plan documented
- [ ] Rollback procedure tested and documented
