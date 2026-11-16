package daemon

import "time"

// observeHandshakes is a stub: the evidence writer lands in S2.
func observeHandshakes(store *PeerStore, handshakes map[string]int64, now time.Time) {}

// planPeers is the peer half of reconcile: the active peers and the desired
// WireGuard configuration built from them and handshakes.
func (d *Daemon) planPeers(handshakes map[string]int64, now time.Time) ([]*PeerInfo, map[string]*desiredPeerConfig, map[string]string, map[string]int) {
	peers := d.peerStore.GetActive()
	desired, relayRoutes, directStable := d.buildDesiredPeerConfigsWithHandshakes(peers, handshakes)
	return peers, desired, relayRoutes, directStable
}
