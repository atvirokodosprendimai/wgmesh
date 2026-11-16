package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// boundDaemon is a node on the address its own key derives, with a state file.
func boundDaemon(t *testing.T, key string) (*Daemon, string) {
	t.Helper()
	d := collisionDaemon(t, key, ownIP(t, key))
	d.stateFile = filepath.Join(t.TempDir(), "wg0.json")
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	return d, d.localNode.GetMeshIP()
}

func readState(t *testing.T, d *Daemon) []byte {
	t.Helper()
	b, err := os.ReadFile(d.stateFile)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// One peer with a lower key, whose own address is not the one it derives,
// claims the address of a node that does derive it: the node keeps it.
func TestLocalNodeOnDerivedAddressKeepsItAgainstUnboundLowerKey(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)
	d, ip := boundDaemon(t, "zzz-local")
	before := readState(t, d)

	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-low", MeshIP: ip}, "test")
	d.CheckAndResolveCollisions()

	if d.localNode.GetMeshIP() != ip || len(*set) != 0 || string(readState(t, d)) != string(before) {
		t.Errorf("the node moved: IP %s (want %s), addresses set %v, state changed %v",
			d.localNode.GetMeshIP(), ip, *set, string(readState(t, d)) != string(before))
	}
}

// The same with every nonce address of the node claimed too: five reconcile
// ticks set no address and write no state.
func TestLocalNodeOnDerivedAddressIsQuietWhenNonceAddressesAreClaimed(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)
	d, ip := boundDaemon(t, "zzz-local")
	before := readState(t, d)

	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-low", MeshIP: ip}, "test")
	for n := 1; n <= node.MaxNonce; n++ {
		d.peerStore.Update(&PeerInfo{
			WGPubKey: fmt.Sprintf("aaa-n%02d", n),
			MeshIP:   DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, "zzz-local", collisionSecret, n),
		}, "test")
	}
	for tick := 0; tick < 5; tick++ {
		d.CheckAndResolveCollisions()
	}

	if len(*set) != 0 || string(readState(t, d)) != string(before) || d.localNode.GetMeshIP() != ip {
		t.Errorf("after 5 ticks: addresses set %d, state changed %v, IP %s (want %s)",
			len(*set), string(readState(t, d)) != string(before), d.localNode.GetMeshIP(), ip)
	}
}

// Two members that both derive the address: the lower key moves, once.
func TestTwoBoundMembersOnOneAddressLowerKeyMoves(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)
	d, ip := boundDaemon(t, "zzz-local")

	// A remote that claims the address with a lower key and does derive it
	// is simulated by a policy whose derive maps both keys to the address.
	derive := func(key string, nonce int) string {
		if nonce == 0 && (key == "zzz-local" || key == "aaa-remote") {
			return ip
		}
		return ""
	}
	pol := *d.peerStore.Policy()
	pol.Derive = derive
	d.peerStore.SetPolicy(&pol)
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: ip}, "test")
	d.CheckAndResolveCollisions()
	d.CheckAndResolveCollisions()

	want := nonceOne(d)
	if d.localNode.GetMeshIP() != want || len(*set) != 1 {
		t.Errorf("IP %s (want %s), addresses set %v (want one)", d.localNode.GetMeshIP(), want, *set)
	}
}

// A node on an address no key derives, with the higher key, yields to a claim
// that is bound: the recheck does not hold back a move the order requires.
func TestLocalUnboundNodeYieldsToBoundClaim(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)
	ip := ownIP(t, "aaa-remote")
	d := collisionDaemon(t, "zzz-local", ip)
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: ip}, "test")
	d.CheckAndResolveCollisions()

	if want := nonceOne(d); d.localNode.GetMeshIP() != want || len(*set) != 1 {
		t.Errorf("IP %s (want %s), addresses set %v (want one)", d.localNode.GetMeshIP(), want, *set)
	}
}
