package privacy

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	// FluffProbability is the probability of transitioning from stem to fluff at each hop
	FluffProbability = 0.10 // 10%
	// MaxStemHops is the maximum number of stem hops before forced fluff
	MaxStemHops = 4
	// DandelionMethod is the discovery method label
	DandelionMethod = "dandelion"
	// DefaultEpochDuration is the default duration for relay epochs
	DefaultEpochDuration = 10 * time.Minute
)

// DandelionAnnounce represents an announcement being relayed through the Dandelion++ protocol
type DandelionAnnounce struct {
	OriginPubkey     string   `json:"origin_pubkey"`
	OriginMeshIP     string   `json:"origin_mesh_ip"`
	OriginEndpoint   string   `json:"origin_endpoint"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	HopCount         uint8    `json:"hop_count"`
	Timestamp        int64    `json:"timestamp"`
	Nonce            []byte   `json:"nonce"`
}

// PeerInfo represents a minimal peer info for relay selection
type PeerInfo struct {
	WGPubKey string
	MeshIP   string
	Endpoint string
}

// Epoch represents a time-based relay configuration
type Epoch struct {
	ID         uint64
	RelayPeers []PeerInfo
	StartedAt  time.Time
	Duration   time.Duration
}

// DandelionRouter manages the Dandelion++ stem/fluff protocol
type DandelionRouter struct {
	epochSeed [32]byte
	epoch     *Epoch

	mu sync.RWMutex

	// Callbacks
	onFluff func(announce DandelionAnnounce)                 // Called when fluff phase begins
	onStem  func(announce DandelionAnnounce, relay PeerInfo) // Called to relay via stem
}

// NewDandelionRouter creates a new Dandelion++ router
func NewDandelionRouter(epochSeed [32]byte) *DandelionRouter {
	return &DandelionRouter{
		epochSeed: epochSeed,
		epoch: &Epoch{
			ID:        0,
			StartedAt: time.Now(),
			Duration:  DefaultEpochDuration,
		},
	}
}

// SetFluffHandler sets the callback for when a message should be fluffed (announced publicly)
func (d *DandelionRouter) SetFluffHandler(handler func(DandelionAnnounce)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onFluff = handler
}

// SetStemHandler sets the callback for when a message should be relayed via stem
func (d *DandelionRouter) SetStemHandler(handler func(DandelionAnnounce, PeerInfo)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onStem = handler
}

// HandleAnnounce processes a Dandelion++ announcement
func (d *DandelionRouter) HandleAnnounce(msg DandelionAnnounce) {
	d.mu.RLock()
	onFluff := d.onFluff
	onStem := d.onStem
	epoch := d.epoch
	d.mu.RUnlock()

	// Cap HopCount to prevent uint8 overflow from malformed messages
	if msg.HopCount >= MaxStemHops {
		msg.HopCount = MaxStemHops
	} else {
		msg.HopCount++
	}

	// Decide: fluff or continue stem?
	if ShouldFluff(msg.HopCount) {
		// Transition to fluff phase - announce publicly
		log.Printf("[Dandelion] Fluffing announcement from %s after %d hops", truncateKey(msg.OriginPubkey), msg.HopCount)
		if onFluff != nil {
			onFluff(msg)
		}
		return
	}

	// Continue stem phase - relay to a deterministic peer
	if epoch != nil && len(epoch.RelayPeers) > 0 {
		relay := epoch.RelayPeers[int(msg.HopCount)%len(epoch.RelayPeers)]
		log.Printf("[Dandelion] Relaying via stem to %s (hop %d)", truncateKey(relay.WGPubKey), msg.HopCount)
		if onStem != nil {
			onStem(msg, relay)
		}
	} else {
		// No relay peers available - fluff immediately
		log.Printf("[Dandelion] No relay peers, fluffing immediately")
		if onFluff != nil {
			onFluff(msg)
		}
	}
}

// ShouldFluff determines whether to transition from stem to fluff
func ShouldFluff(hopCount uint8) bool {
	// Force fluff after max hops
	if hopCount >= MaxStemHops {
		return true
	}
	// Use crypto/rand for unpredictable stem/fluff decisions
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(100))
	if err != nil {
		// On error, default to fluff for safety
		return true
	}
	return n.Int64() < int64(FluffProbability*100)
}

// RotateEpoch rotates the relay epoch with new peers
func (d *DandelionRouter) RotateEpoch(allPeers []PeerInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	newEpochID := d.epoch.ID + 1

	// Select relay peers deterministically using epoch seed
	relayPeers := selectRelayPeers(d.epochSeed, newEpochID, allPeers, 2)

	d.epoch = &Epoch{
		ID:         newEpochID,
		RelayPeers: relayPeers,
		StartedAt:  time.Now(),
		Duration:   DefaultEpochDuration,
	}

	if len(relayPeers) > 0 {
		log.Printf("[Dandelion] Epoch %d: relay peers: %v", newEpochID, peerKeys(relayPeers))
	}
}

// GetEpoch returns the current epoch
func (d *DandelionRouter) GetEpoch() *Epoch {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.epoch == nil {
		return nil
	}

	epochCopy := *d.epoch

	// Deep-copy RelayPeers slice so callers cannot mutate internal state.
	if epochCopy.RelayPeers != nil {
		relayPeersCopy := make([]PeerInfo, len(epochCopy.RelayPeers))
		copy(relayPeersCopy, epochCopy.RelayPeers)
		epochCopy.RelayPeers = relayPeersCopy
	}

	return &epochCopy
}

// selectRelayPeers deterministically selects relay peers using HMAC
func selectRelayPeers(epochSeed [32]byte, epochID uint64, allPeers []PeerInfo, count int) []PeerInfo {
	if len(allPeers) == 0 {
		return nil
	}

	if count > len(allPeers) {
		count = len(allPeers)
	}

	// Create deterministic seed for this epoch
	var epochBytes [8]byte
	binary.BigEndian.PutUint64(epochBytes[:], epochID)
	mac := hmac.New(sha256.New, epochSeed[:])
	mac.Write(epochBytes[:])
	seed := mac.Sum(nil)

	// Sort peers for deterministic output
	sorted := make([]PeerInfo, len(allPeers))
	copy(sorted, allPeers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].WGPubKey < sorted[j].WGPubKey
	})

	// Deterministic shuffle using the seed
	seedInt := int64(binary.BigEndian.Uint64(seed[:8]))
	rng := rand.New(rand.NewSource(seedInt))
	rng.Shuffle(len(sorted), func(i, j int) {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	})

	return sorted[:count]
}

// peerKeys returns abbreviated public keys for logging
func peerKeys(peers []PeerInfo) []string {
	keys := make([]string, len(peers))
	for i, p := range peers {
		keys[i] = truncateKey(p.WGPubKey)
	}
	return keys
}

// truncateKey safely truncates a key for logging
func truncateKey(key string) string {
	if len(key) > 8 {
		return key[:8] + "..."
	}
	return key
}

// EpochRotationLoop runs the epoch rotation in a background goroutine until
// ctx is cancelled.
func (d *DandelionRouter) EpochRotationLoop(ctx context.Context, getPeers func() []PeerInfo) {
	ticker := time.NewTicker(DefaultEpochDuration)
	defer ticker.Stop()

	// Initial rotation
	d.RotateEpoch(getPeers())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.RotateEpoch(getPeers())
		}
	}
}

// CreateAnnounce creates a new Dandelion announcement for the local node
func CreateAnnounce(pubkey, meshIP, endpoint string, routableNetworks []string) DandelionAnnounce {
	nonce := make([]byte, 16)
	if _, err := cryptorand.Read(nonce); err != nil {
		// Fallback: this should never fail with crypto/rand
		log.Printf("[Dandelion] Warning: crypto/rand failed: %v", err)
	}

	return DandelionAnnounce{
		OriginPubkey:     pubkey,
		OriginMeshIP:     meshIP,
		OriginEndpoint:   endpoint,
		RoutableNetworks: routableNetworks,
		HopCount:         0,
		Timestamp:        time.Now().Unix(),
		Nonce:            nonce,
	}
}

// NeedsEpochRotation checks if the current epoch has expired
func (d *DandelionRouter) NeedsEpochRotation() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return time.Since(d.epoch.StartedAt) > d.epoch.Duration
}

// FormatEpochInfo returns a human-readable epoch status
func (d *DandelionRouter) FormatEpochInfo() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	remaining := d.epoch.Duration - time.Since(d.epoch.StartedAt)
	if remaining < 0 {
		remaining = 0
	}

	return fmt.Sprintf("Epoch %d: %d relay peers, %v remaining",
		d.epoch.ID, len(d.epoch.RelayPeers), remaining.Round(time.Second))
}
