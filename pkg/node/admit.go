package node

import (
	"log"
	"net"
	"sync/atomic"
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

// Decision is the result of Decide. Dropped names the route entries removed
// from the candidate (Verdict AdmitPartial); Warnings names the rules that
// would act once enforced and were only counted (Verdict Admit).
type Decision struct {
	Verdict  Verdict
	Reason   Reason
	Dropped  []string
	Warnings []string
}

// EnforceIPBinding switches the refusal of R5: a first-hand claim from a key not
// yet stored, whose claim is unbound, for an address a bound claim of another
// member holds, is refused (ReasonIPHeld). It is false until release N+1
// (ADR-0002 decision 4); until then each such claim is a warning and is admitted.
// The order between two claims for one address (a bound claim beats an unbound
// one, then the lower key) is R4 and lives in Winner and ResolveClaims, which
// read the same switch.
var EnforceIPBinding atomic.Bool

// Policy is what the daemon lends the store. All fields are optional.
type Policy struct {
	Local   func() (key, ip string)
	MeshNet *net.IPNet
	Derive  func(pubKey string, nonce int) string
	OnWarn  func(rule string)
	// AcceptRoutes is the operator's list of approved non-mesh networks (R9);
	// nil or an empty result approves every network.
	AcceptRoutes func() []*net.IPNet
}

// Snapshot is what Decide reads: no I/O, no clock.
type Snapshot struct {
	Count      int
	Known      bool
	Tombstoned bool
	Policy     *Policy
	// Holders lists the stored claims on an IP; nil means none.
	Holders func(ip string) []Claim
}

// Decide is a pure function of a snapshot and a candidate. Its refusals are
// a tombstoned key for a claim that is not first-hand, the peer cap, and (R5,
// once EnforceIPBinding is true) a first-hand claim from a new key for an
// address a bound claim of another key holds; bindClaim also lists route
// entries to drop once EnforceRouteBinding is true.
func Decide(s Snapshot, c Candidate) Decision {
	if s.Tombstoned && c.Hand != FirstHand {
		return Decision{Verdict: Refuse, Reason: ReasonTombstoned}
	}
	if !s.Known && s.Count >= DefaultMaxPeers {
		return Decision{Verdict: Refuse, Reason: ReasonPeerCap}
	}
	if s.Policy == nil {
		return Decision{Verdict: Admit}
	}
	return bindClaim(s, c)
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
		d = Decide(Snapshot{Count: len(ps.peers), Known: known, Tombstoned: ps.isTombstoned(key), Policy: ps.policy, Holders: ps.holders}, c)
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
			// Log once per key: a flood of claims on a held address must not become a log flood.
			if d.Reason == ReasonIPHeld {
				if _, done := ps.ipHeldLogged[key]; !done {
					if ps.ipHeldLogged == nil || len(ps.ipHeldLogged) >= DefaultMaxPeers {
						ps.ipHeldLogged = make(map[string]struct{})
					}
					ps.ipHeldLogged[key] = struct{}{}
					log.Printf("[Admit] refused peer %s... via %s: mesh IP %s (%s): a bound claim of another member holds this address; the peer must derive its own address or rotate its key",
						shortKey(key), c.Method(), c.Info.MeshIP, ReasonIPHeld)
				}
			}
			return
		}
		if c.Hand == FirstHand {
			delete(ps.tombstones, key)
		}
		if d.Verdict == AdmitPartial {
			c.Info = withoutDropped(c.Info, d.Dropped)
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
