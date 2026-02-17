package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
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
