package daemon

import (
	"time"
)

const (
	// ResumeCheckInterval is how often the resume loop reads the wall clock.
	ResumeCheckInterval = 2 * time.Second
	// ResumeGapThreshold is the wall-clock gap between two checks above which
	// the machine is taken to have slept.
	ResumeGapThreshold = 10 * time.Second
	// ResumeGrace is how long after a wake the eviction and stale-handshake
	// decisions are skipped (when EnforceLiveness is on): handshake ages are
	// meaningless until WireGuard has had one rekey.
	ResumeGrace = 60 * time.Second
)

// DetectResume reports whether the wall-clock gap from prev to now is above
// threshold. The monotonic readings are stripped: they do not run while the
// machine sleeps, which is what the check looks for.
func DetectResume(prev, now time.Time, threshold time.Duration) bool {
	return false
}

// now is the daemon's clock: time.Now unless a test injected one, so a
// &Daemon{...} literal keeps working.
func (d *Daemon) now() time.Time {
	if d.clock != nil {
		return d.clock()
	}
	return time.Now()
}

// inResumeGrace reports whether now is within ResumeGrace of the store's last
// wake.
func (d *Daemon) inResumeGrace(now time.Time) bool {
	return false
}

// resumeGraceBlocks says whether a decision that would evict a peer or choose
// a relay from a handshake age must be skipped: the grace blocks only while
// EnforceLiveness is on; while it is off the case is counted.
func (d *Daemon) resumeGraceBlocks(now time.Time) bool {
	return false
}

// resumeLoop reads the clock on every tick and acts on a wake.
func (d *Daemon) resumeLoop(ticks <-chan time.Time) {
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticks:
		}
	}
}

// startResumeLoop starts the resume loop; both run paths call it.
func (d *Daemon) startResumeLoop() {
}
