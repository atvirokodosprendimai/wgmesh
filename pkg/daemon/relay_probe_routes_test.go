package daemon

import "testing"

// A probe-only entry that later also carries a relayed peer's route (the peer is a
// relay for another peer) must send those routes, never the clear.
func TestAllowedCSVForProbeOnlyEntryKeepsRelayedRoutes(t *testing.T) {
	cfg := &desiredPeerConfig{allowed: map[string]struct{}{"10.0.0.9/32": {}}, probeOnly: true}
	if got := allowedCSVFor(cfg); got != "10.0.0.9/32" {
		t.Fatalf("allowedCSVFor = %q, want the relayed route 10.0.0.9/32", got)
	}
	empty := &desiredPeerConfig{allowed: map[string]struct{}{}, probeOnly: true}
	if got := allowedCSVFor(empty); got == "10.0.0.9/32" || got == "" {
		t.Fatalf("an empty probe-only entry sends %q, want the clear", got)
	}
}
