package daemon

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// TestPersistedMeshIPSurvivesSecretChange pins the operator-visible contract:
// rotating the mesh secret (for example after a leak) must not move a node to
// another address. In the default derivation the /16 comes from the secret, so
// a persisted IP has to stay valid under a secret that derives another /16.
func TestPersistedMeshIPSurvivesSecretChange(t *testing.T) {
	t.Parallel()

	cfgOld, err := NewConfig(DaemonOpts{Secret: "rotation-test-secret-old-0000000000"})
	if err != nil {
		t.Fatal(err)
	}

	var cfgNew *Config
	for i := 0; i < 64 && cfgNew == nil; i++ {
		c, err := NewConfig(DaemonOpts{Secret: fmt.Sprintf("rotation-test-secret-new-%010d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if c.Keys.MeshSubnet[0] != cfgOld.Keys.MeshSubnet[0] {
			cfgNew = c
		}
	}
	if cfgNew == nil {
		t.Fatal("no test secret derives a different mesh /16")
	}

	ip := crypto.DeriveMeshIP(cfgOld.Keys.MeshSubnet, "test-pubkey", cfgOld.Secret)
	if !meshIPInSubnet(ip, cfgOld) {
		t.Fatalf("freshly derived IP %s not accepted under its own secret", ip)
	}
	if !meshIPInSubnet(ip, cfgNew) {
		t.Errorf("persisted IP %s rejected after the secret changed: the node would re-derive and lose its address", ip)
	}
}

// TestPersistedMeshIPFollowsExplicitSubnetChange keeps the other half of the
// contract: an operator who passes a different --mesh-subnet asked for a new
// address range, so a persisted IP outside it is re-derived.
func TestPersistedMeshIPFollowsExplicitSubnetChange(t *testing.T) {
	t.Parallel()

	cfg, err := NewConfig(DaemonOpts{Secret: "rotation-test-secret-old-0000000000", MeshSubnet: "192.168.100.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if meshIPInSubnet("10.12.3.4", cfg) {
		t.Error("10.12.3.4 accepted although --mesh-subnet is 192.168.100.0/24")
	}
	if !meshIPInSubnet("192.168.100.7", cfg) {
		t.Error("192.168.100.7 rejected although it is inside --mesh-subnet")
	}
	if meshIPInSubnet("not-an-ip", cfg) {
		t.Error("garbage accepted as a mesh IP")
	}
}

// TestRestartKeepsResolvedIP: a node that lost a collision restarts, finds
// the other peer gone, and keeps the address it moved to (#830). It must not
// run in parallel: it points localNodeStateDir at a temp directory.
func TestRestartKeepsResolvedIP(t *testing.T) {
	enforceIPBinding(t, false)
	recordAddresses(t)

	dir := t.TempDir()
	old := localNodeStateDir
	localNodeStateDir = dir
	t.Cleanup(func() { localNodeStateDir = old })

	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	d.stateFile = filepath.Join(dir, d.config.InterfaceName+".json")
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.CheckAndResolveCollisions()
	want := nonceOne(d)
	if d.localNode.MeshIP != want {
		t.Fatalf("local IP = %s, want the nonce-1 address %s", d.localNode.MeshIP, want)
	}

	restarted := &Daemon{config: d.config}
	if err := restarted.initLocalNode(); err != nil {
		t.Fatalf("initLocalNode: %v", err)
	}
	if restarted.localNode.MeshIP != want {
		t.Errorf("after restart the IP is %s, want the resolved address %s", restarted.localNode.MeshIP, want)
	}
}
