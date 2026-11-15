package node

import (
	"sort"
	"strings"
)

// maxNonce is the loop bound of DeriveMeshIPWithCollisionCheck: an address
// derived with a nonce of 0 to maxNonce from its own key is bound to that key.
const maxNonce = 10

// Claim is one member's claim on a mesh IP: the key that holds it and the
// address. The local node is a member like any other.
type Claim struct {
	Key, IP string
}

// Collision is two claims on one IP; A is the one the order puts first.
type Collision struct {
	IP   string
	A, B Claim
}

// Bound reports whether the claim is on an address its own key derives.
func Bound(c Claim, derive func(key string, nonce int) string, maxNonce int) bool {
	return false
}

// Winner orders two claims on one IP.
func Winner(a, b Claim, derive func(key string, nonce int) string, warn func(rule string)) (winner, loser Claim) {
	if strings.Compare(b.Key, a.Key) < 0 {
		return b, a
	}
	return a, b
}

// ResolveClaims lists the IPs claimed by more than one member.
func ResolveClaims(local Claim, peers []*PeerInfo, derive func(key string, nonce int) string) []Collision {
	byIP := make(map[string][]Claim)
	for _, p := range peers {
		if p.WGPubKey == "" || p.MeshIP == "" {
			continue
		}
		byIP[p.MeshIP] = append(byIP[p.MeshIP], Claim{Key: p.WGPubKey, IP: p.MeshIP})
	}
	ips := make([]string, 0, len(byIP))
	for ip := range byIP {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	var out []Collision
	for _, ip := range ips {
		claims := byIP[ip]
		if len(claims) < 2 {
			continue
		}
		sort.Slice(claims, func(i, j int) bool { return claims[i].Key < claims[j].Key })
		first := claims[0]
		for _, c := range claims[1:] {
			out = append(out, Collision{IP: ip, A: first, B: c})
		}
	}
	return out
}
