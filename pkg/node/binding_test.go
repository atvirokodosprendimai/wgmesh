package node

import (
	"net"
	"reflect"
	"testing"
)

func setSwitches(t *testing.T, ip, route bool) {
	t.Helper()
	oldIP, oldRoute := EnforceIPBinding, EnforceRouteBinding
	EnforceIPBinding, EnforceRouteBinding = ip, route
	t.Cleanup(func() { EnforceIPBinding, EnforceRouteBinding = oldIP, oldRoute })
}

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// bindingStore is a store whose policy derives fakeDerive addresses, has the
// mesh network 10.42.0.0/16 and records every warning.
func bindingStore(t *testing.T) (*PeerStore, *[]string) {
	t.Helper()
	warned := new([]string)
	ps := NewPeerStore()
	ps.SetPolicy(&Policy{
		Derive:  fakeDerive,
		MeshNet: mustCIDR(t, "10.42.0.0/16"),
		OnWarn:  func(rule string) { *warned = append(*warned, rule) },
	})
	return ps, warned
}

func firstHand(key, ip string, routes ...string) Candidate {
	return Candidate{Info: &PeerInfo{WGPubKey: key, MeshIP: ip, RoutableNetworks: routes}, Source: "dht", Hand: FirstHand}
}

func TestIPClaimWarnsAtReleaseN(t *testing.T) {
	setSwitches(t, false, false)
	ps, warned := bindingStore(t)
	victimIP := fakeDerive("victim", 0)
	ps.SetPeerDirectly("victim", &PeerInfo{WGPubKey: "victim", MeshIP: victimIP})

	d := ps.Offer(firstHand("thief", victimIP))
	if d.Verdict != Admit {
		t.Fatalf("decision = %+v, want Admit at release N", d)
	}
	if got, ok := ps.Get("thief"); !ok || got.MeshIP != victimIP {
		t.Errorf("the claim was not stored as today: %+v, %v", got, ok)
	}
	if want := []string{"ip-held-by-bound"}; !reflect.DeepEqual(*warned, want) {
		t.Errorf("warnings = %v, want %v", *warned, want)
	}
}

func TestIPClaimRefusedWhenEnforced(t *testing.T) {
	setSwitches(t, true, false)
	ps, warned := bindingStore(t)
	victimIP := fakeDerive("victim", 0)
	ps.SetPeerDirectly("victim", &PeerInfo{WGPubKey: "victim", MeshIP: victimIP})

	d := ps.Offer(firstHand("thief", victimIP))
	if d.Verdict != Refuse || d.Reason != "ip-held-by-bound" {
		t.Fatalf("decision = %+v, want Refuse ip-held-by-bound", d)
	}
	if _, ok := ps.Get("thief"); ok {
		t.Error("a refused claim left an entry")
	}
	if n := ps.Count(); n != 1 {
		t.Errorf("store holds %d peers, want 1 (unchanged)", n)
	}
	if v, _ := ps.Get("victim"); v == nil || v.MeshIP != victimIP {
		t.Errorf("the victim changed: %+v", v)
	}
	if len(*warned) != 0 {
		t.Errorf("an enforced refusal also warned: %v", *warned)
	}

	// The local node is a member like any other.
	ps.SetPolicy(&Policy{
		Derive: fakeDerive,
		Local:  func() (string, string) { return "me", fakeDerive("me", 0) },
	})
	if d := ps.Offer(firstHand("thief2", fakeDerive("me", 0))); d.Verdict != Refuse || d.Reason != "ip-held-by-bound" {
		t.Errorf("claim on the local node's bound address: %+v, want Refuse ip-held-by-bound", d)
	}
}

func TestRouteInsideMeshWarnsThenDrops(t *testing.T) {
	routes := []string{"10.42.7.0/24", "172.16.5.5/32", "192.168.9.0/24"}

	t.Run("warn at N", func(t *testing.T) {
		setSwitches(t, false, false)
		ps, warned := bindingStore(t)
		ps.SetPeerDirectly("member", &PeerInfo{WGPubKey: "member", MeshIP: "172.16.5.5"})
		c := firstHand("adv", "10.42.1.1", routes...)
		d := ps.Offer(c)
		if d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Fatalf("decision = %+v, want a plain Admit", d)
		}
		if got, _ := ps.Get("adv"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes) {
			t.Errorf("stored routes = %+v, want all of %v", got, routes)
		}
		if want := []string{"route-inside-mesh", "route-inside-member-ip"}; !reflect.DeepEqual(*warned, want) {
			t.Errorf("warnings = %v, want %v", *warned, want)
		}
	})

	t.Run("drop when enforced", func(t *testing.T) {
		setSwitches(t, false, true)
		ps, warned := bindingStore(t)
		ps.SetPeerDirectly("member", &PeerInfo{WGPubKey: "member", MeshIP: "172.16.5.5"})
		c := firstHand("adv", "10.42.1.1", routes...)
		d := ps.Offer(c)
		if d.Verdict != AdmitPartial || !reflect.DeepEqual(d.Dropped, routes[:2]) {
			t.Fatalf("decision = %+v, want AdmitPartial dropping %v", d, routes[:2])
		}
		if got, _ := ps.Get("adv"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes[2:]) {
			t.Errorf("stored routes = %+v, want only %v", got, routes[2:])
		}
		if !reflect.DeepEqual(c.Info.RoutableNetworks, routes) {
			t.Errorf("the caller's candidate was modified: %v", c.Info.RoutableNetworks)
		}
		if len(*warned) != 0 {
			t.Errorf("an enforced drop also warned: %v", *warned)
		}
	})
}

func TestRouteSupernetIsKept(t *testing.T) {
	routes := []string{"0.0.0.0/0", "::/0", "10.0.0.0/8"}
	for _, sw := range [][2]bool{{false, false}, {true, true}} {
		setSwitches(t, sw[0], sw[1])
		ps, warned := bindingStore(t)
		d := ps.Offer(firstHand("gw", "10.42.1.1", routes...))
		if d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Errorf("switches %v: decision = %+v, want a plain Admit", sw, d)
		}
		if got, _ := ps.Get("gw"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes) {
			t.Errorf("switches %v: stored routes = %+v, want %v", sw, got, routes)
		}
		if len(*warned) != 0 {
			t.Errorf("switches %v: supernets warned %v", sw, *warned)
		}
	}
}
 
func TestRouteMappedIPv6IsNotInsideMesh(t *testing.T) {
	routes := []string{"::ffff:10.42.0.0/112", "::ffff:10.42.7.0/120"}
	for _, sw := range [][2]bool{{false, false}, {true, true}} {
		setSwitches(t, sw[0], sw[1])
		ps, warned := bindingStore(t)
		d := ps.Offer(firstHand("gw", "10.42.1.1", routes...))
		if d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Errorf("switches %v: decision = %+v, want a plain Admit", sw, d)
		}
		if got, _ := ps.Get("gw"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes) {
			t.Errorf("switches %v: stored routes = %+v, want %v", sw, got, routes)
		}
		if len(*warned) != 0 {
			t.Errorf("switches %v: IPv6 routes warned %v", sw, *warned)
		}
	}
}

func TestRotatedAddressAdmittedInEveryPhase(t *testing.T) {
	for _, sw := range [][2]bool{{false, false}, {true, false}, {true, true}} {
		setSwitches(t, sw[0], sw[1])
		ps, warned := bindingStore(t)
		ps.SetPeerDirectly("other", &PeerInfo{WGPubKey: "other", MeshIP: fakeDerive("other", 0)})
		if d := ps.Offer(firstHand("rotated", "10.42.200.200")); d.Verdict != Admit {
			t.Errorf("switches %v: unbound address with no competing claim: %+v, want Admit", sw, d)
		}
		if _, ok := ps.Get("rotated"); !ok {
			t.Errorf("switches %v: rotated node not stored", sw)
		}
		if len(*warned) != 0 {
			t.Errorf("switches %v: warned %v", sw, *warned)
		}
	}
}

func TestRotatedNodeInStoreIsNeverRefused(t *testing.T) {
	setSwitches(t, true, true)
	ps, _ := bindingStore(t)
	natural := fakeDerive("newcomer", 0)

	if d := ps.Offer(firstHand("rotated", natural)); d.Verdict != Admit {
		t.Fatalf("rotated node first seen: %+v", d)
	}
	if d := ps.Offer(firstHand("newcomer", natural)); d.Verdict != Admit {
		t.Fatalf("newcomer on its own derived address: %+v, want Admit (bound against unbound is an order, not a refusal)", d)
	}
	// Now the rotated node's address is held by a bound claim of another key;
	// the rotated node is already stored, so R5 never refuses it.
	for i := 0; i < 2; i++ {
		if d := ps.Offer(firstHand("rotated", natural)); d.Verdict != Admit {
			t.Fatalf("stored rotated node, announcement %d: %+v, want Admit", i, d)
		}
	}
	if _, ok := ps.Get("rotated"); !ok {
		t.Error("the rotated node left the store")
	}
}
