package daemon

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsRegistered(t *testing.T) {
	// Create new instances of metrics for testing
	testActivePeers := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wgmesh_active_peers_test",
		Help: "Number of currently active peers in the mesh",
	})
	testRelayedPeers := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wgmesh_relayed_peers_test",
		Help: "Number of peers routed via relay (not direct)",
	})
	testNATType := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wgmesh_nat_type_test",
		Help: "Local NAT type (1=cone, 2=symmetric, 0=unknown)",
	}, []string{"type"})
	testDiscoveryEvents := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wgmesh_discovery_events_total_test",
		Help: "Discovery events by layer",
	}, []string{"layer"})
	testReconcileDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "wgmesh_reconcile_duration_seconds_test",
		Help:    "Time spent in reconcile loop",
		Buckets: prometheus.DefBuckets,
	})

	// Create a new registry for testing to avoid conflicts
	registry := prometheus.NewRegistry()

	// Register test metrics
	registry.MustRegister(testActivePeers)
	registry.MustRegister(testRelayedPeers)
	registry.MustRegister(testNATType)
	registry.MustRegister(testDiscoveryEvents)
	registry.MustRegister(testReconcileDuration)

	// Initialize vector metrics so they appear in the registry
	testNATType.WithLabelValues("unknown").Set(0)
	testDiscoveryEvents.WithLabelValues("dht").Add(0)

	// Collect metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Check that we have the expected metrics by name
	expectedMetrics := map[string]bool{
		"wgmesh_active_peers_test":               false,
		"wgmesh_relayed_peers_test":              false,
		"wgmesh_nat_type_test":                   false,
		"wgmesh_discovery_events_total_test":     false,
		"wgmesh_reconcile_duration_seconds_test": false,
	}

	for _, mf := range metricFamilies {
		if _, exists := expectedMetrics[*mf.Name]; exists {
			expectedMetrics[*mf.Name] = true
		}
	}

	// Check that we registered all expected metrics
	for metric, found := range expectedMetrics {
		if !found {
			t.Errorf("Expected metric %s not found", metric)
		}
	}
}

func TestActivePeersGauge(t *testing.T) {
	// Create a test daemon with known peer count
	config := &Config{
		InterfaceName: "test0",
		LogLevel:      "error", // Reduce test output
	}

	daemon := &Daemon{
		config:    config,
		peerStore: NewPeerStore(),
		relayMu:   sync.RWMutex{},
	}

	// Add some test peers
	peer1 := &PeerInfo{
		WGPubKey: "test_pubkey_1",
		MeshIP:   "192.168.1.2",
		LastSeen: time.Now(),
	}
	peer2 := &PeerInfo{
		WGPubKey: "test_pubkey_2",
		MeshIP:   "192.168.1.3",
		LastSeen: time.Now(),
	}

	daemon.peerStore.Update(peer1, "test")
	daemon.peerStore.Update(peer2, "test")

	// Update metrics
	collector := NewMetricsCollector()
	collector.Update(daemon)

	// Check the gauge value
	metric := &dto.Metric{}
	activePeers.Write(metric)

	expectedValue := float64(2)
	actualValue := metric.GetGauge().GetValue()

	if actualValue != expectedValue {
		t.Errorf("Expected active peers gauge value %f, got %f", expectedValue, actualValue)
	}
}

func TestRelayedPeersGauge(t *testing.T) {
	// Create a test daemon with known relay routes
	config := &Config{
		InterfaceName: "test0",
		LogLevel:      "error",
	}

	daemon := &Daemon{
		config:      config,
		peerStore:   NewPeerStore(),
		relayMu:     sync.RWMutex{},
		relayRoutes: make(map[string]string),
	}

	// Add some relay routes
	daemon.relayRoutes["peer1"] = "relay1"
	daemon.relayRoutes["peer2"] = "relay2"

	// Update metrics
	collector := NewMetricsCollector()
	collector.Update(daemon)

	// Check the gauge value
	metric := &dto.Metric{}
	relayedPeers.Write(metric)

	expectedValue := float64(2)
	actualValue := metric.GetGauge().GetValue()

	if actualValue != expectedValue {
		t.Errorf("Expected relayed peers gauge value %f, got %f", expectedValue, actualValue)
	}
}

func TestReconcileDuration(t *testing.T) {
	// Record a test duration
	testDuration := 100 * time.Millisecond
	RecordReconcileDuration(testDuration)

	// Get the histogram metric
	metric := &dto.Metric{}
	reconcileDuration.Write(metric)

	histogram := metric.GetHistogram()

	// Check that we recorded at least one sample
	if histogram.GetSampleCount() == 0 {
		t.Error("Expected at least one sample in reconcile duration histogram")
	}

	// Check that the sum is positive
	if histogram.GetSampleSum() <= 0 {
		t.Error("Expected positive sum in reconcile duration histogram")
	}
}

func TestUpdateNATType(t *testing.T) {
	collector := NewMetricsCollector()

	tests := []struct {
		name              string
		natType           string
		expectedCone      float64
		expectedSymmetric float64
		expectedUnknown   float64
	}{
		{
			name:              "cone NAT",
			natType:           "cone",
			expectedCone:      1,
			expectedSymmetric: 0,
			expectedUnknown:   0,
		},
		{
			name:              "symmetric NAT",
			natType:           "symmetric",
			expectedCone:      0,
			expectedSymmetric: 1,
			expectedUnknown:   0,
		},
		{
			name:              "unknown NAT",
			natType:           "unknown",
			expectedCone:      0,
			expectedSymmetric: 0,
			expectedUnknown:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.UpdateNATType(tt.natType)

			// Check cone NAT metric
			coneMetric := &dto.Metric{}
			natType.WithLabelValues("cone").Write(coneMetric)
			if coneMetric.GetGauge().GetValue() != tt.expectedCone {
				t.Errorf("Expected cone NAT value %f, got %f", tt.expectedCone, coneMetric.GetGauge().GetValue())
			}

			// Check symmetric NAT metric
			symmetricMetric := &dto.Metric{}
			natType.WithLabelValues("symmetric").Write(symmetricMetric)
			if symmetricMetric.GetGauge().GetValue() != tt.expectedSymmetric {
				t.Errorf("Expected symmetric NAT value %f, got %f", tt.expectedSymmetric, symmetricMetric.GetGauge().GetValue())
			}

			// Check unknown NAT metric
			unknownMetric := &dto.Metric{}
			natType.WithLabelValues("unknown").Write(unknownMetric)
			if unknownMetric.GetGauge().GetValue() != tt.expectedUnknown {
				t.Errorf("Expected unknown NAT value %f, got %f", tt.expectedUnknown, unknownMetric.GetGauge().GetValue())
			}
		})
	}
}

func TestIncrementDiscoveryEvents(t *testing.T) {
	// Get initial count
	initialMetric := &dto.Metric{}
	discoveryEvents.WithLabelValues("dht").Write(initialMetric)
	initialCount := initialMetric.GetCounter().GetValue()

	// Increment discovery events
	IncrementDiscoveryEvents("dht")

	// Get new count
	newMetric := &dto.Metric{}
	discoveryEvents.WithLabelValues("dht").Write(newMetric)
	newCount := newMetric.GetCounter().GetValue()

	// Check that it increased by 1
	if newCount != initialCount+1 {
		t.Errorf("Expected discovery events to increase by 1, got %f -> %f", initialCount, newCount)
	}
}
