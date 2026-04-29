// Package node contains the shared types used by both pkg/daemon and
// pkg/discovery. Neither package may import the other; both import pkg/node.
package node

import (
	"sync"
	"time"
)

// DiscoveryLayer is the interface implemented by every discovery backend.
type DiscoveryLayer interface {
	Start() error
	Stop() error
}

// PeerInfo represents a discovered mesh peer.
type PeerInfo struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	MeshIPv6         string
	Endpoint         string // best known endpoint (ip:port)
	Introducer       bool
	RoutableNetworks []string
	LastSeen         time.Time
	DiscoveredVia    []string       // ["lan", "dht", "gossip"]
	Latency          *time.Duration // measured via WG handshake
	NATType          string         // "cone", "symmetric", or "unknown"
	EndpointMethod   string
}

// LocalNode represents the local WireGuard node.
// Endpoint access is thread-safe via GetEndpoint / SetEndpoint.
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	MeshIPv6         string
	RoutableNetworks []string
	Introducer       bool
	NATType          string
	Hostname         string

	endpointMu sync.RWMutex
	wgEndpoint string
}

// GetEndpoint returns the current WireGuard endpoint (thread-safe).
func (n *LocalNode) GetEndpoint() string {
	n.endpointMu.RLock()
	defer n.endpointMu.RUnlock()
	return n.wgEndpoint
}

// SetEndpoint updates the WireGuard endpoint (thread-safe).
func (n *LocalNode) SetEndpoint(ep string) {
	n.endpointMu.Lock()
	defer n.endpointMu.Unlock()
	n.wgEndpoint = ep
}

// ConfigReader is the narrow interface that pkg/discovery uses to read
// configuration. pkg/daemon.Config implements this interface; callers that
// embed wgmesh may provide their own implementation.
type ConfigReader interface {
	GetSecret() string
	GetNetworkID() [20]byte
	GetGossipKey() [32]byte
	GetGossipPort() uint16
	GetMulticastID() [4]byte
	GetEpochSeed() [32]byte
	GetRendezvousID() [8]byte
	GetMeshSubnet() [2]byte
	GetMeshPrefixV6() [8]byte
	GetInterfaceName() string
	GetWGListenPort() int
	GetAdvertiseRoutes() []string
	IsPrivacy() bool
	IsGossip() bool
	IsLANDiscovery() bool
	IsDisableIPv6() bool
	IsForceRelay() bool
	IsDisablePunching() bool
	IsIntroducer() bool
	GetLogLevel() string
}

// TunnelBackend abstracts WireGuard interface management.
// The default implementation (CLITunnelBackend in pkg/daemon) uses the wg
// CLI subprocess. An embedding application may provide an alternative
// implementation backed by golang.zx2c4.com/wireguard (wireguard-go).
type TunnelBackend interface {
	// CreateInterface creates and configures the WireGuard interface.
	CreateInterface(ifaceName, privateKey string, listenPort int) error
	// SetAddress assigns a CIDR address to the interface.
	SetAddress(ifaceName, cidr string) error
	// BringUp brings the interface up.
	BringUp(ifaceName string) error
	// AddPeer installs a peer (pubkey, endpoint, allowedIPs, keepalive).
	AddPeer(ifaceName string, peer PeerConfig) error
	// RemovePeer removes a peer by public key.
	RemovePeer(ifaceName, pubkey string) error
	// ListPeers returns the current peer list and transfer counters.
	ListPeers(ifaceName string) ([]PeerStatus, error)
	// DestroyInterface removes the interface entirely.
	DestroyInterface(ifaceName string) error
}

// PeerConfig carries the parameters needed to install a WireGuard peer.
type PeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepalive int
}

// PeerStatus carries the live status of an installed peer as returned by
// the kernel / userspace WireGuard implementation.
type PeerStatus struct {
	PublicKey         string
	Endpoint          string
	AllowedIPs        []string
	LastHandshakeTime time.Time
	RxBytes           uint64
	TxBytes           uint64
}
