package node

// LocalNode represents our local WireGuard node
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	WGEndpoint       string
	RoutableNetworks []string
}
