# Specification: Issue #514

## Classification
feature

## Deliverables
code

## Problem Analysis

The NAT relay path in `pkg/daemon/daemon.go` (`shouldRelayPeer`, `buildDesiredPeerConfigsWithHandshakes`,
`selectRelayForPeer`) has no automated integration test coverage. The only existing relay tests are pure
unit tests in `pkg/daemon/relay_test.go` that call `shouldRelayPeer` directly without ever starting
real WireGuard interfaces or verifying end-to-end packet flow through a relay.

There are two gaps:

1. **Config-level gap** — `buildDesiredPeerConfigsWithHandshakes` produces the desired WireGuard peer
   config (AllowedIPs routed via the relay) but no test calls this function with gossip-discovered peers
   and verifies the resulting `relayRoutes` map and the relay node's AllowedIPs list.

2. **End-to-end gap** — No test starts two wgmesh nodes that cannot reach each other directly, confirms
   that they discover each other through an introducer relay (gossip), and then verifies that ICMP
   traffic flows through the relay.

Both gaps must be addressed. Gap 1 is covered by a new unit-level table test in `pkg/daemon/relay_test.go`.
Gap 2 is covered by a Docker Compose integration test in `testlab/nat-relay/`.

---

## Implementation Tasks

### Task 1: Add `TestBuildDesiredPeerConfigs_RelayRouting` to `pkg/daemon/relay_test.go`

This test exercises `buildDesiredPeerConfigsWithHandshakes` end-to-end (without a kernel WireGuard
interface) and verifies that when `ForceRelay=true` and an introducer relay is available:

- The relay node's `desiredPeerConfig.allowed` set contains the NATed peer's mesh IP CIDR.
- The NATed peer does **not** appear in `desired` as a standalone entry.
- `relayRoutes` maps the NATed peer's pubkey to the relay's pubkey.

Append the following test function to `pkg/daemon/relay_test.go`. No new imports are required beyond
those already in the file (`"testing"`, `"time"`).

```go
func TestBuildDesiredPeerConfigs_RelayRouting(t *testing.T) {
	keys := makeTestKeys(t)

	d := &Daemon{
		config: &Config{
			InterfaceName: "wg-test",
			Keys:          keys,
			ForceRelay:    true,
		},
		localNode: &LocalNode{
			WGPubKey: "local-pubkey",
			NATType:  "cone",
		},
		lastAppliedPeerConfigs: make(map[string]string),
		relayRoutes:            make(map[string]string),
		directStableCycles:     make(map[string]int),
		localSubnetsFn:         func() []*net.IPNet { return nil },
		temporaryOffline:       make(map[string]time.Time),
	}

	introducer := &PeerInfo{
		WGPubKey:   "intro-pubkey",
		MeshIP:     "10.250.0.1",
		Endpoint:   "172.20.1.10:51820",
		Introducer: true,
		LastSeen:   time.Now(),
	}
	// node-b was discovered via gossip (not LAN); --force-relay should route via introducer.
	nodeB := &PeerInfo{
		WGPubKey:      "nodeb-pubkey",
		MeshIP:        "10.250.0.3",
		Endpoint:      "172.20.2.20:51820",
		Introducer:    false,
		DiscoveredVia: []string{"gossip"},
		LastSeen:      time.Now(),
	}

	peers := []*PeerInfo{introducer, nodeB}

	desired, relayRoutes, _ := d.buildDesiredPeerConfigsWithHandshakes(peers, nil)

	// 1. node-b must NOT appear as a top-level direct peer entry.
	if _, ok := desired["nodeb-pubkey"]; ok {
		t.Error("node-b should not be a direct WireGuard peer when relay is active")
	}

	// 2. relay route must map node-b → introducer.
	relay, ok := relayRoutes["nodeb-pubkey"]
	if !ok {
		t.Fatal("expected relayRoutes to contain node-b's pubkey")
	}
	if relay != "intro-pubkey" {
		t.Errorf("relayRoutes[nodeb] = %q, want %q", relay, "intro-pubkey")
	}

	// 3. introducer's desired config must include node-b's mesh IP /32.
	introCfg, ok := desired["intro-pubkey"]
	if !ok {
		t.Fatal("introducer must appear in desired config (as relay carrier for node-b)")
	}
	wantCIDR := "10.250.0.3/32"
	if _, hasIt := introCfg.allowed[wantCIDR]; !hasIt {
		t.Errorf("introducer's AllowedIPs must include %q; got: %v", wantCIDR, introCfg.allowed)
	}
}

// makeTestKeys derives a minimal DerivedKeys for tests that need it.
func makeTestKeys(t *testing.T) *crypto.DerivedKeys {
	t.Helper()
	keys, err := crypto.DeriveKeys("relay-integration-test-secret")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	return keys
}
```

Add `"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"` to the import block of `pkg/daemon/relay_test.go`
if it is not already present. (Check first: other test files in the package already import it via
`daemon_test.go` helpers, but `relay_test.go` uses its own package scope.)

Run this test with:
```bash
go test ./pkg/daemon/... -run TestBuildDesiredPeerConfigs_RelayRouting -v
```

---

### Task 2: Create `testlab/nat-relay/docker-compose.yml`

Create the file at `testlab/nat-relay/docker-compose.yml` with the following exact content:

```yaml
# NAT relay integration test — 3-node Docker mesh
#
# Topology:
#   net_a (172.20.1.0/24): introducer + node-a
#   net_b (172.20.2.0/24): introducer + node-b
#
# node-a and node-b cannot reach each other directly (different Docker subnets).
# They discover each other via gossip (announcements relayed through the
# introducer's WireGuard tunnel), and traffic flows via --force-relay through
# the introducer.
#
# Required: MESH_SECRET env var set to a wgmesh://v1/<base64> URI or passphrase.

services:
  introducer:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: wgmesh-relay-intro
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    networks:
      net_a:
        ipv4_address: 172.20.1.10
      net_b:
        ipv4_address: 172.20.2.10
    command: >
      join
      --secret "${MESH_SECRET}"
      --interface wg0
      --listen-port 51820
      --introducer
      --gossip
      --no-lan-discovery
      --mesh-subnet 10.250.0.0/24
      --log-level debug
    healthcheck:
      test: ["CMD-SHELL", "ip link show wg0 || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 10

  node-a:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: wgmesh-relay-node-a
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    networks:
      net_a:
        ipv4_address: 172.20.1.20
    depends_on:
      introducer:
        condition: service_healthy
    command: >
      join
      --secret "${MESH_SECRET}"
      --interface wg0
      --listen-port 51820
      --gossip
      --no-lan-discovery
      --force-relay
      --mesh-subnet 10.250.0.0/24
      --log-level debug
    healthcheck:
      test: ["CMD-SHELL", "ip link show wg0 || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 10

  node-b:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: wgmesh-relay-node-b
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    networks:
      net_b:
        ipv4_address: 172.20.2.20
    depends_on:
      introducer:
        condition: service_healthy
    command: >
      join
      --secret "${MESH_SECRET}"
      --interface wg0
      --listen-port 51820
      --gossip
      --no-lan-discovery
      --force-relay
      --mesh-subnet 10.250.0.0/24
      --log-level debug
    healthcheck:
      test: ["CMD-SHELL", "ip link show wg0 || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 10

networks:
  net_a:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.1.0/24
  net_b:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.2.0/24
```

**Topology rationale:**

- The introducer is multi-homed (both `net_a` and `net_b`). It accepts WireGuard tunnels from both
  node-a and node-b and acts as the relay.
- node-a is on `net_a` only (172.20.1.0/24). Its WireGuard endpoint is `172.20.1.20:51820`.
- node-b is on `net_b` only (172.20.2.0/24). Its WireGuard endpoint is `172.20.2.20:51820`.
- From node-a's perspective, `172.20.2.20` is NOT on any local subnet, so
  `endpointOnAnyLocalSubnet` returns `false` for node-b. Combined with `--force-relay`, relay is
  selected.
- `--no-lan-discovery` prevents LAN discovery from tagging peers as `lan`-discovered (which would
  block relay); node-a and node-b discover each other via gossip through the established
  WireGuard tunnels to the introducer.
- `--gossip` enables the gossip discovery layer so node-a broadcasts its presence to node-b via the
  introducer's WireGuard tunnel and vice-versa.
- `--mesh-subnet 10.250.0.0/24` pins the mesh to a known subnet so the test script can predict
  the mesh IP range when verifying ping targets.

---

### Task 3: Create `testlab/nat-relay/run-test.sh`

Create `testlab/nat-relay/run-test.sh` (mode `0755`):

```bash
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
```

---

### Task 4: Update `Makefile` — add `test-relay` target

Open `Makefile`. The current content is:

```makefile
.PHONY: build clean install test

build:
	go build -o wgmesh

install:
	go install

clean:
	rm -f wgmesh mesh-state.json

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy
```

Replace the first line and add the new `test-relay` target so the file becomes:

```makefile
.PHONY: build clean install test test-relay

build:
	go build -o wgmesh

install:
	go install

clean:
	rm -f wgmesh mesh-state.json

test:
	go test ./...

test-relay:
	MESH_SECRET="${MESH_SECRET:-wgmesh://v1/cmVsYXktaW50ZWdyYXRpb24tdGVzdA}" \
	  bash testlab/nat-relay/run-test.sh

fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod download
	go mod tidy
```

The `test-relay` target uses the `MESH_SECRET` env var if set, otherwise falls back to the
built-in test secret. It can be called from CI as `make test-relay`.

---

### Task 5: Add `.gitignore` entry for the `testlab/nat-relay/` directory

Open `.gitignore` (repo root). Append the following lines so Docker build context cache files
are not committed:

```
# NAT relay integration test artifacts
testlab/nat-relay/.env
```

---

## Affected Files

| File | Change |
|------|--------|
| `pkg/daemon/relay_test.go` | Add `TestBuildDesiredPeerConfigs_RelayRouting` and `makeTestKeys` helper; add `crypto` import |
| `testlab/nat-relay/docker-compose.yml` | **New** — 3-service Docker Compose topology (introducer + node-a + node-b) |
| `testlab/nat-relay/run-test.sh` | **New** — orchestration script (chmod 0755); builds, starts, waits, verifies relay and ping |
| `Makefile` | Add `test-relay` phony target; add `test-relay` to `.PHONY` list |
| `.gitignore` | Append entry for `testlab/nat-relay/.env` |

No changes to `go.mod`/`go.sum`. No new external dependencies. No changes to any `pkg/` source files.

---

## Test Strategy

### Unit-level (runs in `make test` / `go test ./...`)

```bash
go test ./pkg/daemon/... -run TestBuildDesiredPeerConfigs_RelayRouting -v
```

Expected: test passes, relayRoutes contains nodeb→intro mapping, introducer's allowed-ips
contains `10.250.0.3/32`.

Run with race detector:

```bash
go test -race ./pkg/daemon/... -run TestBuildDesiredPeerConfigs_RelayRouting
```

### Integration (requires Docker, runs as `make test-relay`)

```bash
# Default test secret (hardcoded):
make test-relay

# Custom secret:
MESH_SECRET="wgmesh://v1/bXlzdXBlcnNlY3JldA" make test-relay

# Keep containers running for debugging:
KEEP_UP=1 make test-relay
```

Expected output (abbreviated):

```
[relay-test] Building images and starting containers...
[relay-test] Waiting up to 90s for WireGuard interfaces to come up...
[relay-test] PASS: All containers healthy
[relay-test] Waiting an additional 45s for gossip peer discovery to propagate...
[relay-test] Checking WireGuard relay routing on node-a...
[relay-test] PASS: Relay route configured: introducer carries 2 /32 mesh addresses on node-a
[relay-test] node-b mesh IP: 10.250.0.X
[relay-test] Pinging node-b (10.250.0.X) from node-a (5 packets, timeout 10s)...
[relay-test] PASS: Ping from node-a to node-b succeeded through relay
[relay-test] PASS: Reverse ping from node-b to node-a succeeded through relay
[relay-test] PASS: All relay integration tests passed
```

### CI integration

In `.github/workflows/` (existing CI workflow file — locate the relevant Go test job), add a new
job named `integration-relay` that runs **after** the existing unit-test job:

```yaml
integration-relay:
  name: NAT relay integration test
  runs-on: ubuntu-latest
  needs: test          # name of the existing unit-test job
  steps:
    - uses: actions/checkout@v4
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version-file: go.mod
    - name: Run NAT relay integration test
      run: make test-relay
```

The `ubuntu-latest` GitHub Actions runner has Docker Engine pre-installed and available.
`CAP_NET_ADMIN` and `CAP_SYS_MODULE` are allowed inside Docker containers started by the runner.

---

## Estimated Complexity
medium

**Reasoning:** The unit test addition to `relay_test.go` is straightforward (low complexity).
The Docker Compose integration test is medium complexity: it requires careful network topology
design to prevent direct UDP reachability between `node-a` and `node-b`, correct flag selection
(`--no-lan-discovery`, `--force-relay`, `--gossip`) to force the relay path, and a robust wait
loop that doesn't depend on fixed mesh IP addresses. The gossip propagation time (45s) is the main
uncertainty — if the gossip interval in `pkg/discovery/gossip.go` is longer than expected, the test
may need a longer wait. The `WAIT_SECS` variable in `run-test.sh` can be tuned without code changes.
