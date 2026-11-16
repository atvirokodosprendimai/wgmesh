package daemon

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// These tests read and write the process-wide EnforceRelayProbe and the
// setPeer/getPeers/wgRemovePeer seams, so none of them calls t.Parallel.

const probeIntroKey = "intro-pubkey"

// enforceRelayProbe turns the switch on for one test and restores it off.
func enforceRelayProbe(t *testing.T) {
	t.Helper()
	EnforceRelayProbe.Store(true)
	t.Cleanup(func() { EnforceRelayProbe.Store(false) })
}

type setPeerCall struct{ key, endpoint, allowed string }

// wgRecorder records the WireGuard calls the seams carry.
type wgRecorder struct {
	mu      sync.Mutex
	sets    []setPeerCall
	removes []string
	kernel  map[string][]string // peer key -> allowed-ips: the fake kernel the recorder serves as a dump
}

func (r *wgRecorder) setCalls() []setPeerCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]setPeerCall(nil), r.sets...)
}

func (r *wgRecorder) removeCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.removes...)
}

// stubWireGuard returns a recorder that models the WireGuard interface (no wg
// needed): wire(d) points the daemon's set, remove and dump seams at it, so
// the apply step reads back what it wrote.
func stubWireGuard(t *testing.T) *wgRecorder {
	t.Helper()
	return &wgRecorder{kernel: make(map[string][]string)}
}

func (r *wgRecorder) setPeer(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sets = append(r.sets, setPeerCall{key: pubKey, endpoint: endpoint, allowed: allowedIPs})
	switch allowedIPs {
	case wireguard.AllowedIPsClear:
		r.kernel[pubKey] = nil
	case "":
		if _, ok := r.kernel[pubKey]; !ok {
			r.kernel[pubKey] = nil
		}
	default:
		r.kernel[pubKey] = strings.Split(allowedIPs, ",")
	}
	return nil
}

func (r *wgRecorder) removePeer(iface, pubKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removes = append(r.removes, pubKey)
	delete(r.kernel, pubKey)
	return nil
}

func (r *wgRecorder) dump(string) (wireguard.DumpPeers, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(wireguard.DumpPeers, len(r.kernel))
	for key, allowed := range r.kernel {
		out[key] = wireguard.DumpPeer{PublicKey: key, AllowedIPs: append([]string(nil), allowed...)}
	}
	return out, nil
}

// wire points d's WireGuard seams at the recorder.
func (r *wgRecorder) wire(d *Daemon) {
	d.setPeerFn, d.removePeerFn, d.dumpFn = r.setPeer, r.removePeer, r.dump
}

// applyDesired applies a desired peer set the way a tick does: observe the
// kernel, plan, apply, under the converge lock.
func applyDesired(d *Daemon, desired map[string]*desiredPeerConfig) error {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	obs := d.observeKernel()
	if err := obs.Errs[targetPeers]; err != nil {
		return err
	}
	for _, r := range d.applyKernel(planKernel(desired, obs, d.pushedLocked(), d.config.DisableIPv6)) {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// relayProbeDaemon is livenessDaemon plus the health loop's maps.
func relayProbeDaemon(t *testing.T) *Daemon {
	t.Helper()
	d := livenessDaemon(t)
	d.peerHealthFailures = make(map[string]int)
	d.lastPeerTransferTotal = make(map[string]uint64)
	return d
}

// relayProbePeers returns an introducer and a non-introducer discovered only
// through a transitive mention, the peer of #852.
func relayProbePeers(relayedKey string) (intro, relayed *PeerInfo) {
	now := time.Now()
	intro = &PeerInfo{WGPubKey: probeIntroKey, MeshIP: "10.250.0.1", Endpoint: "172.20.1.10:51820", Introducer: true, LastSeen: now}
	relayed = &PeerInfo{WGPubKey: relayedKey, MeshIP: "10.250.0.3", Endpoint: "172.20.2.20:51820", DiscoveredVia: []string{"dht-transitive"}, LastSeen: now}
	return intro, relayed
}

// relayCycle runs the planning half of one reconcile pass over peers (through
// planPeers, as reconcile does) and stores the plan the way reconcile does.
func relayCycle(d *Daemon, peers []*PeerInfo, handshakes map[string]int64) (map[string]*desiredPeerConfig, map[string]string) {
	for _, p := range peers {
		d.peerStore.SetPeerDirectly(p.WGPubKey, p)
	}
	_, desired, relayRoutes, directStable := d.planPeers(handshakes, d.now())
	d.storeRelayPlan(relayRoutes, directStable, d.now())
	return desired, relayRoutes
}

// relayAges reads wgmesh_relay_route_age_seconds by peer.
func relayAges(t *testing.T) map[string]float64 {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(relayRouteAge)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	out := make(map[string]float64)
	for _, mf := range mfs {
		if mf.GetName() != "wgmesh_relay_route_age_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "peer" {
					out[lp.GetValue()] = m.GetGauge().GetValue()
				}
			}
		}
	}
	return out
}

func TestBuildDesiredKeepsRelayedPeerProbeOnly(t *testing.T) {
	enforceRelayProbe(t)
	d := relayProbeDaemon(t)
	intro, peer := relayProbePeers("keeps-probe-only-peer")

	desired, relayRoutes := relayCycle(d, []*PeerInfo{intro, peer}, map[string]int64{})

	if relayRoutes[peer.WGPubKey] != probeIntroKey {
		t.Fatalf("relayRoutes[%s] = %q, want %q", peer.WGPubKey, relayRoutes[peer.WGPubKey], probeIntroKey)
	}
	cfg, ok := desired[peer.WGPubKey]
	if !ok {
		t.Fatal("the relayed peer is not in the desired set with the switch on")
	}
	if !cfg.probeOnly {
		t.Error("the relayed peer's entry is not probeOnly")
	}
	if len(cfg.allowed) != 0 {
		t.Errorf("the probe-only entry has allowed IPs %v, want none", cfg.allowed)
	}
	introCfg, ok := desired[probeIntroKey]
	if !ok {
		t.Fatal("the introducer is not in the desired set")
	}
	if _, has := introCfg.allowed[peer.MeshIP+"/32"]; !has {
		t.Errorf("the relay entry lost the peer's mesh IP: %v", introCfg.allowed)
	}
	if introCfg.probeOnly {
		t.Error("the relay entry is probeOnly")
	}
}

func TestProbeOnlyPeerCarriesNoAllowedIPs(t *testing.T) {
	enforceRelayProbe(t)
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t)
	rec.wire(d)
	intro, peer := relayProbePeers("no-allowed-ips-peer")

	desired, _ := relayCycle(d, []*PeerInfo{intro, peer}, map[string]int64{})
	cfg, ok := desired[peer.WGPubKey]
	if !ok {
		t.Fatal("the relayed peer is not in the desired set")
	}
	if got := allowedCSVFor(cfg); got != wireguard.AllowedIPsClear {
		t.Errorf("allowedCSVFor(probe-only) = %q, want %q", got, wireguard.AllowedIPsClear)
	}
	if got := allowedCSVFor(desired[probeIntroKey]); got == wireguard.AllowedIPsClear || !strings.Contains(got, peer.MeshIP+"/32") {
		t.Errorf("allowedCSVFor(relay) = %q, want a list holding %s/32", got, peer.MeshIP)
	}

	if err := applyDesired(d, desired); err != nil {
		t.Fatalf("applyDesired: %v", err)
	}
	var sentPeer, sentRelay bool
	for _, c := range rec.setCalls() {
		switch c.key {
		case peer.WGPubKey:
			sentPeer = true
			if c.allowed != wireguard.AllowedIPsClear {
				t.Errorf("the probe-only peer was sent allowed-ips %q, want %q", c.allowed, wireguard.AllowedIPsClear)
			}
			if strings.Contains(c.allowed, peer.MeshIP) {
				t.Errorf("the probe-only peer was sent its mesh IP: %q", c.allowed)
			}
			if c.endpoint != peer.Endpoint {
				t.Errorf("the probe-only peer was sent endpoint %q, want %q", c.endpoint, peer.Endpoint)
			}
		case probeIntroKey:
			sentRelay = true
			if !strings.Contains(c.allowed, peer.MeshIP+"/32") {
				t.Errorf("the relay was sent %q, want it to hold %s/32", c.allowed, peer.MeshIP)
			}
		}
	}
	if !sentPeer || !sentRelay {
		t.Errorf("setPeer was not called for both (probe-only peer %v, relay %v)", sentPeer, sentRelay)
	}
}

func TestProbeOnlyHandshakeReturnsDirect(t *testing.T) {
	enforceRelayProbe(t)
	d := relayProbeDaemon(t)
	intro, peer := relayProbePeers("returns-direct-peer")
	d.relayRoutes[peer.WGPubKey] = probeIntroKey // relayed from the start
	handshakes := map[string]int64{peer.WGPubKey: time.Now().Add(-10 * time.Second).Unix()}

	for cycle := 1; cycle <= RelayHysteresisThreshold; cycle++ {
		desired, relayRoutes := relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
		cfg, inDesired := desired[peer.WGPubKey]
		if cycle < RelayHysteresisThreshold {
			if relayRoutes[peer.WGPubKey] != probeIntroKey {
				t.Fatalf("cycle %d: peer is not in the relay route, the hysteresis must hold it", cycle)
			}
			if !inDesired || !cfg.probeOnly {
				t.Fatalf("cycle %d: peer is not probe-only in the desired set (present %v)", cycle, inDesired)
			}
			continue
		}
		if _, still := relayRoutes[peer.WGPubKey]; still {
			t.Fatalf("cycle %d: peer is still relayed after %d fresh-handshake cycles", cycle, RelayHysteresisThreshold)
		}
		if !inDesired || cfg.probeOnly {
			t.Fatalf("cycle %d: peer is not a direct entry (present %v)", cycle, inDesired)
		}
		if _, has := cfg.allowed[peer.MeshIP+"/32"]; !has {
			t.Errorf("cycle %d: direct entry lacks the mesh IP: %v", cycle, cfg.allowed)
		}
	}
}

func TestNeverHandshakenStaysRelayed(t *testing.T) {
	enforceRelayProbe(t)
	d := relayProbeDaemon(t)
	intro, peer := relayProbePeers("never-handshaken-peer")

	for cycle := 1; cycle <= 100; cycle++ {
		handshakes := map[string]int64{}
		if cycle%2 == 0 {
			handshakes[peer.WGPubKey] = 0 // an entry with no handshake yet
		}
		desired, relayRoutes := relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
		if relayRoutes[peer.WGPubKey] != probeIntroKey {
			t.Fatalf("cycle %d: a peer that never handshook left the relay route", cycle)
		}
		if cfg, ok := desired[peer.WGPubKey]; !ok || !cfg.probeOnly {
			t.Fatalf("cycle %d: a peer that never handshook is not probe-only", cycle)
		}
	}
}

func TestRelayProbeOffByDefault(t *testing.T) {
	if EnforceRelayProbe.Load() {
		t.Fatal("EnforceRelayProbe is true by default")
	}
	d := relayProbeDaemon(t)
	intro, peer := relayProbePeers("off-by-default-peer")

	desired, relayRoutes := relayCycle(d, []*PeerInfo{intro, peer}, map[string]int64{})

	if relayRoutes[peer.WGPubKey] != probeIntroKey {
		t.Fatalf("relayRoutes[%s] = %q, want %q", peer.WGPubKey, relayRoutes[peer.WGPubKey], probeIntroKey)
	}
	if _, ok := desired[peer.WGPubKey]; ok {
		t.Error("the relayed peer is in the desired set with the switch off")
	}
	if cfg := desired[probeIntroKey]; cfg == nil || cfg.probeOnly {
		t.Errorf("the relay entry is missing or probeOnly: %+v", cfg)
	} else if _, has := cfg.allowed[peer.MeshIP+"/32"]; !has {
		t.Errorf("the relay entry lost the peer's mesh IP: %v", cfg.allowed)
	}
}

// A probe-only entry the apply step would skip for want of a usable endpoint
// would stay in desired and keep the peer from being removed, yet never be
// sent: the relayed peer then gets no entry, as at N.
func TestBuildDesiredNoProbeOnlyWithoutUsableEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		disableIPv6 bool
		wantEntry   bool
	}{
		{"empty endpoint", "", false, false},
		{"IPv6 endpoint with IPv6 disabled", "[2001:db8::2]:51820", true, false},
		{"IPv4 endpoint with IPv6 enabled", "172.20.2.20:51820", false, true},
		{"IPv4 endpoint with IPv6 disabled", "172.20.2.20:51820", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforceRelayProbe(t)
			d := relayProbeDaemon(t)
			d.config.DisableIPv6 = tt.disableIPv6
			intro, peer := relayProbePeers("usable-endpoint-peer")
			peer.Endpoint = tt.endpoint

			desired, relayRoutes := relayCycle(d, []*PeerInfo{intro, peer}, map[string]int64{})

			if relayRoutes[peer.WGPubKey] != probeIntroKey {
				t.Fatalf("relayRoutes[%s] = %q, want %q", peer.WGPubKey, relayRoutes[peer.WGPubKey], probeIntroKey)
			}
			if _, ok := desired[peer.WGPubKey]; ok != tt.wantEntry {
				t.Errorf("relayed peer in desired = %v, want %v", ok, tt.wantEntry)
			}
		})
	}
}

func TestRelayRouteAgeMetric(t *testing.T) {
	// The gauge is exported from N: the switch stays off.
	d := relayProbeDaemon(t)
	key := "age-metric-peer"
	label := key[:8] // the series carries the first 8 characters, as the probe metrics do
	intro, peer := relayProbePeers(key)
	cur := time.Unix(1_700_000_000, 0)
	d.clock = func() time.Time { return cur }
	handshakes := map[string]int64{}

	relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
	if got, ok := relayAges(t)[label]; !ok || got != 0 {
		t.Fatalf("age on entering the relay routes = %v (present %v), want 0", got, ok)
	}

	cur = cur.Add(40 * time.Second)
	relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
	if got := relayAges(t)[label]; got != 40 {
		t.Fatalf("age 40 s after entering = %v, want 40", got)
	}

	handshakes[key] = cur.Add(-10 * time.Second).Unix()
	for i := 0; i < RelayHysteresisThreshold; i++ {
		cur = cur.Add(5 * time.Second)
		relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
	}
	if d.isRelayRoutedPeer(key) {
		t.Fatal("the peer is still relayed after the hysteresis")
	}
	if got, ok := relayAges(t)[label]; ok {
		t.Errorf("the gauge still has a series for a peer that went direct: %v", got)
	}
}

func TestHealthCheckSkipsRelayRoutedAndProbeOnlyPeers(t *testing.T) {
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t)
	rec.wire(d)
	d.relayRoutes["relayed-peer"] = probeIntroKey
	keys := []string{"direct-peer", "relayed-peer"}
	for i, key := range keys {
		d.peerStore.SetPeerDirectly(key, &PeerInfo{
			WGPubKey: key,
			MeshIP:   fmt.Sprintf("10.250.0.%d", 10+i),
			Endpoint: fmt.Sprintf("172.20.2.%d:51820", 20+i),
			LastSeen: time.Now(),
		})
	}
	stale := time.Now().Add(-5 * time.Minute).Unix()
	handshakes := map[string]int64{"direct-peer": stale, "relayed-peer": stale}
	transfers := map[string]wireguard.PeerTransfer{}

	d.evaluatePeerHealth(handshakes, transfers)
	d.evaluatePeerHealth(handshakes, transfers)

	sets := rec.setCalls()
	if len(sets) != 1 || sets[0].key != "direct-peer" {
		t.Errorf("setPeer calls = %+v, want exactly one, for direct-peer", sets)
	}
	if removed := rec.removeCalls(); len(removed) != 1 || removed[0] != "direct-peer" {
		t.Errorf("wgRemovePeer calls = %v, want exactly direct-peer", removed)
	}
	for _, key := range []string{"relayed-peer"} {
		got, ok := d.peerStore.Get(key)
		if !ok {
			t.Fatalf("%s was evicted from the store", key)
		}
		d.attemptPeerReconnect(got)
	}
	if sets := rec.setCalls(); len(sets) != 1 {
		t.Errorf("attemptPeerReconnect reconnected a relay-routed or probe-only peer: %+v", sets)
	}
}

// A peer switched between direct and probe-only with the same endpoint is sent
// again, because the clear differs from every list in the signature; the same
// configuration twice is sent once.
func TestApplySignatureIncludesProbeOnlyMode(t *testing.T) {
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t)
	rec.wire(d)
	_, peer := relayProbePeers("signature-peer")
	probe := map[string]*desiredPeerConfig{peer.WGPubKey: {peer: peer, allowed: map[string]struct{}{}, probeOnly: true}}
	direct := map[string]*desiredPeerConfig{peer.WGPubKey: {peer: peer, allowed: map[string]struct{}{peer.MeshIP + "/32": {}}}}

	steps := []struct {
		name        string
		desired     map[string]*desiredPeerConfig
		wantCalls   int
		wantAllowed string
	}{
		{"probe-only first", probe, 1, wireguard.AllowedIPsClear},
		{"probe-only again", probe, 1, wireguard.AllowedIPsClear},
		{"direct, same endpoint", direct, 2, peer.MeshIP + "/32"},
		{"direct again", direct, 2, peer.MeshIP + "/32"},
		{"probe-only after direct", probe, 3, wireguard.AllowedIPsClear},
	}
	for _, s := range steps {
		if err := applyDesired(d, s.desired); err != nil {
			t.Fatalf("%s: applyDesired: %v", s.name, err)
		}
		sets := rec.setCalls()
		if len(sets) != s.wantCalls {
			t.Fatalf("%s: %d setPeer calls, want %d", s.name, len(sets), s.wantCalls)
		}
		if last := sets[len(sets)-1]; last.allowed != s.wantAllowed {
			t.Fatalf("%s: last call allowed %q, want %q", s.name, last.allowed, s.wantAllowed)
		}
	}
}

func TestEvictPeerFromPoolRemovesRelayAgeSeries(t *testing.T) {
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t)
	rec.wire(d)
	key := "evicted-age-peer"
	label := key[:8]
	t.Cleanup(func() { relayRouteAge.DeleteLabelValues(label) })
	d.storeRelayPlan(map[string]string{key: probeIntroKey}, map[string]int{}, d.now())
	if _, ok := relayAges(t)[label]; !ok {
		t.Fatal("the relayed peer has no age series before the eviction")
	}

	d.evictPeerFromPool(&PeerInfo{WGPubKey: key, MeshIP: "10.250.0.5", Endpoint: "172.20.2.22:51820"})

	if got, ok := relayAges(t)[label]; ok {
		t.Errorf("the age series of an evicted peer is still exported: %v", got)
	}
	if d.isRelayRoutedPeer(key) {
		t.Error("an evicted peer is still relay-routed")
	}
}

func TestRelayRouteAgeIsZeroWhenClockStepsBack(t *testing.T) {
	d := relayProbeDaemon(t)
	key := "clamped-age-peer"
	label := key[:8]
	t.Cleanup(func() { relayRouteAge.DeleteLabelValues(label) })
	t0 := time.Unix(1_700_000_000, 0)
	routes := map[string]string{key: probeIntroKey}
	d.storeRelayPlan(routes, map[string]int{}, t0)

	d.storeRelayPlan(routes, map[string]int{}, t0.Add(-time.Hour))

	if got, ok := relayAges(t)[label]; !ok || got != 0 {
		t.Errorf("age after the clock stepped back an hour = %v (present %v), want 0", got, ok)
	}
}

// callsIn returns the names of the selectors called inside the method name of
// file: the wiring tests read the source, not the behaviour.
func callsIn(t *testing.T, file, name string) map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	called := map[string]bool{}
	found := false
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != name || fn.Body == nil {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					called[fun.Sel.Name] = true
				case *ast.Ident:
					called[fun.Name] = true
				}
			}
			return true
		})
	}
	if !found {
		t.Fatalf("%s not found in %s: the walk sees nothing", name, file)
	}
	return called
}

// TestReconcileStoresRelayPlan pins the wiring: reconcile runs the converge
// tick, which records the handshakes as evidence, reads the active peers,
// builds the desired state once and hands the relay plan to storeRelayPlan,
// the only writer of the relay routes, before anything is applied, so the
// gauge and the health skip see what the apply step sees.
func TestReconcileStoresRelayPlan(t *testing.T) {
	for _, c := range []struct {
		file, fn string
		want     []string
	}{
		{"daemon.go", "reconcile", []string{"convergeTick"}},
		{"converge.go", "convergeTick", []string{"convergeWith"}},
		{"converge.go", "convergeWith", []string{"peersFn", "desire", "storeRelayPlan", "applyKernel"}},
		{"liveness.go", "activePeers", []string{"observeHandshakes", "GetActive"}},
	} {
		called := callsIn(t, c.file, c.fn)
		for _, name := range c.want {
			if !called[name] {
				t.Errorf("%s does not call %s", c.fn, name)
			}
		}
	}
}

// TestProbeOnlyStateIsRaceFree runs the writers and readers of the relay
// state from several goroutines. A dropped lock is detected only under -race,
// which the task's fence uses.
func TestProbeOnlyStateIsRaceFree(t *testing.T) {
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t) // relayRouteSince starts nil
	rec.wire(d)
	key, evictKey := "race-probe-peer", "race-evicted-peer"
	evicted := &PeerInfo{WGPubKey: evictKey, MeshIP: "10.250.0.4", Endpoint: "172.20.2.21:51820"}
	routes := func() map[string]string { return map[string]string{key: probeIntroKey, evictKey: probeIntroKey} }
	const rounds = 300

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			d.storeRelayPlan(routes(), map[string]int{}, time.Now())
			d.storeRelayPlan(map[string]string{}, map[string]int{}, time.Now())
		}
		d.storeRelayPlan(routes(), map[string]int{}, time.Now())
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			d.isRelayRoutedPeer(key)
			d.currentRelayRoutesSnapshot()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds/10; i++ {
			d.evictPeerFromPool(evicted) // deletes the route and age state of its own key
		}
	}()
	wg.Wait()

	if !d.isRelayRoutedPeer(key) {
		t.Error("after the last plan the peer is not relay-routed")
	}
}
