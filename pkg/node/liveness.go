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

// Assess is a stub.
func Assess(e Evidence, now time.Time) Liveness { return Alive }

// Alive is a stub.
func (p *PeerInfo) Alive(now time.Time) Liveness { return Alive }

// LastAlive is a stub.
func (p *PeerInfo) LastAlive(now time.Time) time.Time { return time.Time{} }

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
