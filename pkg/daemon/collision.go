package daemon

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// CollisionInfo represents a mesh IP collision between two peers
type CollisionInfo struct {
	MeshIP string
	Peer1  *PeerInfo
	Peer2  *PeerInfo
}

// DetectCollisions checks for mesh IP collisions in the peer store
func (ps *PeerStore) DetectCollisions() []CollisionInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ipMap := make(map[string]*PeerInfo)
	var collisions []CollisionInfo

	for _, peer := range ps.peers {
		if peer.MeshIP == "" {
			continue
		}

		if existing, ok := ipMap[peer.MeshIP]; ok {
			if existing.WGPubKey != peer.WGPubKey {
				collisions = append(collisions, CollisionInfo{
					MeshIP: peer.MeshIP,
					Peer1:  existing,
					Peer2:  peer,
				})
			}
		} else {
			ipMap[peer.MeshIP] = peer
		}
	}

	return collisions
}

// DeterministicWinner returns the peer that wins a collision (lower pubkey wins)
func DeterministicWinner(peer1, peer2 *PeerInfo) (*PeerInfo, *PeerInfo) {
	if strings.Compare(peer1.WGPubKey, peer2.WGPubKey) < 0 {
		return peer1, peer2
	}
	return peer2, peer1
}

// ResolveCollision resolves a mesh IP collision by re-deriving the loser's IP with a nonce
func ResolveCollision(collision CollisionInfo, meshSubnet [2]byte, secret string) string {
	_, loser := DeterministicWinner(collision.Peer1, collision.Peer2)

	// Re-derive mesh IP with nonce
	return DeriveMeshIPWithNonce(meshSubnet, loser.WGPubKey, secret, 1)
}

// DeriveMeshIPWithNonce derives a mesh IP with a collision avoidance nonce
func DeriveMeshIPWithNonce(meshSubnet [2]byte, wgPubKey, secret string, nonce int) string {
	input := fmt.Sprintf("%s%s|nonce=%d", wgPubKey, secret, nonce)
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

// CheckAndResolveCollisions checks for collisions and resolves them
func (d *Daemon) CheckAndResolveCollisions() {
	collisions := d.peerStore.DetectCollisions()
	if len(collisions) == 0 {
		return
	}

	for _, collision := range collisions {
		winner, loser := DeterministicWinner(collision.Peer1, collision.Peer2)
		log.Printf("[Collision] Mesh IP collision detected: %s claimed by %s and %s",
			collision.MeshIP, winner.WGPubKey[:8]+"...", loser.WGPubKey[:8]+"...")

		// If we are the loser, re-derive our IP
		if loser.WGPubKey == d.localNode.WGPubKey {
			newIP := DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret, 1)
			log.Printf("[Collision] We lost collision, re-deriving mesh IP: %s -> %s", d.localNode.MeshIP, newIP)
			d.localNode.MeshIP = newIP

			// Reconfigure WireGuard with new IP
			if err := setInterfaceAddress(d.config.InterfaceName, newIP+"/16"); err != nil {
				log.Printf("[Collision] Failed to update interface address: %v", err)
			}
		} else {
			// The loser is a remote peer - update our expectation of their IP
			newIP := ResolveCollision(collision, d.config.Keys.MeshSubnet, d.config.Secret)
			log.Printf("[Collision] Remote peer %s should re-derive to %s", loser.WGPubKey[:8]+"...", newIP)
		}
	}
}

// DeriveMeshIPWithCollisionCheck derives a mesh IP and checks for collisions
func DeriveMeshIPWithCollisionCheck(meshSubnet [2]byte, wgPubKey, secret string, existingIPs map[string]string) string {
	ip := crypto.DeriveMeshIP(meshSubnet, wgPubKey, secret)

	// Check for collision
	for nonce := 1; nonce <= 10; nonce++ {
		if owner, exists := existingIPs[ip]; !exists || owner == wgPubKey {
			return ip
		}
		ip = DeriveMeshIPWithNonce(meshSubnet, wgPubKey, secret, nonce)
	}

	return ip
}
