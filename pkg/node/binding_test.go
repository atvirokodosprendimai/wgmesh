package node

import (
	"bytes"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
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

func TestIPClaimRefusalIsLoggedOnce(t *testing.T) {
	setSwitches(t, true, false)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	ps, _ := bindingStore(t)
	victimIP := fakeDerive("victim", 0)
	ps.SetPeerDirectly("victim", &PeerInfo{WGPubKey: "victim", MeshIP: victimIP})
	for i := 0; i < 3; i++ {
		ps.Offer(firstHand("thief", victimIP))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("%d log lines for three refusals of one key, want 1:\n%s", len(lines), buf.String())
	}
	for _, want := range []string{"[Admit]", "thief", victimIP, "ip-held-by-bound", "bound claim of another member holds this address"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log line %q lacks %q", lines[0], want)
		}
	}
	ps.Offer(firstHand("thief2", victimIP))
	if n := strings.Count(buf.String(), "ip-held-by-bound"); n != 2 {
		t.Errorf("a second key's refusal logged %d lines in all, want 2", n)
	}
}

// R5 refuses only a first-hand new key whose claim is unbound for an address a
// bound claim of another member holds.
func TestIPClaimNeverRefusedWithoutHeldBoundAddress(t *testing.T) {
	setSwitches(t, true, true)

	t.Run("empty mesh IP", func(t *testing.T) {
		ps, warned := bindingStore(t)
		ps.SetPeerDirectly("victim", &PeerInfo{WGPubKey: "victim", MeshIP: fakeDerive("victim", 0)})
		if d := ps.Offer(firstHand("noip", "")); d.Verdict != Admit {
			t.Errorf("empty MeshIP: %+v, want Admit", d)
		}
		if len(*warned) != 0 {
			t.Errorf("empty MeshIP warned %v", *warned)
		}
	})

	t.Run("bound claimant on a bound holder", func(t *testing.T) {
		// Two keys whose natural derivation collides: both claims are bound,
		// the order between them belongs to R4, not to R5.
		const shared = "10.42.5.5"
		ps := NewPeerStore()
		ps.SetPolicy(&Policy{Derive: func(string, int) string { return shared }})
		ps.SetPeerDirectly("holder", &PeerInfo{WGPubKey: "holder", MeshIP: shared})
		if d := ps.Offer(firstHand("claimant", shared)); d.Verdict != Admit {
			t.Errorf("bound claimant on a bound holder: %+v, want Admit", d)
		}
		if _, ok := ps.Get("claimant"); !ok {
			t.Error("the claimant was not stored")
		}
	})
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
	// 10.42.0.0/15 starts inside the mesh network (10.42.0.0/16) but is shorter: a supernet.
	routes := []string{"0.0.0.0/0", "::/0", "10.0.0.0/8", "10.42.0.0/15"}
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

// acceptStore is bindingStore with an accept list (R9).
func acceptStore(t *testing.T, list ...string) (*PeerStore, *[]string) {
	t.Helper()
	ps, warned := bindingStore(t)
	var nets []*net.IPNet
	for _, s := range list {
		nets = append(nets, mustCIDR(t, s))
	}
	ps.Policy().AcceptRoutes = func() []*net.IPNet { return nets }
	return ps, warned
}

func TestAcceptRoutesEmptyIsOpen(t *testing.T) {
	setSwitches(t, false, true)
	for _, list := range [][]*net.IPNet{nil, {}} {
		for _, n := range []string{"0.0.0.0/0", "::/0", "192.168.5.0/24", "10.0.0.0/8"} {
			if !Approved(n, list) {
				t.Errorf("Approved(%s, %v) = false, want true for an empty list", n, list)
			}
		}
	}
	ps, warned := acceptStore(t)
	d := ps.Offer(firstHand("adv", "10.42.1.1", "0.0.0.0/0", "192.168.5.0/24", "::/0"))
	if d.Verdict != Admit || len(d.Dropped) != 0 || len(d.Warnings) != 0 || len(*warned) != 0 {
		t.Errorf("decision = %+v, warnings %v: want a plain Admit with an unset list", d, *warned)
	}
}

func TestAcceptRoutesContainmentApproves(t *testing.T) {
	list := []*net.IPNet{mustCIDR(t, "192.168.0.0/16")}
	for _, tc := range []struct {
		network string
		list    []*net.IPNet
		want    bool
	}{
		{"192.168.5.0/24", list, true},
		{"192.168.0.0/16", list, true},
		{"192.168.0.0/15", list, false},
		{"192.168.5.0/24", []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}, false},
		{"192.168.5.0/24", []*net.IPNet{mustCIDR(t, "10.0.0.0/8"), mustCIDR(t, "192.168.5.0/24")}, true},
		{"::ffff:192.168.5.0/120", list, false},
		{"fd00::/64", list, false},
		{"not-a-cidr", list, false},
	} {
		if got := Approved(tc.network, tc.list); got != tc.want {
			t.Errorf("Approved(%s, %v) = %v, want %v", tc.network, tc.list, got, tc.want)
		}
	}
}

func TestDefaultRouteNeedsSlashZeroEntry(t *testing.T) {
	lists := map[string][]*net.IPNet{
		"v4 /0":  {mustCIDR(t, "0.0.0.0/0")},
		"v6 /0":  {mustCIDR(t, "::/0")},
		"others": {mustCIDR(t, "10.0.0.0/8"), mustCIDR(t, "192.168.0.0/16"), mustCIDR(t, "fd00::/8")},
	}
	for _, tc := range []struct {
		list, network string
		want          bool
	}{
		{"v4 /0", "0.0.0.0/0", true},
		{"v4 /0", "::/0", false},
		{"v4 /0", "10.0.0.0/8", true},
		{"v4 /0", "fd00::/8", false},
		{"v6 /0", "::/0", true},
		{"v6 /0", "0.0.0.0/0", false},
		{"v6 /0", "fd00::/8", true},
		{"v6 /0", "10.0.0.0/8", false},
		{"others", "0.0.0.0/0", false},
		{"others", "::/0", false},
	} {
		if got := Approved(tc.network, lists[tc.list]); got != tc.want {
			t.Errorf("Approved(%s, %s) = %v, want %v", tc.network, tc.list, got, tc.want)
		}
	}
}

func TestHAAdvertisersBothApproved(t *testing.T) {
	setSwitches(t, false, true)
	ps, warned := acceptStore(t, "192.168.0.0/16")
	for _, key := range []string{"ha1", "ha2"} {
		d := ps.Offer(firstHand(key, "10.42.1."+key[2:], "192.168.5.0/24"))
		if d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Fatalf("%s: decision = %+v, want a plain Admit although the other member advertises the same network", key, d)
		}
		if got, _ := ps.Get(key); got == nil || !reflect.DeepEqual(got.RoutableNetworks, []string{"192.168.5.0/24"}) {
			t.Errorf("%s: stored routes = %+v, want the shared network kept", key, got)
		}
	}
	if len(*warned) != 0 {
		t.Errorf("warnings = %v, want none", *warned)
	}
}

func TestRouteNotApprovedWarnsThenDrops(t *testing.T) {
	routes := []string{"192.168.5.0/24", "172.16.0.0/12", "10.1.0.0/16"}

	t.Run("warn at N", func(t *testing.T) {
		setSwitches(t, false, false)
		ps, warned := acceptStore(t, "192.168.0.0/16")
		d := ps.Offer(firstHand("adv", "10.42.1.1", routes...))
		if d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Fatalf("decision = %+v, want a plain Admit at release N", d)
		}
		if got, _ := ps.Get("adv"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes) {
			t.Errorf("stored routes = %+v, want all of %v", got, routes)
		}
		if want := []string{"route-not-approved", "route-not-approved"}; !reflect.DeepEqual(*warned, want) {
			t.Errorf("warnings = %v, want %v", *warned, want)
		}
	})

	t.Run("drop when enforced", func(t *testing.T) {
		setSwitches(t, false, true)
		ps, warned := acceptStore(t, "192.168.0.0/16")
		c := firstHand("adv", "10.42.1.1", routes...)
		d := ps.Offer(c)
		if d.Verdict != AdmitPartial || !reflect.DeepEqual(d.Dropped, routes[1:]) {
			t.Fatalf("decision = %+v, want AdmitPartial dropping %v", d, routes[1:])
		}
		if got, _ := ps.Get("adv"); got == nil || !reflect.DeepEqual(got.RoutableNetworks, routes[:1]) {
			t.Errorf("stored routes = %+v, want only %v", got, routes[:1])
		}
		if !reflect.DeepEqual(c.Info.RoutableNetworks, routes) {
			t.Errorf("the caller's candidate was modified: %v", c.Info.RoutableNetworks)
		}
		if len(*warned) != 0 {
			t.Errorf("an enforced drop also warned: %v", *warned)
		}
	})

	t.Run("restored is judged and second-hand is not", func(t *testing.T) {
		setSwitches(t, false, true)
		ps, _ := acceptStore(t, "192.168.0.0/16")
		r := firstHand("cached", "10.42.1.2", routes...)
		r.Hand, r.Source = Restored, "cache"
		if d := ps.Offer(r); d.Verdict != AdmitPartial || !reflect.DeepEqual(d.Dropped, routes[1:]) {
			t.Errorf("restored: decision = %+v, want AdmitPartial dropping %v", d, routes[1:])
		}
		s := firstHand("mentioned", "10.42.1.3", routes...)
		s.Hand, s.Source = SecondHand, "gossip"
		if d := ps.Offer(s); d.Verdict != Admit || len(d.Dropped) != 0 {
			t.Errorf("second-hand: decision = %+v, want it untouched", d)
		}
	})
}
