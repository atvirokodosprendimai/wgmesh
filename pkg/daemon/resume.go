package daemon

import (
	"log"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
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
	return now.Round(0).Sub(prev.Round(0)) > threshold
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
	if d.peerStore == nil {
		return false
	}
	at := d.peerStore.ResumedAt()
	if at.IsZero() {
		return false
	}
	since := now.Round(0).Sub(at.Round(0))
	return since >= 0 && since < ResumeGrace
}

// resumeGraceBlocks says whether a decision that would evict a peer or choose
// a relay from a handshake age must be skipped: the grace blocks only while
// EnforceLiveness is on; while it is off the case is counted.
func (d *Daemon) resumeGraceBlocks(now time.Time) bool {
	if !d.inResumeGrace(now) {
		return false
	}
	if node.EnforceLiveness.Load() {
		return true
	}
	recordLivenessDisagree(SiteResumeGrace)
	return false
}

// resumeLoop reads the clock on every tick and acts on a wake. Its only state
// is the previous reading, a local; everything it shares is behind a lock, an
// atomic or a channel.
func (d *Daemon) resumeLoop(ticks <-chan time.Time) {
	prev := d.now()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticks:
			now := d.now()
			if DetectResume(prev, now, ResumeGapThreshold) {
				d.handleResume(now, now.Round(0).Sub(prev.Round(0)))
			}
			prev = now
		}
	}
}

// handleResume acts on a detected wake: the store takes the wake as evidence
// for every peer, reconcileLoop is asked for a pass and the discovery layer, if
// it is a node.Resumer, runs its own wake actions. It never calls reconcile:
// that has one caller at a time, reconcileLoop. The daemon does not import
// discovery, so the layer is reached through the interface.
func (d *Daemon) handleResume(now time.Time, gap time.Duration) {
	log.Printf("[Resume] wall clock jumped %s", gap.Round(time.Second))
	resumeDetected.Inc()
	d.peerStore.NoteResume(now)
	select {
	case d.resumeCh <- struct{}{}:
	default: // a pass is already requested
	}
	if r, ok := d.dhtDiscovery.(node.Resumer); ok {
		r.OnResume()
	}
}

// startResumeLoop starts the resume loop; both run paths call it.
func (d *Daemon) startResumeLoop() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(ResumeCheckInterval)
		defer ticker.Stop()
		d.resumeLoop(ticker.C)
	}()
}
