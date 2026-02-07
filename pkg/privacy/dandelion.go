package privacy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
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
	onFluff func(announce DandelionAnnounce) // Called when fluff phase begins
	onStem  func(announce DandelionAnnounce, relay PeerInfo) // Called to relay via stem
}

// NewDandelionRouter creates a new Dandelion++ router
func NewDandelionRouter(epochSeed [32]byte) *DandelionRouter {
	return &DandelionRouter{
		epochSeed: epochSeed,
		epoch: &Epoch{
			ID:        0,
			StartedAt: time.Now(),
			Duration:  10 * time.Minute,
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

	msg.HopCount++

	// Decide: fluff or continue stem?
	if ShouldFluff(msg.HopCount) {
		// Transition to fluff phase - announce publicly
		log.Printf("[Dandelion] Fluffing announcement from %s after %d hops", msg.OriginPubkey[:8]+"...", msg.HopCount)
		if onFluff != nil {
			onFluff(msg)
		}
		return
	}

	// Continue stem phase - relay to a deterministic peer
	if epoch != nil && len(epoch.RelayPeers) > 0 {
		relay := epoch.RelayPeers[int(msg.HopCount)%len(epoch.RelayPeers)]
		log.Printf("[Dandelion] Relaying via stem to %s (hop %d)", relay.WGPubKey[:8]+"...", msg.HopCount)
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
	// 10% probability per hop
	return rand.Float64() < FluffProbability
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
		Duration:   10 * time.Minute,
	}

	if len(relayPeers) > 0 {
		log.Printf("[Dandelion] Epoch %d: relay peers: %v", newEpochID, peerKeys(relayPeers))
	}
}

// GetEpoch returns the current epoch
func (d *DandelionRouter) GetEpoch() *Epoch {
	d.mu.RLock()
	defer d.mu.RUnlock()
	epochCopy := *d.epoch
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
		if len(p.WGPubKey) > 8 {
			keys[i] = p.WGPubKey[:8] + "..."
		} else {
			keys[i] = p.WGPubKey
		}
	}
	return keys
}

// EpochRotationLoop runs the epoch rotation in a background goroutine
func (d *DandelionRouter) EpochRotationLoop(stopCh <-chan struct{}, getPeers func() []PeerInfo) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// Initial rotation
	d.RotateEpoch(getPeers())

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			d.RotateEpoch(getPeers())
		}
	}
}

// CreateAnnounce creates a new Dandelion announcement for the local node
func CreateAnnounce(pubkey, meshIP, endpoint string, routableNetworks []string) DandelionAnnounce {
	nonce := make([]byte, 16)
	rand.Read(nonce)

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
