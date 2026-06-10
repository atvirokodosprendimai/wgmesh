package pilot

import (
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if m.DiscoveryLayerCounts == nil {
		t.Error("DiscoveryLayerCounts not initialized")
	}
	if m.NATTypes == nil {
		t.Error("NATTypes not initialized")
	}
}

func TestMetricsStart(t *testing.T) {
	m := NewMetrics()

	if m.IsStarted() {
		t.Error("metrics should not be started initially")
	}

	m.Start("test-pilot-id")

	if !m.IsStarted() {
		t.Error("metrics should be started after Start()")
	}

	if m.PilotID != "test-pilot-id" {
		t.Errorf("PilotID not set: got %s, want test-pilot-id", m.PilotID)
	}
}

func TestRecordPeerDiscovery(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordPeerDiscovery("dht")
	m.RecordPeerDiscovery("lan")
	m.RecordPeerDiscovery("dht")

	if m.DiscoveryLayerCounts["dht"] != 2 {
		t.Errorf("dht count: got %d, want 2", m.DiscoveryLayerCounts["dht"])
	}
	if m.DiscoveryLayerCounts["lan"] != 1 {
		t.Errorf("lan count: got %d, want 1", m.DiscoveryLayerCounts["lan"])
	}
	if m.DiscoveryLayerCounts["registry"] != 0 {
		t.Errorf("registry count: got %d, want 0", m.DiscoveryLayerCounts["registry"])
	}
}

func TestRecordPeerDiscoveryNotStarted(t *testing.T) {
	m := NewMetrics()

	// Recording before starting should be a no-op
	m.RecordPeerDiscovery("dht")

	if m.DiscoveryLayerCounts["dht"] != 0 {
		t.Error("recording should not work before start")
	}
}

func TestRecordMeshConnectivity(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordMeshConnectivity(99.5)

	if m.MeshUptimePercent != 99.5 {
		t.Errorf("mesh uptime: got %.2f, want 99.5", m.MeshUptimePercent)
	}
}

func TestRecordRoutePropagation(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	duration := 12 * time.Second
	m.RecordRoutePropagation(duration)

	if m.RoutePropagationTime != duration {
		t.Errorf("route propagation: got %v, want %v", m.RoutePropagationTime, duration)
	}
}

func TestRecordThroughput(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordThroughput(85.5)

	if m.ThroughputMbps != 85.5 {
		t.Errorf("throughput: got %.1f, want 85.5", m.ThroughputMbps)
	}
}

func TestRecordLatency(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordLatency(45.0, 5.0)

	if m.LatencyMs != 45.0 {
		t.Errorf("latency: got %.1f, want 45.0", m.LatencyMs)
	}
	if m.ConnectionOverheadMs != 5.0 {
		t.Errorf("overhead: got %.1f, want 5.0", m.ConnectionOverheadMs)
	}
}

func TestRecordDaemonRestart(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordDaemonRestart()
	m.RecordDaemonRestart()

	if m.DaemonRestarts != 2 {
		t.Errorf("daemon restarts: got %d, want 2", m.DaemonRestarts)
	}
}

func TestRecordWireGuardRestart(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordWireGuardRestart()

	if m.WireGuardRestarts != 1 {
		t.Errorf("wg restarts: got %d, want 1", m.WireGuardRestarts)
	}
}

func TestRecordNetworkPartition(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordNetworkPartition(30)

	if m.NetworkPartitions != 1 {
		t.Errorf("network partitions: got %d, want 1", m.NetworkPartitions)
	}
	if m.RecoveryTimeSec != 30 {
		t.Errorf("recovery time: got %d, want 30", m.RecoveryTimeSec)
	}
}

func TestRecordNATType(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordNATType("cone")
	m.RecordNATType("symmetric")
	m.RecordNATType("cone")

	if m.NATTypes["cone"] != 2 {
		t.Errorf("cone NAT count: got %d, want 2", m.NATTypes["cone"])
	}
	if m.NATTypes["symmetric"] != 1 {
		t.Errorf("symmetric NAT count: got %d, want 1", m.NATTypes["symmetric"])
	}
}

func TestRecordHolePunchSuccess(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	// Record successful punch
	m.RecordHolePunchSuccess(true)
	if m.HolePunchSuccess < 0.5 {
		t.Errorf("success rate after success: got %.2f, want >= 0.5", m.HolePunchSuccess)
	}

	// Record failed punch
	m.RecordHolePunchSuccess(false)
	if m.HolePunchSuccess >= 0.75 {
		t.Errorf("success rate after failure: got %.2f, want < 0.75", m.HolePunchSuccess)
	}
}

func TestRecordRelayFallback(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordRelayFallback()
	m.RecordRelayFallback()
	m.RecordRelayFallback()

	if m.RelayFallbackCount != 3 {
		t.Errorf("relay fallbacks: got %d, want 3", m.RelayFallbackCount)
	}
}

func TestSetPhase(t *testing.T) {
	m := NewMetrics()

	m.SetPhase("Mesh Stability")

	if m.Phase != "Mesh Stability" {
		t.Errorf("phase: got %s, want 'Mesh Stability'", m.Phase)
	}
}

func TestSetDaysElapsed(t *testing.T) {
	m := NewMetrics()

	m.SetDaysElapsed(5)

	if m.DaysElapsed != 5 {
		t.Errorf("days elapsed: got %d, want 5", m.DaysElapsed)
	}
}

func TestSnapshot(t *testing.T) {
	m := NewMetrics()
	m.Start("test-pilot")

	m.RecordPeerDiscovery("dht")
	m.RecordMeshConnectivity(99.9)
	m.RecordNATType("cone")

	snapshot := m.Snapshot()

	// Verify snapshot is a copy, not the same object
	if snapshot == m {
		t.Error("snapshot should be a different object")
	}

	// Verify values are copied
	if snapshot.PilotID != m.PilotID {
		t.Error("PilotID not copied")
	}
	if snapshot.MeshUptimePercent != m.MeshUptimePercent {
		t.Error("MeshUptimePercent not copied")
	}

	// Verify maps are deep copied
	snapshot.DiscoveryLayerCounts["lan"] = 999
	if m.DiscoveryLayerCounts["lan"] != 0 {
		t.Error("modifying snapshot should not affect original")
	}

	snapshot.NATTypes["symmetric"] = 999
	if m.NATTypes["symmetric"] != 0 {
		t.Error("modifying snapshot NAT types should not affect original")
	}
}

func TestAllMetricsNotStarted(t *testing.T) {
	m := NewMetrics()

	// All recording methods should be no-ops when not started
	m.RecordMeshConnectivity(99.9)
	m.RecordRoutePropagation(time.Second)
	m.RecordThroughput(80.0)
	m.RecordLatency(50.0, 10.0)
	m.RecordDaemonRestart()
	m.RecordWireGuardRestart()
	m.RecordNetworkPartition(30)
	m.RecordNATType("cone")
	m.RecordHolePunchSuccess(true)
	m.RecordRelayFallback()

	// All values should remain at zero/default
	if m.MeshUptimePercent != 0 {
		t.Error("values should not be recorded before start")
	}
	if m.DaemonRestarts != 0 {
		t.Error("values should not be recorded before start")
	}
}
