package daemon

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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
	probeRTT = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "wgmesh_probe_rtt_seconds",
		Help:    "Round-trip time of mesh health probes",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
	}, []string{"peer_key"})
	natTraversalAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wgmesh_nat_traversal_attempts_total",
		Help: "NAT traversal attempts by method",
	}, []string{"method"})
	natTraversalSuccesses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "wgmesh_nat_traversal_successes_total",
		Help: "Successful NAT traversal exchanges by method",
	}, []string{"method"})

	goCollector      = collectors.NewGoCollector()
	processCollector = collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
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
	prometheus.MustRegister(probeRTT)
	prometheus.MustRegister(natTraversalAttempts)
	prometheus.MustRegister(natTraversalSuccesses)
	prometheus.MustRegister(goCollector)
	prometheus.MustRegister(processCollector)
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

// ObserveProbeRTT records the round-trip time for a mesh probe to the given peer.
// peerKey should be the first 8 characters of the WireGuard public key.
func ObserveProbeRTT(peerKey string, start time.Time) {
	probeRTT.WithLabelValues(peerKey).Observe(time.Since(start).Seconds())
}

// RecordNATTraversalAttempt increments the attempt counter for the given method.
// method is the discovery method string, e.g. "dht", "dht-rendezvous", "dht-ipv6-sync".
func RecordNATTraversalAttempt(method string) {
	natTraversalAttempts.WithLabelValues(method).Inc()
}

// RecordNATTraversalSuccess increments the success counter for the given method.
func RecordNATTraversalSuccess(method string) {
	natTraversalSuccesses.WithLabelValues(method).Inc()
}
