package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// The loser's resolved address is written to the state file (#830).
func TestCollisionLoserPersistsResolvedIP(t *testing.T) {
	enforceIPBinding(t, false)
	recordAddresses(t)

	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	d.stateFile = filepath.Join(t.TempDir(), "wg0.json")
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.CheckAndResolveCollisions()

	want := nonceOne(d)
	if want == "10.42.1.1" {
		t.Fatalf("test needs a resolved address that differs from the natural one, both %s", want)
	}
	if d.localNode.MeshIP != want {
		t.Fatalf("local IP = %s, want the nonce-1 address %s", d.localNode.MeshIP, want)
	}
	saved, err := loadLocalNode(d.stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.MeshIP != want {
		t.Errorf("state file holds %s, want the resolved address %s", saved.MeshIP, want)
	}
}

// A failed save keeps the resolved address in memory and on the interface (#830).
func TestCollisionKeepsAddressWhenSaveFails(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)

	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	blocker := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.stateFile = filepath.Join(blocker, "wg0.json")
	if err := saveLocalNode(d.stateFile, d.localNode); err == nil {
		t.Fatal("test needs a state file that cannot be saved")
	}
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.CheckAndResolveCollisions()

	want := nonceOne(d)
	if want == "10.42.1.1" {
		t.Fatalf("test needs a resolved address that differs from the natural one, both %s", want)
	}
	if d.localNode.MeshIP != want {
		t.Errorf("local IP = %s, want the resolved address %s", d.localNode.MeshIP, want)
	}
	if wantSet := fmt.Sprintf("%s/%d", want, d.config.PrefixLen()); len(*set) != 1 || (*set)[0] != wantSet {
		t.Errorf("addresses set = %v, want [%s]", *set, wantSet)
	}
}

func TestDetectCollisions(t *testing.T) {
	ps := NewPeerStore()

	// No collisions with empty store
	collisions := DetectCollisions(ps, node.Claim{})
	if len(collisions) != 0 {
		t.Errorf("Expected 0 collisions, got %d", len(collisions))
	}

	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1"}, "test")
	ps.Update(&PeerInfo{WGPubKey: "key2", MeshIP: "10.0.0.2"}, "test")

	collisions = DetectCollisions(ps, node.Claim{})
	if len(collisions) != 0 {
		t.Errorf("Expected 0 collisions, got %d", len(collisions))
	}

	ps.Update(&PeerInfo{WGPubKey: "key3", MeshIP: "10.0.0.1"}, "test")

	collisions = DetectCollisions(ps, node.Claim{})
	if len(collisions) != 1 {
		t.Errorf("Expected 1 collision, got %d", len(collisions))
	}

	if len(collisions) > 0 && collisions[0].MeshIP != "10.0.0.1" {
		t.Errorf("Expected collision on 10.0.0.1, got %s", collisions[0].MeshIP)
	}
}

func TestDeterministicWinner(t *testing.T) {
	peer1 := &PeerInfo{WGPubKey: "aaa"}
	peer2 := &PeerInfo{WGPubKey: "bbb"}

	winner, loser := DeterministicWinner(peer1, peer2)
	if winner.WGPubKey != "aaa" {
		t.Error("Lower pubkey should win")
	}
	if loser.WGPubKey != "bbb" {
		t.Error("Higher pubkey should lose")
	}

	// Test reverse order
	winner, loser = DeterministicWinner(peer2, peer1)
	if winner.WGPubKey != "aaa" {
		t.Error("Lower pubkey should win regardless of order")
	}
}

func TestDeriveMeshIPWithNonce(t *testing.T) {
	meshSubnet := [2]byte{42, 0}

	ip0 := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 0)
	ip1 := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 1)

	if ip0 == ip1 {
		t.Error("Different nonces should produce different IPs")
	}

	// Should be deterministic
	ip1b := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 1)
	if ip1 != ip1b {
		t.Error("Same nonce should produce same IP")
	}
}

func TestDeriveMeshIPWithCollisionCheck(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"

	existingIPs := map[string]string{} // No existing IPs

	// Legacy mode (nil custom subnet)
	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, nil)
	if ip == "" {
		t.Error("Expected non-empty IP")
	}

	// Test with a collision
	existingIPs[ip] = "other-pubkey"
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, nil)

	// Should get a different IP due to nonce
	if ip == ip2 {
		t.Error("Should derive different IP when collision exists")
	}
}

func TestDeriveMeshIPWithCollisionCheckCustomSubnet(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"
	_, customSubnet, _ := net.ParseCIDR("192.168.100.0/24")

	existingIPs := map[string]string{}

	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, customSubnet)
	if ip == "" {
		t.Error("Expected non-empty IP")
	}

	// Must be in custom subnet
	parsed := net.ParseIP(ip)
	if !customSubnet.Contains(parsed) {
		t.Errorf("IP %s not in custom subnet %s", ip, customSubnet)
	}

	// Test collision within custom subnet
	existingIPs[ip] = "other-pubkey"
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, customSubnet)
	if ip == ip2 {
		t.Error("Should derive different IP when collision exists")
	}
	parsed2 := net.ParseIP(ip2)
	if !customSubnet.Contains(parsed2) {
		t.Errorf("Collision-resolved IP %s not in custom subnet %s", ip2, customSubnet)
	}
}

const collisionSecret = "test-secret-for-daemon-tests"

func collisionDaemon(t *testing.T, key, ip string) *Daemon {
	t.Helper()
	cfg := testConfig(t)
	cfg.Secret = collisionSecret
	d, err := NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.localNode = &LocalNode{WGPubKey: key, MeshIP: ip}
	return d
}

// ownIP is the address a key derives for itself, so a claim on it is bound.
func ownIP(t *testing.T, key string) string {
	t.Helper()
	return crypto.DeriveMeshIP(testConfig(t).Keys.MeshSubnet, key, collisionSecret)
}

func enforceIPBinding(t *testing.T, on bool) {
	t.Helper()
	old := node.EnforceIPBinding.Load()
	node.EnforceIPBinding.Store(on)
	t.Cleanup(func() { node.EnforceIPBinding.Store(old) })
}

// recordAddresses replaces the interface call so a test sees what the daemon
// would set.
func recordAddresses(t *testing.T) *[]string {
	t.Helper()
	var set []string
	old := applyInterfaceAddress
	applyInterfaceAddress = func(name, address string) error {
		set = append(set, address)
		return nil
	}
	t.Cleanup(func() { applyInterfaceAddress = old })
	return &set
}

func winnerDiffers() float64 {
	return testutil.ToFloat64(admitWarnings.WithLabelValues("winner-differs"))
}

// checkAs runs one collision check of a node that applies the given order.
func checkAs(d *Daemon, enforce bool) {
	old := node.EnforceIPBinding.Load()
	node.EnforceIPBinding.Store(enforce)
	defer func() { node.EnforceIPBinding.Store(old) }()
	d.CheckAndResolveCollisions()
}

func nonceOne(d *Daemon) string {
	return DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, collisionSecret, 1)
}

func TestLocalNodeLosesCollisionAgainstRemote(t *testing.T) {
	cases := []struct {
		name                string
		localKey, remoteKey string
		moves               bool
	}{
		{"local has the higher key", "zzz-local", "aaa-remote", true},
		{"local has the lower key", "aaa-local", "zzz-remote", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enforceIPBinding(t, false)
			set := recordAddresses(t)

			d := collisionDaemon(t, tc.localKey, "10.42.1.1")
			d.peerStore.Update(&PeerInfo{WGPubKey: tc.remoteKey, MeshIP: "10.42.1.1"}, "test")
			d.CheckAndResolveCollisions()

			if !tc.moves {
				if d.localNode.MeshIP != "10.42.1.1" || len(*set) != 0 {
					t.Errorf("the winner moved: IP %s, addresses set %v", d.localNode.MeshIP, *set)
				}
				return
			}
			want := nonceOne(d)
			if d.localNode.MeshIP != want {
				t.Errorf("local IP = %s, want the nonce-1 address %s", d.localNode.MeshIP, want)
			}
			if wantSet := fmt.Sprintf("%s/%d", want, d.config.PrefixLen()); len(*set) != 1 || (*set)[0] != wantSet {
				t.Errorf("addresses set = %v, want [%s]", *set, wantSet)
			}
		})
	}
}

func TestLegacyWinnerAtReleaseN(t *testing.T) {
	cases := []struct {
		name                string
		enforce             bool
		localKey, remoteKey string
		ip                  func(t *testing.T) string
		moves               bool
		warnings            float64
	}{
		{"lower key stays though the other is bound", false, "aaa-local", "zzz-remote",
			func(t *testing.T) string { return ownIP(t, "zzz-remote") }, false, 1},
		{"higher key moves though it is bound", false, "zzz-local", "aaa-remote",
			func(t *testing.T) string { return ownIP(t, "zzz-local") }, true, 1},
		{"same class is not a warning", false, "zzz-local", "aaa-remote",
			func(t *testing.T) string { return "10.42.5.5" }, true, 0},
		{"enforced: the bound claim stays", true, "zzz-local", "aaa-remote",
			func(t *testing.T) string { return ownIP(t, "zzz-local") }, false, 0},
		{"enforced: the unbound claim moves", true, "aaa-local", "zzz-remote",
			func(t *testing.T) string { return ownIP(t, "zzz-remote") }, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enforceIPBinding(t, tc.enforce)
			set := recordAddresses(t)
			ip := tc.ip(t)

			d := collisionDaemon(t, tc.localKey, ip)
			d.peerStore.Update(&PeerInfo{WGPubKey: tc.remoteKey, MeshIP: ip}, "test")
			before := winnerDiffers()
			d.CheckAndResolveCollisions()

			moved := d.localNode.MeshIP != ip
			if moved != tc.moves || (moved && d.localNode.MeshIP != nonceOne(d)) || (len(*set) == 1) != tc.moves {
				t.Errorf("moved = %v (IP %s, addresses set %v), want %v", moved, d.localNode.MeshIP, *set, tc.moves)
			}
			if got := winnerDiffers() - before; got != tc.warnings {
				t.Errorf("winner-differs counted %v, want %v", got, tc.warnings)
			}
		})
	}
}

// A node of release N (lower key decides) and one that already applies the
// bound order disagree on who moves; every pair of observers still ends on
// different addresses.
func TestMixedWinnerRulesConverge(t *testing.T) {
	cases := []struct {
		name   string
		ip     func(t *testing.T) string
		aMoves bool
		bMoves bool
	}{
		{"the rules disagree: both move", func(t *testing.T) string { return ownIP(t, "zzz-node") }, true, true},
		{"the rules agree: the higher key moves", func(t *testing.T) string { return "10.42.7.7" }, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordAddresses(t)
			ip := tc.ip(t)

			a := collisionDaemon(t, "aaa-node", ip)
			b := collisionDaemon(t, "zzz-node", ip)
			a.peerStore.Update(&PeerInfo{WGPubKey: "zzz-node", MeshIP: ip}, "test")
			b.peerStore.Update(&PeerInfo{WGPubKey: "aaa-node", MeshIP: ip}, "test")

			checkAs(a, true)
			checkAs(b, false)

			if (a.localNode.MeshIP != ip) != tc.aMoves || (tc.aMoves && a.localNode.MeshIP != nonceOne(a)) {
				t.Errorf("node a: IP %s, moves = %v wanted", a.localNode.MeshIP, tc.aMoves)
			}
			if (b.localNode.MeshIP != ip) != tc.bMoves || (tc.bMoves && b.localNode.MeshIP != nonceOne(b)) {
				t.Errorf("node b: IP %s, moves = %v wanted", b.localNode.MeshIP, tc.bMoves)
			}
			if a.localNode.MeshIP == b.localNode.MeshIP {
				t.Errorf("both nodes ended on %s", a.localNode.MeshIP)
			}
		})
	}
}

// Old node: lower key, unbound address. New node: higher key, bound address.
// Each keeps its address and the collision stays, as it does today; the old
// node counts the case.
func TestMixedWinnerNeitherMovesIsNotWorse(t *testing.T) {
	set := recordAddresses(t)
	ip := ownIP(t, "zzz-node")

	oldNode := collisionDaemon(t, "aaa-node", ip)
	newNode := collisionDaemon(t, "zzz-node", ip)
	oldNode.peerStore.Update(&PeerInfo{WGPubKey: "zzz-node", MeshIP: ip}, "test")
	newNode.peerStore.Update(&PeerInfo{WGPubKey: "aaa-node", MeshIP: ip}, "test")

	before := winnerDiffers()
	checkAs(oldNode, false)
	checkAs(newNode, true)

	if oldNode.localNode.MeshIP != ip || newNode.localNode.MeshIP != ip || len(*set) != 0 {
		t.Errorf("an address moved: old %s, new %s, addresses set %v", oldNode.localNode.MeshIP, newNode.localNode.MeshIP, *set)
	}
	if got := winnerDiffers() - before; got != 1 {
		t.Errorf("winner-differs counted %v, want 1 (the old node's)", got)
	}
}

func TestCheckAndResolveCollisionsToleratesNoLocalNode(t *testing.T) {
	set := recordAddresses(t)
	d := &Daemon{peerStore: NewPeerStore()}
	d.CheckAndResolveCollisions()
	if len(*set) != 0 {
		t.Errorf("addresses set with no local node: %v", *set)
	}
}

// A third member holds the nonce-1 address: the mover takes the nonce-2 one and
// the next check finds no collision.
func TestLocalMoverSkipsClaimedNonceAddress(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)

	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.peerStore.Update(&PeerInfo{WGPubKey: "mmm-third", MeshIP: nonceOne(d)}, "test")

	d.CheckAndResolveCollisions()
	want := DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, "zzz-local", collisionSecret, 2)
	if d.localNode.MeshIP != want {
		t.Errorf("local IP = %s, want the nonce-2 address %s", d.localNode.MeshIP, want)
	}
	d.CheckAndResolveCollisions()
	if d.localNode.MeshIP != want || len(*set) != 1 {
		t.Errorf("the next check moved again: IP %s, addresses set %v", d.localNode.MeshIP, *set)
	}
}

func TestLocalNodeLosesWithCustomSubnet(t *testing.T) {
	enforceIPBinding(t, false)
	set := recordAddresses(t)
	_, subnet, _ := net.ParseCIDR("192.168.100.0/24")

	cfg := testConfig(t)
	cfg.Secret = collisionSecret
	cfg.CustomSubnet = subnet
	d, err := NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.localNode = &LocalNode{WGPubKey: "zzz-local", MeshIP: "192.168.100.7"}
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "192.168.100.7"}, "test")
	d.CheckAndResolveCollisions()

	want, err := crypto.DeriveMeshIPInSubnetWithNonce(subnet, "zzz-local", collisionSecret, 1)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if d.localNode.MeshIP != want || !subnet.Contains(net.ParseIP(want)) {
		t.Errorf("local IP = %s, want %s inside %s", d.localNode.MeshIP, want, subnet)
	}
	if wantSet := fmt.Sprintf("%s/%d", want, d.config.PrefixLen()); len(*set) != 1 || (*set)[0] != wantSet {
		t.Errorf("addresses set = %v, want [%s]", *set, wantSet)
	}
}
