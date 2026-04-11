package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

// newTestDaemon creates a minimal Daemon for metrics testing.
func newTestDaemon() *Daemon {
	d := &Daemon{
		peerStore: NewPeerStore(),
		localNode: &LocalNode{NATType: "unknown"},
	}
	return d
}

func TestMetricsRegistered(t *testing.T) {
	// Use a local registry to avoid conflicts with the global registry.
	reg := prometheus.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_registered"})
	reg.Register(g)

	// Verify we can collect from the registry without panic.
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(mfs) == 0 {
		t.Error("expected at least one metric family")
	}
}

func TestActivePeersGauge(t *testing.T) {
	// Reset gauge for a clean test.
	activePeers.Set(0)

	d := newTestDaemon()

	// No peers — gauge should be 0.
	UpdateMetrics(d)
	val := testutil.ToFloat64(activePeers)
	if val != 0 {
		t.Errorf("expected 0 active peers, got %v", val)
	}

	// Add 3 peers.
	for i := 0; i < 3; i++ {
		d.peerStore.Update(&PeerInfo{
			WGPubKey: "pub" + strings.Repeat("x", 40) + string(rune('a'+i)),
			MeshIP:   "10.0.0." + string(rune('1'+i)),
			LastSeen: time.Now(),
		}, "test")
	}

	UpdateMetrics(d)
	val = testutil.ToFloat64(activePeers)
	if val != 3 {
		t.Errorf("expected 3 active peers, got %v", val)
	}
}

func TestRelayedPeersGauge(t *testing.T) {
	relayedPeers.Set(0)

	d := newTestDaemon()

	// No relay routes — gauge should be 0.
	UpdateMetrics(d)
	val := testutil.ToFloat64(relayedPeers)
	if val != 0 {
		t.Errorf("expected 0 relayed peers, got %v", val)
	}

	// Simulate 2 relay routes.
	d.relayMu.Lock()
	d.relayRoutes = map[string]string{
		"peerAAA": "relayBBB",
		"peerCCC": "relayBBB",
	}
	d.relayMu.Unlock()

	UpdateMetrics(d)
	val = testutil.ToFloat64(relayedPeers)
	if val != 2 {
		t.Errorf("expected 2 relayed peers, got %v", val)
	}
}

func TestReconcileDuration(t *testing.T) {
	// Record a duration and verify the histogram is non-empty.
	start := time.Now()
	time.Sleep(1 * time.Millisecond)
	ObserveReconcileDuration(start)

	// Gather the histogram metric and verify it has at least one observation.
	ch := make(chan prometheus.Metric, 1)
	reconcileDuration.Collect(ch)
	m := <-ch

	var metric dto.Metric
	if err := m.Write(&metric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	h := metric.GetHistogram()
	if h == nil {
		t.Fatal("expected histogram metric")
	}
	if h.GetSampleCount() == 0 {
		t.Error("expected at least one observation in reconcile histogram")
	}
	if h.GetSampleSum() <= 0 {
		t.Error("expected positive duration sum in reconcile histogram")
	}
}

func TestNATTypeGauge(t *testing.T) {
	// Reset all labels.
	for _, label := range []string{"cone", "symmetric", "unknown"} {
		natType.WithLabelValues(label).Set(0)
	}

	tests := []struct {
		natType  string
		expected string // label that should be 1
	}{
		{"cone", "cone"},
		{"symmetric", "symmetric"},
		{"unknown", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.natType, func(t *testing.T) {
			d := newTestDaemon()
			d.localNode.NATType = tt.natType

			UpdateMetrics(d)

			for _, label := range []string{"cone", "symmetric", "unknown"} {
				val := testutil.ToFloat64(natType.WithLabelValues(label))
				want := 0.0
				if label == tt.expected {
					want = 1.0
				}
				if val != want {
					t.Errorf("nat_type label %q = %v, want %v", label, val, want)
				}
			}
		})
	}
}

func TestUpdateMetricsNilDaemon(t *testing.T) {
	// Should not panic.
	UpdateMetrics(nil)
}

func TestRecordDiscoveryEvent(t *testing.T) {
	// Reset counters for known layers.
	for _, layer := range []string{"dht", "lan", "gossip", "registry"} {
		discoveryEvents.DeleteLabelValues(layer)
	}

	RecordDiscoveryEvent("dht")
	RecordDiscoveryEvent("dht")
	RecordDiscoveryEvent("lan")

	dhtVal := testutil.ToFloat64(discoveryEvents.WithLabelValues("dht"))
	if dhtVal != 2 {
		t.Errorf("expected 2 dht events, got %v", dhtVal)
	}

	lanVal := testutil.ToFloat64(discoveryEvents.WithLabelValues("lan"))
	if lanVal != 1 {
		t.Errorf("expected 1 lan event, got %v", lanVal)
	}
}

func TestProbeRTTHistogram(t *testing.T) {
	// Record a probe RTT and verify histogram has at least one observation.
	start := time.Now()
	time.Sleep(1 * time.Millisecond)
	ObserveProbeRTT("abcdefgh", start)

	ch := make(chan prometheus.Metric, 16)
	probeRTT.Collect(ch)
	close(ch)

	var found bool
	for m := range ch {
		var metric dto.Metric
		if err := m.Write(&metric); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		for _, lp := range metric.GetLabel() {
			if lp.GetName() == "peer_key" && lp.GetValue() == "abcdefgh" {
				h := metric.GetHistogram()
				if h == nil {
					t.Fatal("expected histogram metric")
				}
				if h.GetSampleCount() == 0 {
					t.Error("expected at least one RTT observation")
				}
				found = true
			}
		}
	}
	if !found {
		t.Error("expected histogram observation for peer_key=abcdefgh")
	}
}

func TestNATTraversalCounters(t *testing.T) {
	natTraversalAttempts.DeleteLabelValues("dht")
	natTraversalSuccesses.DeleteLabelValues("dht")

	RecordNATTraversalAttempt("dht")
	RecordNATTraversalAttempt("dht")
	RecordNATTraversalSuccess("dht")

	attempts := testutil.ToFloat64(natTraversalAttempts.WithLabelValues("dht"))
	if attempts != 2 {
		t.Errorf("expected 2 dht attempts, got %v", attempts)
	}
	successes := testutil.ToFloat64(natTraversalSuccesses.WithLabelValues("dht"))
	if successes != 1 {
		t.Errorf("expected 1 dht success, got %v", successes)
	}
}
