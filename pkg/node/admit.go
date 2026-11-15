package node

import (
	"log"
	"net"
)

// Source labels where a claim came from: lan, dht, dht-rendezvous,
// dht-ipv6-sync, gossip, cache. It is a typed string so a new label passes
// through unchanged into DiscoveredVia and EndpointMethod.
type Source string

// Hand says how a claim knows what it claims.
type Hand int

const (
	// FirstHand: the sender describes itself.
	FirstHand Hand = iota
	// SecondHand: another member mentions the peer (a KnownPeers entry).
	SecondHand
	// Restored: the on-disk peer cache.
	Restored
)

// Candidate is one claim about a peer offered to the store.
type Candidate struct {
	Info   *PeerInfo
	Source Source
	Hand   Hand
}

// Method returns the discovery method string stored in DiscoveredVia and
// EndpointMethod.
func (c Candidate) Method() string {
	if c.Hand == SecondHand && !isTransitiveMethod(string(c.Source)) {
		return string(c.Source) + "-transitive"
	}
	return string(c.Source)
}

// Verdict is the outcome class of a Decision.
type Verdict int

const (
	Admit Verdict = iota
	AdmitPartial
	Refuse
)

// Reason names why a candidate was refused.
type Reason string

const ReasonPeerCap Reason = "peer-cap"

// Decision is the result of Decide. AdmitPartial, Dropped and Warnings are
// declared for the rules of T3 to T10 and nothing produces them yet; Dropped
// would name fields removed from a candidate, Warnings rules that would refuse in enforce mode.
type Decision struct {
	Verdict  Verdict
	Reason   Reason
	Dropped  []string
	Warnings []string
}

// Policy is what the daemon lends the store. All fields are optional.
type Policy struct {
	Local   func() (key, ip string)
	MeshNet *net.IPNet
	Derive  func(pubKey string, nonce int) string
	OnWarn  func(rule string)
}

// Snapshot is what Decide reads: no I/O, no clock.
type Snapshot struct {
	Count  int
	Known  bool
	Policy *Policy
}

// Decide is a pure function of a snapshot and a candidate. Its one refusal
// today is the peer cap.
func Decide(s Snapshot, c Candidate) Decision {
	if !s.Known && s.Count >= DefaultMaxPeers {
		return Decision{Verdict: Refuse, Reason: ReasonPeerCap}
	}
	return Decision{Verdict: Admit}
}

// SetPolicy sets the policy used by Offer; nil means no policy.
func (ps *PeerStore) SetPolicy(p *Policy) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.policy = p
}

// Offer decides what a claim may do and applies it, under the store's one lock.
func (ps *PeerStore) Offer(c Candidate) Decision {
	var d Decision
	var eventKey string
	var eventKind PeerEventKind

	func() {
		ps.mu.Lock()
		defer ps.mu.Unlock()
		ps.offers++

		_, known := ps.peers[c.Info.WGPubKey]
		d = Decide(Snapshot{Count: len(ps.peers), Known: known, Policy: ps.policy}, c)
		if d.Verdict == Refuse {
			if d.Reason == ReasonPeerCap {
				log.Printf("[PeerStore] peer cap reached (%d); dropping new peer %s... via %s",
					DefaultMaxPeers, shortKey(c.Info.WGPubKey), c.Method())
			}
			return
		}
		eventKey, eventKind = ps.merge(c)
	}()

	if eventKey != "" {
		ps.notify(eventKey, eventKind)
	}
	return d
}

// legacyCandidate reads a method string the way Update always has.
func legacyCandidate(info *PeerInfo, method string) Candidate {
	c := Candidate{Info: info, Source: Source(method)}
	switch {
	case method == "cache":
		c.Hand = Restored
	case isTransitiveMethod(method):
		c.Hand = SecondHand
	}
	return c
}
