package daemon

import (
	"context"
	"log"

	"github.com/atvirokodosprendimai/wgmesh/pkg/privacy"
)

// EpochManager manages relay peer epochs for Dandelion++ privacy
type EpochManager struct {
	router *privacy.DandelionRouter
	cancel context.CancelFunc
}

// NewEpochManager creates a new epoch manager
func NewEpochManager(epochSeed [32]byte) *EpochManager {
	return &EpochManager{
		router: privacy.NewDandelionRouter(epochSeed),
	}
}

// Start begins epoch rotation, stopping when ctx is cancelled.
func (em *EpochManager) Start(ctx context.Context, getPeers func() []privacy.PeerInfo) {
	epochCtx, cancel := context.WithCancel(ctx)
	em.cancel = cancel
	go em.router.EpochRotationLoop(epochCtx, getPeers)
	log.Printf("[Epoch] Epoch management started (rotation every %v)", privacy.DefaultEpochDuration)
}

// Stop stops epoch rotation.
func (em *EpochManager) Stop() {
	if em.cancel != nil {
		em.cancel()
	}
}

// GetRouter returns the Dandelion router
func (em *EpochManager) GetRouter() *privacy.DandelionRouter {
	return em.router
}

// GetCurrentEpoch returns the current epoch info
func (em *EpochManager) GetCurrentEpoch() *privacy.Epoch {
	return em.router.GetEpoch()
}
