package daemon

import (
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

func TestPolicyMeshNet(t *testing.T) {
	cfg := testConfig(t)
	cfg.Keys.MeshSubnet = [2]byte{42, 7}
	d, err := NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if got := d.peerStore.Policy().MeshNet.String(); got != "10.42.0.0/16" {
		t.Errorf("default MeshNet = %s, want 10.42.0.0/16", got)
	}

	_, custom, _ := net.ParseCIDR("192.168.100.0/24")
	cfg.CustomSubnet = custom
	d, err = NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if got := d.peerStore.Policy().MeshNet.String(); got != "192.168.100.0/24" {
		t.Errorf("custom MeshNet = %s, want 192.168.100.0/24", got)
	}
}

// The frozen admit fixture records the addresses the derivations produce; this
// keeps the recording honest against today's code.
func TestAdmitFixtureMatchesDerivation(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/compat/admit/v1/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Secret  string `json:"secret"`
		Subnets map[string]struct {
			MeshNet         string  `json:"mesh_net"`
			MeshSubnetBytes [2]byte `json:"mesh_subnet_bytes"`
		} `json:"subnets"`
		Keys map[string]map[string][]string `json:"keys"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for name, sn := range f.Subnets {
		cfg := &Config{Secret: f.Secret, InterfaceName: "wg-test", Keys: &crypto.DerivedKeys{MeshSubnet: sn.MeshSubnetBytes}}
		if name == "custom" {
			_, cfg.CustomSubnet, _ = net.ParseCIDR(sn.MeshNet)
		}
		d, err := NewDaemon(cfg)
		if err != nil {
			t.Fatalf("NewDaemon: %v", err)
		}
		if got := d.meshNet().String(); got != sn.MeshNet {
			t.Errorf("subnet %s: meshNet = %s, fixture says %s", name, got, sn.MeshNet)
		}
		for key, perSubnet := range f.Keys {
			for nonce, want := range perSubnet[name] {
				if got := d.deriveMeshIP(key, nonce); got != want {
					t.Errorf("subnet %s key %s nonce %d: derives %s, fixture says %s", name, key, nonce, got, want)
				}
			}
		}
	}
}
