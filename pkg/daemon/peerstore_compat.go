package daemon

import "github.com/atvirokodosprendimai/wgmesh/pkg/node"

// Type aliases so that all existing daemon-internal code continues to compile
// without modification.
type PeerInfo = node.PeerInfo
type PeerStore = node.PeerStore
type PeerEvent = node.PeerEvent
type PeerEventKind = node.PeerEventKind
type Candidate = node.Candidate
type Hand = node.Hand
type Source = node.Source
type Decision = node.Decision
type Policy = node.Policy
type LocalNode = node.LocalNode
type EndpointSource = node.EndpointSource

const (
	PeerDeadTimeout   = node.PeerDeadTimeout
	PeerRemoveTimeout = node.PeerRemoveTimeout
	PeerEventBufSize  = node.PeerEventBufSize
	DefaultMaxPeers   = node.DefaultMaxPeers
	TombstoneTTL      = node.TombstoneTTL
	PeerEventNew      = node.PeerEventNew
	PeerEventUpdated  = node.PeerEventUpdated

	LANMethod        = node.LANMethod
	RendezvousMethod = node.RendezvousMethod
)

const (
	FirstHand  = node.FirstHand
	SecondHand = node.SecondHand
	Restored   = node.Restored
)

const (
	EndpointInterface  = node.EndpointInterface
	EndpointReflection = node.EndpointReflection
	EndpointSTUN       = node.EndpointSTUN
)

func NewPeerStore() *PeerStore { return node.NewPeerStore() }
