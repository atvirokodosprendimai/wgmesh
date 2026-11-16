package node

import (
	"sync/atomic"
	"time"
)

// Liveness is the verdict of Assess on one peer's evidence.
type Liveness int

const (
	// Alive: the newest evidence is younger than PeerDeadTimeout.
	Alive Liveness = iota
	// Suspect: the newest evidence is at least PeerDeadTimeout and at most PeerRemoveTimeout old.
	Suspect
	// Dead: the newest evidence is older than PeerRemoveTimeout, or there is none.
	Dead
)

// Evidence is the times Assess judges; any of them may be zero.
type Evidence struct {
	LastSeen      time.Time
	LastDataPlane time.Time
	RestoredAt    time.Time
	ResumedAt     time.Time
}

// EnforceLiveness makes the Assess verdict act in GetActive, CleanupStale and IsDead.
var EnforceLiveness atomic.Bool

// newest returns the newest non-zero time of e, each as a wall-clock instant
// (monotonic reading stripped), or the zero time when all are zero.
func (e Evidence) newest() time.Time {
	var ref time.Time
	for _, t := range [...]time.Time{e.LastSeen, e.LastDataPlane, e.RestoredAt, e.ResumedAt} {
		t = t.Round(0)
		if !t.IsZero() && t.After(ref) {
			ref = t
		}
	}
	return ref
}

// Assess judges one peer's evidence at now. It takes the newest of the four
// times and ages it as a wall-clock instant: younger than PeerDeadTimeout is
// Alive, up to and including PeerRemoveTimeout is Suspect, older is Dead; no
// evidence at all is Dead. It reads no clock except now.
func Assess(e Evidence, now time.Time) Liveness {
	ref := e.newest()
	if ref.IsZero() {
		return Dead
	}
	age := now.Round(0).Sub(ref)
	switch {
	case age < PeerDeadTimeout:
		return Alive
	case age <= PeerRemoveTimeout:
		return Suspect
	default:
		return Dead
	}
}

// evidence is the peer's four times; resumedAt is the one the store stamps
// into the copies it returns.
func (p *PeerInfo) evidence() Evidence {
	return Evidence{LastSeen: p.LastSeen, LastDataPlane: p.LastDataPlane, RestoredAt: p.RestoredAt, ResumedAt: p.resumedAt}
}

// Alive is Assess of the peer's evidence at now.
func (p *PeerInfo) Alive(now time.Time) Liveness { return Assess(p.evidence(), now) }

// LastAlive is the newest of the peer's evidence times as one wall-clock
// instant, or the zero time when there is none, for callers that keep a
// window of their own: now.Sub(p.LastAlive(now)) <= window. now is not read
// today; it is in the signature so the rule may depend on it without
// changing callers.
func (p *PeerInfo) LastAlive(now time.Time) time.Time { return p.evidence().newest() }

// LastContact is the newest of LastSeen and LastDataPlane as one wall-clock
// instant, or the zero time when there is none: measured contact only. It
// leaves out RestoredAt and the store's resume time, which are grace, so a
// peer never heard from does not look contacted at the moment it was restored.
func (p *PeerInfo) LastContact() time.Time { return time.Time{} }

// DecideByAge applies a reader's own age test (its window and operator) to the
// peer. While EnforceLiveness is false the legacy LastSeen age decides; once
// it is true the age of LastAlive does. disagree is true when the two ages give
// different decisions and the legacy one is the one acting, so a caller can
// count the case.
func (p *PeerInfo) DecideByAge(now time.Time, test func(age time.Duration) bool) (decision, disagree bool) {
	legacy := test(now.Sub(p.LastSeen))
	alive := test(now.Sub(p.LastAlive(now)))
	if EnforceLiveness.Load() {
		return alive, false
	}
	return legacy, legacy != alive
}

// stamped returns a copy of peer that carries the store-wide resumedAt; the
// caller holds ps.mu.
func (ps *PeerStore) stamped(peer *PeerInfo) *PeerInfo {
	c := *peer
	c.resumedAt = ps.resumedAt
	return &c
}

// assess is Assess of peer's evidence with the store-wide resumedAt; the
// caller holds ps.mu.
func (ps *PeerStore) assess(peer *PeerInfo, now time.Time) Liveness {
	e := peer.evidence()
	e.ResumedAt = ps.resumedAt
	return Assess(e, now)
}

// judge picks the answer of a reader: Assess's when EnforceLiveness is on,
// the legacy age test's otherwise. disagree is true when the legacy test
// drops a peer that Assess keeps and the legacy test is the one acting.
func judge(legacyDrop, assessDrop bool) (drop, disagree bool) {
	if EnforceLiveness.Load() {
		return assessDrop, false
	}
	return legacyDrop, legacyDrop && !assessDrop
}

// reportDisagreement calls hook n times with site; it is called outside ps.mu.
func reportDisagreement(hook func(site string), site string, n int) {
	if hook == nil {
		return
	}
	for i := 0; i < n; i++ {
		hook(site)
	}
}

// ObserveDataPlane records at as the newest data-plane evidence (a WireGuard
// handshake or a successful mesh probe) of pubKey. It is a local measurement,
// not a claim: a time never moves backwards (compared as wall-clock instants),
// a time after the store clock is taken as the store clock, no event is sent,
// no claim field and not LastSeen is touched, and an unknown key is ignored.
func (ps *PeerStore) ObserveDataPlane(pubKey string, at time.Time) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return
	}
	if now := ps.now(); at.Round(0).After(now.Round(0)) {
		at = now
	}
	if at.Round(0).After(peer.LastDataPlane.Round(0)) {
		peer.LastDataPlane = at
	}
}

// NoteResume records at as the time the node last woke; it is evidence for
// every peer. The time never moves backwards (compared as wall-clock
// instants) and a time after the store clock is taken as the store clock.
func (ps *PeerStore) NoteResume(at time.Time) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if now := ps.now(); at.Round(0).After(now.Round(0)) {
		at = now
	}
	if at.Round(0).After(ps.resumedAt.Round(0)) {
		ps.resumedAt = at
	}
}

// ResumedAt returns the time given to the last NoteResume, or the zero time.
func (ps *PeerStore) ResumedAt() time.Time {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.resumedAt
}

// GetProbeable returns the peers that are Alive or Suspect: the ones a probe
// may still reach. It always judges by Assess.
func (ps *PeerStore) GetProbeable() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]*PeerInfo, 0, len(ps.peers))
	now := ps.now()
	for _, peer := range ps.peers {
		if ps.assess(peer, now) != Dead {
			result = append(result, ps.stamped(peer))
		}
	}
	return result
}

// SetLivenessDisagreeHook sets the hook that hears, with the reader's site
// name (get-active, cleanup, is-dead), each case the legacy age test drops a
// peer that Assess would keep while EnforceLiveness is false. It is per store
// and may be nil.
func (ps *PeerStore) SetLivenessDisagreeHook(fn func(site string)) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.livenessDisagree = fn
}
