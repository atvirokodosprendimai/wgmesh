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
	}, []string{"layer"})
	reconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "wgmesh_reconcile_duration_seconds",
		Help:    "Time spent in reconcile loop",
		Buckets: prometheus.DefBuckets,
	})
)

// metricsRegistered ensures RegisterMetrics is called at most once.
var metricsRegistered bool

// RegisterMetrics registers all wgmesh Prometheus metrics with the default
// registry. It is safe to call multiple times; subsequent calls are no-ops.
func RegisterMetrics() {
	if metricsRegistered {
		return
	}
	metricsRegistered = true
	prometheus.MustRegister(activePeers)
	prometheus.MustRegister(relayedPeers)
	prometheus.MustRegister(natType)
	prometheus.MustRegister(discoveryEvents)
	prometheus.MustRegister(reconcileDuration)
}

// UpdateMetrics reads state from the daemon and updates all metric gauges.
// This should be called at the end of each reconcile cycle.
func UpdateMetrics(d *Daemon) {
	if d == nil {
		return
	}

	// Active peers
	activePeers.Set(float64(d.peerStore.Count()))

	// Relayed peers
	relaySnapshot := d.currentRelayRoutesSnapshot()
	relayedPeers.Set(float64(len(relaySnapshot)))

	// NAT type gauge — set exactly one label to 1, the rest to 0.
	nt := "unknown"
	if d.localNode != nil && d.localNode.NATType != "" {
		nt = d.localNode.NATType
	}
	for _, label := range []string{"cone", "symmetric", "unknown"} {
		var v float64
		if label == nt {
			v = 1
		}
		natType.WithLabelValues(label).Set(v)
	}
}

// ObserveReconcileDuration records the duration of a reconcile cycle.
func ObserveReconcileDuration(start time.Time) {
	reconcileDuration.Observe(time.Since(start).Seconds())
}

// RecordDiscoveryEvent increments the discovery event counter for the given layer.
func RecordDiscoveryEvent(layer string) {
	discoveryEvents.WithLabelValues(layer).Inc()
}
