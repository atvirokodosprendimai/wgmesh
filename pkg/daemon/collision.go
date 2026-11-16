package daemon

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	node "github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// CollisionInfo represents a mesh IP collision between two peers
type CollisionInfo struct {
	MeshIP string
	Peer1  *node.PeerInfo
	Peer2  *node.PeerInfo
}

// applyInterfaceAddress is setInterfaceAddress behind an atomic pointer so the
// collision tests can record what the daemon would set while a collision pass
// of an earlier test may still run.
var applyInterfaceAddress atomic.Pointer[func(name, address string) error]

func init() {
	f := setInterfaceAddress
	applyInterfaceAddress.Store(&f)
}

// DetectCollisions lists the mesh IPs claimed by more than one member: the
// peers in the store and the local node (local). The order of the claims
// follows the store's policy.
func DetectCollisions(ps *node.PeerStore, local node.Claim) []CollisionInfo {
	derive, _ := policyFuncs(ps)

	var collisions []CollisionInfo
	for _, c := range node.ResolveClaims(local, ps.GetAll(), derive) {
		collisions = append(collisions, CollisionInfo{
			MeshIP: c.IP,
			Peer1:  &node.PeerInfo{WGPubKey: c.A.Key, MeshIP: c.A.IP},
			Peer2:  &node.PeerInfo{WGPubKey: c.B.Key, MeshIP: c.B.IP},
		})
	}
	return collisions
}

// policyFuncs returns the derive and warn functions of the store's policy,
// both nil when it has none.
func policyFuncs(ps *node.PeerStore) (derive func(pubKey string, nonce int) string, warn func(rule string)) {
	if p := ps.Policy(); p != nil {
		return p.Derive, p.OnWarn
	}
	return nil, nil
}

// DeterministicWinner returns the peer that wins a collision (lower pubkey wins)
func DeterministicWinner(peer1, peer2 *node.PeerInfo) (*node.PeerInfo, *node.PeerInfo) {
	return orderPeers(peer1, peer2, nil, nil)
}

// orderPeers returns the winner and the loser of a collision by node.Winner.
func orderPeers(peer1, peer2 *node.PeerInfo, derive func(pubKey string, nonce int) string, warn func(rule string)) (*node.PeerInfo, *node.PeerInfo) {
	w, _ := node.Winner(
		node.Claim{Key: peer1.WGPubKey, IP: peer1.MeshIP},
		node.Claim{Key: peer2.WGPubKey, IP: peer2.MeshIP},
		derive, warn)
	if w.Key == peer1.WGPubKey {
		return peer1, peer2
	}
	return peer2, peer1
}

// ResolveCollision returns the address the loser of a mesh IP collision re-derives with a nonce.
// If customSubnet is non-nil, uses subnet-aware derivation; otherwise uses legacy derivation.
func ResolveCollision(loser *node.PeerInfo, meshSubnet [2]byte, secret string, customSubnet *net.IPNet) string {

	if customSubnet != nil {
		ip, err := crypto.DeriveMeshIPInSubnetWithNonce(customSubnet, loser.WGPubKey, secret, 1)
		if err != nil {
			log.Printf("[Collision] CRITICAL: Failed to derive IP in custom subnet: %v", err)
			// Do NOT fall back to legacy derivation — that would put the IP in the wrong address space
			return ""
		}
		return ip
	}
	return DeriveMeshIPWithNonce(meshSubnet, loser.WGPubKey, secret, 1)
}

// DeriveMeshIPWithNonce derives a mesh IP with a collision avoidance nonce
func DeriveMeshIPWithNonce(meshSubnet [2]byte, wgPubKey, secret string, nonce int) string {
	input := fmt.Sprintf("%d:%s|%d:%s|nonce=%d", len(wgPubKey), wgPubKey, len(secret), secret, nonce)
	hash := sha256.Sum256([]byte(input))

	suffix := binary.BigEndian.Uint16(hash[:2])

	if suffix == 0 {
		suffix = 1
	} else if suffix == 65535 {
		suffix = 65534
	}

	return fmt.Sprintf("10.%d.%d.%d",
		meshSubnet[0],
		(suffix>>8)&0xFF,
		suffix&0xFF,
	)
}

// collisionMoveInterval is the least time between two moves of the local node
// after a collision: a node moves at most once in this window, whoever holds
// the claims on its address. The time of the last move is memory only: a
// restart starts a new window.
const collisionMoveInterval = 5 * time.Minute

// collisionTime is the clock of the move cap; a test sets d.collisionNow.
func (d *Daemon) collisionTime() time.Time {
	if d.collisionNow != nil {
		return d.collisionNow()
	}
	return time.Now()
}

// CheckAndResolveCollisions checks for collisions and resolves them. When the
// local node loses and moves to a nonce-derived address, the address is written
// to the local node state file, so it survives a restart (#830). The local node
// moves at most once per collisionMoveInterval; a move the cap holds back
// changes nothing and is logged once per window.
// The check (is the address still the colliding one), the move and the interface
// address are one check-then-set. Callers outside reconcile() must not run it
// at the same time as reconcile: reconcileMu serializes reconcile(), which is
// the only production caller (reconcileLoop and handleSIGHUP).
func (d *Daemon) CheckAndResolveCollisions() {
	d.collisionMu.Lock()
	defer d.collisionMu.Unlock()
	var local node.Claim
	if d.localNode != nil {
		key, ip := d.localNode.Identity()
		local = node.Claim{Key: key, IP: ip}
	}
	collisions := DetectCollisions(d.peerStore, local)
	if len(collisions) == 0 {
		d.collisionSeen = nil
		return
	}

	derive, warn := policyFuncs(d.peerStore)
	seen := make(map[string]struct{}, len(collisions))
	defer func() { d.collisionSeen = seen }()
	for _, collision := range collisions {
		// A collision that stands is logged and counted once, not on every tick.
		id := collision.MeshIP + "|" + collision.Peer1.WGPubKey + "|" + collision.Peer2.WGPubKey
		_, repeat := d.collisionSeen[id]
		seen[id] = struct{}{}
		pairWarn := warn
		if repeat {
			pairWarn = nil
		}
		winner, loser := orderPeers(collision.Peer1, collision.Peer2, derive, pairWarn)
		if !repeat {
			log.Printf("[Collision] Mesh IP collision detected: %s claimed by %s and %s",
				collision.MeshIP, safeKeyPrefix(winner.WGPubKey), safeKeyPrefix(loser.WGPubKey))
		}

		// If we are the loser, re-derive our IP
		if loser.WGPubKey == local.Key {
			if d.localNode.GetMeshIP() != collision.MeshIP {
				continue // already moved by an earlier collision of this pass
			}
			if !d.losesBoundFirst(local.Key, collision.MeshIP, derive) {
				if !repeat {
					log.Printf("[Collision] Keeping mesh IP %s: the other claim on it (%s) is not ahead of this node's claim, which its key derives; no move. If both members are meant to stay, one of them must use its own derived address or a new key", collision.MeshIP, safeKeyPrefix(winner.WGPubKey))
				}
				continue // the bound-first order decides: an unbound claim does not move a node on an address its key derives
			}
			now := d.collisionTime()
			if !d.collisionLastMove.IsZero() && now.Sub(d.collisionLastMove) < collisionMoveInterval {
				if !repeat {
					log.Printf("[Collision] Mesh IP move suppressed: this node moved %s ago and moves at most once per %s; it keeps %s until the window passes",
						now.Sub(d.collisionLastMove).Round(time.Second), collisionMoveInterval, collision.MeshIP)
				}
				continue
			}
			newIP := d.moverIP()
			if newIP == "" {
				log.Printf("[Collision] CRITICAL: Failed to derive IP in custom subnet — keeping current IP")
				continue
			}
			if newIP == d.localNode.GetMeshIP() {
				continue // every other address is claimed too: nothing to move to, nothing to write
			}
			log.Printf("[Collision] We lost collision, re-deriving mesh IP: %s -> %s", d.localNode.GetMeshIP(), newIP)
			d.localNode.SetMeshIP(newIP)
			d.collisionLastMove = now
			// Persist it so a restart and `service add` keep the address (#830);
			// a failed save keeps the in-memory address.
			if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
				log.Printf("[Collision] Warning: could not persist resolved mesh IP %s to %s; after a restart this node re-derives its natural IP: check write access to that directory: %v", newIP, d.stateFile, err)
			}

			// Reconfigure WireGuard with new IP using correct prefix length
			if err := (*applyInterfaceAddress.Load())(d.config.InterfaceName, fmt.Sprintf("%s/%d", newIP, d.config.PrefixLen())); err != nil {
				log.Printf("[Collision] Failed to update interface address: %v", err)
			}
		} else {
			// The loser is a remote peer - update our expectation of their IP
			newIP := ResolveCollision(loser, d.config.Keys.MeshSubnet, d.config.Secret, d.config.CustomSubnet)
			if newIP != "" {
				log.Printf("[Collision] Remote peer %s should re-derive to %s", safeKeyPrefix(loser.WGPubKey), newIP)
			}
		}
	}
}

// losesBoundFirst reports whether the local node, on ip, loses to another
// stored claim on ip under the bound-first order (node.BoundFirst): a bound
// claim beats an unbound one, between two of one class the lower key wins.
// The lower-key order of release N alone does not make the node move.
func (d *Daemon) losesBoundFirst(key, ip string, derive func(pubKey string, nonce int) string) bool {
	self := node.Claim{Key: key, IP: ip}
	for _, p := range d.peerStore.GetAll() {
		if p.WGPubKey == self.Key || p.MeshIP != ip {
			continue
		}
		if _, loser := node.BoundFirst(self, node.Claim{Key: p.WGPubKey, IP: p.MeshIP}, derive); loser == self {
			return true
		}
	}
	return false
}

// moverIP is the address the local node moves to after it lost a collision:
// the first nonce of 1 to node.MaxNonce whose address no other member claims
// (the local node's current address counts as claimed). When every one is
// claimed it is the nonce-1 address. It is "" when the custom subnet is unusable.
func (d *Daemon) moverIP() string {
	claimed := map[string]bool{d.localNode.GetMeshIP(): true}
	for _, p := range d.peerStore.GetAll() {
		if p.WGPubKey != d.localNode.WGPubKey {
			claimed[p.MeshIP] = true
		}
	}
	first := ""
	for nonce := 1; nonce <= node.MaxNonce; nonce++ {
		ip := d.deriveMeshIP(d.localNode.WGPubKey, nonce)
		if ip == "" {
			return ""
		}
		if !claimed[ip] {
			return ip
		}
		if nonce == 1 {
			first = ip
		}
	}
	log.Printf("[Collision] Every nonce address of %s is claimed, keeping the nonce-1 address %s", safeKeyPrefix(d.localNode.WGPubKey), first)
	return first
}

// safeKeyPrefix safely returns a prefix of a key for logging
func safeKeyPrefix(key string) string {
	if len(key) > 8 {
		return key[:8] + "..."
	}
	return key
}

// DeriveMeshIPWithCollisionCheck derives a mesh IP and checks for collisions.
// If customSubnet is non-nil, uses subnet-aware derivation.
func DeriveMeshIPWithCollisionCheck(meshSubnet [2]byte, wgPubKey, secret string, existingIPs map[string]string, customSubnet *net.IPNet) string {
	var ip string
	if customSubnet != nil {
		derived, err := crypto.DeriveMeshIPInSubnet(customSubnet, wgPubKey, secret)
		if err != nil {
			log.Printf("[Collision] CRITICAL: Failed to derive IP in custom subnet: %v", err)
			return ""
		}
		ip = derived
	} else {
		ip = crypto.DeriveMeshIP(meshSubnet, wgPubKey, secret)
	}

	// Check for collision
	for nonce := 1; nonce <= node.MaxNonce; nonce++ {
		if owner, exists := existingIPs[ip]; !exists || owner == wgPubKey {
			return ip
		}
		if customSubnet != nil {
			derived, err := crypto.DeriveMeshIPInSubnetWithNonce(customSubnet, wgPubKey, secret, nonce)
			if err != nil {
				log.Printf("[Collision] CRITICAL: Failed to derive IP with nonce in custom subnet: %v", err)
				return ""
			}
			ip = derived
		} else {
			ip = DeriveMeshIPWithNonce(meshSubnet, wgPubKey, secret, nonce)
		}
	}

	return ip
}
