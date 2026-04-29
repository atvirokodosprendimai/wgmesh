package daemon

import "github.com/atvirokodosprendimai/wgmesh/pkg/node"

// Type aliases so that all existing daemon-internal code continues to compile
// without modification.
type PeerInfo = node.PeerInfo
type PeerStore = node.PeerStore
type PeerEvent = node.PeerEvent
type PeerEventKind = node.PeerEventKind

const (
	PeerDeadTimeout   = node.PeerDeadTimeout
	PeerRemoveTimeout = node.PeerRemoveTimeout
	PeerEventBufSize  = node.PeerEventBufSize
	DefaultMaxPeers   = node.DefaultMaxPeers
	PeerEventNew      = node.PeerEventNew
	PeerEventUpdated  = node.PeerEventUpdated

	LANMethod        = node.LANMethod
	RendezvousMethod = node.RendezvousMethod
)

func NewPeerStore() *PeerStore { return node.NewPeerStore() }
