package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	MinSecretLength = 16
)

// DerivedKeys holds all keys and parameters derived from a shared secret
type DerivedKeys struct {
	NetworkID     [20]byte // DHT infohash (20 bytes for BEP 5)
	GossipKey     [32]byte // Symmetric encryption key for peer exchange
	MeshSubnet    [2]byte  // Deterministic /16 subnet
	MeshPrefixV6  [8]byte  // Deterministic ULA /64 prefix (fdxx:...)
	MulticastID   [4]byte  // Multicast group discriminator
	PSK           [32]byte // WireGuard PresharedKey
	GossipPort    uint16   // In-mesh gossip port
	RendezvousID  [8]byte  // For GitHub Issue search term
	MembershipKey [32]byte // For token generation/validation
	EpochSeed     [32]byte // For relay peer rotation
}

// DeriveKeys derives all cryptographic keys from a shared secret
func DeriveKeys(secret string) (*DerivedKeys, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("secret must be at least %d characters", MinSecretLength)
	}

	keys := &DerivedKeys{}

	// network_id = SHA256(secret)[0:20] → DHT infohash (20 bytes)
	hash := sha256.Sum256([]byte(secret))
	copy(keys.NetworkID[:], hash[:20])

	// gossip_key = HKDF(secret, info="wgmesh-gossip-v1", 32 bytes)
	if err := deriveHKDF(secret, "wgmesh-gossip-v1", keys.GossipKey[:]); err != nil {
		return nil, fmt.Errorf("failed to derive gossip key: %w", err)
	}

	// mesh_subnet = HKDF(secret, info="wgmesh-subnet-v1", 2 bytes)
	if err := deriveHKDF(secret, "wgmesh-subnet-v1", keys.MeshSubnet[:]); err != nil {
		return nil, fmt.Errorf("failed to derive mesh subnet: %w", err)
	}

	// mesh_prefix_v6 = fd + HKDF(secret, info="wgmesh-ipv6-prefix-v1", 7 bytes)
	var prefixTail [7]byte
	if err := deriveHKDF(secret, "wgmesh-ipv6-prefix-v1", prefixTail[:]); err != nil {
		return nil, fmt.Errorf("failed to derive mesh ipv6 prefix: %w", err)
	}
	keys.MeshPrefixV6[0] = 0xfd
	copy(keys.MeshPrefixV6[1:], prefixTail[:])

	// multicast_id = HKDF(secret, info="wgmesh-mcast-v1", 4 bytes)
	if err := deriveHKDF(secret, "wgmesh-mcast-v1", keys.MulticastID[:]); err != nil {
		return nil, fmt.Errorf("failed to derive multicast ID: %w", err)
	}

	// psk = HKDF(secret, info="wgmesh-wg-psk-v1", 32 bytes)
	if err := deriveHKDF(secret, "wgmesh-wg-psk-v1", keys.PSK[:]); err != nil {
		return nil, fmt.Errorf("failed to derive PSK: %w", err)
	}

	// gossip_port = 51821 + (uint16(HKDF(secret, "gossip-port")) % 1000)
	var portBytes [2]byte
	if err := deriveHKDF(secret, "wgmesh-gossip-port-v1", portBytes[:]); err != nil {
		return nil, fmt.Errorf("failed to derive gossip port: %w", err)
	}
	keys.GossipPort = 51821 + (binary.BigEndian.Uint16(portBytes[:]) % 1000)

	// rendezvous_id = SHA256(secret || "rv")[0:8]
	rvHash := sha256.Sum256([]byte(secret + "rv"))
	copy(keys.RendezvousID[:], rvHash[:8])

	// membership_key = HKDF(secret, info="wgmesh-membership-v1", 32 bytes)
	if err := deriveHKDF(secret, "wgmesh-membership-v1", keys.MembershipKey[:]); err != nil {
		return nil, fmt.Errorf("failed to derive membership key: %w", err)
	}

	// epoch_seed = HKDF(secret, info="wgmesh-epoch-v1", 32 bytes)
	if err := deriveHKDF(secret, "wgmesh-epoch-v1", keys.EpochSeed[:]); err != nil {
		return nil, fmt.Errorf("failed to derive epoch seed: %w", err)
	}

	return keys, nil
}

// DeriveNetworkIDWithTime derives a time-rotating network ID for DHT privacy
// This rotates hourly to prevent DHT surveillance
func DeriveNetworkIDWithTime(secret string, t time.Time) ([20]byte, error) {
	var networkID [20]byte

	// Include hour component: floor(unix_time / 3600)
	hourEpoch := t.Unix() / 3600
	input := fmt.Sprintf("%s||%d", secret, hourEpoch)

	hash := sha256.Sum256([]byte(input))
	copy(networkID[:], hash[:20])

	return networkID, nil
}

// GetCurrentAndPreviousNetworkIDs returns both current and previous hour's network IDs
// for smooth transition during hourly rotation
func GetCurrentAndPreviousNetworkIDs(secret string) (current, previous [20]byte, err error) {
	now := time.Now().UTC()

	current, err = DeriveNetworkIDWithTime(secret, now)
	if err != nil {
		return current, previous, err
	}

	previous, err = DeriveNetworkIDWithTime(secret, now.Add(-1*time.Hour))
	if err != nil {
		return current, previous, err
	}

	return current, previous, nil
}

// DeriveMeshIP derives a deterministic mesh IP from WG public key and secret.
// Format: 10.<meshSubnet[0]>.<meshSubnet[1] XOR high>.<low>
// Both subnet bytes are used. The last octet is clamped to [1,254] to avoid
// network (.0) and broadcast (.255) addresses.
func DeriveMeshIP(meshSubnet [2]byte, wgPubKey, secret string) string {
	input := wgPubKey + secret
	hash := sha256.Sum256([]byte(input))

	// Use first two bytes of hash for host part
	highByte := hash[0] ^ meshSubnet[1] // mix subnet[1] into third octet
	lowByte := hash[1]

	// Clamp last octet to [1, 254] — avoid .0 (network) and .255 (broadcast)
	if lowByte == 0 {
		lowByte = 1
	} else if lowByte == 255 {
		lowByte = 254
	}

	return fmt.Sprintf("10.%d.%d.%d",
		meshSubnet[0],
		highByte,
		lowByte,
	)
}

// DeriveMeshIPv6 derives a deterministic ULA IPv6 address from WG public key and secret.
// Prefix is a mesh-scoped /64, interface ID is a stable SLAAC-like value from pubkey+secret hash.
func DeriveMeshIPv6(meshPrefixV6 [8]byte, wgPubKey, secret string) string {
	input := wgPubKey + "|" + secret + "|ipv6"
	hash := sha256.Sum256([]byte(input))

	var iid [8]byte
	copy(iid[:], hash[:8])
	// SLAAC-like IID flags: locally administered unicast
	iid[0] = (iid[0] | 0x02) & 0xfe

	allZero := true
	for _, b := range iid {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		iid[7] = 1
	}

	ip := make(net.IP, net.IPv6len)
	copy(ip[:8], meshPrefixV6[:])
	copy(ip[8:], iid[:])

	return ip.String()
}

// deriveHKDF derives key material using HKDF-SHA256.
// The info parameter provides domain separation (e.g. "wgmesh-gossip-v1").
// Salt is nil (HKDF uses a zero-filled salt internally per RFC 5869).
func deriveHKDF(secret, info string, output []byte) error {
	reader := hkdf.New(sha256.New, []byte(secret), nil, []byte(info))
	_, err := io.ReadFull(reader, output)
	return err
}
