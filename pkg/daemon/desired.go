package daemon

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// desiredState is what one reconcile tick wants: the AllowedIPs of every
// peer and the kernel routes, both read from the same owner choice.
type desiredState struct {
	Peers        map[string]*desiredPeerConfig
	Routes       []routes.Entry
	relayRoutes  map[string]string
	directStable map[string]int
}

// desiredMember is one eligible peer, the peer that carries its traffic (the
// peer itself or its relay) and the gateway of its routes.
type desiredMember struct {
	peer, carrier *PeerInfo
	gateway       string
}

// desire computes the AllowedIPs of every peer and the kernel routes from one
// owner choice per shared network, so the two cannot disagree: a network is
// in the route table if and only if it is in exactly one peer's AllowedIPs.
//
// Relay selection and hysteresis are as before. A peer's mesh /32 and /128
// always stay; every other network goes to the owner picked by
// routes.Resolve (handshake newer than HandshakeStaleAfter, then
// observedOwners, then the lowest mesh IP). When the owner is relayed its
// networks go to the relay's AllowedIPs and the route gateway is the relay's
// mesh IP. A network this node advertises itself is owned by nobody: this is
// the only place that applies that rule to a peer's advertisement.
// observedOwners is canonical network -> owner key read back from the kernel.
func (d *Daemon) desire(peers []*PeerInfo, handshakes map[string]int64, observedOwners map[string]string) *desiredState {
	ds := &desiredState{
		Peers:        make(map[string]*desiredPeerConfig),
		relayRoutes:  make(map[string]string),
		directStable: make(map[string]int),
	}
	now := time.Now()
	localSubnets := d.getLocalSubnets()
	localKey := ""
	if d.localNode != nil {
		localKey = d.localNode.WGPubKey
	}

	relayCandidates := make([]*PeerInfo, 0)
	for _, p := range peers {
		if p.WGPubKey == localKey || p.WGPubKey == "" {
			continue
		}
		if d.isTemporarilyOffline(p.WGPubKey) {
			continue
		}
		if p.Introducer && p.Endpoint != "" && now.Sub(p.LastSeen) <= RelayCandidateMaxAge {
			relayCandidates = append(relayCandidates, p)
		}
	}

	prevRelayRoutes := d.currentRelayRoutesSnapshot()
	prevDirectStable := d.directStableCyclesSnapshot()
	ownNetworks := d.ownRoutableNetworks()

	var members []desiredMember
	var advertisers []routes.Advertiser
	for _, p := range peers {
		if p.WGPubKey == localKey || p.WGPubKey == "" || p.MeshIP == "" {
			continue
		}
		if d.isTemporarilyOffline(p.WGPubKey) {
			continue
		}

		shouldRelay := d.shouldRelayPeerWithSubnets(p, relayCandidates, handshakes, localSubnets)

		// Hysteresis: don't switch relay→direct without N consecutive stable cycles.
		// This prevents route flapping when NAT punching intermittently succeeds.
		_, wasRelayed := prevRelayRoutes[p.WGPubKey]
		if wasRelayed && !shouldRelay {
			n := prevDirectStable[p.WGPubKey] + 1
			if n < RelayHysteresisThreshold {
				// Hold relay route; direct path not yet stable enough to trust
				shouldRelay = true
				ds.directStable[p.WGPubKey] = n
			}
			// else: hysteresis satisfied — allow transition to direct
		}

		carrier := p
		if shouldRelay {
			if relay := d.selectRelayForPeer(p, relayCandidates); relay != nil {
				ds.relayRoutes[p.WGPubKey] = relay.WGPubKey
				carrier = relay
			}
		}
		gateway := carrier.MeshIP
		if gateway == "" {
			gateway = p.MeshIP
		}
		members = append(members, desiredMember{peer: p, carrier: carrier, gateway: gateway})

		ts := handshakes[p.WGPubKey]
		advertisers = append(advertisers, routes.Advertiser{
			Key:      p.WGPubKey,
			MeshIP:   p.MeshIP,
			MeshIPv6: p.MeshIPv6,
			Fresh:    ts > 0 && now.Sub(time.Unix(ts, 0)) < HandshakeStaleAfter,
			Networks: p.RoutableNetworks,
		})
	}

	owners := routes.Resolve(advertisers, ownNetworks, observedOwners)

	claimants := make(map[string][]desiredMember)
	for _, m := range members {
		p := m.peer
		d.addAllowedIP(ds.Peers, m.carrier, p.MeshIP+"/32")
		if p.MeshIPv6 != "" {
			d.addAllowedIP(ds.Peers, m.carrier, p.MeshIPv6+"/128")
		}
		seen := make(map[string]struct{})
		for _, network := range p.RoutableNetworks {
			network = strings.TrimSpace(network)
			if network == "" {
				continue
			}
			canonical := routes.Canonical(network, strings.Contains(network, ":"))
			owner, ok := owners[canonical]
			if !ok {
				continue // not advertised by a peer after the own-network rule
			}
			if _, dup := seen[canonical]; dup {
				continue
			}
			seen[canonical] = struct{}{}
			claimants[canonical] = append(claimants[canonical], m)
			if owner != p.WGPubKey {
				continue
			}
			d.addAllowedIP(ds.Peers, m.carrier, network)
			ds.Routes = append(ds.Routes, routes.Entry{Network: network, Gateway: m.gateway})
		}
	}

	d.logSharedOwners(claimants, owners)
	return ds
}

// isOwnNetwork reports whether this node advertises network itself.
func (d *Daemon) isOwnNetwork(network string) bool {
	return len(routes.ExcludeOwn([]string{network}, d.ownRoutableNetworks())) == 0
}

// logSharedOwners logs the gateway that lost a shared network once per
// change of owner or of contenders, not every tick.
func (d *Daemon) logSharedOwners(claimants map[string][]desiredMember, owners map[string]string) {
	networks := make([]string, 0, len(claimants))
	for network, c := range claimants {
		if len(c) > 1 {
			networks = append(networks, network)
		}
	}
	sort.Strings(networks)

	d.sharedLogMu.Lock()
	defer d.sharedLogMu.Unlock()
	if d.sharedLogged == nil {
		d.sharedLogged = make(map[string]string)
	}
	current := make(map[string]struct{}, len(networks))
	for _, network := range networks {
		owner := owners[network]
		var kept string
		var lost []string
		for _, m := range claimants[network] {
			if m.peer.WGPubKey == owner {
				kept = m.gateway
			} else {
				lost = append(lost, fmt.Sprintf("%s via %s", shortKey(m.peer.WGPubKey), m.gateway))
			}
		}
		sort.Strings(lost)
		msg := fmt.Sprintf("owner %s via %s, ignored %s", shortKey(owner), kept, strings.Join(lost, ", "))
		current[network] = struct{}{}
		if d.sharedLogged[network] != msg {
			d.sharedLogged[network] = msg
			log.Printf("Shared network %s: %s", network, msg)
		}
	}
	for network := range d.sharedLogged {
		if _, ok := current[network]; !ok {
			delete(d.sharedLogged, network)
		}
	}
}
