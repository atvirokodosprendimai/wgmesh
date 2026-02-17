package daemon

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics instruments for the daemon package.
// When no MeterProvider is configured (noop), all recording is zero-cost.
var (
	meter = otel.Meter("wgmesh.daemon")

	metricPeersActive     metric.Int64UpDownCounter
	metricReconcileDurMs  metric.Float64Histogram
	metricPeersDiscovered metric.Int64Counter
	metricHandshakeStale  metric.Int64Counter
	metricHealthEvictions metric.Int64Counter
	metricReconcileErrors metric.Int64Counter
)

func init() {
	var err error

	metricPeersActive, err = meter.Int64UpDownCounter("wgmesh.peers.active",
		metric.WithDescription("Number of active peers in the mesh"),
		metric.WithUnit("{peers}"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}

	metricReconcileDurMs, err = meter.Float64Histogram("wgmesh.reconcile.duration_ms",
		metric.WithDescription("Time spent in each reconcile cycle"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}

	metricPeersDiscovered, err = meter.Int64Counter("wgmesh.peers.discovered",
		metric.WithDescription("Total peers discovered"),
		metric.WithUnit("{peers}"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}

	metricHandshakeStale, err = meter.Int64Counter("wgmesh.handshake.stale",
		metric.WithDescription("Stale WireGuard handshake detections"),
		metric.WithUnit("{events}"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}

	metricHealthEvictions, err = meter.Int64Counter("wgmesh.health.evictions",
		metric.WithDescription("Peers evicted due to health check failures"),
		metric.WithUnit("{peers}"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}

	metricReconcileErrors, err = meter.Int64Counter("wgmesh.reconcile.errors",
		metric.WithDescription("Errors during reconcile cycles"),
		metric.WithUnit("{errors}"),
	)
	if err != nil {
		panic("otel meter: " + err.Error())
	}
}
