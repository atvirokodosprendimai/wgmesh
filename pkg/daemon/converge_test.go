package daemon

import (
	"errors"
	"fmt"
	"go/ast"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

type setCall struct{ key, endpoint, allowed string }

// fakeKernel stands in for `wg`: a dump to read, and sets and removes that
// change it the way the kernel would (a network set on one peer leaves the
// others).
type fakeKernel struct {
	mu        sync.Mutex
	peers     wireguard.DumpPeers
	dumpErr   error
	reads     int
	sets      []setCall
	removes   []string
	setErr    map[string]error
	removeErr map[string]error
}

func newFakeKernel() *fakeKernel {
	return &fakeKernel{peers: wireguard.DumpPeers{}, setErr: map[string]error{}, removeErr: map[string]error{}}
}

func (k *fakeKernel) with(key, endpoint string, allowed ...string) *fakeKernel {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.peers[key] = wireguard.DumpPeer{PublicKey: key, Endpoint: endpoint, AllowedIPs: allowed}
	return k
}

func (k *fakeKernel) dump(string) (wireguard.DumpPeers, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.reads++
	if k.dumpErr != nil {
		return nil, k.dumpErr
	}
	out := make(wireguard.DumpPeers, len(k.peers))
	for key, p := range k.peers {
		out[key] = p
	}
	return out, nil
}

func (k *fakeKernel) set(_, key string, _ [32]byte, endpoint, allowed string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.sets = append(k.sets, setCall{key, endpoint, allowed})
	if err := k.setErr[key]; err != nil {
		return err
	}
	nets := strings.Split(allowed, ",")
	for other, p := range k.peers {
		if other == key {
			continue
		}
		var keep []string
		for _, n := range p.AllowedIPs {
			if !contains(nets, n) {
				keep = append(keep, n)
			}
		}
		p.AllowedIPs = keep
		k.peers[other] = p
	}
	p := k.peers[key]
	p.PublicKey = key
	if endpoint != "" {
		p.Endpoint = endpoint
	}
	p.AllowedIPs = nets
	k.peers[key] = p
	return nil
}

func (k *fakeKernel) remove(_, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.removes = append(k.removes, key)
	if err := k.removeErr[key]; err != nil {
		return err
	}
	delete(k.peers, key)
	return nil
}

func (k *fakeKernel) setCalls() []setCall {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]setCall(nil), k.sets...)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

type logSink struct {
	mu    sync.Mutex
	lines []string
}

func (l *logSink) printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logSink) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, substr) {
			n++
		}
	}
	return n
}

func (l *logSink) total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.lines)
}

// newConvergeDaemon is a Daemon whose kernel is k and whose log is ls.
func newConvergeDaemon(t *testing.T, k *fakeKernel, ls *logSink) *Daemon {
	t.Helper()
	return &Daemon{
		config:             &Config{InterfaceName: "wgmesh-t3-none", Keys: makeTestKeys(t)},
		localNode:          nodeWithAddr("local-key", "10.99.0.1", "cone"),
		peerStore:          NewPeerStore(),
		relayRoutes:        make(map[string]string),
		directStableCycles: make(map[string]int),
		temporaryOffline:   make(map[string]time.Time),
		localSubnetsFn:     func() []*net.IPNet { return nil },
		setPeerFn:          k.set,
		removePeerFn:       k.remove,
		dumpFn:             k.dump,
		logFn:              ls.printf,
		goos:               "plan9", // no kernel target is wanted: no iptables, sysctl, ip or route on the test host
	}
}

func testPeer(key, meshIP, endpoint string, networks ...string) *PeerInfo {
	return &PeerInfo{WGPubKey: key, MeshIP: meshIP, Endpoint: endpoint, RoutableNetworks: networks, LastSeen: time.Now()}
}

// seedPushed records endpoints as already pushed.
func seedPushed(d *Daemon, pushed map[string]string) {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	for k, v := range pushed {
		d.pushedLocked()[k] = v
	}
}

func want(peer *PeerInfo, allowed ...string) *desiredPeerConfig {
	set := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return &desiredPeerConfig{peer: peer, allowed: set}
}

func observed(allowed ...string) observedPeer {
	set := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return observedPeer{Allowed: set}
}

func setKeys(p kernelPlan) []string {
	var keys []string
	for _, w := range p.Set {
		keys = append(keys, w.Key)
	}
	return keys
}

func TestObserveFailureSkipsApply(t *testing.T) {
	k := newFakeKernel().with("obsolete-key", "", "10.99.0.9/32")
	k.dumpErr = errors.New("wg show dump failed: exit status 1")
	ls := &logSink{}
	d := newConvergeDaemon(t, k, ls)
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"), testPeer("keyB", "10.99.0.3", "203.0.113.3:51820")}

	for i := 0; i < 3; i++ {
		if ds := d.converge(peers); ds != nil {
			t.Fatalf("tick %d: converge returned a desired state although the kernel could not be read", i)
		}
	}
	if got := k.setCalls(); len(got) != 0 {
		t.Errorf("SetPeer ran after a failed read: %v", got)
	}
	if len(k.removes) != 0 {
		t.Errorf("RemovePeer ran after a failed read: %v", k.removes)
	}
	if n := ls.count("exit status 1"); n != 1 {
		t.Errorf("the observe error was logged %d times over 3 ticks, want 1: %v", n, ls.lines)
	}
}

func TestPlanPeersAllowedIPsDriftReapplies(t *testing.T) {
	// keyA's kernel entry lost 192.168.50.0/24, which the desired state gives
	// it; nothing in the desired state changed.
	k := newFakeKernel().
		with("keyA", "203.0.113.2:51820", "10.99.0.2/32").
		with("keyB", "203.0.113.3:51820", "10.99.0.3/32")
	ls := &logSink{}
	d := newConvergeDaemon(t, k, ls)
	seedPushed(d, map[string]string{"keyA": "203.0.113.2:51820", "keyB": "203.0.113.3:51820"})
	peers := []*PeerInfo{
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.50.0/24"),
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
	}

	if ds := d.converge(peers); ds == nil {
		t.Fatal("converge skipped the tick")
	}
	expect := []setCall{{"keyA", "", "10.99.0.2/32,192.168.50.0/24"}}
	if got := k.setCalls(); !reflect.DeepEqual(got, expect) {
		t.Errorf("sets = %v, want %v (drift-only set: no endpoint, only the drifted peer)", got, expect)
	}
}

func TestPlanPeersFailoverMovesNetwork(t *testing.T) {
	const shared = "192.168.50.0/24"
	ep := "203.0.113.5:51820"
	survivor := testPeer("aaa-survivor", "10.99.0.2", ep, shared)
	shrinker := testPeer("zzz-shrinker", "10.99.0.3", ep)
	desired := map[string]*desiredPeerConfig{
		"aaa-survivor": want(survivor, "10.99.0.2/32", shared),
		"zzz-shrinker": want(shrinker, "10.99.0.3/32"),
	}
	kernel := kernelObserved{Peers: map[string]observedPeer{
		"aaa-survivor": observed("10.99.0.2/32"),
		"zzz-shrinker": observed("10.99.0.3/32", "172.16.0.0/16"),
		"old-owner":    observed("10.99.0.4/32", shared),
	}}
	pushed := map[string]string{"aaa-survivor": ep, "zzz-shrinker": ep, "old-owner": ep}

	plan := planKernel(desired, kernel, pushed, false)

	if !reflect.DeepEqual(plan.Remove, []string{"old-owner"}) {
		t.Errorf("remove = %v, want [old-owner]", plan.Remove)
	}
	if got := setKeys(plan); !reflect.DeepEqual(got, []string{"zzz-shrinker", "aaa-survivor"}) {
		t.Fatalf("set order = %v, want the shrinking peer before the growing one", got)
	}
	if got := plan.Set[1].Allowed; !reflect.DeepEqual(got, []string{"10.99.0.2/32", shared}) {
		t.Errorf("survivor allowed = %v, want its /32 and the shared network", got)
	}
	for _, w := range plan.Set {
		if w.Endpoint != "" {
			t.Errorf("%s: endpoint %q pushed although the desired endpoint was already pushed", w.Key, w.Endpoint)
		}
	}
}

func TestPlanPeersSteadyStateEmpty(t *testing.T) {
	k := newFakeKernel().
		with("keyA", "203.0.113.2:51820", "10.99.0.2/32", "192.168.50.0/24").
		with("keyB", "203.0.113.3:51820", "10.99.0.3/32")
	ls := &logSink{}
	d := newConvergeDaemon(t, k, ls)
	seedPushed(d, map[string]string{"keyA": "203.0.113.2:51820", "keyB": "203.0.113.3:51820"})
	peers := []*PeerInfo{
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.50.0/24"),
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
	}

	for tick := 1; tick <= 5; tick++ {
		if ds := d.converge(peers); ds == nil {
			t.Fatalf("tick %d: skipped", tick)
		}
		if got := k.setCalls(); len(got) != 0 || len(k.removes) != 0 {
			t.Fatalf("tick %d: writes in a steady state: sets %v removes %v", tick, got, k.removes)
		}
	}
	if k.reads != 5 {
		t.Errorf("dump reads = %d over 5 ticks, want 5", k.reads)
	}
	if ls.total() != 0 {
		t.Errorf("a steady state logged: %v", ls.lines)
	}
}

func TestEndpointPushedOnlyWhenDesiredChanges(t *testing.T) {
	const ep1, ep2, roamed = "203.0.113.2:51820", "203.0.113.22:51820", "198.51.100.9:4000"
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	a := testPeer("keyA", "10.99.0.2", ep1)

	d.converge([]*PeerInfo{a})
	if got := k.setCalls(); !reflect.DeepEqual(got, []setCall{{"keyA", ep1, "10.99.0.2/32"}}) {
		t.Fatalf("new peer: sets = %v, want one with its endpoint", got)
	}

	// The peer roamed and the kernel lost its AllowedIPs: the repair must not
	// overwrite the roamed endpoint.
	k.mu.Lock()
	p := k.peers["keyA"]
	p.Endpoint, p.AllowedIPs = roamed, nil
	k.peers["keyA"] = p
	k.mu.Unlock()
	d.converge([]*PeerInfo{a})
	got := k.setCalls()
	if len(got) != 2 || got[1].endpoint != "" {
		t.Fatalf("drift repair: sets = %v, want a second set without endpoint", got)
	}
	if k.peers["keyA"].Endpoint != roamed {
		t.Errorf("kernel endpoint = %q, want the roamed %q kept", k.peers["keyA"].Endpoint, roamed)
	}
	d.converge([]*PeerInfo{a})
	if n := len(k.setCalls()); n != 2 {
		t.Fatalf("a settled peer was written again: %d sets", n)
	}

	// The desired endpoint changes: pushed once.
	a2 := testPeer("keyA", "10.99.0.2", ep2)
	d.converge([]*PeerInfo{a2})
	got = k.setCalls()
	if len(got) != 3 || got[2].endpoint != ep2 {
		t.Fatalf("changed endpoint: sets = %v, want a third set with %s", got, ep2)
	}
	d.converge([]*PeerInfo{a2})
	if n := len(k.setCalls()); n != 3 {
		t.Errorf("the changed endpoint was pushed again: %d sets", n)
	}
}

func TestApplyIsolatesPeerErrors(t *testing.T) {
	t.Run("converge", func(t *testing.T) {
		k := newFakeKernel().with("zz-old", "", "10.99.0.9/32")
		k.setErr["keyB"] = errors.New("wg set failed: boom")
		k.removeErr["zz-old"] = errors.New("wg remove failed")
		ls := &logSink{}
		d := newConvergeDaemon(t, k, ls)
		peers := []*PeerInfo{
			testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"),
			testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
			testPeer("keyC", "10.99.0.4", "203.0.113.4:51820"),
		}
		before := testutil.ToFloat64(kernelChanges.WithLabelValues("peer"))

		d.converge(peers)

		if got := len(k.setCalls()); got != 3 {
			t.Errorf("sets attempted = %d, want 3 (keyB failing must not skip keyC)", got)
		}
		for _, key := range []string{"keyA", "keyC"} {
			if _, ok := k.peers[key]; !ok {
				t.Errorf("%s was not applied", key)
			}
		}
		if _, ok := k.peers["keyB"]; ok {
			t.Error("keyB is in the kernel although its set failed")
		}
		if got := testutil.ToFloat64(kernelChanges.WithLabelValues("peer")) - before; got != 2 {
			t.Errorf("wgmesh_kernel_changes_total rose by %v, want 2 (two sets succeeded)", got)
		}
		d.converge(peers)
		if got := len(k.setCalls()); got != 4 {
			t.Errorf("after the failed peer was retried: %d sets, want 4 (only keyB again)", got)
		}
	})

	t.Run("results", func(t *testing.T) {
		k := newFakeKernel()
		k.setErr["keyB"] = errors.New("boom")
		d := newConvergeDaemon(t, k, &logSink{})
		plan := kernelPlan{
			Remove: []string{"old"},
			Set: []peerWrite{
				{Key: "keyA", Endpoint: "e:1", Allowed: []string{"10.0.0.1/32"}},
				{Key: "keyB", Endpoint: "e:2", Allowed: []string{"10.0.0.2/32"}},
				{Key: "keyC", Endpoint: "e:3", Allowed: []string{"10.0.0.3/32"}},
			},
		}
		d.convergeMu.Lock()
		res := d.applyKernel(plan)
		d.convergeMu.Unlock()
		if len(res) != 4 {
			t.Fatalf("results = %d, want one per item (4)", len(res))
		}
		for _, r := range res {
			if (r.Key == "keyB") != (r.Err != nil) {
				t.Errorf("%s %s: err = %v", r.Op, r.Key, r.Err)
			}
		}
	})
}

func TestStaleCleanupRemovedPeerIsReadded(t *testing.T) {
	const ep = "203.0.113.2:51820"
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	a := testPeer("keyA", "10.99.0.2", ep, "192.168.50.0/24")

	d.converge([]*PeerInfo{a})
	if len(k.setCalls()) != 1 {
		t.Fatalf("first tick: sets = %v", k.setCalls())
	}

	// staleCleanupLoop removes the peer from the kernel; it returns later with
	// the same endpoint and AllowedIPs.
	if err := d.removePeerFromKernel("keyA"); err != nil {
		t.Fatal(err)
	}
	if _, ok := k.peers["keyA"]; ok {
		t.Fatal("stale cleanup left the peer in the kernel")
	}
	if got := pushedOf(d); len(got) != 0 {
		t.Errorf("pushed = %v after removePeerFromKernel, want empty", got)
	}
	d.converge([]*PeerInfo{a})

	got := k.setCalls()
	if len(got) != 2 || got[1].key != "keyA" || got[1].endpoint != ep {
		t.Fatalf("sets = %v, want the returning peer set again with its endpoint", got)
	}
	if _, ok := k.peers["keyA"]; !ok {
		t.Error("the returning peer is absent from the kernel")
	}
}

func TestReconcileForksWgOncePerTick(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	for _, p := range []*PeerInfo{
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"),
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
	} {
		d.peerStore.Update(p, "test")
	}

	d.reconcile()

	if k.reads != 1 {
		t.Errorf("one reconcile read the dump %d times, want 1", k.reads)
	}
	if got := len(k.setCalls()); got != 2 {
		t.Errorf("sets = %d, want 2", got)
	}

	// The old forks `wg show <iface> peers` and `latest-handshakes` are
	// wireguard.GetPeers and wireguard.GetLatestHandshakes: the tick must not
	// reach them.
	banned := map[string]bool{"GetPeers": true, "GetLatestHandshakes": true}
	tick := map[string]bool{"reconcile": true, "converge": true, "observeKernel": true, "applyKernel": true,
		"planKernel": true, "logResults": true, "noteObserve": true}
	for name, f := range parseProduction(t) {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !tick[fn.Name.Name] {
				continue
			}
			selectors(fn, func(x, sel string) {
				if x == "wireguard" && banned[sel] {
					t.Errorf("%s: %s calls wireguard.%s", name, fn.Name.Name, sel)
				}
			})
		}
	}
}

func TestResultLoggedOnChangeOnly(t *testing.T) {
	k := newFakeKernel()
	k.setErr["keyA"] = errors.New("wg set failed: boom")
	ls := &logSink{}
	d := newConvergeDaemon(t, k, ls)
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")}

	for i := 0; i < 10; i++ {
		d.converge(peers)
	}
	if n := ls.count("boom"); n != 1 {
		t.Fatalf("the same failure over ten ticks printed %d lines, want 1: %v", n, ls.lines)
	}
	if got := len(k.setCalls()); got != 10 {
		t.Fatalf("the failing set was tried %d times, want every tick (10)", got)
	}

	before := ls.total()
	delete(k.setErr, "keyA")
	d.converge(peers)
	if n := ls.total() - before; n != 1 {
		t.Fatalf("recovery printed %d lines, want 1: %v", n, ls.lines)
	}
	if ls.count("recovered") != 1 {
		t.Errorf("the recovery line does not say so: %v", ls.lines)
	}
	before = ls.total()
	for i := 0; i < 3; i++ {
		d.converge(peers)
	}
	if n := ls.total() - before; n != 0 {
		t.Errorf("a steady state after recovery printed %d lines", n)
	}
}

// A failure is logged again when its text changes, and again when the item
// dropped out of the plan in between (its entry is pruned).
func TestResultLogRepeatsOnNewTextOrAfterPrune(t *testing.T) {
	t.Run("different text", func(t *testing.T) {
		k := newFakeKernel()
		ls := &logSink{}
		d := newConvergeDaemon(t, k, ls)
		peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")}
		k.setErr["keyA"] = errors.New("boom one")
		for i := 0; i < 3; i++ {
			d.converge(peers)
		}
		k.setErr["keyA"] = errors.New("boom two")
		for i := 0; i < 3; i++ {
			d.converge(peers)
		}
		if n := ls.count("boom one"); n != 1 {
			t.Errorf("first text logged %d times, want 1: %v", n, ls.lines)
		}
		if n := ls.count("boom two"); n != 1 {
			t.Errorf("second text logged %d times, want 1: %v", n, ls.lines)
		}
	})

	t.Run("pruned when the item leaves the plan", func(t *testing.T) {
		k := newFakeKernel()
		ls := &logSink{}
		d := newConvergeDaemon(t, k, ls)
		peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")}
		k.setErr["keyA"] = errors.New("boom")
		d.converge(peers)
		d.converge(peers)
		if n := ls.count("boom"); n != 1 {
			t.Fatalf("same failure logged %d times in 2 ticks, want 1: %v", n, ls.lines)
		}
		d.converge(nil) // keyA is not wanted: no item, the entry is pruned
		d.converge(peers)
		if n := ls.count("boom"); n != 2 {
			t.Errorf("failure after the item came back logged %d times in total, want 2: %v", n, ls.lines)
		}
	})
}

// A reconnect records what it pushed: the next tick has nothing to write.
func TestConvergeAfterReconnectWritesNothing(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	a := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")
	if err := d.reconnectPeerInKernel(a, []*PeerInfo{a}); err != nil {
		t.Fatal(err)
	}
	if got := len(k.setCalls()); got != 1 {
		t.Fatalf("reconnect wrote %d sets, want 1", got)
	}
	if ds := d.converge([]*PeerInfo{a}); ds == nil {
		t.Fatal("converge skipped the tick")
	}
	if got := k.setCalls(); len(got) != 1 {
		t.Errorf("converge after reconnect wrote again: %v", got)
	}
}

// One plan removes first, then sets.
func TestPlanRemovalsRunBeforeSets(t *testing.T) {
	k := newFakeKernel().with("old-key", "", "10.99.0.9/32")
	d := newConvergeDaemon(t, k, &logSink{})
	var order []string
	d.setPeerFn = func(iface, key string, psk [32]byte, ep, allowed string) error {
		order = append(order, "set "+key)
		return k.set(iface, key, psk, ep, allowed)
	}
	d.removePeerFn = func(iface, key string) error {
		order = append(order, "remove "+key)
		return k.remove(iface, key)
	}
	d.converge([]*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"), testPeer("keyB", "10.99.0.3", "203.0.113.3:51820")})
	if want := []string{"remove old-key", "set keyA", "set keyB"}; !reflect.DeepEqual(order, want) {
		t.Errorf("apply order = %v, want %v", order, want)
	}
}

// TestReconnectOnlyTouchesPeerWithOwnEntry: a forced reconnect of a relayed
// peer must not set its mesh /32 on the peer, pulling it off its relay.
func TestReconnectOnlyTouchesPeerWithOwnEntry(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	d.config.ForceRelay = true
	relay := testPeer("intro-key", "10.250.0.1", "172.20.1.10:51820")
	relay.Introducer = true
	relayed := testPeer("relayed-key", "10.250.0.3", "172.20.2.20:51820")
	direct := testPeer("direct-key", "10.250.0.4", "172.20.3.30:51820")
	// Update records how a peer was discovered; a LAN peer stays direct.
	d.peerStore.Update(relay, "dht")
	d.peerStore.Update(relayed, "gossip")
	d.peerStore.Update(direct, LANMethod)

	d.attemptPeerReconnect(relayed)
	if got := k.setCalls(); len(got) != 0 {
		t.Fatalf("reconnect of a relayed peer wrote %v", got)
	}

	d.attemptPeerReconnect(direct)
	if got, want := k.setCalls(), []setCall{{"direct-key", "172.20.3.30:51820", "10.250.0.4/32"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("reconnect of a direct peer: sets = %v, want %v", got, want)
	}
}

// TestConvergeConcurrentWithHealthWrites: a health-path write waits for a
// tick's apply in flight and runs after it (run with -race: both goroutines
// touch the pushed endpoints). The tick's first write blocks on a channel, so
// the order is forced, not hoped for.
func TestConvergeConcurrentWithHealthWrites(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	var mu sync.Mutex
	var order []string
	note := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, s)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once, rel sync.Once
	releaseTick := func() { rel.Do(func() { close(release) }) }
	defer releaseTick()
	d.setPeerFn = func(_, key string, _ [32]byte, _, _ string) error {
		once.Do(func() {
			close(entered)
			<-release
		})
		note("set " + key)
		return nil
	}
	d.removePeerFn = func(_, key string) error { note("remove " + key); return nil }
	peers := []*PeerInfo{
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"),
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
	}

	tick := make(chan struct{})
	go func() {
		defer close(tick)
		d.reconcileMu.Lock()
		defer d.reconcileMu.Unlock()
		d.converge(peers)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("the tick never reached its first write")
	}

	health := make(chan struct{})
	go func() {
		defer close(health)
		_ = d.removePeerFromKernel("keyZ")
	}()
	select {
	case <-health:
		t.Fatal("removePeerFromKernel returned while a tick's write was in flight")
	case <-time.After(time.Second):
	}

	releaseTick()
	for name, done := range map[string]chan struct{}{"tick": tick, "health write": health} {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not finish after the release", name)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"set keyA", "set keyB", "remove keyZ"}; !reflect.DeepEqual(order, want) {
		t.Errorf("write order = %v, want %v (the health write after the whole apply)", order, want)
	}
}

// pushedOf is a copy of the endpoints recorded as pushed.
func pushedOf(d *Daemon) map[string]string {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	out := make(map[string]string)
	for k, v := range d.pushedLocked() {
		out[k] = v
	}
	return out
}

// reconnectPeerInKernel pushes what the desired state says, not what the
// caller's PeerInfo says, and skips a peer planKernel would not configure.
func TestReconnectPeerGuards(t *testing.T) {
	const ep = "203.0.113.2:51820"
	stale := testPeer("keyA", "10.99.0.2", "198.51.100.1:1")

	t.Run("pushes the desired endpoint", func(t *testing.T) {
		k := newFakeKernel()
		d := newConvergeDaemon(t, k, &logSink{})
		if err := d.reconnectPeerInKernel(stale, []*PeerInfo{testPeer("keyA", "10.99.0.2", ep)}); err != nil {
			t.Fatal(err)
		}
		if got, want := k.setCalls(), []setCall{{"keyA", ep, "10.99.0.2/32"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("sets = %v, want %v", got, want)
		}
		if got := pushedOf(d); !reflect.DeepEqual(got, map[string]string{"keyA": ep}) {
			t.Errorf("pushed = %v, want the desired endpoint", got)
		}
	})

	skipped := map[string]struct {
		desired     *PeerInfo
		disableIPv6 bool
	}{
		"empty desired endpoint":       {testPeer("keyA", "10.99.0.2", ""), false},
		"IPv6 endpoint, IPv6 disabled": {testPeer("keyA", "10.99.0.2", "[2001:db8::1]:51820"), true},
	}
	for name, c := range skipped {
		t.Run(name, func(t *testing.T) {
			k := newFakeKernel()
			ls := &logSink{}
			d := newConvergeDaemon(t, k, ls)
			d.config.DisableIPv6 = c.disableIPv6
			if err := d.reconnectPeerInKernel(stale, []*PeerInfo{c.desired}); err != nil {
				t.Fatal(err)
			}
			if got := k.setCalls(); len(got) != 0 {
				t.Errorf("wrote %v for a peer that is not configurable", got)
			}
			if got := pushedOf(d); len(got) != 0 {
				t.Errorf("pushed = %v, want nothing recorded", got)
			}
			if n := ls.total(); n != 1 {
				t.Errorf("logged %d lines, want 1: %v", n, ls.lines)
			}
		})
	}

	t.Run("IPv6 endpoint with IPv6 on is pushed", func(t *testing.T) {
		k := newFakeKernel()
		d := newConvergeDaemon(t, k, &logSink{})
		v6 := "[2001:db8::1]:51820"
		if err := d.reconnectPeerInKernel(stale, []*PeerInfo{testPeer("keyA", "10.99.0.2", v6)}); err != nil {
			t.Fatal(err)
		}
		if got := k.setCalls(); len(got) != 1 || got[0].endpoint != v6 {
			t.Errorf("sets = %v, want one with %s", got, v6)
		}
	})

	// desire never yields an entry without AllowedIPs, so the empty-set guard
	// is checked on the shared predicate.
	t.Run("empty allowed set", func(t *testing.T) {
		cfg := &desiredPeerConfig{peer: testPeer("keyA", "10.99.0.2", ep), allowed: map[string]struct{}{}}
		if cfg.configurable(false) {
			t.Error("a peer with no AllowedIPs is configurable")
		}
	})
}

// A reconnect is a kernel write and is counted like every other one.
func TestReconnectCountsKernelChange(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	p := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")
	before := testutil.ToFloat64(kernelChanges.WithLabelValues("peer"))
	if err := d.reconnectPeerInKernel(p, []*PeerInfo{p}); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(kernelChanges.WithLabelValues("peer")) - before; got != 1 {
		t.Errorf("wgmesh_kernel_changes_total rose by %v, want 1", got)
	}
}
