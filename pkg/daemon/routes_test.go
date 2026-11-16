package daemon

import (
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// routeResults runs the route target of one tick for peers, without the peer
// and forwarding targets, and returns the result of every route command. It is
// a test wrapper: converge runs the route target with the desired state it
// holds, and only converge stores the warn-once set, so repeated calls warn
// again.
func (d *Daemon) routeResults(peers []*PeerInfo) []itemResult {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	var obs kernelObserved
	d.observeRoutes(&obs)
	return d.convergeRoutes(d.desire(peers, nil, map[string]string{}), obs)
}

// syncPeerRoutes is routeResults with the error of the first failed add, as
// the apply of routes returned it before.
func (d *Daemon) syncPeerRoutes(peers []*PeerInfo) error {
	for _, r := range d.routeResults(peers) {
		if r.Err != nil && r.Op == "replace" {
			return r.Err
		}
	}
	return nil
}

// TestCalculateRouteDiff verifies add/remove logic without any exec calls.
func TestCalculateRouteDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		current       []routes.Entry
		desired       []routes.Entry
		wantAddLen    int
		wantRemoveLen int
	}{
		{
			name:          "empty both",
			current:       nil,
			desired:       nil,
			wantAddLen:    0,
			wantRemoveLen: 0,
		},
		{
			name:    "add new route",
			current: nil,
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			wantAddLen:    1,
			wantRemoveLen: 0,
		},
		{
			name: "remove stale route",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired:       nil,
			wantAddLen:    0,
			wantRemoveLen: 1,
		},
		{
			name: "no change when identical",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			wantAddLen:    0,
			wantRemoveLen: 0,
		},
		{
			name: "gateway change triggers remove and add",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.2"},
			},
			wantAddLen:    1,
			wantRemoveLen: 1,
		},
		{
			name: "mix of add remove and keep",
			current: []routes.Entry{
				{Network: "10.0.0.0/8", Gateway: "172.16.0.1"},
				{Network: "stale.net/24", Gateway: "172.16.0.1"},
			},
			desired: []routes.Entry{
				{Network: "10.0.0.0/8", Gateway: "172.16.0.1"},
				{Network: "192.168.0.0/16", Gateway: "172.16.0.1"},
			},
			wantAddLen:    1,
			wantRemoveLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			add, remove := routes.CalculateDiff(tt.current, tt.desired)
			if len(add) != tt.wantAddLen {
				t.Errorf("toAdd: got %d, want %d", len(add), tt.wantAddLen)
			}
			if len(remove) != tt.wantRemoveLen {
				t.Errorf("toRemove: got %d, want %d", len(remove), tt.wantRemoveLen)
			}
		})
	}
}

// TestNormalizeNetwork verifies host-route normalization.
func TestNormalizeNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"10.0.0.1", "10.0.0.1/32"},
		{"10.0.0.1/32", "10.0.0.1/32"},
		{"192.168.0.0/24", "192.168.0.0/24"},
		{"fd00::1", "fd00::1/128"},
		{"fd00::1/128", "fd00::1/128"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := routes.NormalizeNetwork(tt.input)
			if got != tt.want {
				t.Errorf("normalizeNetwork(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestReadRoutes uses a mock executor so no real "ip" binary is required.
func TestReadRoutes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("readRoutes is Linux-only")
	}

	ipOutput := strings.Join([]string{
		"192.168.1.0/24 via 10.0.0.1 dev wg0 proto static",
		"10.0.0.0/8 via 10.0.0.2 dev wg0 proto static",
		"",
	}, "\n")

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			if name == "ip" {
				return &MockCommand{outputFunc: func() ([]byte, error) {
					return []byte(ipOutput), nil
				}}
			}
			return &MockCommand{}
		},
	}

	withMockExecutor(t, mock, func() {
		routes, err := readRoutes("wg0", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(routes))
		}
		if routes[0].Network != "192.168.1.0/24" {
			t.Errorf("route[0].Network = %q, want 192.168.1.0/24", routes[0].Network)
		}
		if routes[0].Gateway != "10.0.0.1" {
			t.Errorf("route[0].Gateway = %q, want 10.0.0.1", routes[0].Gateway)
		}
	})
}

// TestReadRoutesSkipsNoGateway verifies that routes without "via" are ignored.
func TestReadRoutesSkipsNoGateway(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("readRoutes is Linux-only")
	}

	// "local" route lines have no "via" — must be skipped
	ipOutput := "10.0.0.0/8 dev wg0 proto kernel scope link src 10.0.0.1\n"

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			return &MockCommand{outputFunc: func() ([]byte, error) {
				return []byte(ipOutput), nil
			}}
		},
	}

	withMockExecutor(t, mock, func() {
		routes, err := readRoutes("wg0", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(routes) != 0 {
			t.Errorf("expected 0 routes (no via), got %d", len(routes))
		}
	})
}

// TestApplyRouteDiff_AddAndRemove verifies the commands issued during a diff apply.
func TestApplyRouteDiff_AddAndRemove(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("applyRouteDiff is Linux-only")
	}

	var cmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			cmds = append(cmds, name+" "+strings.Join(args, " "))
			return &MockCommand{}
		},
	}

	toAdd := []routes.Entry{{Network: "10.0.1.0/24", Gateway: "10.0.0.1"}}
	toRemove := []routes.Entry{{Network: "10.0.2.0/24", Gateway: "10.0.0.1"}}

	withMockExecutor(t, mock, func() {
		_, results := applyRouteDiff("wg0", toAdd, toRemove)
		if len(results) != 2 {
			t.Fatalf("applyRouteDiff returned %d results for 2 routes: %+v", len(results), results)
		}
		for _, r := range results {
			if r.Err != nil {
				t.Fatalf("applyRouteDiff failed: %+v", r)
			}
		}
	})

	found := func(substr string) bool {
		for _, c := range cmds {
			if strings.Contains(c, substr) {
				return true
			}
		}
		return false
	}

	if !found("route del 10.0.2.0/24") {
		t.Errorf("expected 'route del' command, got: %v", cmds)
	}
	if !found("route replace 10.0.1.0/24") {
		t.Errorf("expected 'route replace' command, got: %v", cmds)
	}
}

// TestApplyRouteDiffIssuesNoForwardingCommands: forwarding belongs to the
// forwarding target of converge; applying routes runs neither sysctl nor
// iptables, however many routes it adds and removes.
func TestApplyRouteDiffIssuesNoForwardingCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("applyRouteDiff is Linux-only")
	}
	var cmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			cmds = append(cmds, name+" "+strings.Join(args, " "))
			return &MockCommand{}
		},
	}
	toAdd := []routes.Entry{{Network: "10.0.1.0/24", Gateway: "10.0.0.1"}, {Network: "fd00:5::/64", Gateway: "fd99::2"}}
	toRemove := []routes.Entry{{Network: "10.0.2.0/24", Gateway: "10.0.0.1"}}
	withMockExecutor(t, mock, func() { applyRouteDiff("wg0", toAdd, toRemove) })
	if len(cmds) != 3 {
		t.Errorf("applyRouteDiff ran %d commands for 3 routes, want 3: %q", len(cmds), cmds)
	}
	for _, c := range cmds {
		if strings.HasPrefix(c, "sysctl") || strings.HasPrefix(c, "iptables") {
			t.Errorf("applyRouteDiff ran a forwarding command: %q", c)
		}
	}
}

// TestApplyRouteDiff_AddFailure verifies error propagation when "ip route replace" fails.
func TestApplyRouteDiff_AddFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("applyRouteDiff is Linux-only")
	}

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			if name == "ip" && len(args) > 0 && args[0] == "route" && args[1] == "replace" {
				return &MockCommand{
					combinedOutputFunc: func() ([]byte, error) {
						return []byte("RTNETLINK answers: Operation not permitted"), fmt.Errorf("exit status 2")
					},
				}
			}
			return &MockCommand{}
		},
	}

	toAdd := []routes.Entry{{Network: "10.0.1.0/24", Gateway: "10.0.0.1"}}

	withMockExecutor(t, mock, func() {
		_, results := applyRouteDiff("wg0", toAdd, nil)
		if len(results) != 1 || results[0].Err == nil {
			t.Fatalf("expected one failed result from failed route replace, got %+v", results)
		}
		if !strings.Contains(results[0].Err.Error(), "failed to add route") {
			t.Errorf("unexpected error message: %v", results[0].Err)
		}
	})
}

// makeRelayTestDaemon creates a minimal Daemon for hysteresis/relay route tests.
// The relay candidate has pubkey "relay1" and the target peer has pubkey "peer1".
// An empty &Config{} is used intentionally: these unit tests exercise relay
// decision logic which only reads config.Introducer, config.ForceRelay, and
// config.DisableIPv6 — all zero-valued to match the default (no-force) scenario.
func makeRelayTestDaemon() *Daemon {
	return &Daemon{
		config: &Config{},
		localNode: &LocalNode{
			WGPubKey: "local1",
			NATType:  "symmetric",
		},
		relayRoutes:        make(map[string]string),
		directStableCycles: make(map[string]int),
		temporaryOffline:   make(map[string]time.Time),
		localSubnetsFn:     func() []*net.IPNet { return nil },
	}
}

// TestSyncPeerRoutes_RelayNotRemovedOnIntermittentDirect verifies that when a peer
// is currently relay-routed and the direct path appears to be working (fresh handshake),
// the relay route is NOT removed until the direct path is stable for
// RelayHysteresisThreshold consecutive reconcile cycles.
func TestSyncPeerRoutes_RelayNotRemovedOnIntermittentDirect(t *testing.T) {
	t.Parallel()

	d := makeRelayTestDaemon()

	relay := &PeerInfo{
		WGPubKey:   "relay1",
		MeshIP:     "10.0.0.10",
		Endpoint:   "1.2.3.4:51820",
		Introducer: true,
		LastSeen:   time.Now(),
	}
	target := &PeerInfo{
		WGPubKey: "peer1",
		MeshIP:   "10.0.0.20",
		NATType:  "symmetric",
	}
	peers := []*PeerInfo{relay, target}

	// Seed an existing relay route — simulates prior relay being active.
	d.relayMu.Lock()
	d.relayRoutes["peer1"] = "relay1"
	d.relayMu.Unlock()

	// Fresh handshake — direct path looks good.
	freshHS := map[string]int64{"peer1": time.Now().Add(-10 * time.Second).Unix()}

	// Reconcile cycles 1 through (threshold-1): relay must be kept.
	for cycle := 1; cycle < RelayHysteresisThreshold; cycle++ {
		_, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, freshHS)
		d.relayMu.Lock()
		d.relayRoutes = relayRoutes
		d.directStableCycles = directStable
		d.relayMu.Unlock()

		if _, stillRelayed := relayRoutes["peer1"]; !stillRelayed {
			t.Errorf("cycle %d: relay route was removed before hysteresis threshold (%d)", cycle, RelayHysteresisThreshold)
		}
		stableCount := directStable["peer1"]
		if stableCount != cycle {
			t.Errorf("cycle %d: directStableCycles = %d, want %d", cycle, stableCount, cycle)
		}
	}

	// Reconcile cycle at threshold: relay must be dropped.
	_, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, freshHS)
	d.relayMu.Lock()
	d.relayRoutes = relayRoutes
	d.directStableCycles = directStable
	d.relayMu.Unlock()

	if _, stillRelayed := relayRoutes["peer1"]; stillRelayed {
		t.Errorf("relay route was not dropped after %d stable cycles", RelayHysteresisThreshold)
	}
}

// TestSyncPeerRoutes_RelayRestoredAfterDirectFailure verifies that when a direct
// path fails (stale WG handshake + symmetric NAT), the relay route is established
// within one reconcile cycle.
func TestSyncPeerRoutes_RelayRestoredAfterDirectFailure(t *testing.T) {
	t.Parallel()

	d := makeRelayTestDaemon()

	relay := &PeerInfo{
		WGPubKey:   "relay1",
		MeshIP:     "10.0.0.10",
		Endpoint:   "1.2.3.4:51820",
		Introducer: true,
		LastSeen:   time.Now(),
	}
	target := &PeerInfo{
		WGPubKey: "peer1",
		MeshIP:   "10.0.0.20",
		NATType:  "symmetric",
	}
	peers := []*PeerInfo{relay, target}

	// Stale handshake — direct path has broken.
	staleHS := map[string]int64{"peer1": time.Now().Add(-5 * time.Minute).Unix()}

	_, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, staleHS)
	d.relayMu.Lock()
	d.relayRoutes = relayRoutes
	d.directStableCycles = directStable
	d.relayMu.Unlock()

	if _, relayed := relayRoutes["peer1"]; !relayed {
		t.Error("relay route was not established within one cycle after direct path failure")
	}
}

// TestSyncPeerRoutes_GatewayFlap verifies that rapid gateway changes (direct ↔ relay)
// do not produce oscillating route diffs: the hysteresis counter must absorb the
// direct appearance and only switch once the path is sustainably stable.
func TestSyncPeerRoutes_GatewayFlap(t *testing.T) {
	t.Parallel()

	d := makeRelayTestDaemon()

	relay := &PeerInfo{
		WGPubKey:   "relay1",
		MeshIP:     "10.0.0.10",
		Endpoint:   "1.2.3.4:51820",
		Introducer: true,
		LastSeen:   time.Now(),
	}
	target := &PeerInfo{
		WGPubKey: "peer1",
		MeshIP:   "10.0.0.20",
		NATType:  "symmetric",
	}
	peers := []*PeerInfo{relay, target}

	// Start with an established relay route.
	d.relayMu.Lock()
	d.relayRoutes["peer1"] = "relay1"
	d.relayMu.Unlock()

	// Simulate alternating fresh/stale handshakes across multiple cycles.
	for cycle := 0; cycle < 6; cycle++ {
		var hs map[string]int64
		if cycle%2 == 0 {
			// Even cycle: fresh handshake (direct seems available)
			hs = map[string]int64{"peer1": time.Now().Add(-10 * time.Second).Unix()}
		} else {
			// Odd cycle: stale handshake (direct fails again)
			hs = map[string]int64{"peer1": time.Now().Add(-5 * time.Minute).Unix()}
		}

		_, newRelay, newDirect := d.buildDesiredPeerConfigsWithHandshakes(peers, hs)
		d.relayMu.Lock()
		d.relayRoutes = newRelay
		d.directStableCycles = newDirect
		d.relayMu.Unlock()

		// With alternating direct/stale, relay should never fully drop below
		// the hysteresis threshold — peer must always remain relayed.
		if _, relayed := newRelay["peer1"]; !relayed {
			t.Errorf("cycle %d: relay dropped unexpectedly during gateway flap", cycle)
		}
	}
}

// TestHAExitNodesDropEachOthersDefault: a node that advertises 0.0.0.0/0 does
// not install a peer's 0.0.0.0/0 (routes.ExcludeOwn, applied after admit), and
// a node that does not advertise it installs the peer's.
func TestHAExitNodesDropEachOthersDefault(t *testing.T) {
	t.Parallel()

	peer := &PeerInfo{WGPubKey: "exit2", MeshIP: "10.0.0.30", RoutableNetworks: []string{"0.0.0.0/0", "192.168.5.0/24"}}
	for _, tc := range []struct {
		name  string
		own   []string
		wantD bool
	}{
		{"local is an exit node too", []string{"0.0.0.0/0"}, false},
		{"local is not an exit node", nil, true},
	} {
		d := makeRelayTestDaemon()
		d.localNode.RoutableNetworks = tc.own
		desired, _, _ := d.buildDesiredPeerConfigsWithHandshakes([]*PeerInfo{peer}, nil)
		cfg := desired["exit2"]
		if cfg == nil {
			t.Fatalf("%s: peer has no desired config", tc.name)
		}
		if _, got := cfg.allowed["0.0.0.0/0"]; got != tc.wantD {
			t.Errorf("%s: default route installed = %v, want %v", tc.name, got, tc.wantD)
		}
		if _, ok := cfg.allowed["192.168.5.0/24"]; !ok {
			t.Errorf("%s: the peer's other network is missing", tc.name)
		}
	}
}
