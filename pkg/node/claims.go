package node

import (
	"sort"
	"strings"
)

// MaxNonce is the highest collision nonce a member derives with: a mover
// tries nonces 1 to MaxNonce (daemon.moverIP), and an address derived with a
// nonce of 0 to MaxNonce from its own key is bound to that key.
const MaxNonce = 10

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

// Bound reports whether the claim is on an address its own key derives with a
// nonce of 0 to maxNonce. A nil derive binds nothing.
func Bound(c Claim, derive func(key string, nonce int) string, maxNonce int) bool {
	if derive == nil || c.IP == "" {
		return false
	}
	for nonce := 0; nonce <= maxNonce; nonce++ {
		if derive(c.Key, nonce) == c.IP {
			return true
		}
	}
	return false
}

// BoundFirst orders two claims on one IP: a bound claim beats an unbound one,
// equal classes fall to the lower key. It does not read EnforceIPBinding.
func BoundFirst(a, b Claim, derive func(key string, nonce int) string) (winner, loser Claim) {
	lower, higher := a, b
	if strings.Compare(b.Key, a.Key) < 0 {
		lower, higher = b, a
	}
	if Bound(higher, derive, MaxNonce) && !Bound(lower, derive, MaxNonce) {
		return higher, lower
	}
	return lower, higher
}

// Winner orders two claims on one IP. When EnforceIPBinding is true that is
// BoundFirst. Otherwise the lower key wins, and each time the bound order would
// have chosen the other claim warn("winner-differs") is called.
func Winner(a, b Claim, derive func(key string, nonce int) string, warn func(rule string)) (winner, loser Claim) {
	if EnforceIPBinding.Load() {
		return BoundFirst(a, b, derive)
	}
	lower, higher := a, b
	if strings.Compare(b.Key, a.Key) < 0 {
		lower, higher = b, a
	}
	if boundWinner, _ := BoundFirst(a, b, derive); boundWinner != lower && warn != nil {
		warn("winner-differs")
	}
	return lower, higher
}

// ResolveClaims lists the IPs claimed by more than one member, the local node
// included. Every other claim on an IP is paired with the claim the order puts
// first; the list is sorted by IP, then by key, so every observer computes the same one.
func ResolveClaims(local Claim, peers []*PeerInfo, derive func(key string, nonce int) string) []Collision {
	byIP := make(map[string][]Claim)
	add := func(c Claim) {
		if c.Key != "" && c.IP != "" {
			byIP[c.IP] = append(byIP[c.IP], c)
		}
	}
	add(local)
	for _, p := range peers {
		if p.WGPubKey != local.Key {
			add(Claim{Key: p.WGPubKey, IP: p.MeshIP})
		}
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
		champion := claims[0]
		for _, c := range claims[1:] {
			champion, _ = Winner(champion, c, derive, nil)
		}
		for _, c := range claims {
			if c != champion {
				out = append(out, Collision{IP: ip, A: champion, B: c})
			}
		}
	}
	return out
}
