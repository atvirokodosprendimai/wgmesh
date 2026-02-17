package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	NonceSize                  = 12
	MaxMessageAge              = 10 * time.Minute
	ProtocolVersion            = "wgmesh-v1"
	MessageTypeHello           = "HELLO"
	MessageTypeReply           = "REPLY"
	MessageTypeAnnounce        = "ANNOUNCE"
	MessageTypeGoodbye         = "GOODBYE"
	MessageTypeRendezvousOffer = "RENDEZVOUS_OFFER"
	MessageTypeRendezvousStart = "RENDEZVOUS_START"
)

// MaxHostnameLength is the RFC 1035 maximum hostname length
const MaxHostnameLength = 253

// MaxRoutableNetworks is the maximum number of CIDR entries a peer can advertise
const MaxRoutableNetworks = 100

// MaxKnownPeers is the maximum number of transitive peers in a single announcement
const MaxKnownPeers = 1000

// PeerAnnouncement is the encrypted message format for peer discovery
type PeerAnnouncement struct {
	Protocol         string      `json:"protocol"`
	WGPubKey         string      `json:"wg_pubkey"`
	Hostname         string      `json:"hostname,omitempty"`
	MeshIP           string      `json:"mesh_ip"`
	MeshIPv6         string      `json:"mesh_ipv6,omitempty"`
	WGEndpoint       string      `json:"wg_endpoint"`
	Introducer       bool        `json:"introducer,omitempty"`
	RoutableNetworks []string    `json:"routable_networks,omitempty"`
	Timestamp        int64       `json:"timestamp"`
	KnownPeers       []KnownPeer `json:"known_peers,omitempty"`

	// ObservedEndpoint is the sender's public IP:port as seen by the
	// responder (peer-as-STUN reflector). Only populated in REPLY messages.
	// Recipients use this to learn their own NAT-mapped address.
	ObservedEndpoint string `json:"observed_endpoint,omitempty"`

	// NATType is the sender's detected NAT behavior: "cone", "symmetric",
	// or "unknown". Peers use this to decide whether relay is needed.
	NATType string `json:"nat_type,omitempty"`
}

// KnownPeer represents a peer that this node knows about (for transitive discovery)
type KnownPeer struct {
	WGPubKey   string `json:"wg_pubkey"`
	Hostname   string `json:"hostname,omitempty"`
	MeshIP     string `json:"mesh_ip"`
	MeshIPv6   string `json:"mesh_ipv6,omitempty"`
	WGEndpoint string `json:"wg_endpoint"`
	Introducer bool   `json:"introducer,omitempty"`
	NATType    string `json:"nat_type,omitempty"`
}

// Validate checks all fields of a KnownPeer for correctness.
func (kp *KnownPeer) Validate() error {
	if err := validateWGPubKey(kp.WGPubKey); err != nil {
		return fmt.Errorf("WGPubKey: %w", err)
	}
	if err := validateMeshIP(kp.MeshIP); err != nil {
		return fmt.Errorf("MeshIP: %w", err)
	}
	if kp.WGEndpoint != "" {
		if err := validateEndpoint(kp.WGEndpoint); err != nil {
			return fmt.Errorf("WGEndpoint: %w", err)
		}
	}
	if kp.Hostname != "" {
		if err := validateHostname(kp.Hostname); err != nil {
			return fmt.Errorf("Hostname: %w", err)
		}
	}
	return nil
}

// Validate checks all fields of a PeerAnnouncement for correctness.
// Called from OpenEnvelope after deserialization.
func (pa *PeerAnnouncement) Validate() error {
	if err := validateWGPubKey(pa.WGPubKey); err != nil {
		return fmt.Errorf("WGPubKey: %w", err)
	}
	if err := validateMeshIP(pa.MeshIP); err != nil {
		return fmt.Errorf("MeshIP: %w", err)
	}
	if pa.WGEndpoint != "" {
		if err := validateEndpoint(pa.WGEndpoint); err != nil {
			return fmt.Errorf("WGEndpoint: %w", err)
		}
	}
	if pa.Hostname != "" {
		if err := validateHostname(pa.Hostname); err != nil {
			return fmt.Errorf("Hostname: %w", err)
		}
	}
	if len(pa.RoutableNetworks) > MaxRoutableNetworks {
		return fmt.Errorf("RoutableNetworks: too many entries (%d, max %d)", len(pa.RoutableNetworks), MaxRoutableNetworks)
	}
	for i, cidr := range pa.RoutableNetworks {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("RoutableNetworks[%d]: invalid CIDR %q: %w", i, cidr, err)
		}
	}
	if len(pa.KnownPeers) > MaxKnownPeers {
		return fmt.Errorf("KnownPeers: too many entries (%d, max %d)", len(pa.KnownPeers), MaxKnownPeers)
	}
	for i, kp := range pa.KnownPeers {
		if err := kp.Validate(); err != nil {
			return fmt.Errorf("KnownPeers[%d]: %w", i, err)
		}
	}
	return nil
}

// validateWGPubKey checks that the key is valid base64 encoding of exactly 32 bytes.
func validateWGPubKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid base64: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("decoded length %d, want 32", len(decoded))
	}
	return nil
}

// validateMeshIP checks that the string is a valid IP address (not CIDR).
func validateMeshIP(ip string) error {
	if ip == "" {
		return fmt.Errorf("empty IP")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %q", ip)
	}
	return nil
}

// validateEndpoint checks that the string is a valid host:port with port in 1-65535.
func validateEndpoint(endpoint string) error {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid host:port: %w", err)
	}
	_ = host
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d out of range 1-65535", port)
	}
	return nil
}

// validateHostname checks RFC 1035 length and printable ASCII.
func validateHostname(hostname string) error {
	if len(hostname) > MaxHostnameLength {
		return fmt.Errorf("hostname too long: %d characters (max %d)", len(hostname), MaxHostnameLength)
	}
	for i, b := range []byte(hostname) {
		if b < 32 || b > 126 {
			return fmt.Errorf("hostname contains invalid character at position %d (byte 0x%02x)", i, b)
		}
	}
	return nil
}

// Envelope wraps encrypted messages with nonce for transmission
type Envelope struct {
	MessageType string `json:"type"`
	Nonce       []byte `json:"nonce"`
	Ciphertext  []byte `json:"ciphertext"`
}

// SealEnvelope encrypts a message using AES-256-GCM with the gossip key
func SealEnvelope(messageType string, payload interface{}, gossipKey [32]byte) ([]byte, error) {
	// Serialize payload to JSON
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(gossipKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Create envelope
	envelope := Envelope{
		MessageType: messageType,
		Nonce:       nonce,
		Ciphertext:  ciphertext,
	}

	// Serialize envelope
	return json.Marshal(envelope)
}

// OpenEnvelope decrypts a message using AES-256-GCM with the gossip key
func OpenEnvelope(data []byte, gossipKey [32]byte) (*Envelope, *PeerAnnouncement, error) {
	envelope, plaintext, err := OpenEnvelopeRaw(data, gossipKey)
	if err != nil {
		return nil, nil, err
	}

	// Parse announcement
	var announcement PeerAnnouncement
	if err := json.Unmarshal(plaintext, &announcement); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal announcement: %w", err)
	}

	// Validate announcement fields
	if err := announcement.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid announcement: %w", err)
	}

	return envelope, &announcement, nil
}

// OpenEnvelopeRaw decrypts a message and returns raw plaintext payload.
func OpenEnvelopeRaw(data []byte, gossipKey [32]byte) (*Envelope, []byte, error) {
	// Parse envelope
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	if len(envelope.Nonce) != NonceSize {
		return nil, nil, fmt.Errorf("invalid nonce size: %d", len(envelope.Nonce))
	}

	// Create AES cipher
	block, err := aes.NewCipher(gossipKey[:])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("decryption failed (wrong key?): %w", err)
	}

	// Validate shared payload metadata for replay protection.
	var meta struct {
		Protocol  string `json:"protocol"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(plaintext, &meta); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal payload metadata: %w", err)
	}

	// Verify protocol version
	if meta.Protocol != ProtocolVersion {
		return nil, nil, fmt.Errorf("unsupported protocol version: %s", meta.Protocol)
	}

	// Check timestamp to prevent replay attacks
	msgTime := time.Unix(meta.Timestamp, 0)
	if time.Since(msgTime) > MaxMessageAge {
		return nil, nil, fmt.Errorf("message too old: %v", time.Since(msgTime))
	}
	if msgTime.After(time.Now().Add(MaxMessageAge)) {
		return nil, nil, fmt.Errorf("message timestamp in future")
	}

	return &envelope, plaintext, nil
}

// CreateAnnouncement creates a new peer announcement with all identity fields.
func CreateAnnouncement(wgPubKey, meshIP, wgEndpoint string, introducer bool, routableNetworks []string, knownPeers []KnownPeer, hostname, meshIPv6, natType string) *PeerAnnouncement {
	return &PeerAnnouncement{
		Protocol:         ProtocolVersion,
		WGPubKey:         wgPubKey,
		Hostname:         hostname,
		MeshIP:           meshIP,
		MeshIPv6:         meshIPv6,
		WGEndpoint:       wgEndpoint,
		Introducer:       introducer,
		RoutableNetworks: routableNetworks,
		Timestamp:        time.Now().Unix(),
		KnownPeers:       knownPeers,
		NATType:          natType,
	}
}
