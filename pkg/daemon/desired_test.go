package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

func routesTo(ds *desiredState, network string) []routes.Entry {
	var out []routes.Entry
	for _, r := range ds.Routes {
		if r.Network == network {
			out = append(out, r)
		}
	}
	return out
}

// allowedOf is the AllowedIPs of key in ds; a peer missing from ds has none.
func allowedOf(ds *desiredState, key string) map[string]struct{} {
	if cfg, ok := ds.Peers[key]; ok {
		return cfg.allowed
	}
	return map[string]struct{}{}
}

func ownersInAllowed(ds *desiredState, network string) []string {
	var out []string
	for key, cfg := range ds.Peers {
		if _, ok := cfg.allowed[network]; ok {
			out = append(out, key)
		}
	}
	return out
}

func TestDesireRoutesAndAllowedIPsFromOneOwner(t *testing.T) {
	const shared = "192.168.50.0/24"
	pa := &PeerInfo{WGPubKey: "pa", MeshIP: "10.0.0.30", LastSeen: time.Now(), RoutableNetworks: []string{shared, "192.168.70.0/24"}}
	pb := &PeerInfo{WGPubKey: "pb", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{shared}}
	peers := []*PeerInfo{pa, pb}

	check := func(t *testing.T, ds *desiredState, wantOwner, wantGateway string) {
		t.Helper()
		if got := ownersInAllowed(ds, shared); len(got) != 1 || got[0] != wantOwner {
			t.Errorf("AllowedIPs owners of %s = %v, want [%s]", shared, got, wantOwner)
		}
		rs := routesTo(ds, shared)
		if len(rs) != 1 || rs[0].Gateway != wantGateway {
			t.Errorf("routes to %s = %v, want one via %s", shared, rs, wantGateway)
		}
		for _, p := range peers {
			if _, ok := allowedOf(ds, p.WGPubKey)[p.MeshIP+"/32"]; !ok {
				t.Errorf("peer %s lost its mesh /32: %v", p.WGPubKey, allowedOf(ds, p.WGPubKey))
			}
		}
		if rs := routesTo(ds, "192.168.70.0/24"); len(rs) != 1 || rs[0].Gateway != "10.0.0.30" {
			t.Errorf("unshared network route = %v, want one via 10.0.0.30", rs)
		}
	}

	t.Run("lowest mesh IP", func(t *testing.T) {
		d := makeRelayTestDaemon()
		check(t, d.desire(peers, nil, map[string]string{}), "pb", "10.0.0.20")
	})

	t.Run("observed fresh owner keeps the network", func(t *testing.T) {
		d := makeRelayTestDaemon()
		fresh := time.Now().Add(-10 * time.Second).Unix()
		hs := map[string]int64{"pa": fresh, "pb": fresh}
		check(t, d.desire(peers, hs, map[string]string{shared: "pa"}), "pa", "10.0.0.30")
	})

	t.Run("fresh beats stale", func(t *testing.T) {
		d := makeRelayTestDaemon()
		hs := map[string]int64{
			"pa": time.Now().Add(-10 * time.Second).Unix(),
			"pb": time.Now().Add(-HandshakeStaleAfter - time.Minute).Unix(),
		}
		check(t, d.desire(peers, hs, map[string]string{}), "pa", "10.0.0.30")
	})
}

func TestDesireHAExitNodeDropsPeerDefault(t *testing.T) {
	const def = "0.0.0.0/0"
	peers := func() []*PeerInfo {
		return []*PeerInfo{{WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{def, "192.168.1.0/24"}}}
	}

	t.Run("local advertises default", func(t *testing.T) {
		d := makeRelayTestDaemon()
		d.localNode.RoutableNetworks = []string{def}
		ds := d.desire(peers(), nil, map[string]string{})
		if _, ok := allowedOf(ds, "peer1")[def]; ok {
			t.Errorf("peer default in AllowedIPs: %v", allowedOf(ds, "peer1"))
		}
		if rs := routesTo(ds, def); len(rs) != 0 {
			t.Errorf("default route installed: %v", rs)
		}
		if _, ok := allowedOf(ds, "peer1")["192.168.1.0/24"]; !ok {
			t.Errorf("other network dropped: %v", allowedOf(ds, "peer1"))
		}
	})

	t.Run("local does not advertise default", func(t *testing.T) {
		d := makeRelayTestDaemon()
		ds := d.desire(peers(), nil, map[string]string{})
		if _, ok := allowedOf(ds, "peer1")[def]; !ok {
			t.Errorf("peer default missing from AllowedIPs: %v", allowedOf(ds, "peer1"))
		}
		if rs := routesTo(ds, def); len(rs) != 1 || rs[0].Gateway != "10.0.0.20" {
			t.Errorf("default route = %v, want one via 10.0.0.20", rs)
		}
	})
}

func TestDesireRelayedOwnerGoesToRelay(t *testing.T) {
	const network = "192.168.50.0/24"
	d := makeRelayTestDaemon()
	d.config.ForceRelay = true
	relay := &PeerInfo{WGPubKey: "relay1", MeshIP: "10.0.0.10", Endpoint: "1.2.3.4:51820", Introducer: true, LastSeen: time.Now()}
	target := &PeerInfo{WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{network}}

	ds := d.desire([]*PeerInfo{relay, target}, nil, map[string]string{})
	if ds.relayRoutes["peer1"] != "relay1" {
		t.Fatalf("relay branch not taken: relayRoutes = %v", ds.relayRoutes)
	}
	allowed := allowedOf(ds, "relay1")
	for _, want := range []string{network, "10.0.0.20/32", "10.0.0.10/32"} {
		if _, ok := allowed[want]; !ok {
			t.Errorf("relay AllowedIPs %v miss %s", allowed, want)
		}
	}
	if cfg, ok := ds.Peers["peer1"]; ok {
		t.Errorf("relayed peer has its own AllowedIPs: %v", cfg.allowed)
	}
	if rs := routesTo(ds, network); len(rs) != 1 || rs[0].Gateway != "10.0.0.10" {
		t.Errorf("routes to %s = %v, want one via the relay 10.0.0.10", network, rs)
	}
}

func TestReconnectAllowedMatchesDesired(t *testing.T) {
	const shared = "192.168.50.0/24"
	d := makeRelayTestDaemon()
	pa := &PeerInfo{WGPubKey: "pa", MeshIP: "10.0.0.30", LastSeen: time.Now(), RoutableNetworks: []string{shared, "192.168.70.0/24"}}
	pb := &PeerInfo{WGPubKey: "pb", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{shared}}
	peers := []*PeerInfo{pa, pb}
	ds := d.desire(peers, nil, map[string]string{})

	for _, p := range peers {
		want := strings.Join(mapKeysSorted(allowedOf(ds, p.WGPubKey)), ",")
		if got := d.reconnectAllowedCSV(p, peers, nil); got != want {
			t.Errorf("reconnectAllowedCSV(%s) = %q, desired = %q", p.WGPubKey, got, want)
		}
	}
	// pa is not the owner of the shared network: its reconnect list must not carry it.
	if got, want := d.reconnectAllowedCSV(pa, peers, nil), "10.0.0.30/32,192.168.70.0/24"; got != want {
		t.Errorf("non-owner reconnect = %q, want %q", got, want)
	}
	if got, want := d.reconnectAllowedCSV(pb, peers, nil), "10.0.0.20/32,"+shared; got != want {
		t.Errorf("owner reconnect = %q, want %q", got, want)
	}
}

func TestExcludeOwnOnlyInDesire(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "desired.go" {
			continue
		}
		checked++
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "ExcludeOwn" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "routes" {
				t.Errorf("%s: routes.ExcludeOwn used outside desired.go", fset.Position(sel.Pos()))
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no production files parsed")
	}
}

// reconcile runs from the ticker and from handleSIGHUP; two at once would
// race on every kernel write. The probe is localSubnetsFn, which desire calls
// inside reconcile on every OS: the first caller stays inside until the test
// sees a second reconcile enter (or 2 s pass), and the entries are counted.
func TestReconcileRunsSerialised(t *testing.T) {
	d := makeRelayTestDaemon()
	d.config.InterfaceName = "wgtest-ser0"
	d.peerStore = NewPeerStore()

	var inside, maxInside, calls atomic.Int32
	secondEntered := make(chan struct{})
	d.localSubnetsFn = func() []*net.IPNet {
		n := inside.Add(1)
		for {
			m := maxInside.Load()
			if n <= m || maxInside.CompareAndSwap(m, n) {
				break
			}
		}
		switch calls.Add(1) {
		case 1:
			select {
			case <-secondEntered:
			case <-time.After(2 * time.Second):
			}
		case 2:
			close(secondEntered)
		}
		inside.Add(-1)
		return nil
	}

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command { return &MockCommand{} },
	}
	withMockExecutor(t, mock, func() {
		var wg sync.WaitGroup
		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				d.reconcile()
			}()
		}
		wg.Wait()
	})
	if calls.Load() < 2 {
		t.Fatalf("desire ran %d times, want one per reconcile", calls.Load())
	}
	if m := maxInside.Load(); m != 1 {
		t.Errorf("%d reconciles ran at the same time, want 1", m)
	}
}
