package daemon

import "time"

// observeHandshakes records every non-zero WireGuard handshake time in the
// store as data-plane evidence. A handshake stamped after now is taken as now,
// so a skewed clock cannot make a peer fresher than the present.
func observeHandshakes(store *PeerStore, handshakes map[string]int64, now time.Time) {
	for key, ts := range handshakes {
		if ts <= 0 {
			continue
		}
		at := time.Unix(ts, 0)
		if at.After(now) {
			at = now
		}
		store.ObserveDataPlane(key, at)
	}
}

// planPeers is the peer half of reconcile: it records the handshakes as
// evidence, then takes the active peers, then builds the desired WireGuard
// configuration from them and the same handshakes. The order matters: a peer
// whose only evidence is a handshake must already be Alive when the active
// set is read.
func (d *Daemon) planPeers(handshakes map[string]int64, now time.Time) ([]*PeerInfo, map[string]*desiredPeerConfig, map[string]string, map[string]int) {
	peers := d.activePeers(handshakes, now)
	desired, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, handshakes)
	return peers, desired, relayRoutes, directStable
}

// activePeers records the handshakes as data-plane evidence and then reads the
// active peers, in that order: a peer whose only evidence is a handshake must
// already be Alive when the active set is read.
func (d *Daemon) activePeers(handshakes map[string]int64, now time.Time) []*PeerInfo {
	observeHandshakes(d.peerStore, handshakes, now)
	return d.peerStore.GetActive()
}

// DecideByLiveness is PeerInfo.DecideByAge that counts each disagreement in
// wgmesh_liveness_disagree_total{site}: one call, one count, however often the
// same peer is asked.
func DecideByLiveness(site string, p *PeerInfo, now time.Time, test func(age time.Duration) bool) bool {
	decision, disagree := p.DecideByAge(now, test)
	if disagree {
		recordLivenessDisagree(site)
	}
	return decision
}
