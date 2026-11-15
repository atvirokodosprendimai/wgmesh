package daemon

import (
	"encoding/json"
	"net"
	"os"
	"reflect"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
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
 
// A hand-built subnet with host bits set still names its network.
func TestPolicyMeshNetNormalisesHostBits(t *testing.T) {
	cfg := testConfig(t)
	cfg.CustomSubnet = &net.IPNet{IP: net.IPv4(192, 168, 100, 7).To4(), Mask: net.CIDRMask(24, 32)}
	d, err := NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	var warned []string
	p := *d.peerStore.Policy()
	p.OnWarn = func(rule string) { warned = append(warned, rule) }
	d.peerStore.SetPolicy(&p)

	d.peerStore.Offer(node.Candidate{
		Info:   &node.PeerInfo{WGPubKey: "adv", MeshIP: "192.168.100.9", RoutableNetworks: []string{"192.168.100.0/24"}},
		Source: "dht", Hand: node.FirstHand,
	})
	if want := []string{"route-inside-mesh"}; !reflect.DeepEqual(warned, want) {
		t.Errorf("warnings = %v, want %v (MeshNet = %s)", warned, want, p.MeshNet)
	}
	if got := p.MeshNet.String(); got != "192.168.100.0/24" {
		t.Errorf("MeshNet = %s, want the network 192.168.100.0/24 without the host bits", got)
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
