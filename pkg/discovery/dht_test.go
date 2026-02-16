package discovery

import (
	"fmt"
	"strings"
	"testing"

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
