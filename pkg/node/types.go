// Package node contains the shared types used by both pkg/daemon and
// pkg/discovery. Neither package may import the other; both import pkg/node.
package node

import (
	"net"
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
	EndpointGuessed  bool // endpoint built from the sender address and the default port, not advertised: ranks below every advertisement, never announced or cached (#839)
	heardFirstHand   bool // a first-hand claim was accepted at least once
}

// LocalNode represents our local WireGuard node. It is the one local-node
// type: pkg/daemon aliases it (peerstore_compat.go), so a method added here
// reaches the value initLocalNode, discovery and the tests use.
// The WG endpoint is accessed via GetEndpoint/SetEndpoint to allow
// concurrent updates from discovery goroutines. RoutableNetworks changes on
// SIGHUP reload, so once the node is shared it must be accessed only via
// GetRoutableNetworks/SetRoutableNetworks.
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	MeshIPv6         string
	RoutableNetworks []string
	Introducer       bool
	NATType          string // Detected NAT type: "cone", "symmetric", or "unknown"
	Hostname         string

	endpointMu sync.RWMutex
	wgEndpoint string
	// pendingSTUN is the STUN value waiting for its third identical
	// observation; both fields are guarded by endpointMu.
	pendingSTUN      string
	pendingSTUNCount int

	routesMu sync.RWMutex // guards RoutableNetworks
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

// EndpointSource names where an endpoint observation came from.
type EndpointSource string

const (
	// EndpointInterface is a global IPv6 address read from a local interface.
	EndpointInterface EndpointSource = "interface"
	// EndpointReflection is the address a peer reflected back to us.
	EndpointReflection EndpointSource = "reflection"
	// EndpointSTUN is the answer of a STUN server.
	EndpointSTUN EndpointSource = "stun"
)

// stunConfirmations is how many identical STUN observations in a row replace
// a usable endpoint (#828: dual-egress networks flapped between a STUN answer
// and a peer reflection).
const stunConfirmations = 3

// ObserveEndpoint applies an observed endpoint and reports whether the
// announced endpoint changed. While the current endpoint is empty or
// unspecified any source applies at once. An interface address or a peer
// reflection applies at once and discards a pending STUN value. A STUN answer
// that differs from the current endpoint applies only on its third identical
// observation in a row; one equal to it discards the pending value.
// It sets the field itself under endpointMu and does not call SetEndpoint:
// SetEndpoint takes endpointMu too, so calling it here would deadlock.
func (n *LocalNode) ObserveEndpoint(source EndpointSource, endpoint string) bool {
	n.endpointMu.Lock()
	defer n.endpointMu.Unlock()
	if source == EndpointSTUN && endpoint != n.wgEndpoint && !endpointUnspecified(n.wgEndpoint) {
		if endpoint == n.pendingSTUN {
			n.pendingSTUNCount++
		} else {
			n.pendingSTUN, n.pendingSTUNCount = endpoint, 1
		}
		if n.pendingSTUNCount < stunConfirmations {
			return false
		}
	}
	n.pendingSTUN, n.pendingSTUNCount = "", 0
	changed := n.wgEndpoint != endpoint
	n.wgEndpoint = endpoint
	return changed
}

// endpointUnspecified reports whether ep is empty or has an unspecified
// host (0.0.0.0, ::).
func endpointUnspecified(ep string) bool {
	if ep == "" {
		return true
	}
	host, _, err := net.SplitHostPort(ep)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

// GetRoutableNetworks returns a copy of the advertised networks (thread-safe).
func (n *LocalNode) GetRoutableNetworks() []string {
	n.routesMu.RLock()
	defer n.routesMu.RUnlock()
	return append([]string(nil), n.RoutableNetworks...)
}

// SetRoutableNetworks stores a copy of the advertised networks (thread-safe).
func (n *LocalNode) SetRoutableNetworks(networks []string) {
	networks = append([]string(nil), networks...)
	n.routesMu.Lock()
	defer n.routesMu.Unlock()
	n.RoutableNetworks = networks
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
