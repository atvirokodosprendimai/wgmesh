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

const (
	ReasonPeerCap    Reason = "peer-cap"
	ReasonTombstoned Reason = "tombstoned"
	ReasonIPHeld     Reason = "ip-held-by-bound"
)

// Decision is the result of Decide. AdmitPartial, Dropped and Warnings are
// declared for the rules of T3 to T10 and nothing produces them yet; Dropped
// would name fields removed from a candidate, Warnings rules that would refuse in enforce mode.
type Decision struct {
	Verdict  Verdict
	Reason   Reason
	Dropped  []string
	Warnings []string
}

// EnforceIPBinding switches the claim order of R4 (a bound claim beats an
// unbound one, then the lower key). It is false until the binding is enforced
// (release N+1, ADR-0002 decision 4): the lower key decides and each case the
// bound order would decide differently is a warning.
var EnforceIPBinding = false

// Policy is what the daemon lends the store. All fields are optional.
type Policy struct {
	Local   func() (key, ip string)
	MeshNet *net.IPNet
	Derive  func(pubKey string, nonce int) string
	OnWarn  func(rule string)
}

// Snapshot is what Decide reads: no I/O, no clock.
type Snapshot struct {
	Count      int
	Known      bool
	Tombstoned bool
	Policy     *Policy
}

// Decide is a pure function of a snapshot and a candidate. Its refusals are
// a tombstoned key for a claim that is not first-hand, and the peer cap.
func Decide(s Snapshot, c Candidate) Decision {
	if s.Tombstoned && c.Hand != FirstHand {
		return Decision{Verdict: Refuse, Reason: ReasonTombstoned}
	}
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

// Policy returns the policy set by SetPolicy, or nil.
func (ps *PeerStore) Policy() *Policy {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.policy
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

		key := c.Info.WGPubKey
		_, known := ps.peers[key]
		d = Decide(Snapshot{Count: len(ps.peers), Known: known, Tombstoned: ps.isTombstoned(key), Policy: ps.policy}, c)
		if d.Verdict == Refuse {
			if d.Reason == ReasonPeerCap {
				log.Printf("[PeerStore] peer cap reached (%d); dropping new peer %s... via %s",
					DefaultMaxPeers, shortKey(key), c.Method())
			}
			// Log once per tombstone: a mention flood must not become a log flood.
			if t := ps.tombstones[key]; d.Reason == ReasonTombstoned && !t.logged {
				t.logged = true
				ps.tombstones[key] = t
				log.Printf("[Admit] refused tombstoned peer %s... via %s (withdrawn: %s); until TombstoneTTL passes or the peer speaks first", shortKey(key), c.Method(), t.cause)
			}
			return
		}
		if c.Hand == FirstHand {
			delete(ps.tombstones, key)
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
