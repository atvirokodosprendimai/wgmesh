package daemon

import (
	"path/filepath"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// Tests adapted from #831 by nycterent: a node's mesh IP must stay the same
// for its membership (issue #827) whatever option is toggled between runs.

const stabilityPubKey = "stability-test-pubkey"

// startWithPersisted writes a state file holding persisted, runs the real
// initLocalNode under opts and returns the resulting node and the state file
// as it is on disk afterwards.
func startWithPersisted(t *testing.T, persisted localNodeState, opts DaemonOpts) (*LocalNode, localNodeState) {
	t.Helper()

	dir := t.TempDir()
	old := localNodeStateDir
	localNodeStateDir = dir
	t.Cleanup(func() { localNodeStateDir = old })

	if opts.Secret == "" {
		opts.Secret = testConfigSecret
	}
	cfg, err := NewConfig(opts)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, cfg.InterfaceName+".json")
	if err := saveLocalNode(path, &LocalNode{
		WGPubKey:     persisted.WGPubKey,
		WGPrivateKey: persisted.WGPrivateKey,
		MeshIP:       persisted.MeshIP,
		MeshIPv6:     persisted.MeshIPv6,
	}); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{config: cfg}
	if err := d.initLocalNode(); err != nil {
		t.Fatalf("initLocalNode: %v", err)
	}
	disk, err := loadLocalNode(path)
	if err != nil {
		t.Fatal(err)
	}
	return d.localNode, localNodeState{MeshIP: disk.MeshIP, MeshIPv6: disk.MeshIPv6}
}

func defaultPersisted(t *testing.T) localNodeState {
	t.Helper()
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret})
	if err != nil {
		t.Fatal(err)
	}
	return localNodeState{
		WGPubKey:     stabilityPubKey,
		WGPrivateKey: "stability-test-private",
		// Inside 10.0.0.0/8 but not what the derivation would pick, so a
		// re-derivation is visible as a change.
		MeshIP:   "10.77.5.9",
		MeshIPv6: crypto.DeriveMeshIPv6(cfg.Keys.MeshPrefixV6, stabilityPubKey, testConfigSecret),
	}
}

// TestPersistedMeshIPStableAcrossOptionChanges runs initLocalNode once per
// option toggle; none of them may move the persisted mesh IP, in memory or in
// the state file.
func TestPersistedMeshIPStableAcrossOptionChanges(t *testing.T) {
	persisted := defaultPersisted(t)

	tests := []struct {
		name string
		opts DaemonOpts
	}{
		{"baseline", DaemonOpts{}},
		{"no-ipv6", DaemonOpts{DisableIPv6: true}},
		{"advertise-routes changed", DaemonOpts{AdvertiseRoutes: []string{"192.168.50.0/24", "172.16.0.0/16"}}},
		{"force-relay", DaemonOpts{ForceRelay: true}},
		{"disable-punching", DaemonOpts{DisablePunching: true}},
		{"introducer", DaemonOpts{Introducer: true}},
		{"relay + punching + no-ipv6 + routes", DaemonOpts{ForceRelay: true, DisablePunching: true, DisableIPv6: true, AdvertiseRoutes: []string{"10.99.0.0/24"}}},
		{"privacy", DaemonOpts{Privacy: true}},
		{"gossip", DaemonOpts{Gossip: true}},
		{"lan discovery disabled", DaemonOpts{DisableLANDiscovery: true}},
		{"listen port", DaemonOpts{WGListenPort: 51999}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, disk := startWithPersisted(t, persisted, tt.opts)
			if node.MeshIP != persisted.MeshIP || disk.MeshIP != persisted.MeshIP {
				t.Errorf("MeshIP memory=%q disk=%q, want persisted %q", node.MeshIP, disk.MeshIP, persisted.MeshIP)
			}
			if node.MeshIPv6 != persisted.MeshIPv6 || disk.MeshIPv6 != persisted.MeshIPv6 {
				t.Errorf("MeshIPv6 memory=%q disk=%q, want persisted %q", node.MeshIPv6, disk.MeshIPv6, persisted.MeshIPv6)
			}
		})
	}
}

// TestPersistedMeshIPKeptInsideCustomSubnet: a node that joined with
// --mesh-subnet keeps its address when restarted with the same subnet, even
// though that address is not the one the derivation would pick today.
func TestPersistedMeshIPKeptInsideCustomSubnet(t *testing.T) {
	p := defaultPersisted(t)
	p.MeshIP = "192.168.100.7"

	node, disk := startWithPersisted(t, p, DaemonOpts{MeshSubnet: "192.168.100.0/24"})
	if node.MeshIP != p.MeshIP || disk.MeshIP != p.MeshIP {
		t.Errorf("MeshIP memory=%q disk=%q, want persisted %q", node.MeshIP, disk.MeshIP, p.MeshIP)
	}
}

// TestPersistedMeshIPOutsideCustomSubnetIsRederived pins today's behaviour for
// an explicit --mesh-subnet change: the persisted IP is replaced by the one
// derived inside the new subnet and the new value is saved.
func TestPersistedMeshIPOutsideCustomSubnetIsRederived(t *testing.T) {
	p := defaultPersisted(t)
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret, MeshSubnet: "192.168.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	want, err := crypto.DeriveMeshIPInSubnet(cfg.CustomSubnet, p.WGPubKey, testConfigSecret)
	if err != nil {
		t.Fatal(err)
	}
	if want == p.MeshIP {
		t.Fatal("test setup: derived IP equals the persisted one")
	}

	node, disk := startWithPersisted(t, p, DaemonOpts{MeshSubnet: "192.168.100.0/24"})
	if node.MeshIP != want || disk.MeshIP != want {
		t.Errorf("MeshIP memory=%q disk=%q, want re-derived %q", node.MeshIP, disk.MeshIP, want)
	}
}

// TestPersistedMeshIPKeptWhenCustomSubnetDropped: a node that joined with
// --mesh-subnet and is restarted without the flag must keep its address.
func TestPersistedMeshIPKeptWhenCustomSubnetDropped(t *testing.T) {
	p := defaultPersisted(t)
	p.MeshIP = "192.168.100.7"

	node, disk := startWithPersisted(t, p, DaemonOpts{})
	if node.MeshIP != p.MeshIP || disk.MeshIP != p.MeshIP {
		t.Errorf("MeshIP memory=%q disk=%q, want persisted %q", node.MeshIP, disk.MeshIP, p.MeshIP)
	}
}

