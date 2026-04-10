package daemon

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	activePeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wgmesh_active_peers",
		Help: "Number of currently active peers in the mesh",
	})
	relayedPeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wgmesh_relayed_peers",
		Help: "Number of peers routed via relay (not direct)",
	})
	natType = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wgmesh_nat_type",
		Help: "Local NAT type (1=cone, 2=symmetric, 0=unknown)",
	}, []string{"type"})
	discoveryEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wgmesh_discovery_events_total",
		Help: "Discovery events by layer",
	}, []string{"layer"}) // labels: "dht", "lan", "gossip", "registry"
	reconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "wgmesh_reconcile_duration_seconds",
		Help:    "Time spent in reconcile loop",
		Buckets: prometheus.DefBuckets,
	})
)

// MetricsCollector collects metrics from the daemon state
type MetricsCollector struct{}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// Update updates all metrics with current daemon state
func (m *MetricsCollector) Update(d *Daemon) {
	// Update active peers count
	peers := d.peerStore.GetActive()
	activePeers.Set(float64(len(peers)))

	// Update relayed peers count
	d.relayMu.RLock()
	relayedCount := len(d.relayRoutes)
	d.relayMu.RUnlock()
	relayedPeers.Set(float64(relayedCount))

	// NAT type will be updated by discovery layers when available
	// For now, we just ensure the metric exists
}

// UpdateNATType updates the NAT type metric
func (m *MetricsCollector) UpdateNATType(natTypeStr string) {
	// Reset all NAT type gauges
	natType.WithLabelValues("cone").Set(0)
	natType.WithLabelValues("symmetric").Set(0)
	natType.WithLabelValues("unknown").Set(0)

	// Set the current NAT type
	switch natTypeStr {
	case "cone":
		natType.WithLabelValues("cone").Set(1)
	case "symmetric":
		natType.WithLabelValues("symmetric").Set(1)
	default:
		natType.WithLabelValues("unknown").Set(1)
	}
}

// IncrementDiscoveryEvents increments the discovery events counter for a given layer
func IncrementDiscoveryEvents(layer string) {
	discoveryEvents.WithLabelValues(layer).Inc()
}

// RecordReconcileDuration records the duration of a reconcile operation
func RecordReconcileDuration(duration time.Duration) {
	reconcileDuration.Observe(duration.Seconds())
}

// RegisterMetrics registers all metrics with the default prometheus registry
func RegisterMetrics() {
	prometheus.MustRegister(activePeers)
	prometheus.MustRegister(relayedPeers)
	prometheus.MustRegister(natType)
	prometheus.MustRegister(discoveryEvents)
	prometheus.MustRegister(reconcileDuration)
}

// Global metrics collector instance
var defaultMetricsCollector = NewMetricsCollector()

// UpdateMetrics updates metrics using the default collector
func UpdateMetrics(d *Daemon) {
	defaultMetricsCollector.Update(d)
}
