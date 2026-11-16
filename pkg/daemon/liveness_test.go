package daemon

import (
	"bufio"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// enforceLiveness turns the process-wide switch on for one test and restores
// it off. Tests that call it do not call t.Parallel: parallel tests resume
// only after every serial test has finished.
func enforceLiveness(t *testing.T) {
	t.Helper()
	node.EnforceLiveness.Store(true)
	t.Cleanup(func() { node.EnforceLiveness.Store(false) })
}

// livenessDaemon is a daemon with a store, the relay state and the probe maps
// that planPeers, probePeer and probePeersOverMesh touch.
func livenessDaemon(t *testing.T) *Daemon {
	t.Helper()
	return &Daemon{
		config:                 &Config{InterfaceName: "wg-test", Keys: makeTestKeys(t)},
		localNode:              &LocalNode{WGPubKey: "local-pubkey", NATType: "cone"},
		peerStore:              NewPeerStore(),
		lastAppliedPeerConfigs: make(map[string]string),
		relayRoutes:            make(map[string]string),
		directStableCycles:     make(map[string]int),
		localSubnetsFn:         func() []*net.IPNet { return nil },
		temporaryOffline:       make(map[string]time.Time),
		probeSessions:          make(map[string]*peerProbeSession),
		probeFailures:          make(map[string]int),
	}
}

// seedLivenessPeer stores a direct peer whose control plane last spoke at lastSeen.
func seedLivenessPeer(d *Daemon, key, meshIP string, lastSeen time.Time) {
	d.peerStore.SetPeerDirectly(key, &PeerInfo{
		WGPubKey: key,
		MeshIP:   meshIP,
		Endpoint: "172.20.1.10:51820",
		LastSeen: lastSeen,
	})
}

func peerKeys(peers []*PeerInfo) map[string]bool {
	keys := make(map[string]bool, len(peers))
	for _, p := range peers {
		keys[p.WGPubKey] = true
	}
	return keys
}

func TestReconcileRecordsHandshakes(t *testing.T) {
	d := livenessDaemon(t)
	now := time.Now()
	seen := now.Add(-time.Minute)
	seedLivenessPeer(d, "peer-a", "10.250.0.2", seen)
	seedLivenessPeer(d, "peer-b", "10.250.0.3", seen)
	hsA := now.Add(-30 * time.Second).Unix()

	d.planPeers(map[string]int64{"peer-a": hsA, "peer-b": 0, "stranger": hsA}, now)

	a, _ := d.peerStore.Get("peer-a")
	if want := time.Unix(hsA, 0); !a.LastDataPlane.Equal(want) {
		t.Errorf("peer-a LastDataPlane = %v, want the handshake time %v", a.LastDataPlane, want)
	}
	if !a.LastSeen.Equal(seen) {
		t.Errorf("peer-a LastSeen = %v, want it untouched at %v", a.LastSeen, seen)
	}
	b, _ := d.peerStore.Get("peer-b")
	if !b.LastDataPlane.IsZero() {
		t.Errorf("peer-b has a zero handshake, LastDataPlane = %v, want zero", b.LastDataPlane)
	}
	if _, ok := d.peerStore.Get("stranger"); ok {
		t.Error("a handshake of an unknown key created a peer")
	}
}

func TestReconcileObservesBeforeGetActive(t *testing.T) {
	enforceLiveness(t)
	d := livenessDaemon(t)
	now := time.Now()
	old := now.Add(-20 * time.Minute)
	seedLivenessPeer(d, "peer-hs", "10.250.0.2", old)
	seedLivenessPeer(d, "peer-silent", "10.250.0.3", old)

	peers, desired, _, _ := d.planPeers(map[string]int64{"peer-hs": now.Add(-30 * time.Second).Unix()}, now)

	got := peerKeys(peers)
	if !got["peer-hs"] {
		t.Error("peer with a 30-second-old handshake and a 20-minute-old LastSeen is not in the peer list reconcile builds from: observed after GetActive")
	}
	if _, ok := desired["peer-hs"]; !ok {
		t.Error("peer with a fresh handshake is not in the desired set of the same pass")
	}
	if got["peer-silent"] {
		t.Error("peer with no evidence but a 20-minute-old LastSeen is in the peer list")
	}
	if _, ok := desired["peer-silent"]; ok {
		t.Error("peer with no fresh evidence is in the desired set")
	}
}

func TestHandshakeKeepsPeerActiveWithSilentControlPlane(t *testing.T) {
	enforceLiveness(t)
	d := livenessDaemon(t)
	now := time.Now()
	seedLivenessPeer(d, "peer-hs", "10.250.0.2", now.Add(-20*time.Minute))

	peers, desired, _, _ := d.planPeers(map[string]int64{"peer-hs": now.Add(-30 * time.Second).Unix()}, now)

	if !peerKeys(peers)["peer-hs"] {
		t.Error("peer is not in GetActive")
	}
	if _, ok := desired["peer-hs"]; !ok {
		t.Error("peer is not in the desired set")
	}
	if removed := d.peerStore.CleanupStale(); len(removed) != 0 {
		t.Errorf("CleanupStale removed %v, want nothing", removed)
	}
	if _, ok := d.peerStore.Get("peer-hs"); !ok {
		t.Error("peer is gone from the store")
	}
}

// pipeSession returns a probe session over an in-memory pipe whose server end
// answers each "ping" line with reply and counts the pings.
func pipeSession(t *testing.T, reply string) (*peerProbeSession, *atomic.Int32) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	var pings atomic.Int32
	go func() {
		r := bufio.NewReader(server)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if line == "ping\n" {
				pings.Add(1)
				if _, err := server.Write([]byte(reply + "\n")); err != nil {
					return
				}
			}
		}
	}()
	return &peerProbeSession{conn: client, reader: bufio.NewReaderSize(client, probeMaxLineBytes)}, &pings
}

func TestProbeSuccessRecordsDataPlane(t *testing.T) {
	tests := []struct {
		name   string
		reply  string
		wantOK bool
	}{
		{"pong records the time", "pong", true},
		{"another reply records nothing", "nope", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := livenessDaemon(t)
			seedLivenessPeer(d, "peer-probe", "10.250.0.2", time.Now().Add(-time.Minute))
			session, _ := pipeSession(t, tt.reply)
			d.probeSessions["peer-probe"] = session
			peer, _ := d.peerStore.Get("peer-probe")

			before := time.Now()
			if got := d.probePeer(peer); got != tt.wantOK {
				t.Fatalf("probePeer = %v, want %v", got, tt.wantOK)
			}

			after, _ := d.peerStore.Get("peer-probe")
			if tt.wantOK {
				if after.LastDataPlane.Before(before.Add(-time.Second)) || after.LastDataPlane.IsZero() {
					t.Errorf("LastDataPlane = %v after a pong, want about %v", after.LastDataPlane, before)
				}
			} else if !after.LastDataPlane.IsZero() {
				t.Errorf("LastDataPlane = %v after a failed probe, want zero", after.LastDataPlane)
			}
		})
	}
}

// disagreeCount reads one series of wgmesh_liveness_disagree_total.
func disagreeCount(site string) float64 {
	return testutil.ToFloat64(LivenessDisagreeCounter(site))
}

func TestRelayCandidateUsesAlive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		enforce       bool
		lastSeen      time.Time
		lastDataPlane time.Time
		wantRelayed   bool
		wantDisagree  float64
	}{
		{"off: old LastSeen and a fresh data-plane time stays legacy and is counted", false, now.Add(-10 * time.Minute), now.Add(-30 * time.Second), false, 1},
		{"on: old LastSeen and a fresh data-plane time is a candidate", true, now.Add(-10 * time.Minute), now.Add(-30 * time.Second), true, 0},
		{"on: data-plane time past 90 seconds is not a candidate", true, now.Add(-10 * time.Minute), now.Add(-2 * time.Minute), false, 0},
		{"on: data-plane time at 80 seconds is a candidate", true, now.Add(-10 * time.Minute), now.Add(-80 * time.Second), true, 0},
		{"on: data-plane time at 100 seconds is not a candidate", true, now.Add(-10 * time.Minute), now.Add(-100 * time.Second), false, 0},
		{"off: data-plane time past 90 seconds, both rules refuse, nothing counted", false, now.Add(-10 * time.Minute), now.Add(-2 * time.Minute), false, 0},
		{"on: old LastSeen and no data-plane time is not a candidate", true, now.Add(-10 * time.Minute), time.Time{}, false, 0},
		{"off: fresh LastSeen and no data-plane time is a candidate", false, now.Add(-30 * time.Second), time.Time{}, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enforce {
				enforceLiveness(t)
			}
			d := livenessDaemon(t)
			d.config.ForceRelay = true
			introducer := &PeerInfo{
				WGPubKey: "intro-pubkey", MeshIP: "10.250.0.1", Endpoint: "172.20.1.10:51820",
				Introducer: true, LastSeen: tt.lastSeen, LastDataPlane: tt.lastDataPlane,
			}
			target := &PeerInfo{
				WGPubKey: "nodeb-pubkey", MeshIP: "10.250.0.3", Endpoint: "172.20.2.20:51820",
				DiscoveredVia: []string{"gossip"}, LastSeen: now,
			}
			before := disagreeCount(SiteRelayCandidate)

			_, relayRoutes, _ := d.buildDesiredPeerConfigsWithHandshakes([]*PeerInfo{introducer, target}, nil)

			relay, relayed := relayRoutes["nodeb-pubkey"]
			if relayed != tt.wantRelayed || (relayed && relay != "intro-pubkey") {
				t.Errorf("relayRoutes[nodeb] = %q, %v; want relayed = %v via intro-pubkey", relay, relayed, tt.wantRelayed)
			}
			if got := disagreeCount(SiteRelayCandidate) - before; got != tt.wantDisagree {
				t.Errorf("relay-candidate disagreements = %v, want %v", got, tt.wantDisagree)
			}
		})
	}
}

func TestProbeLoopVisitsSuspectPeers(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(d *Daemon)
		wantPings int32
	}{
		{"still configured", func(d *Daemon) { d.lastAppliedPeerConfigs["peer-susp"] = "sig" }, 1},
		{"relay routed", func(d *Daemon) { d.relayRoutes["peer-susp"] = "relay-pubkey" }, 1},
		{"neither configured nor relay routed", func(d *Daemon) {}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := livenessDaemon(t)
			seedLivenessPeer(d, "peer-susp", "10.250.0.2", time.Now().Add(-7*time.Minute))
			tt.setup(d)
			session, pings := pipeSession(t, "pong")
			d.probeSessions["peer-susp"] = session

			d.probePeersOverMesh()

			if got := pings.Load(); got != tt.wantPings {
				t.Errorf("pings = %d, want %d", got, tt.wantPings)
			}
			peer, _ := d.peerStore.Get("peer-susp")
			alive := peer.Alive(time.Now()) == node.Alive
			if alive != (tt.wantPings == 1) {
				t.Errorf("Alive after the loop = %v, want %v", alive, tt.wantPings == 1)
			}
		})
	}
}

func TestProbeEnforceKeepsLegacyUntilEnforced(t *testing.T) {
	tests := []struct {
		name         string
		enforce      bool
		wantPings    int32
		wantDisagree float64
	}{
		{"off: 7 minutes of silence is enforced as before and counted", false, 1, 1},
		{"on: a 10-second-old data-plane time is not enforced", true, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enforce {
				enforceLiveness(t)
			}
			d := livenessDaemon(t)
			now := time.Now()
			seedLivenessPeer(d, "peer-probe-enforce", "10.250.0.2", now.Add(-7*time.Minute))
			d.peerStore.ObserveDataPlane("peer-probe-enforce", now.Add(-10*time.Second))
			session, pings := pipeSession(t, "pong")
			d.probeSessions["peer-probe-enforce"] = session
			before := disagreeCount(SiteProbeEnforce)

			d.probePeersOverMesh()

			if got := pings.Load(); got != tt.wantPings {
				t.Errorf("pings = %d, want %d", got, tt.wantPings)
			}
			if got := disagreeCount(SiteProbeEnforce) - before; got != tt.wantDisagree {
				t.Errorf("probe-enforce disagreements = %v, want %v", got, tt.wantDisagree)
			}
		})
	}
}

func TestLivenessDisagreeCounterCounts(t *testing.T) {
	d, err := NewDaemon(testConfig(t))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.peerStore.SetPeerDirectly("peer-x", &PeerInfo{WGPubKey: "peer-x", LastSeen: time.Now().Add(-7 * time.Minute)})
	d.peerStore.ObserveDataPlane("peer-x", time.Now().Add(-30*time.Second))

	counter := livenessDisagree.WithLabelValues("get-active")
	other := livenessDisagree.WithLabelValues("cleanup")
	before, beforeOther := testutil.ToFloat64(counter), testutil.ToFloat64(other)

	if got := d.peerStore.GetActive(); len(got) != 0 {
		t.Fatalf("the legacy test acts while the switch is off: GetActive = %v, want none", got)
	}

	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Errorf("wgmesh_liveness_disagree_total{site=get-active} rose by %v, want 1", got)
	}
	if got := testutil.ToFloat64(other) - beforeOther; got != 0 {
		t.Errorf("the cleanup series rose by %v on a GetActive call, want 0", got)
	}
}

func TestLivenessDisagreeSeriesExistAtZero(t *testing.T) {
	livenessDisagree.Reset()
	reg := prometheus.NewRegistry()
	registerMetrics(reg)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	seen := map[string]float64{}
	for _, mf := range mfs {
		if mf.GetName() != "wgmesh_liveness_disagree_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if len(m.GetLabel()) == 1 && m.GetLabel()[0].GetName() == "site" {
				seen[m.GetLabel()[0].GetValue()] = m.GetCounter().GetValue()
			}
		}
	}
	for _, site := range LivenessDisagreeSites {
		v, ok := seen[site]
		if !ok {
			t.Errorf("no wgmesh_liveness_disagree_total{site=%q} series before the first disagreement", site)
		} else if v != 0 {
			t.Errorf("site %q series = %v, want 0", site, v)
		}
	}
}

// TestObserveHandshakesIsRaceFree runs the evidence writer against readers of
// the store from two goroutines; it is meaningful under -race.
func TestObserveHandshakesIsRaceFree(t *testing.T) {
	d := livenessDaemon(t)
	now := time.Now()
	seedLivenessPeer(d, "peer-r", "10.250.0.2", now.Add(-time.Minute))
	hs := map[string]int64{"peer-r": now.Add(-10 * time.Second).Unix()}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				observeHandshakes(d.peerStore, hs, time.Now())
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				d.peerStore.GetActive()
				if p, ok := d.peerStore.Get("peer-r"); ok {
					p.Alive(time.Now())
				}
			}
		}()
	}
	wg.Wait()
}
