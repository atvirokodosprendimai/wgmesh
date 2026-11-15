package node

import (
	"reflect"
	"testing"
)

// A transitive mention is second-hand: some other member says "I know peer X
// with this address". It must not rewrite what the peer itself announced, or
// any member holding the shared secret can move another member's mesh IP or
// routes, or flip its introducer flag, by naming it in KnownPeers.
func TestUpdateTransitiveDoesNotOverwriteFirstHandFields(t *testing.T) {
	t.Parallel()

	const key = "victim-pubkey"
	direct := &PeerInfo{
		WGPubKey:         key,
		MeshIP:           "10.1.2.3",
		MeshIPv6:         "fd00::3",
		RoutableNetworks: []string{"192.168.5.0/24"},
		Introducer:       true,
	}
	claim := &PeerInfo{
		WGPubKey:         key,
		MeshIP:           "10.9.9.9",
		MeshIPv6:         "fd00::9",
		RoutableNetworks: []string{"0.0.0.0/0"},
		Introducer:       false,
	}

	for _, method := range []string{"dht-transitive", "gossip-transitive"} {
		ps := NewPeerStore()
		ps.Update(direct, "dht")
		ps.Update(claim, method)

		got, ok := ps.Get(key)
		if !ok {
			t.Fatalf("%s: peer vanished", method)
		}
		if got.MeshIP != "10.1.2.3" || got.MeshIPv6 != "fd00::3" {
			t.Errorf("%s: mesh IP overwritten: %s / %s", method, got.MeshIP, got.MeshIPv6)
		}
		if !reflect.DeepEqual(got.RoutableNetworks, []string{"192.168.5.0/24"}) {
			t.Errorf("%s: routes overwritten: %v", method, got.RoutableNetworks)
		}
		if !got.Introducer {
			t.Errorf("%s: introducer flag flipped", method)
		}
	}
}

// A first-hand update from the peer itself still replaces its own fields.
func TestUpdateDirectOverwritesFirstHandFields(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	ps.Update(&PeerInfo{WGPubKey: "k", MeshIP: "10.1.1.1", RoutableNetworks: []string{"10.5.0.0/16"}}, "dht")
	ps.Update(&PeerInfo{WGPubKey: "k", MeshIP: "10.2.2.2", RoutableNetworks: []string{"10.6.0.0/16"}, Introducer: true}, "lan")

	got, _ := ps.Get("k")
	if got.MeshIP != "10.2.2.2" || !reflect.DeepEqual(got.RoutableNetworks, []string{"10.6.0.0/16"}) || !got.Introducer {
		t.Errorf("direct update not applied: %+v", got)
	}
}

// A transitive mention may still fill what was never announced first-hand:
// mesh IPs and routes.
func TestUpdateTransitiveFillsMissingFields(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	ps.Update(&PeerInfo{WGPubKey: "k"}, "dht")
	ps.Update(&PeerInfo{WGPubKey: "k", MeshIP: "10.3.3.3", MeshIPv6: "fd00::33", RoutableNetworks: []string{"10.7.0.0/16"}}, "gossip-transitive")

	got, _ := ps.Get("k")
	if got.MeshIP != "10.3.3.3" || got.MeshIPv6 != "fd00::33" {
		t.Errorf("transitive did not fill empty fields: %s / %s", got.MeshIP, got.MeshIPv6)
	}
	if !reflect.DeepEqual(got.RoutableNetworks, []string{"10.7.0.0/16"}) {
		t.Errorf("transitive did not fill empty routes: %v", got.RoutableNetworks)
	}
}

// A non-empty first-hand announcement replaces the previous routes.
func TestUpdateDirectNonEmptyRoutesReplace(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"dht", "gossip"} {
		ps := NewPeerStore()
		ps.Update(&PeerInfo{WGPubKey: "k", RoutableNetworks: []string{"10.5.0.0/16"}}, "lan")
		ps.Update(&PeerInfo{WGPubKey: "k", RoutableNetworks: []string{"10.6.0.0/16"}}, method)

		got, _ := ps.Get("k")
		if !reflect.DeepEqual(got.RoutableNetworks, []string{"10.6.0.0/16"}) {
			t.Errorf("%s: routes not replaced: %v", method, got.RoutableNetworks)
		}
	}
}
