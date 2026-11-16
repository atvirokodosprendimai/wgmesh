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

// ObserveDataPlane is a stub.
func (ps *PeerStore) ObserveDataPlane(pubKey string, at time.Time) {}

// NoteResume is a stub.
func (ps *PeerStore) NoteResume(at time.Time) {}

// ResumedAt is a stub.
func (ps *PeerStore) ResumedAt() time.Time { return time.Time{} }

// GetProbeable is a stub.
func (ps *PeerStore) GetProbeable() []*PeerInfo { return nil }

// SetLivenessDisagreeHook is a stub.
func (ps *PeerStore) SetLivenessDisagreeHook(fn func(site string)) {}
