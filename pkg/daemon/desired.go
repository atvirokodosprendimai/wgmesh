package daemon

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// forwardDesired is what a tick wants of ip_forward and the FORWARD rule.
// Needed: the node is an introducer or advertises a route. Wanted: the
// target is converged (always on Linux at release N, Needed once
// EnforceForwardScope is on, never elsewhere).
type forwardDesired struct {
	Wanted, Needed bool
}

// addressDesired is what a tick wants of the addresses on the mesh interface:
// Addrs, each with its prefix length, and the mesh networks (the masked
// prefixes of Addrs) inside which any other address is stale. Wanted: Linux
// and the node has a valid mesh IP.
type addressDesired struct {
	Wanted bool
	Addrs  []netip.Prefix
}

// desiredState is what one reconcile tick wants: the AllowedIPs of every
// peer and the kernel routes, both read from the same owner choice.
type desiredState struct {
	Peers        map[string]*desiredPeerConfig
	Routes       []routes.Entry
	Forward      forwardDesired
	Addresses    addressDesired
	relayRoutes  map[string]string
	directStable map[string]int
	// routeWarned is the set of rule cases this desire warned about and that
	// hold. desire only reads the previous set (d.routeWarned) and returns the
	// new one; converge alone stores it, under convergeMu.
	routeWarned map[string]struct{}
}

// desiredMember is one eligible peer, the peer that carries its traffic (the
// peer itself or its relay) and the gateway of its routes.
type desiredMember struct {
	peer, carrier *PeerInfo
	gateway       string // the carrier's mesh IP, the gateway of IPv4 routes
	gateway6      string // the carrier's mesh IPv6, the gateway of IPv6 routes; "" = none
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
// mesh IP. A network this node advertises itself is owned by nobody: that rule
// is applied through routes.Resolve (own networks passed in) and isOwnNetwork,
// both called only from desired.go.
// An IPv6 network is left out of both when IPv6 is disabled on this node. The
// function writes no Daemon field: the warn-once set it reads is
// d.routeWarned and the one it returns is ds.routeWarned, which converge stores.
// observedOwners is canonical network -> owner key read back from the kernel.
func (d *Daemon) desire(peers []*PeerInfo, handshakes map[string]int64, observedOwners map[string]string) *desiredState {
	ds := &desiredState{
		Peers:        make(map[string]*desiredPeerConfig),
		Forward:      d.desireForward(),
		relayRoutes:  make(map[string]string),
		directStable: make(map[string]int),
	}
	warned := make(map[string]struct{})
	enforceGuard := EnforceRouteGuard.Load()
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
		gateway6 := carrier.MeshIPv6
		if gateway6 == "" {
			gateway6 = p.MeshIPv6
		}
		members = append(members, desiredMember{peer: p, carrier: carrier, gateway: gateway, gateway6: gateway6})

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
			if d.config.DisableIPv6 && strings.Contains(network, ":") {
				if d.warnRoute(warned, "ipv6-disabled "+routes.Canonical(network, true)) {
					d.logf("Peer %s... advertises %s but IPv6 is disabled on this node (--no-ipv6), so the mesh interface has no IPv6: the network is left out of its AllowedIPs and of the routes", shortKey(p.WGPubKey), network)
				}
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
			gateway := m.gateway
			if strings.Contains(canonical, ":") {
				if m.gateway6 == "" {
					if d.warnRoute(warned, "no-ipv6 "+p.WGPubKey+" "+canonical) {
						d.logf("Peer %s... advertises %s but has no mesh IPv6, so the network has no gateway: it is left out of its AllowedIPs and of the routes", shortKey(p.WGPubKey), network)
					}
					continue
				}
				gateway = m.gateway6
			}
			if attached := d.attachedSubnet(canonical, localSubnets); attached != nil {
				first := d.warnRoute(warned, "attached "+canonical)
				if enforceGuard {
					if first {
						d.logf("attached-network: %s from peer %s... is equal to or inside %s, a network this host is attached to: it is left out of the routes and the AllowedIPs", network, shortKey(p.WGPubKey), attached)
					}
					continue
				}
				if first {
					d.logf("attached-network: %s from peer %s... is equal to or inside %s, a network this host is attached to: it is still installed, and installing it replaces the host's own connected route for that network; a later release leaves it out (EnforceRouteGuard)", network, shortKey(p.WGPubKey), attached)
					RecordKernelWarning("attached-network")
				}
			}
			d.addAllowedIP(ds.Peers, m.carrier, network)
			ds.Routes = append(ds.Routes, routes.Entry{Network: network, Gateway: canonicalGateway(gateway)})
		}
	}

	d.logSharedOwners(claimants, owners)
	ds.routeWarned = warned
	return ds
}

// warnRoute marks key in this tick's set of warned cases and reports whether
// it is new: not warned in the previous tick either. A case that stops
// holding leaves the set, so it warns again when it returns. The caller holds
// convergeMu.
func (d *Daemon) warnRoute(warned map[string]struct{}, key string) bool {
	warned[key] = struct{}{}
	_, was := d.routeWarned[key]
	return !was
}

// canonicalGateway is the gateway in the form the kernel prints it (IPv6
// compressed, in lower case), which is what the observed routes carry: a
// peer announces its mesh IPv6 in any form, and the diff compares strings. An
// unparsable gateway stays as it is.
func canonicalGateway(gateway string) string {
	if a, err := netip.ParseAddr(gateway); err == nil {
		return a.String()
	}
	return gateway
}

// prefixOf is the masked prefix of a local subnet.
func prefixOf(n *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := n.Mask.Size()
	return netip.PrefixFrom(addr.Unmap(), ones).Masked(), true
}

// attachedSubnet returns the subnet of this host that the canonical network is
// equal to or inside (a supernet is not), or nil. The mesh networks are not
// attached ones: the mesh interface's own addresses (the IPv4 mesh subnet and
// the IPv6 /64 setupWireGuard configures) are among the local subnets, and a
// route inside the mesh is the mesh's; nor is a link-local subnet, which
// belongs to the link. Linux only; macOS is T6.
func (d *Daemon) attachedSubnet(canonical string, localSubnets []*net.IPNet) *net.IPNet {
	if d.osName() != "linux" {
		return nil
	}
	prefix, err := netip.ParsePrefix(canonical)
	if err != nil {
		return nil
	}
	var mesh, mesh6 netip.Prefix
	meshKnown := false
	for _, s := range localSubnets {
		sp, ok := prefixOf(s)
		if !ok || !sp.Contains(prefix.Addr()) || prefix.Bits() < sp.Bits() || sp.Addr().IsLinkLocalUnicast() {
			continue
		}
		if !meshKnown {
			mesh, _ = prefixOf(d.meshNet())
			mesh6 = d.meshPrefixV6()
			meshKnown = true
		}
		if sp == mesh || sp == mesh6 {
			continue
		}
		return s
	}
	return nil
}

// meshPrefixV6 is the IPv6 /64 of the mesh interface (the node's mesh IPv6
// with the /64 setupWireGuard gives it), the zero Prefix when there is none.
func (d *Daemon) meshPrefixV6() netip.Prefix {
	if d.localNode == nil || d.localNode.MeshIPv6 == "" {
		return netip.Prefix{}
	}
	addr, err := netip.ParseAddr(d.localNode.MeshIPv6)
	if err != nil {
		return netip.Prefix{}
	}
	return netip.PrefixFrom(addr, 64).Masked()
}

// desireForward is what this node wants of forwarding. Needed is an
// introducer (immutable) or a node with an advertised route (read under the
// config lock: a reload writes it). At release N every Linux node is wanted;
// with EnforceForwardScope only a needed one is; elsewhere none is.
func (d *Daemon) desireForward() forwardDesired {
	needed := d.config.Introducer || len(d.GetAdvertiseRoutes()) > 0
	linux := d.osName() == "linux"
	return forwardDesired{
		Needed: needed,
		Wanted: linux && (needed || !EnforceForwardScope.Load()),
	}
}

// desireAddresses is what this node wants of the addresses of the mesh
// interface: its mesh IPv4 with the prefix length of the mesh, and its mesh
// IPv6 with /64 unless IPv6 is disabled, as setupWireGuard sets them. It reads
// the node's addresses once each: collision resolution, which writes them,
// runs on the same path (reconcile) after the tick. Not wanted off Linux, or
// without a valid mesh IPv4.
func (d *Daemon) desireAddresses() addressDesired {
	if d.osName() != "linux" || d.localNode == nil {
		return addressDesired{}
	}
	meshIP, meshIPv6 := d.localNode.MeshIP, d.localNode.MeshIPv6
	ip, err := netip.ParseAddr(meshIP)
	v4 := netip.PrefixFrom(ip, d.config.PrefixLen())
	if err != nil || !ip.Is4() || !v4.IsValid() {
		return addressDesired{}
	}
	want := addressDesired{Wanted: true, Addrs: []netip.Prefix{v4}}
	if meshIPv6 != "" && !d.config.DisableIPv6 {
		if ip6, err := netip.ParseAddr(meshIPv6); err == nil && ip6.Is6() {
			want.Addrs = append(want.Addrs, netip.PrefixFrom(ip6, 64))
		}
	}
	return want
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
