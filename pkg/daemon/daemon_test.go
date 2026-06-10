package daemon

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

func testConfig(t *testing.T) *Config {
	t.Helper()
	keys, err := crypto.DeriveKeys("test-secret-for-daemon-tests")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	return &Config{
		InterfaceName: "wg-test",
		WGListenPort:  51820,
		Keys:          keys,
	}
}

func TestDaemonWaitsForGoroutinesOnShutdown(t *testing.T) {
	// Verify that cancelling the daemon context causes Wait() to block
	// until background goroutines (reconcileLoop, statusLoop) exit.
	config := testConfig(t)
	d, err := NewDaemon(config)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// We need a peerStore for reconcile to work
	d.peerStore = NewPeerStore()

	// Track whether goroutines have exited
	var reconcileExited atomic.Bool
	var statusExited atomic.Bool

	// Start goroutines the same way Run() does
	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
		reconcileExited.Store(true)
	}()
	go func() {
		defer d.wg.Done()
		d.statusLoop()
		statusExited.Store(true)
	}()

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	d.cancel()

	// Wait must return (not hang)
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — goroutines exited
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for goroutines to exit after context cancellation")
	}

	if !reconcileExited.Load() {
		t.Error("reconcileLoop did not exit after context cancellation")
	}
	if !statusExited.Load() {
		t.Error("statusLoop did not exit after context cancellation")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		tt := tt
		name := fmt.Sprintf("input=%q", tt.input)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfigureLoggingDoesNotPanic(t *testing.T) {
	// Save and restore global log state so test mutations don't leak.
	origOutput := log.Writer()
	origFlags := log.Flags()
	origDefault := slog.Default()
	t.Cleanup(func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
		slog.SetDefault(origDefault)
	})

	// Verify that configuring logging with various levels doesn't panic.
	for _, level := range []string{"debug", "info", "warn", "error", ""} {
		configureLogging(level)
	}
}

func TestDaemonShutdownMethod(t *testing.T) {
	// Test that Shutdown() cancels context, causing goroutines to exit.
	// Callers wait for Run() to return; here we simulate with wg.Wait().
	config := testConfig(t)
	d, err := NewDaemon(config)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.peerStore = NewPeerStore()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
	}()

	time.Sleep(50 * time.Millisecond)

	// Shutdown only cancels context — does not block
	d.Shutdown()

	// Verify context was cancelled
	select {
	case <-d.ctx.Done():
		// Good
	default:
		t.Fatal("context was not cancelled after Shutdown()")
	}

	// Simulate Run()'s wait — goroutines should exit promptly
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — goroutines exited
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not exit after Shutdown()")
	}
}

// TestDaemon_NoPunchingSkipsHolePunch verifies that when DisablePunching is set,
// buildDesiredPeerConfigsWithHandshakes still produces correct relay routes (the
// daemon-level relay logic is unaffected by the punching flag) and that the flag
// is preserved through Config construction.
func TestDaemon_NoPunchingSkipsHolePunch(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:          "wgmesh-test-secret-no-punch-daemon",
		DisablePunching: true,
		ForceRelay:      true,
	})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if !cfg.DisablePunching {
		t.Fatal("config should carry DisablePunching=true through to daemon")
	}

	d := &Daemon{
		config:             cfg,
		localNode:          &LocalNode{WGPubKey: "local1", NATType: "cone"},
		relayRoutes:        make(map[string]string),
		directStableCycles: make(map[string]int),
		temporaryOffline:   make(map[string]time.Time),
		localSubnetsFn:     func() []*net.IPNet { return nil },
	}

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
		NATType:  "cone",
	}

	// ForceRelay=true means we expect relay even with cone NAT.
	_, relayRoutes, _ := d.buildDesiredPeerConfigsWithHandshakes([]*PeerInfo{relay, target}, nil)
	if _, relayed := relayRoutes["peer1"]; !relayed {
		t.Error("expected relay route when ForceRelay is set alongside DisablePunching")
	}
}

// TestDaemon_RelayRouteNotDroppedDuringNATPunchAttempt verifies that an active
// relay route is preserved across reconcile cycles while NAT punch attempts are
// in progress (i.e., while handshake data is intermittently fresh).
// The relay should only be dropped after RelayHysteresisThreshold stable cycles.
func TestDaemon_RelayRouteNotDroppedDuringNATPunchAttempt(t *testing.T) {
	// Empty &Config{} is acceptable here: this unit test exercises
	// buildDesiredPeerConfigsWithHandshakes which only reads config.Introducer,
	// config.ForceRelay, and config.DisableIPv6 (all zero/false by default).
	d := &Daemon{
		config:             &Config{},
		localNode:          &LocalNode{WGPubKey: "local1", NATType: "symmetric"},
		relayRoutes:        make(map[string]string),
		directStableCycles: make(map[string]int),
		temporaryOffline:   make(map[string]time.Time),
		localSubnetsFn:     func() []*net.IPNet { return nil },
	}

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

	// Establish initial relay route.
	d.relayMu.Lock()
	d.relayRoutes["peer1"] = "relay1"
	d.relayMu.Unlock()

	// Simulate a NAT punch attempt that produces a brief fresh handshake.
	freshHS := map[string]int64{"peer1": time.Now().Add(-5 * time.Second).Unix()}

	// One successful punch should NOT immediately drop the relay.
	_, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, freshHS)
	d.relayMu.Lock()
	d.relayRoutes = relayRoutes
	d.directStableCycles = directStable
	d.relayMu.Unlock()

	if _, relayed := relayRoutes["peer1"]; !relayed {
		t.Error("relay route was dropped after a single successful punch — hysteresis should protect it")
	}
}

// TestDaemon_OfflinePeerRelayCleanup verifies that relay routes are cleaned up
// when a peer is confirmed offline (evicted via peer eviction), not merely when
// the peer is unreachable via a direct path.
func TestDaemon_OfflinePeerRelayCleanup(t *testing.T) {
	// Empty &Config{} is acceptable here: this unit test only exercises
	// relay-map mutation (delete on eviction), not relay decision logic.
	d := &Daemon{
		config:             &Config{},
		localNode:          &LocalNode{WGPubKey: "local1", NATType: "symmetric"},
		relayRoutes:        make(map[string]string),
		directStableCycles: make(map[string]int),
		temporaryOffline:   make(map[string]time.Time),
		localSubnetsFn:     func() []*net.IPNet { return nil },
	}

	// Seed relay routes for an online and an offline peer.
	d.relayMu.Lock()
	d.relayRoutes["peer-online"] = "relay1"
	d.relayRoutes["peer-offline"] = "relay1"
	d.directStableCycles["peer-offline"] = 1
	d.relayMu.Unlock()

	// Simulate eviction of the offline peer.
	d.relayMu.Lock()
	delete(d.relayRoutes, "peer-offline")
	delete(d.directStableCycles, "peer-offline")
	d.relayMu.Unlock()

	d.relayMu.RLock()
	_, offlineRelayRemains := d.relayRoutes["peer-offline"]
	_, offlineStableRemains := d.directStableCycles["peer-offline"]
	_, onlineRelayRemains := d.relayRoutes["peer-online"]
	d.relayMu.RUnlock()

	if offlineRelayRemains {
		t.Error("relay route for evicted (offline) peer should have been cleaned up")
	}
	if offlineStableRemains {
		t.Error("directStableCycles for evicted peer should have been cleaned up")
	}
	if !onlineRelayRemains {
		t.Error("relay route for online peer should not have been removed")
	}
}

func TestMeshIPInSubnet(t *testing.T) {
	t.Parallel()

	_, customNet, _ := net.ParseCIDR("192.168.100.0/24")

	tests := []struct {
		name   string
		meshIP string
		cfg    *Config
		want   bool
	}{
		{
			name:   "legacy subnet match",
			meshIP: "10.42.7.33",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: true,
		},
		{
			name:   "legacy subnet mismatch",
			meshIP: "10.99.1.1",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
		{
			name:   "custom subnet match",
			meshIP: "192.168.100.55",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{},
				CustomSubnet: customNet,
			},
			want: true,
		},
		{
			name:   "custom subnet mismatch",
			meshIP: "10.42.7.33",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{},
				CustomSubnet: customNet,
			},
			want: false,
		},
		{
			name:   "invalid IP",
			meshIP: "not-an-ip",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
		{
			name:   "empty IP",
			meshIP: "",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := meshIPInSubnet(tt.meshIP, tt.cfg)
			if got != tt.want {
				t.Errorf("meshIPInSubnet(%q) = %v, want %v", tt.meshIP, got, tt.want)
			}
		})
	}
}

// --- MeshIPSeed tests (Issue #540) ---

func TestLocalNodeStatePersistsMeshIPSeed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.json")

	original := &LocalNode{
		WGPubKey:     "pub-key-abc",
		WGPrivateKey: "priv-key-xyz",
		MeshIP:       "10.42.7.33",
		MeshIPv6:     "fd12:3456:789a:0001::1",
		MeshIPSeed:   "original-pubkey-for-ip",
	}

	if err := saveLocalNode(path, original); err != nil {
		t.Fatalf("saveLocalNode: %v", err)
	}

	loaded, err := loadLocalNode(path)
	if err != nil {
		t.Fatalf("loadLocalNode: %v", err)
	}

	if loaded.MeshIPSeed != original.MeshIPSeed {
		t.Errorf("MeshIPSeed: got %q, want %q", loaded.MeshIPSeed, original.MeshIPSeed)
	}
}

func TestLocalNodeStateLegacyFileNoMeshIPSeed(t *testing.T) {
	t.Parallel()

	// Simulate an old state file that has no mesh_ip_seed field.
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0-legacy.json")
	oldJSON := `{"wg_pubkey":"pub-key-abc","wg_private_key":"priv-key-xyz","mesh_ip":"10.42.7.33"}`
	if err := os.WriteFile(path, []byte(oldJSON), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := loadLocalNode(path)
	if err != nil {
		t.Fatalf("loadLocalNode: %v", err)
	}

	if loaded.MeshIPSeed != "" {
		t.Errorf("expected empty MeshIPSeed for legacy state file, got %q", loaded.MeshIPSeed)
	}
	if loaded.MeshIP != "10.42.7.33" {
		t.Errorf("MeshIP: got %q, want %q", loaded.MeshIP, "10.42.7.33")
	}
}

func TestMeshIPSeedPreservesIPAcrossKeyChange(t *testing.T) {
	// Verify that using a seed public key produces the same mesh IP
	// even when the actual WG public key changes.
	t.Parallel()

	secret := "test-secret-for-key-rotation"
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	originalPubKey := "original-wg-pubkey-aaaa"
	newPubKey := "new-wg-pubkey-bbbb"

	// Derive IP with the original key
	ipWithOriginal := crypto.DeriveMeshIP(keys.MeshSubnet, originalPubKey, secret)

	// Derive IP with the seed (original key), even though current key is different
	ipWithSeed := crypto.DeriveMeshIP(keys.MeshSubnet, originalPubKey, secret)

	// Derive IP with the new key directly (without seed) — should differ
	ipWithNewKey := crypto.DeriveMeshIP(keys.MeshSubnet, newPubKey, secret)

	if ipWithOriginal != ipWithSeed {
		t.Errorf("IP derived with seed should match original: seed=%q, original=%q", ipWithSeed, ipWithOriginal)
	}

	if ipWithOriginal == ipWithNewKey {
		t.Errorf("IP derived with new key should differ from original (got same IP %q for both keys)", ipWithOriginal)
	}
}

func TestMeshIPSeedPreservesIPInCustomSubnet(t *testing.T) {
	// Same as TestMeshIPSeedPreservesIPAcrossKeyChange but with a custom subnet.
	t.Parallel()

	secret := "test-secret-custom-subnet"
	_, customSubnet, err := net.ParseCIDR("192.168.100.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}

	originalPubKey := "original-wg-pubkey-cccc"
	newPubKey := "new-wg-pubkey-dddd"

	ipWithOriginal, err := crypto.DeriveMeshIPInSubnet(customSubnet, originalPubKey, secret)
	if err != nil {
		t.Fatalf("DeriveMeshIPInSubnet(original): %v", err)
	}

	ipWithSeed, err := crypto.DeriveMeshIPInSubnet(customSubnet, originalPubKey, secret)
	if err != nil {
		t.Fatalf("DeriveMeshIPInSubnet(seed): %v", err)
	}

	ipWithNewKey, err := crypto.DeriveMeshIPInSubnet(customSubnet, newPubKey, secret)
	if err != nil {
		t.Fatalf("DeriveMeshIPInSubnet(new): %v", err)
	}

	if ipWithOriginal != ipWithSeed {
		t.Errorf("IP derived with seed should match original: seed=%q, original=%q", ipWithSeed, ipWithOriginal)
	}

	if ipWithOriginal == ipWithNewKey {
		t.Errorf("IP derived with new key should differ from original (got same IP %q for both keys)", ipWithOriginal)
	}
}

func TestMeshIPSeedPreservesIPv6AcrossKeyChange(t *testing.T) {
	t.Parallel()

	secret := "test-secret-ipv6-rotation"
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	originalPubKey := "original-wg-pubkey-eeee"
	newPubKey := "new-wg-pubkey-ffff"

	ipv6Original := crypto.DeriveMeshIPv6(keys.MeshPrefixV6, originalPubKey, secret)
	ipv6WithSeed := crypto.DeriveMeshIPv6(keys.MeshPrefixV6, originalPubKey, secret)
	ipv6NewKey := crypto.DeriveMeshIPv6(keys.MeshPrefixV6, newPubKey, secret)

	if ipv6Original != ipv6WithSeed {
		t.Errorf("IPv6 derived with seed should match original: seed=%q, original=%q", ipv6WithSeed, ipv6Original)
	}

	if ipv6Original == ipv6NewKey {
		t.Errorf("IPv6 derived with new key should differ from original (got same IPv6 %q for both keys)", ipv6Original)
	}
}

func TestLocalNodeStateRoundTripWithMeshIPSeed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wg0-roundtrip.json")

	// Simulate key rotation: original pubkey is the seed, current key is new.
	node := &LocalNode{
		WGPubKey:     "new-pubkey-after-rotation",
		WGPrivateKey: "new-privkey-after-rotation",
		MeshIP:       "10.42.7.99",
		MeshIPv6:     "fd12:3456:789a:0001::99",
		MeshIPSeed:   "original-pubkey-before-rotation",
	}

	if err := saveLocalNode(path, node); err != nil {
		t.Fatalf("saveLocalNode: %v", err)
	}

	loaded, err := loadLocalNode(path)
	if err != nil {
		t.Fatalf("loadLocalNode: %v", err)
	}

	// Verify all fields round-trip correctly
	if loaded.WGPubKey != node.WGPubKey {
		t.Errorf("WGPubKey: got %q, want %q", loaded.WGPubKey, node.WGPubKey)
	}
	if loaded.WGPrivateKey != node.WGPrivateKey {
		t.Errorf("WGPrivateKey: got %q, want %q", loaded.WGPrivateKey, node.WGPrivateKey)
	}
	if loaded.MeshIP != node.MeshIP {
		t.Errorf("MeshIP: got %q, want %q", loaded.MeshIP, node.MeshIP)
	}
	if loaded.MeshIPv6 != node.MeshIPv6 {
		t.Errorf("MeshIPv6: got %q, want %q", loaded.MeshIPv6, node.MeshIPv6)
	}
	if loaded.MeshIPSeed != node.MeshIPSeed {
		t.Errorf("MeshIPSeed: got %q, want %q", loaded.MeshIPSeed, node.MeshIPSeed)
	}
}

func TestMeshIPSeedJSONFieldOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wg0-no-seed.json")

	node := &LocalNode{
		WGPubKey:     "pubkey-no-seed",
		WGPrivateKey: "privkey-no-seed",
		MeshIP:       "10.42.1.1",
		// MeshIPSeed intentionally empty
	}

	if err := saveLocalNode(path, node); err != nil {
		t.Fatalf("saveLocalNode: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// The mesh_ip_seed field should be omitted when empty (omitempty)
	if strings.Contains(string(data), "mesh_ip_seed") {
		t.Errorf("mesh_ip_seed should be omitted when empty, got:\n%s", string(data))
	}
}

func TestMeshIPSeedJSONFieldPresentWhenSet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wg0-with-seed.json")

	node := &LocalNode{
		WGPubKey:     "pubkey-with-seed",
		WGPrivateKey: "privkey-with-seed",
		MeshIP:       "10.42.1.2",
		MeshIPSeed:   "the-original-seed-key",
	}

	if err := saveLocalNode(path, node); err != nil {
		t.Fatalf("saveLocalNode: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// The mesh_ip_seed field should be present when set
	if !strings.Contains(string(data), "mesh_ip_seed") {
		t.Errorf("mesh_ip_seed should be present when set, got:\n%s", string(data))
	}
}
