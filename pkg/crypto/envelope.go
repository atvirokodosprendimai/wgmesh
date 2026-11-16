package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"
	"unicode/utf8"
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

var now = time.Now

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
// Called from FilterAnnouncement after deserialization. Its KnownPeers checks
// serve direct callers: FilterAnnouncement sets KnownPeers aside before it calls this.
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

// FilterAnnouncement is the one validation path for a received announcement on
// every reader (LAN, gossip, registry, exchange). The sender's own fields must
// be valid and KnownPeers must not exceed MaxKnownPeers, or the message is
// rejected. Invalid KnownPeers entries are dropped individually instead, so one
// bad transitive entry relayed by an otherwise valid peer (e.g. an older node
// that stored it before this check existed) does not hide that peer.
func FilterAnnouncement(a *PeerAnnouncement) error {
	known := a.KnownPeers
	if len(known) > MaxKnownPeers {
		return fmt.Errorf("KnownPeers: too many entries (%d, max %d)", len(known), MaxKnownPeers)
	}
	a.KnownPeers = nil
	if err := a.Validate(); err != nil {
		a.KnownPeers = known
		return err
	}
	for _, kp := range known {
		if err := kp.Validate(); err != nil {
			slog.Debug("[crypto] dropping invalid known peer", "error", err)
			continue
		}
		a.KnownPeers = append(a.KnownPeers, kp)
	}
	if dropped := len(known) - len(a.KnownPeers); dropped > 0 {
		slog.Warn("[crypto] dropped invalid known peers", "dropped", dropped, "kept", len(a.KnownPeers))
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
	// V selects how the ciphertext was sealed: 0 (absent) is the legacy form
	// with no additional data, 2 binds the message type to the GCM tag.
	V          int    `json:"v,omitempty"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`

	// LegacyAAD is set by OpenEnvelopeRaw when the envelope was sealed without
	// additional data, so its type was not authenticated. Not serialised.
	LegacyAAD bool `json:"-"`
}

// envelopeAADVersion is the Envelope.V value of an envelope whose message type
// is bound to the GCM tag (ADR-0001).
const envelopeAADVersion = 2

// sealWithAAD makes SealEnvelope seal in the AAD-bound form, and acceptLegacyAAD
// lets OpenEnvelopeRaw open envelopes sealed without additional data. They are
// the two switches of the staged rollout in docs/adr/ADR-0001-envelope-aad, and
// the seams the tests use; nothing in production code sets them.
var (
	sealWithAAD     = false
	acceptLegacyAAD = true
)

// envelopeAAD is the GCM additional data of an AAD-bound envelope: the
// protocol version, a NUL byte and the message type.
func envelopeAAD(messageType string) []byte {
	return []byte(ProtocolVersion + "\x00" + messageType)
}

// SealEnvelope encrypts a message using AES-256-GCM with the gossip key. The
// message type is bound to the tag only when sealWithAAD is set; see
// SealEnvelopeAAD.
func SealEnvelope(messageType string, payload interface{}, gossipKey [32]byte) ([]byte, error) {
	return sealEnvelope(messageType, payload, gossipKey, sealWithAAD)
}

// SealEnvelopeAAD is SealEnvelope with the message type bound to the GCM tag,
// so an envelope whose type field is changed in transit no longer opens.
func SealEnvelopeAAD(messageType string, payload interface{}, gossipKey [32]byte) ([]byte, error) {
	return sealEnvelope(messageType, payload, gossipKey, true)
}

func sealEnvelope(messageType string, payload interface{}, gossipKey [32]byte, withAAD bool) ([]byte, error) {
	// The message type travels as JSON, which replaces invalid UTF-8, so the
	// receiver would rebuild different additional data and the tag would fail.
	if withAAD && !utf8.ValidString(messageType) {
		return nil, fmt.Errorf("message type is not valid UTF-8")
	}

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
	var aad []byte
	version := 0
	if withAAD {
		aad = envelopeAAD(messageType)
		version = envelopeAADVersion
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// Create envelope
	envelope := Envelope{
		MessageType: messageType,
		V:           version,
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

	if err := FilterAnnouncement(&announcement); err != nil {
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

	// Choose the additional data from the envelope version. An unknown version
	// is refused, never opened with nil additional data.
	var aad []byte
	switch envelope.V {
	case 0:
		if !acceptLegacyAAD {
			return nil, nil, fmt.Errorf("envelope without type authentication refused: its sender must be upgraded to a release that seals the message type")
		}
		envelope.LegacyAAD = true
	case envelopeAADVersion:
		aad = envelopeAAD(envelope.MessageType)
	default:
		return nil, nil, fmt.Errorf("unsupported envelope version %d: this node reads 0 and %d; upgrade it if the sender is newer", envelope.V, envelopeAADVersion)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, aad)
	if err != nil {
		if envelope.V == envelopeAADVersion {
			return nil, nil, fmt.Errorf("decryption failed (wrong key, or the message type was changed after sealing): %w", err)
		}
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
	currentTime := now()
	msgTime := time.Unix(meta.Timestamp, 0)
	if currentTime.Sub(msgTime) > MaxMessageAge {
		return nil, nil, fmt.Errorf("message too old: %v", currentTime.Sub(msgTime))
	}
	if msgTime.After(currentTime.Add(MaxMessageAge)) {
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
