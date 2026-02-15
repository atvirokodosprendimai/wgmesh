package daemon

import (
	"log"

	"github.com/atvirokodosprendimai/wgmesh/pkg/privacy"
)

// EpochManager manages relay peer epochs for Dandelion++ privacy
type EpochManager struct {
	router *privacy.DandelionRouter
	stopCh chan struct{}
}

// NewEpochManager creates a new epoch manager
func NewEpochManager(epochSeed [32]byte) *EpochManager {
	return &EpochManager{
		router: privacy.NewDandelionRouter(epochSeed),
		stopCh: make(chan struct{}),
	}
}

// Start begins epoch rotation
func (em *EpochManager) Start(getPeers func() []privacy.PeerInfo) {
	go em.router.EpochRotationLoop(em.stopCh, getPeers)
	log.Printf("[Epoch] Epoch management started (rotation every %v)", privacy.DefaultEpochDuration)
}

// Stop stops epoch rotation
func (em *EpochManager) Stop() {
	close(em.stopCh)
}

// GetRouter returns the Dandelion router
func (em *EpochManager) GetRouter() *privacy.DandelionRouter {
	return em.router
}

// GetCurrentEpoch returns the current epoch info
func (em *EpochManager) GetCurrentEpoch() *privacy.Epoch {
	return em.router.GetEpoch()
}
