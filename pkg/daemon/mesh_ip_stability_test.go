package daemon

import (
	"net"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// stabilityTestConfig builds a Config for the stability tests. The persisted
// mesh IP is derived from testConfigSecret + testPubKey, so it always falls
// within the subnet implied by keys.
func stabilityTestConfig(t *testing.T) (*Config, *crypto.DerivedKeys) {
	t.Helper()

	keys, err := crypto.DeriveKeys(testConfigSecret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	return &Config{Secret: testConfigSecret, Keys: keys}, keys
}

const stabilityTestPubKey = "stability-test-pubkey"

// TestResolveLoadedNodeIPs_StableAcrossOptionChanges asserts the issue #827
// invariant: once a node has a persisted mesh IP inside the mesh subnet,
// toggling unrelated options (--no-ipv6 / DisableIPv6, relay/punching flags,
// privacy, gossip, introducer, LAN discovery) must not change it.
func TestResolveLoadedNodeIPs_StableAcrossOptionChanges(t *testing.T) {
	_, keys := stabilityTestConfig(t)
	persistedIP := crypto.DeriveMeshIP(keys.MeshSubnet, stabilityTestPubKey, testConfigSecret)
	persistedIPv6 := crypto.DeriveMeshIPv6(keys.MeshPrefixV6, stabilityTestPubKey, testConfigSecret)

	tests := []struct {
		name   string
		mutate func(cfg *Config)
	}{
		{"baseline (no options)", func(cfg *Config) {}},
		{"DisableIPv6 toggled on (--no-ipv6)", func(cfg *Config) { cfg.DisableIPv6 = true }},
		{"ForceRelay toggled on", func(cfg *Config) { cfg.ForceRelay = true }},
		{"DisablePunching toggled on", func(cfg *Config) { cfg.DisablePunching = true }},
		{"relay + punching + no-ipv6 combined", func(cfg *Config) {
			cfg.ForceRelay = true
			cfg.DisablePunching = true
			cfg.DisableIPv6 = true
		}},
		{"Privacy toggled on", func(cfg *Config) { cfg.Privacy = true }},
		{"Gossip toggled on", func(cfg *Config) { cfg.Gossip = true }},
		{"LANDiscovery toggled off", func(cfg *Config) { cfg.LANDiscovery = false }},
		{"Introducer toggled on", func(cfg *Config) { cfg.Introducer = true }},
		{"different interface name", func(cfg *Config) { cfg.InterfaceName = "wg1" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Secret: testConfigSecret, Keys: keys}
			tt.mutate(cfg)

			node := &LocalNode{
				WGPubKey: stabilityTestPubKey,
				MeshIP:   persistedIP,
				MeshIPv6: persistedIPv6,
			}

			changed, err := resolveLoadedNodeIPs(node, cfg)
			if err != nil {
				t.Fatalf("resolveLoadedNodeIPs: %v", err)
			}
			if changed {
				t.Error("changed = true, want false (persisted IPs are valid; nothing should be re-derived)")
			}
			if node.MeshIP != persistedIP {
				t.Errorf("MeshIP = %q, want persisted %q", node.MeshIP, persistedIP)
			}
			if node.MeshIPv6 != persistedIPv6 {
				t.Errorf("MeshIPv6 = %q, want persisted %q", node.MeshIPv6, persistedIPv6)
			}
		})
	}
}

// TestResolveLoadedNodeIPs_ReDerivesWhenSubnetChanged pins the existing
// behavior: a persisted IP that falls outside a newly-passed --mesh-subnet is
// no longer valid and must be re-derived.
func TestResolveLoadedNodeIPs_ReDerivesWhenSubnetChanged(t *testing.T) {
	t.Parallel()

	_, customNet, err := net.ParseCIDR("192.168.100.0/24")
	if err != nil {
		t.Fatalf("failed to parse custom subnet: %v", err)
	}

	keys, err := crypto.DeriveKeys(testConfigSecret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	staleIP := crypto.DeriveMeshIP(keys.MeshSubnet, stabilityTestPubKey, testConfigSecret)

	tests := []struct {
		name       string
		cfg        *Config
		persisted  string
		wantIP     string
		wantChange bool
	}{
		{
			name: "legacy subnet byte changed",
			cfg: &Config{
				Secret: testConfigSecret,
				Keys:   &crypto.DerivedKeys{MeshSubnet: [2]byte{keys.MeshSubnet[0] + 1, keys.MeshSubnet[1]}},
			},
			persisted:  staleIP,
			wantIP:     "", // computed below: DeriveMeshIP with the new subnet
			wantChange: true,
		},
		{
			name: "custom subnet passed after legacy join",
			cfg: &Config{
				Secret:       testConfigSecret,
				Keys:         keys,
				CustomSubnet: customNet,
			},
			persisted:  staleIP,
			wantIP:     "", // computed below: DeriveMeshIPInSubnet(customNet)
			wantChange: true,
		},
	}

	for i := range tests {
		tt := &tests[i]
		if tt.name == "legacy subnet byte changed" {
			tt.wantIP = crypto.DeriveMeshIP(tt.cfg.Keys.MeshSubnet, stabilityTestPubKey, testConfigSecret)
		} else {
			ip, err := crypto.DeriveMeshIPInSubnet(customNet, stabilityTestPubKey, testConfigSecret)
			if err != nil {
				t.Fatalf("DeriveMeshIPInSubnet: %v", err)
			}
			tt.wantIP = ip
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := &LocalNode{WGPubKey: stabilityTestPubKey, MeshIP: tt.persisted}

			changed, err := resolveLoadedNodeIPs(node, tt.cfg)
			if err != nil {
				t.Fatalf("resolveLoadedNodeIPs: %v", err)
			}
			if changed != tt.wantChange {
				t.Errorf("changed = %v, want %v", changed, tt.wantChange)
			}
			if node.MeshIP != tt.wantIP {
				t.Errorf("MeshIP = %q, want re-derived %q", node.MeshIP, tt.wantIP)
			}
		})
	}
}

// TestResolveLoadedNodeIPs_DerivesAbsentFields covers old state files that
// predate mesh_ip persistence (#540): absent fields are derived once.
func TestResolveLoadedNodeIPs_DerivesAbsentFields(t *testing.T) {
	t.Parallel()

	cfg, keys := stabilityTestConfig(t)
	node := &LocalNode{WGPubKey: stabilityTestPubKey}

	changed, err := resolveLoadedNodeIPs(node, cfg)
	if err != nil {
		t.Fatalf("resolveLoadedNodeIPs: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true (absent IPs must be derived)")
	}
	if want := crypto.DeriveMeshIP(keys.MeshSubnet, stabilityTestPubKey, testConfigSecret); node.MeshIP != want {
		t.Errorf("MeshIP = %q, want %q", node.MeshIP, want)
	}
	if want := crypto.DeriveMeshIPv6(keys.MeshPrefixV6, stabilityTestPubKey, testConfigSecret); node.MeshIPv6 != want {
		t.Errorf("MeshIPv6 = %q, want %q", node.MeshIPv6, want)
	}
}
