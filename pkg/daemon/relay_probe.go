package daemon

import (
	"sync/atomic"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// EnforceRelayProbe makes a relayed peer stay configured on the WireGuard
// interface with its endpoint and no AllowedIPs, so a direct handshake can
// bring it back (ADR-0004 Decision 6). False at N: the age of every relay
// route is exported and nothing else changes. It is read by the reconcile and
// health goroutines; tests call Store.
var EnforceRelayProbe atomic.Bool

// The WireGuard calls of the apply step and the health loop, as variables so a
// test can record them without wg. Production never writes them.
var (
	setPeer    = wireguard.SetPeer
	getPeers   = wireguard.GetPeers
	removePeer = wireguard.RemovePeer
)

// allowedCSVFor is the allowed-ips argument of cfg. STUB (T6 S1).
func allowedCSVFor(cfg *desiredPeerConfig) string {
	return ""
}

// isProbeOnlyPeer reports whether the peer is configured without AllowedIPs.
// STUB (T6 S1).
func (d *Daemon) isProbeOnlyPeer(pubKey string) bool {
	return false
}

// storeRelayPlan stores the relay plan of one reconcile pass. STUB (T6 S1):
// only the two maps reconcile always stored.
func (d *Daemon) storeRelayPlan(relayRoutes map[string]string, directStable map[string]int, desired map[string]*desiredPeerConfig, now time.Time) {
	d.relayMu.Lock()
	d.relayRoutes = relayRoutes
	d.directStableCycles = directStable
	d.relayMu.Unlock()
}
