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
//
// Every field a second goroutine can touch sits behind a lock, one lock per
// group, and a method holds at most one of them:
//   - stateMu guards meshIP, meshIPv6, natType and introducer (reconcile,
//     STUN refresh and the announcement builders race on them);
//   - endpointMu guards wgEndpoint, pendingSTUN and pendingSTUNCount;
//   - routesMu guards routableNetworks (SIGHUP reload).
//
// WGPubKey, WGPrivateKey and Hostname are assigned once, by NewLocalNode and
// by initLocalNode/loadLocalNode, before any goroutine starts, and are never
// written again (TestLocalNodeFieldsNotWrittenDirectly), so they stay plain
// exported fields and need no lock.
type LocalNode struct {
	WGPubKey     string
	WGPrivateKey string
	Hostname     string

	stateMu    sync.RWMutex
	meshIP     string
	meshIPv6   string
	natType    string // detected NAT type: "cone", "symmetric", or "unknown"
	introducer bool

	endpointMu sync.RWMutex
	wgEndpoint string
	// pendingSTUN is the STUN value waiting for its third identical
	// observation; both fields are guarded by endpointMu.
	pendingSTUN      string
	pendingSTUNCount int

	routesMu         sync.RWMutex
	routableNetworks []string
}

// NewLocalNode builds the node with its identity and addresses. Call it before
// any goroutine starts: the key and the hostname are written only here.
func NewLocalNode(pubKey, privKey, hostname, meshIP, meshIPv6 string) *LocalNode {
	return &LocalNode{
		WGPubKey:     pubKey,
		WGPrivateKey: privKey,
		Hostname:     hostname,
		meshIP:       meshIP,
		meshIPv6:     meshIPv6,
	}
}

// GetMeshIP returns the mesh IPv4 address (thread-safe).
func (n *LocalNode) GetMeshIP() string {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.meshIP
}

// SetMeshIP stores the mesh IPv4 address (thread-safe).
func (n *LocalNode) SetMeshIP(ip string) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.meshIP = ip
}

// GetMeshIPv6 returns the mesh IPv6 address (thread-safe).
func (n *LocalNode) GetMeshIPv6() string {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.meshIPv6
}

// SetMeshIPv6 stores the mesh IPv6 address (thread-safe).
func (n *LocalNode) SetMeshIPv6(ip string) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.meshIPv6 = ip
}

// GetNATType returns the detected NAT type (thread-safe).
func (n *LocalNode) GetNATType() string {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.natType
}

// SetNATType stores the detected NAT type (thread-safe).
func (n *LocalNode) SetNATType(t string) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.natType = t
}

// IsIntroducer reports whether this node is an introducer (thread-safe).
func (n *LocalNode) IsIntroducer() bool {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.introducer
}

// SetIntroducer stores the introducer flag (thread-safe).
func (n *LocalNode) SetIntroducer(v bool) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.introducer = v
}

// Identity returns the public key and the mesh IP, the mesh IP read under
// stateMu so a reader never sees it half-written.
func (n *LocalNode) Identity() (pubKey, meshIP string) {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.WGPubKey, n.meshIP
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
	return append([]string(nil), n.routableNetworks...)
}

// SetRoutableNetworks stores a copy of the advertised networks (thread-safe).
func (n *LocalNode) SetRoutableNetworks(networks []string) {
	networks = append([]string(nil), networks...)
	n.routesMu.Lock()
	defer n.routesMu.Unlock()
	n.routableNetworks = networks
}

// LocalSnapshot is a plain copy of the local node's announced state, what an
// announcement builder reads once instead of eight fields one by one.
type LocalSnapshot struct {
	PubKey     string
	Hostname   string
	MeshIP     string
	MeshIPv6   string
	NATType    string
	Introducer bool
	Routes     []string // a copy
	Endpoint   string
}

// Snapshot copies the announced state. Each group is read under its own lock,
// in turn and never nested: a group is consistent in itself, the groups are
// not read at one instant.
func (n *LocalNode) Snapshot() LocalSnapshot {
	s := LocalSnapshot{PubKey: n.WGPubKey, Hostname: n.Hostname}
	n.stateMu.RLock()
	s.MeshIP, s.MeshIPv6, s.NATType, s.Introducer = n.meshIP, n.meshIPv6, n.natType, n.introducer
	n.stateMu.RUnlock()
	s.Routes = n.GetRoutableNetworks()
	s.Endpoint = n.GetEndpoint()
	return s
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
