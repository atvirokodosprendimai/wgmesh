package daemon

import "github.com/atvirokodosprendimai/wgmesh/pkg/routes"

// desiredState is what one reconcile tick wants: the AllowedIPs of every
// peer and the kernel routes, both read from the same owner choice.
type desiredState struct {
	Peers        map[string]*desiredPeerConfig
	Routes       []routes.Entry
	relayRoutes  map[string]string
	directStable map[string]int
}

// desire is a stub; the implementation follows.
func (d *Daemon) desire(peers []*PeerInfo, handshakes map[string]int64, observedOwners map[string]string) *desiredState {
	return &desiredState{
		Peers:        map[string]*desiredPeerConfig{},
		relayRoutes:  map[string]string{},
		directStable: map[string]int{},
	}
}
