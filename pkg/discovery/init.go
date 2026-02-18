package discovery

import (
	"context"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func init() {
	// Register the DHT discovery factory with the daemon package
	daemon.SetDHTDiscoveryFactory(createDHTDiscovery)
}

// createDHTDiscovery creates a new DHT discovery instance
// This is called by the daemon when starting with DHT discovery enabled
func createDHTDiscovery(ctx context.Context, config *daemon.Config, localNode *daemon.LocalNode, peerStore *daemon.PeerStore) (daemon.DiscoveryLayer, error) {
	return NewDHTDiscovery(ctx, config, localNode, peerStore)
}
