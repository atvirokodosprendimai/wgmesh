package discovery

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func TestNodesFilePathIncludesNetworkTag(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-secret-network-tag-1"})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	d, err := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "a"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery failed: %v", err)
	}

	path := d.nodesFilePath()
	if !strings.Contains(path, cfg.InterfaceName+"-") || !strings.HasSuffix(path, "-dht.nodes") {
		t.Fatalf("unexpected nodes file path format: %s", path)
	}

	expectedTag := fmt.Sprintf("%x", cfg.Keys.NetworkID[:8])
	if !strings.Contains(strings.ToLower(path), expectedTag) {
		t.Fatalf("expected network tag %s in path %s", expectedTag, path)
	}
}

func TestNodesFilePathDiffersBySecret(t *testing.T) {
	cfgA, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-secret-a-1234567890"})
	if err != nil {
		t.Fatalf("NewConfig A failed: %v", err)
	}
	cfgB, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-secret-b-0987654321"})
	if err != nil {
		t.Fatalf("NewConfig B failed: %v", err)
	}

	dA, err := NewDHTDiscovery(cfgA, &LocalNode{WGPubKey: "a"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery A failed: %v", err)
	}
	dB, err := NewDHTDiscovery(cfgB, &LocalNode{WGPubKey: "b"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery B failed: %v", err)
	}

	if dA.nodesFilePath() == dB.nodesFilePath() {
		t.Fatalf("expected different nodes file paths for different secrets, got %s", dA.nodesFilePath())
	}
}

func TestCanAttemptRendezvous_NewPeer(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	if !d.canAttemptRendezvous("newpeer") {
		t.Error("New peer should be allowed to attempt rendezvous")
	}
}

func TestCanAttemptRendezvous_BackoffNotExpired(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	d.mu.Lock()
	d.rendezvousBackoff["peer1"] = backoffEntry{nextAttempt: time.Now().Add(30 * time.Second), duration: 30 * time.Second}
	d.mu.Unlock()

	if d.canAttemptRendezvous("peer1") {
		t.Error("Peer in backoff should not be allowed to attempt rendezvous")
	}
}

func TestCanAttemptRendezvous_BackoffExpired(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	d.mu.Lock()
	d.rendezvousBackoff["peer1"] = backoffEntry{nextAttempt: time.Now().Add(-1 * time.Second), duration: RendezvousMinBackoff}
	d.mu.Unlock()

	if !d.canAttemptRendezvous("peer1") {
		t.Error("Peer with expired backoff should be allowed to attempt rendezvous")
	}
}

func TestRecordRendezvousAttempt_SuccessResetsBackoff(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	d.mu.Lock()
	d.rendezvousBackoff["peer1"] = backoffEntry{nextAttempt: time.Now().Add(30 * time.Second), duration: 30 * time.Second}
	d.mu.Unlock()

	d.recordRendezvousAttempt("peer1", true)

	d.mu.RLock()
	_, exists := d.rendezvousBackoff["peer1"]
	d.mu.RUnlock()

	if exists {
		t.Error("Success should remove peer from backoff map")
	}
}

func TestRecordRendezvousAttempt_FailureSetsMinBackoff(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	d.recordRendezvousAttempt("peer1", false)

	d.mu.RLock()
	nextAttempt, exists := d.rendezvousBackoff["peer1"]
	d.mu.RUnlock()

	if !exists {
		t.Fatal("Failure should add peer to backoff map")
	}

	minAllowed := time.Now().Add(RendezvousMinBackoff - 100*time.Millisecond)
	if nextAttempt.nextAttempt.Before(minAllowed) {
		t.Errorf("First failure backoff too short: %v (expected at least %v)", nextAttempt.nextAttempt, minAllowed)
	}
	if nextAttempt.duration != RendezvousMinBackoff {
		t.Errorf("First failure duration should be min backoff: got %v", nextAttempt.duration)
	}
}

func TestRecordRendezvousAttempt_ExponentialBackoff(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	d.recordRendezvousAttempt("peer1", false)
	d.mu.RLock()
	firstBackoff := d.rendezvousBackoff["peer1"]
	d.mu.RUnlock()

	time.Sleep(10 * time.Millisecond)

	d.recordRendezvousAttempt("peer1", false)
	d.mu.RLock()
	secondBackoff := d.rendezvousBackoff["peer1"]
	d.mu.RUnlock()

	if secondBackoff.duration <= firstBackoff.duration {
		t.Errorf("Second failure should have longer backoff duration: first=%v, second=%v", firstBackoff.duration, secondBackoff.duration)
	}
	if !secondBackoff.nextAttempt.After(firstBackoff.nextAttempt) {
		t.Errorf("Second failure should have later next attempt: first=%v, second=%v", firstBackoff.nextAttempt, secondBackoff.nextAttempt)
	}
}

func TestRecordRendezvousAttempt_BackoffCappedAtMax(t *testing.T) {
	cfg, _ := daemon.NewConfig(daemon.DaemonOpts{Secret: "test"})
	d, _ := NewDHTDiscovery(cfg, &LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())

	for i := 0; i < 10; i++ {
		d.recordRendezvousAttempt("peer1", false)
		time.Sleep(time.Millisecond)
	}

	d.mu.RLock()
	nextAttempt := d.rendezvousBackoff["peer1"]
	d.mu.RUnlock()

	maxAllowed := time.Now().Add(RendezvousMaxBackoff + 100*time.Millisecond)
	if nextAttempt.nextAttempt.After(maxAllowed) {
		t.Errorf("Backoff should be capped at max: %v (expected at most %v)", nextAttempt.nextAttempt, maxAllowed)
	}
	if nextAttempt.duration > RendezvousMaxBackoff {
		t.Errorf("Backoff duration should be capped at max: got %v", nextAttempt.duration)
	}
}

func TestIsPublicIPv6(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"2001:db8::1", false}, // Documentation prefix
		{"200:b64f::1", false}, // Apple Private Relay / Teredo range
		{"fc00::1", false},     // ULA
		{"fd00::1", false},     // ULA
		{"fe80::1", false},     // Link-local
		{"::1", false},         // Loopback
		{"ff02::1", false},     // Multicast
		{"2607:f8b0::1", true}, // Google public IPv6
		{"2a00:1450::1", true}, // Google public IPv6
		{"2606:4700::1", true}, // Cloudflare public IPv6
		{"::", false},          // Unspecified
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP %s", tt.ip)
			}
			got := isPublicIPv6(ip)
			if got != tt.expected {
				t.Errorf("isPublicIPv6(%s) = %v, want %v", tt.ip, got, tt.expected)
			}
		})
	}
}
