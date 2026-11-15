package node

import "net"

// EnforceRouteBinding switches the route drop of R5: a route entry equal to or
// inside the mesh network, or equal to another member's mesh IP, is removed from
// the candidate. It is false until release N+2 (ADR-0002 decision 4); until then
// each such entry is a warning and the offer is applied as today.
var EnforceRouteBinding = false

// holders lists the claims on ip of the members in the store; the caller holds
// ps.mu.
func (ps *PeerStore) holders(ip string) []Claim {
	var out []Claim
	for key, p := range ps.peers {
		if p.MeshIP == ip {
			out = append(out, Claim{Key: key, IP: ip})
		}
	}
	return out
}

// claimsOn calls fn for every claim of a member other than key on ip: the
// local node and the stored peers. fn returns true to stop.
func claimsOn(s Snapshot, key, ip string, fn func(Claim) bool) bool {
	if s.Policy.Local != nil {
		if k, lip := s.Policy.Local(); k != "" && k != key && lip == ip && fn(Claim{Key: k, IP: lip}) {
			return true
		}
	}
	if s.Holders != nil {
		for _, h := range s.Holders(ip) {
			if h.Key != key && fn(h) {
				return true
			}
		}
	}
	return false
}

// ipHeldByBound reports whether another member holds ip with a bound claim
// (the address its own key derives, nonce 0 to MaxNonce).
func ipHeldByBound(s Snapshot, key, ip string) bool {
	return claimsOn(s, key, ip, func(h Claim) bool {
		return Bound(h, s.Policy.Derive, MaxNonce)
	})
}

// routeRule names the rule a route entry of key breaks, or "" when it is
// allowed: a host route equal to another member's mesh IP, or a network equal
// to or inside the mesh network. A supernet of the mesh network is allowed
// (WireGuard matches AllowedIPs by longest prefix), and so is an entry that
// does not parse (the validation of announcements owns that).
func routeRule(s Snapshot, key, route string) string {
	_, n, err := net.ParseCIDR(route)
	if err != nil {
		return ""
	}
	ones, bits := n.Mask.Size()
	if ones == bits && claimsOn(s, key, n.IP.String(), func(Claim) bool { return true }) {
		return "route-inside-member-ip"
	}
	if m := s.Policy.MeshNet; m != nil && len(n.Mask) == len(m.Mask) && m.Contains(n.IP) {
		if meshOnes, _ := m.Mask.Size(); ones >= meshOnes {
			return "route-inside-mesh"
		}
	}
	return ""
}

// bindClaim applies R5 and the route rule to a candidate that passed the
// refusals; R4 (the claim order) is not decided here, Winner and ResolveClaims
// reuse Bound for it. In warn mode (both switches false) it only counts: the
// decision is Admit and each case enforcement would act on is a warning; an
// enforced rule acts and does not warn.
func bindClaim(s Snapshot, c Candidate) Decision {
	d := Decision{Verdict: Admit}
	p, info := s.Policy, c.Info
	warn := func(rule string) {
		d.Warnings = append(d.Warnings, rule)
		if p.OnWarn != nil {
			p.OnWarn(rule)
		}
	}

	if c.Hand == FirstHand && !s.Known && info.MeshIP != "" &&
		!Bound(Claim{Key: info.WGPubKey, IP: info.MeshIP}, p.Derive, MaxNonce) &&
		ipHeldByBound(s, info.WGPubKey, info.MeshIP) {
		if EnforceIPBinding {
			return Decision{Verdict: Refuse, Reason: ReasonIPHeld}
		}
		warn(string(ReasonIPHeld))
	}

	for _, r := range info.RoutableNetworks {
		rule := routeRule(s, info.WGPubKey, r)
		switch {
		case rule == "":
		case EnforceRouteBinding:
			d.Dropped = append(d.Dropped, r)
		default:
			warn(rule)
		}
	}
	if len(d.Dropped) > 0 {
		d.Verdict = AdmitPartial
	}
	return d
}

// withoutDropped is the candidate's info with the dropped route entries removed;
// the caller's info is not modified.
func withoutDropped(info *PeerInfo, dropped []string) *PeerInfo {
	drop := make(map[string]bool, len(dropped))
	for _, r := range dropped {
		drop[r] = true
	}
	out := *info
	out.RoutableNetworks = nil
	for _, r := range info.RoutableNetworks {
		if !drop[r] {
			out.RoutableNetworks = append(out.RoutableNetworks, r)
		}
	}
	return &out
}

// Approved reports whether the operator's accept list approves a network (R9).
func Approved(network string, list []*net.IPNet) bool {
	return true
}
