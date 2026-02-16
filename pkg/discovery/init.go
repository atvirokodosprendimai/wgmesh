package discovery

import (
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func init() {
	// Register the DHT discovery factory with the daemon package
	daemon.SetDHTDiscoveryFactory(createDHTDiscovery)
}

// createDHTDiscovery creates a new DHT discovery instance
// This is called by the daemon when starting with DHT discovery enabled
func createDHTDiscovery(config *daemon.Config, localNode *daemon.LocalNode, peerStore *daemon.PeerStore) (daemon.DiscoveryLayer, error) {
	// Convert daemon.LocalNode to discovery.LocalNode
	discoveryLocalNode := &LocalNode{
		WGPubKey:         localNode.WGPubKey,
		WGPrivateKey:     localNode.WGPrivateKey,
		MeshIP:           localNode.MeshIP,
		WGEndpoint:       localNode.WGEndpoint,
		Hostname:         localNode.Hostname,
		RoutableNetworks: localNode.RoutableNetworks,
	}

	return NewDHTDiscovery(config, discoveryLocalNode, peerStore)
}
