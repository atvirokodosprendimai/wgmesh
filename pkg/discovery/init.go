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
		Hostname:         localNode.Hostname,
		WGPrivateKey:     localNode.WGPrivateKey,
		MeshIP:           localNode.MeshIP,
		MeshIPv6:         localNode.MeshIPv6,
		WGEndpoint:       localNode.WGEndpoint,
		RoutableNetworks: localNode.RoutableNetworks,
		Introducer:       localNode.Introducer,
		NATType:          NATType(localNode.NATType),
	}

	return NewDHTDiscovery(config, discoveryLocalNode, peerStore)
}
