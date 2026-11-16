package daemon

import (
	"strings"
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
	setPeer      = wireguard.SetPeer
	getPeers     = wireguard.GetPeers
	wgRemovePeer = wireguard.RemovePeer
)

// allowedCSVFor is the allowed-ips argument of cfg: the sorted list, or the
// explicit clear for a probe-only peer, which carries no AllowedIPs.
func allowedCSVFor(cfg *desiredPeerConfig) string {
	if cfg.probeOnly {
		return wireguard.AllowedIPsClear
	}
	return strings.Join(mapKeysSorted(cfg.allowed), ",")
}

// applyMode names how cfg is applied. It is part of the signature of the last
// applied configuration, so a peer that switches between probe-only and direct
// is sent again.
func applyMode(cfg *desiredPeerConfig) string {
	if cfg.probeOnly {
		return "probe-only"
	}
	return "direct"
}

// storeRelayPlan stores the relay plan of one reconcile pass under relayMu:
// the relay routes, the hysteresis counters and the time each relayed peer
// entered the routes, and sets wgmesh_relay_route_age_seconds from it. A peer
// that left the routes loses its series. now is the daemon's wall-clock
// instant (d.now).
func (d *Daemon) storeRelayPlan(relayRoutes map[string]string, directStable map[string]int, now time.Time) {
	d.relayMu.Lock()
	defer d.relayMu.Unlock()
	since := make(map[string]time.Time, len(relayRoutes))
	for key := range relayRoutes {
		at, ok := d.relayRouteSince[key]
		if !ok {
			at = now
		}
		since[key] = at
		age := now.Sub(at).Seconds()
		if age < 0 {
			age = 0 // the clock stepped back
		}
		relayRouteAge.WithLabelValues(key).Set(age)
	}
	for key := range d.relayRouteSince {
		if _, ok := relayRoutes[key]; !ok {
			relayRouteAge.DeleteLabelValues(key)
		}
	}
	d.relayRoutes = relayRoutes
	d.directStableCycles = directStable
	d.relayRouteSince = since
}
