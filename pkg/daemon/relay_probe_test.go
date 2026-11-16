package daemon

import (
	"fmt"
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

// stubWireGuard swaps the three seams for a recorder (no wg needed) and puts
// them back when the test ends.
func stubWireGuard(t *testing.T) *wgRecorder {
	t.Helper()
	r := &wgRecorder{}
	oldSet, oldGet, oldRemove := setPeer, getPeers, wgRemovePeer
	setPeer = func(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.sets = append(r.sets, setPeerCall{key: pubKey, endpoint: endpoint, allowed: allowedIPs})
		return nil
	}
	getPeers = func(string) ([]wireguard.WGPeer, error) { return nil, nil }
	wgRemovePeer = func(iface, pubKey string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.removes = append(r.removes, pubKey)
		return nil
	}
	t.Cleanup(func() { setPeer, getPeers, wgRemovePeer = oldSet, oldGet, oldRemove })
	return r
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

// relayCycle runs the planning half of one reconcile pass over peers and
// stores the plan the way reconcile does.
func relayCycle(d *Daemon, peers []*PeerInfo, handshakes map[string]int64) (map[string]*desiredPeerConfig, map[string]string) {
	desired, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, handshakes)
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
	intro, peer := relayProbePeers("no-allowed-ips-peer")

	desired, _ := relayCycle(d, []*PeerInfo{intro, peer}, map[string]int64{})
	cfg, ok := desired[peer.WGPubKey]
	if !ok {
		t.Fatal("the relayed peer is not in the desired set")
	}
	if got := allowedCSVFor(cfg); got != "-" {
		t.Errorf("allowedCSVFor(probe-only) = %q, want %q", got, "-")
	}
	if got := allowedCSVFor(desired[probeIntroKey]); got == "-" || !strings.Contains(got, peer.MeshIP+"/32") {
		t.Errorf("allowedCSVFor(relay) = %q, want a list holding %s/32", got, peer.MeshIP)
	}

	if err := d.applyDesiredPeerConfigs(desired); err != nil {
		t.Fatalf("applyDesiredPeerConfigs: %v", err)
	}
	var sentPeer, sentRelay bool
	for _, c := range rec.setCalls() {
		switch c.key {
		case peer.WGPubKey:
			sentPeer = true
			if c.allowed != "-" {
				t.Errorf("the probe-only peer was sent allowed-ips %q, want %q", c.allowed, "-")
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
	intro, peer := relayProbePeers(key)
	cur := time.Unix(1_700_000_000, 0)
	d.clock = func() time.Time { return cur }
	handshakes := map[string]int64{}

	relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
	if got, ok := relayAges(t)[key]; !ok || got != 0 {
		t.Fatalf("age on entering the relay routes = %v (present %v), want 0", got, ok)
	}

	cur = cur.Add(40 * time.Second)
	relayCycle(d, []*PeerInfo{intro, peer}, handshakes)
	if got := relayAges(t)[key]; got != 40 {
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
	if got, ok := relayAges(t)[key]; ok {
		t.Errorf("the gauge still has a series for a peer that went direct: %v", got)
	}
}

func TestHealthCheckSkipsRelayRoutedAndProbeOnlyPeers(t *testing.T) {
	rec := stubWireGuard(t)
	d := relayProbeDaemon(t)
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
	_, peer := relayProbePeers("signature-peer")
	probe := map[string]*desiredPeerConfig{peer.WGPubKey: {peer: peer, allowed: map[string]struct{}{}, probeOnly: true}}
	direct := map[string]*desiredPeerConfig{peer.WGPubKey: {peer: peer, allowed: map[string]struct{}{peer.MeshIP + "/32": {}}}}

	steps := []struct {
		name        string
		desired     map[string]*desiredPeerConfig
		wantCalls   int
		wantAllowed string
	}{
		{"probe-only first", probe, 1, "-"},
		{"probe-only again", probe, 1, "-"},
		{"direct, same endpoint", direct, 2, peer.MeshIP + "/32"},
		{"direct again", direct, 2, peer.MeshIP + "/32"},
		{"probe-only after direct", probe, 3, "-"},
	}
	for _, s := range steps {
		if err := d.applyDesiredPeerConfigs(s.desired); err != nil {
			t.Fatalf("%s: applyDesiredPeerConfigs: %v", s.name, err)
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

func TestProbeOnlyStateIsRaceFree(t *testing.T) {
	stubWireGuard(t)
	d := relayProbeDaemon(t) // relayRouteSince starts nil
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
