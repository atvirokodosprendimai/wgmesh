package daemon

import (
	"errors"
	"fmt"
	"go/ast"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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
		config:                 &Config{InterfaceName: "wgmesh-t3-none", Keys: makeTestKeys(t)},
		localNode:              &LocalNode{WGPubKey: "local-key", MeshIP: "10.99.0.1", NATType: "cone"},
		peerStore:              NewPeerStore(),
		lastAppliedPeerConfigs: make(map[string]string),
		relayRoutes:            make(map[string]string),
		directStableCycles:     make(map[string]int),
		temporaryOffline:       make(map[string]time.Time),
		localSubnetsFn:         func() []*net.IPNet { return nil },
		setPeerFn:              k.set,
		removePeerFn:           k.remove,
		dumpFn:                 k.dump,
		logFn:                  ls.printf,
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
	// B was set last and took 192.168.50.0/24 from A in the kernel; nothing
	// in the desired state changed.
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
	want := []setCall{{"keyA", "", "10.99.0.2/32,192.168.50.0/24"}}
	if got := k.setCalls(); !reflect.DeepEqual(got, want) {
		t.Errorf("sets = %v, want %v (drift-only set: no endpoint, only the drifted peer)", got, want)
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

// TestReconnectOnlyTouchesPeerWithOwnEntry: a forced reconnect of a relayed
// peer must not set its mesh /32 on the peer, pulling it off its relay.
func TestReconnectOnlyTouchesPeerWithOwnEntry(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	d.config.ForceRelay = true
	relay := testPeer("intro-key", "10.250.0.1", "172.20.1.10:51820")
	relay.Introducer = true
	relayed := testPeer("relayed-key", "10.250.0.3", "172.20.2.20:51820")
	relayed.DiscoveredVia = []string{"gossip"}
	direct := testPeer("direct-key", "10.250.0.4", "172.20.3.30:51820")
	direct.DiscoveredVia = []string{LANMethod}
	for _, p := range []*PeerInfo{relay, relayed, direct} {
		d.peerStore.Update(p, "test")
	}

	d.attemptPeerReconnect(relayed)
	if got := k.setCalls(); len(got) != 0 {
		t.Fatalf("reconnect of a relayed peer wrote %v", got)
	}

	d.attemptPeerReconnect(direct)
	if got, want := k.setCalls(), []setCall{{"direct-key", "172.20.3.30:51820", "10.250.0.4/32"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("reconnect of a direct peer: sets = %v, want %v", got, want)
	}
}

// TestConvergeConcurrentWithHealthWrites: a tick's apply and a health-path
// write never interleave, and the shared state has no race (run with -race).
func TestConvergeConcurrentWithHealthWrites(t *testing.T) {
	k := newFakeKernel()
	d := newConvergeDaemon(t, k, &logSink{})
	var inflight atomic.Int32
	var overlap atomic.Bool
	var writes atomic.Int32
	write := func() {
		if inflight.Add(1) != 1 {
			overlap.Store(true)
		}
		time.Sleep(200 * time.Microsecond)
		writes.Add(1)
		inflight.Add(-1)
	}
	d.setPeerFn = func(string, string, [32]byte, string, string) error { write(); return nil }
	d.removePeerFn = func(string, string) error { write(); return nil }
	peers := []*PeerInfo{
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820"),
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820"),
		testPeer("keyC", "10.99.0.4", "203.0.113.4:51820"),
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			d.reconcileMu.Lock()
			d.converge(peers)
			d.reconcileMu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = d.removePeerFromKernel("keyZ")
		}
	}()
	wg.Wait()

	if overlap.Load() {
		t.Error("two wg writes ran at the same time")
	}
	if writes.Load() < 40 {
		t.Errorf("only %d writes ran", writes.Load())
	}
}
