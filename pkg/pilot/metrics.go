package pilot

import (
	"sync"
	"time"
)

// Metrics tracks evaluation-specific measurements
type Metrics struct {
	PilotID     string
	Phase       string
	DaysElapsed int

	// Connectivity metrics
	MeshUptimePercent    float64
	PeerDiscoverySuccess float64
	RoutePropagationTime time.Duration

	// Performance metrics
	ThroughputMbps       float64
	LatencyMs            float64
	ConnectionOverheadMs float64

	// Reliability metrics
	DaemonRestarts    int
	WireGuardRestarts int
	NetworkPartitions int
	RecoveryTimeSec   int

	// Discovery layer usage
	DiscoveryLayerCounts map[string]int

	// NAT traversal
	NATTypes           map[string]int
	HolePunchSuccess   float64
	RelayFallbackCount int

	mu      sync.RWMutex
	started bool
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		DiscoveryLayerCounts: make(map[string]int),
		NATTypes:             make(map[string]int),
	}
}

// Start initializes metrics collection for a pilot
func (m *Metrics) Start(pilotID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PilotID = pilotID
	m.started = true
}

// RecordPeerDiscovery records a successful peer discovery
func (m *Metrics) RecordPeerDiscovery(layer string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.DiscoveryLayerCounts[layer]++
}

// RecordMeshConnectivity updates the mesh uptime percentage
func (m *Metrics) RecordMeshConnectivity(percent float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.MeshUptimePercent = percent
}

// RecordRoutePropagation records a route propagation time
func (m *Metrics) RecordRoutePropagation(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.RoutePropagationTime = duration
}

// RecordThroughput records the throughput measurement
func (m *Metrics) RecordThroughput(mbps float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.ThroughputMbps = mbps
}

// RecordLatency records the latency measurement
func (m *Metrics) RecordLatency(latencyMs, overheadMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.LatencyMs = latencyMs
	m.ConnectionOverheadMs = overheadMs
}

// RecordDaemonRestart records a daemon restart
func (m *Metrics) RecordDaemonRestart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.DaemonRestarts++
}

// RecordWireGuardRestart records a WireGuard restart
func (m *Metrics) RecordWireGuardRestart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.WireGuardRestarts++
}

// RecordNetworkPartition records a network partition event
func (m *Metrics) RecordNetworkPartition(recoveryTimeSec int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.NetworkPartitions++
	m.RecoveryTimeSec = recoveryTimeSec
}

// RecordNATType records the detected NAT type
func (m *Metrics) RecordNATType(natType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.NATTypes[natType]++
}

// RecordHolePunchSuccess records a successful hole punch
func (m *Metrics) RecordHolePunchSuccess(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	// Update success rate as running average
	if success {
		m.HolePunchSuccess = (m.HolePunchSuccess + 1.0) / 2.0
	} else {
		m.HolePunchSuccess = m.HolePunchSuccess / 2.0
	}
}

// RecordRelayFallback records a relay fallback event
func (m *Metrics) RecordRelayFallback() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.RelayFallbackCount++
}

// Snapshot returns a copy of current metrics
func (m *Metrics) Snapshot() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &Metrics{
		PilotID:              m.PilotID,
		Phase:                m.Phase,
		DaysElapsed:          m.DaysElapsed,
		MeshUptimePercent:    m.MeshUptimePercent,
		PeerDiscoverySuccess: m.PeerDiscoverySuccess,
		RoutePropagationTime: m.RoutePropagationTime,
		ThroughputMbps:       m.ThroughputMbps,
		LatencyMs:            m.LatencyMs,
		ConnectionOverheadMs: m.ConnectionOverheadMs,
		DaemonRestarts:       m.DaemonRestarts,
		WireGuardRestarts:    m.WireGuardRestarts,
		NetworkPartitions:    m.NetworkPartitions,
		RecoveryTimeSec:      m.RecoveryTimeSec,
		HolePunchSuccess:     m.HolePunchSuccess,
		RelayFallbackCount:   m.RelayFallbackCount,
		DiscoveryLayerCounts: make(map[string]int),
		NATTypes:             make(map[string]int),
	}

	// Deep copy maps
	for k, v := range m.DiscoveryLayerCounts {
		snapshot.DiscoveryLayerCounts[k] = v
	}
	for k, v := range m.NATTypes {
		snapshot.NATTypes[k] = v
	}

	return snapshot
}

// SetPhase updates the current phase
func (m *Metrics) SetPhase(phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Phase = phase
}

// SetDaysElapsed updates the days elapsed
func (m *Metrics) SetDaysElapsed(days int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DaysElapsed = days
}

// IsStarted returns whether metrics collection has started
func (m *Metrics) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}
