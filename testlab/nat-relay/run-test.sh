#!/usr/bin/env bash
# NAT relay integration test
#
# Usage:
#   MESH_SECRET=<secret> ./run-test.sh          # Run test and print PASS/FAIL
#   MESH_SECRET=<secret> KEEP_UP=1 ./run-test.sh  # Don't tear down on success (for debugging)
#
# Requires: docker, docker compose (v2), jq, ping

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
MESH_SECRET="${MESH_SECRET:-wgmesh://v1/cmVsYXktaW50ZWdyYXRpb24tdGVzdA}"
KEEP_UP="${KEEP_UP:-0}"
WAIT_SECS="${WAIT_SECS:-90}"   # seconds to wait for mesh to form
PING_COUNT=5
PING_TIMEOUT=10

log()  { echo "[relay-test] $*"; }
pass() { echo "[relay-test] PASS: $*"; }
fail() { echo "[relay-test] FAIL: $*" >&2; exit 1; }

cleanup() {
    if [ "${KEEP_UP}" = "1" ] && [ $? -eq 0 ]; then
        log "KEEP_UP=1 — leaving containers running"
        return
    fi
    log "Tearing down..."
    MESH_SECRET="$MESH_SECRET" docker compose -f "$COMPOSE_FILE" down --remove-orphans --volumes 2>/dev/null || true
}
trap cleanup EXIT

# ── 1. Build and start ───────────────────────────────────────────────────────
log "Building images and starting containers..."
MESH_SECRET="$MESH_SECRET" docker compose -f "$COMPOSE_FILE" build --quiet
MESH_SECRET="$MESH_SECRET" docker compose -f "$COMPOSE_FILE" up -d

# ── 2. Wait for all nodes to be healthy ─────────────────────────────────────
log "Waiting up to ${WAIT_SECS}s for WireGuard interfaces to come up..."
deadline=$((SECONDS + WAIT_SECS))
all_up=0
while [ $SECONDS -lt $deadline ]; do
    intro_ok=$(docker inspect --format='{{.State.Health.Status}}' wgmesh-relay-intro 2>/dev/null || echo "missing")
    node_a_ok=$(docker inspect --format='{{.State.Health.Status}}' wgmesh-relay-node-a 2>/dev/null || echo "missing")
    node_b_ok=$(docker inspect --format='{{.State.Health.Status}}' wgmesh-relay-node-b 2>/dev/null || echo "missing")
    if [ "$intro_ok" = "healthy" ] && [ "$node_a_ok" = "healthy" ] && [ "$node_b_ok" = "healthy" ]; then
        all_up=1
        break
    fi
    sleep 5
    log "  intro=$intro_ok  node-a=$node_a_ok  node-b=$node_b_ok"
done

if [ $all_up -eq 0 ]; then
    log "Container logs:"
    MESH_SECRET="$MESH_SECRET" docker compose -f "$COMPOSE_FILE" logs
    fail "Timed out waiting for containers to become healthy"
fi
pass "All containers healthy"

# ── 3. Wait for gossip peer discovery ───────────────────────────────────────
log "Waiting an additional 45s for gossip peer discovery to propagate..."
sleep 45

# ── 4. Verify relay route is configured on node-a ───────────────────────────
# node-a should list node-b's mesh IP under the introducer's AllowedIPs in 'wg show'.
log "Checking WireGuard relay routing on node-a..."
wg_show=$(docker exec wgmesh-relay-node-a wg show wg0 2>/dev/null)
if [ -z "$wg_show" ]; then
    fail "wg show on node-a returned no output (is wg0 up?)"
fi
log "wg show wg0 (node-a):"
echo "$wg_show"

# The introducer's allowed-ips should include at least two /32 entries:
# its own mesh IP and node-b's mesh IP.
intro_allowed=$(echo "$wg_show" | awk '/allowed ips/{print}' | head -1)
count=$(echo "$intro_allowed" | tr ',' '\n' | grep -c '/32' || true)
if [ "$count" -lt 2 ]; then
    fail "Relay not active: introducer's allowed-ips on node-a has fewer than 2 /32 entries: $intro_allowed"
fi
pass "Relay route configured: introducer carries ${count} /32 mesh addresses on node-a"

# ── 5. Discover node-b's mesh IP ────────────────────────────────────────────
# Extract all mesh IPs from the introducer's allowed-ips, then exclude the
# introducer's own mesh IP by checking which IP is reachable from node-b.
log "Resolving node-b mesh IP..."
node_b_mesh_ip=""
for candidate_cidr in $(echo "$intro_allowed" | tr ',' '\n' | grep '/32' | tr -d ' '); do
    candidate_ip="${candidate_cidr%/32}"
    if docker exec wgmesh-relay-node-b ping -c 1 -W 1 "$candidate_ip" > /dev/null 2>&1; then
        node_b_mesh_ip="$candidate_ip"
        break
    fi
done
if [ -z "$node_b_mesh_ip" ]; then
    fail "Could not determine node-b's mesh IP from relay routes"
fi
log "node-b mesh IP: $node_b_mesh_ip"

# ── 6. Verify bidirectional ping through relay ───────────────────────────────
log "Pinging node-b ($node_b_mesh_ip) from node-a (${PING_COUNT} packets, timeout ${PING_TIMEOUT}s)..."
if ! docker exec wgmesh-relay-node-a ping -c "$PING_COUNT" -W "$PING_TIMEOUT" "$node_b_mesh_ip" > /dev/null 2>&1; then
    log "Ping failed. Container logs:"
    MESH_SECRET="$MESH_SECRET" docker compose -f "$COMPOSE_FILE" logs
    fail "Ping from node-a to node-b via relay failed"
fi
pass "Ping from node-a to node-b succeeded through relay"

# Resolve node-a mesh IP for reverse test.
log "Checking reverse ping (node-b to node-a)..."
wg_show_b=$(docker exec wgmesh-relay-node-b wg show wg0 2>/dev/null)
node_a_mesh_ip=""
intro_allowed_b=$(echo "$wg_show_b" | awk '/allowed ips/{print}' | head -1)
for candidate_cidr in $(echo "$intro_allowed_b" | tr ',' '\n' | grep '/32' | tr -d ' '); do
    candidate_ip="${candidate_cidr%/32}"
    if docker exec wgmesh-relay-node-a ping -c 1 -W 1 "$candidate_ip" > /dev/null 2>&1; then
        node_a_mesh_ip="$candidate_ip"
        break
    fi
done
if [ -z "$node_a_mesh_ip" ]; then
    fail "Could not determine node-a's mesh IP from relay routes on node-b"
fi
if ! docker exec wgmesh-relay-node-b ping -c "$PING_COUNT" -W "$PING_TIMEOUT" "$node_a_mesh_ip" > /dev/null 2>&1; then
    fail "Ping from node-b to node-a via relay failed"
fi
pass "Reverse ping from node-b to node-a succeeded through relay"

# ── 7. Done ──────────────────────────────────────────────────────────────────
pass "All relay integration tests passed"
