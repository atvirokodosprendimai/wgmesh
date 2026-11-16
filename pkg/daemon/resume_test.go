package daemon

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// resumeDaemon is a livenessDaemon with the state the resume code touches and
// a context the test owns.
func resumeDaemon(t *testing.T) *Daemon {
	t.Helper()
	d := livenessDaemon(t)
	d.peerHealthFailures = make(map[string]int)
	d.lastPeerTransferTotal = make(map[string]uint64)
	d.resumeCh = make(chan struct{}, 1)
	d.ctx, d.cancel = context.WithCancel(context.Background())
	t.Cleanup(d.cancel)
	return d
}

// seqClock returns its times in order and then the last one forever; it is
// safe for several goroutines.
type seqClock struct {
	n     atomic.Int64
	times []time.Time
}

func (c *seqClock) now() time.Time {
	i := int(c.n.Add(1)) - 1
	if i >= len(c.times) {
		i = len(c.times) - 1
	}
	return c.times[i]
}

// resumerLayer is a discovery layer that counts OnResume calls.
type resumerLayer struct{ resumes atomic.Int32 }

func (r *resumerLayer) Start() error { return nil }
func (r *resumerLayer) Stop() error  { return nil }
func (r *resumerLayer) OnResume()    { r.resumes.Add(1) }

func sendTick(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Now():
	case <-time.After(5 * time.Second):
		t.Fatal("the resume loop did not take a tick in 5 seconds")
	}
}

// runResumeJump runs the loop on a clock that jumps one hour between its
// first reading and the first tick, lets it handle that tick and stops it.
func runResumeJump(t *testing.T, d *Daemon) {
	t.Helper()
	t0 := time.Now()
	clk := &seqClock{times: []time.Time{t0, t0.Add(time.Hour)}}
	d.clock = clk.now
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.resumeLoop(ticks)
	}()
	sendTick(t, ticks) // the jump
	sendTick(t, ticks) // taken only after the loop finished the first one
	d.cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the resume loop did not stop on cancel")
	}
}

func TestDetectResume(t *testing.T) {
	t.Parallel()
	prev := time.Now()
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"2 seconds", prev.Add(2 * time.Second), false},
		{"exactly the threshold", prev.Add(ResumeGapThreshold), false},
		{"11 seconds", prev.Add(11 * time.Second), true},
		{"clock stepped back", prev.Add(-30 * time.Second), false},
		{"8 hours by wall clock only", time.Unix(0, prev.UnixNano()+int64(8*time.Hour)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectResume(prev, tt.now, ResumeGapThreshold); got != tt.want {
				t.Errorf("DetectResume(prev, %v, %v) = %v, want %v", tt.now.Sub(prev), ResumeGapThreshold, got, tt.want)
			}
		})
	}
}

func TestResumeLoopCallsHooks(t *testing.T) {
	d := resumeDaemon(t)
	layer := &resumerLayer{}
	d.dhtDiscovery = layer
	before := testutil.ToFloat64(resumeDetected)

	runResumeJump(t, d)

	if got := testutil.ToFloat64(resumeDetected) - before; got != 1 {
		t.Errorf("wgmesh_resume_detected_total rose by %v, want 1", got)
	}
	if at := d.peerStore.ResumedAt(); at.IsZero() || at.Before(time.Now().Add(-time.Minute)) {
		t.Errorf("ResumedAt = %v after a wake, want about now", at)
	}
	if got := len(d.resumeCh); got != 1 {
		t.Errorf("%d signals on the reconcile channel, want 1", got)
	}
	if got := layer.resumes.Load(); got != 1 {
		t.Errorf("OnResume called %d times, want 1", got)
	}
}

func TestResumeLoopWithoutResumerOrChannel(t *testing.T) {
	d := resumeDaemon(t)
	d.resumeCh = nil
	d.dhtDiscovery = nil

	runResumeJump(t, d)

	if d.peerStore.ResumedAt().IsZero() {
		t.Error("a daemon without a Resumer or a signal channel ignored the wake")
	}
}

func TestBothRunPathsStartResumeLoop(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "daemon.go", nil, 0)
	if err != nil {
		t.Fatalf("parse daemon.go: %v", err)
	}
	starts := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || (fn.Name.Name != "Run" && fn.Name.Name != "RunWithDHTDiscovery") {
			continue
		}
		starts[fn.Name.Name] = false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "startResumeLoop" {
					starts[fn.Name.Name] = true
				}
			}
			return true
		})
	}
	for _, name := range []string{"Run", "RunWithDHTDiscovery"} {
		started, found := starts[name]
		if !found {
			t.Errorf("daemon.go has no func %s", name)
		} else if !started {
			t.Errorf("%s does not call startResumeLoop", name)
		}
	}
}

// graceDaemon is a daemon holding one peer, woken just now; its clock reads
// the wake time plus after, so the grace is on for a small after and over for
// a large one. The peer's control-plane time is a minute old.
func graceDaemon(t *testing.T, key string, after time.Duration) *Daemon {
	t.Helper()
	d := resumeDaemon(t)
	seedLivenessPeer(d, key, "10.250.0.2", time.Now().Add(-time.Minute))
	d.peerStore.NoteResume(time.Now())
	at := d.peerStore.ResumedAt().Add(after)
	d.clock = func() time.Time { return at }
	return d
}

// staleHealthCycle runs one health cycle for key with a handshake four
// minutes old and no transfer.
func staleHealthCycle(d *Daemon, key string) {
	hs := map[string]int64{key: time.Now().Add(-4 * time.Minute).Unix()}
	d.evaluatePeerHealth(hs, map[string]wireguard.PeerTransfer{})
}

// failingProbeCycle runs the probe loop for key, which is relay routed and
// one failure short of MeshProbeFailLimit, with a session that never pongs.
func failingProbeCycle(t *testing.T, d *Daemon, key string) {
	t.Helper()
	d.relayRoutes[key] = "relay-pubkey"
	d.probeFailures[key] = MeshProbeFailLimit - 1
	session, _ := pipeSession(t, "nope")
	d.probeSessions[key] = session
	d.probePeersOverMesh()
}

func peerStored(d *Daemon, key string) bool {
	_, ok := d.peerStore.Get(key)
	return ok
}

func TestResumeGraceBlocksHealthEviction(t *testing.T) {
	enforceLiveness(t)
	d := graceDaemon(t, "peer-grace", 10*time.Second)

	staleHealthCycle(d, "peer-grace")
	staleHealthCycle(d, "peer-grace")

	if !peerStored(d, "peer-grace") {
		t.Error("a stale handshake inside the wake grace evicted the peer on the second health cycle")
	}
}

func TestResumeGraceBlocksProbeEviction(t *testing.T) {
	enforceLiveness(t)
	d := graceDaemon(t, "peer-grace", 10*time.Second)

	failingProbeCycle(t, d, "peer-grace")

	if !peerStored(d, "peer-grace") {
		t.Errorf("%d failed probes inside the wake grace evicted the peer", MeshProbeFailLimit)
	}
}

func TestResumeGraceExpires(t *testing.T) {
	runs := []struct {
		name string
		run  func(t *testing.T, d *Daemon)
	}{
		{"health", func(t *testing.T, d *Daemon) {
			staleHealthCycle(d, "peer-grace")
			staleHealthCycle(d, "peer-grace")
		}},
		{"probe", func(t *testing.T, d *Daemon) { failingProbeCycle(t, d, "peer-grace") }},
	}
	afters := []struct {
		name  string
		after time.Duration
	}{
		{"one second after the grace", ResumeGrace + time.Second},
		{"exactly the grace", ResumeGrace},
	}
	for _, r := range runs {
		for _, a := range afters {
			t.Run(r.name+"/"+a.name, func(t *testing.T) {
				enforceLiveness(t)
				d := graceDaemon(t, "peer-grace", a.after)

				r.run(t, d)

				if peerStored(d, "peer-grace") {
					t.Error("the same inputs outside the grace did not evict the peer")
				}
			})
		}
	}
}

func TestInResumeGraceBounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		after time.Duration
		want  bool
	}{
		{"at the wake", 0, true},
		{"just inside", ResumeGrace - time.Nanosecond, true},
		{"exactly the grace", ResumeGrace, false},
		{"before the wake", -time.Second, false},
		{"an hour before the wake", -time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := graceDaemon(t, "peer-grace", 0)
			at := d.peerStore.ResumedAt()
			if got := d.inResumeGrace(at.Add(tt.after)); got != tt.want {
				t.Errorf("inResumeGrace(wake%+v) = %v, want %v", tt.after, got, tt.want)
			}
		})
	}
}

func TestNoGraceWhileLivenessNotEnforced(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, d *Daemon)
	}{
		{"health", func(t *testing.T, d *Daemon) {
			staleHealthCycle(d, "peer-grace")
			staleHealthCycle(d, "peer-grace")
		}},
		{"probe", func(t *testing.T, d *Daemon) { failingProbeCycle(t, d, "peer-grace") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := graceDaemon(t, "peer-grace", 10*time.Second)
			before := disagreeCount(SiteResumeGrace)

			tt.run(t, d)

			if peerStored(d, "peer-grace") {
				t.Error("with the switch off the grace blocked the eviction")
			}
			if got := disagreeCount(SiteResumeGrace) - before; got < 1 {
				t.Errorf("resume-grace disagreements rose by %v, want at least 1", got)
			}
		})
	}
}

func TestResumeGraceStaleHandshakeKeepsRelayDecision(t *testing.T) {
	tests := []struct {
		name        string
		enforce     bool
		after       time.Duration
		wantRelayed bool
		wantCounted float64
	}{
		{"on, inside the grace: no relay from a handshake age", true, 10 * time.Second, false, 0},
		{"on, after the grace: the stale handshake relays", true, ResumeGrace + time.Second, true, 0},
		{"off, inside the grace: relays as before and is counted", false, 10 * time.Second, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enforce {
				enforceLiveness(t)
			}
			d := graceDaemon(t, "peer-grace", tt.after)
			peer, _ := d.peerStore.Get("peer-grace")
			peer.Endpoint = "172.20.2.20:51820"
			relay := &PeerInfo{WGPubKey: "relay-pubkey", MeshIP: "10.250.0.1", Introducer: true, LastSeen: time.Now()}
			hs := map[string]int64{"peer-grace": time.Now().Add(-4 * time.Minute).Unix()}
			before := disagreeCount(SiteResumeGrace)

			got := d.shouldRelayPeerWithSubnets(peer, []*PeerInfo{relay}, hs, nil)

			if got != tt.wantRelayed {
				t.Errorf("shouldRelayPeerWithSubnets = %v, want %v", got, tt.wantRelayed)
			}
			if c := disagreeCount(SiteResumeGrace) - before; c != tt.wantCounted {
				t.Errorf("resume-grace disagreements rose by %v, want %v", c, tt.wantCounted)
			}
		})
	}
}

func TestResumeKeepsAllPeersAlive(t *testing.T) {
	enforceLiveness(t)
	d := resumeDaemon(t)
	old := time.Now().Add(-5 * time.Hour)
	for i, key := range []string{"peer-a", "peer-b", "peer-c"} {
		seedLivenessPeer(d, key, "10.250.0."+string(rune('2'+i)), old)
		d.peerStore.ObserveDataPlane(key, old)
	}
	if got := len(d.peerStore.GetActive()); got != 0 {
		t.Fatalf("%d peers Alive before the wake, want 0", got)
	}

	runResumeJump(t, d)

	got := d.peerStore.GetActive()
	if len(got) != 3 {
		t.Fatalf("%d peers Alive after the wake, want 3", len(got))
	}
	for _, p := range got {
		if v := p.Alive(time.Now()); v != node.Alive {
			t.Errorf("%s is %v after the wake, want Alive", p.WGPubKey, v)
		}
	}
}

// TestResumeLoopStateIsRaceFree runs the loop on a jumping clock while the
// health cycle, the grace check, the reconcile signal receiver and NoteResume
// run in other goroutines; it is meaningful under -race.
func TestResumeLoopStateIsRaceFree(t *testing.T) {
	enforceLiveness(t)
	d := graceDaemon(t, "peer-race", 0)
	d.dhtDiscovery = &resumerLayer{}
	base := time.Now()
	var n atomic.Int64
	d.clock = func() time.Time { return base.Add(time.Duration(n.Add(1)) * time.Hour) }

	ticks := make(chan time.Time)
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		d.resumeLoop(ticks)
	}()

	var wg sync.WaitGroup
	run := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				f()
			}
		}()
	}
	run(func() { staleHealthCycle(d, "peer-race") })
	run(func() { d.inResumeGrace(d.now()) })
	run(func() {
		select {
		case <-d.resumeCh:
		default:
		}
	})
	run(func() { d.peerStore.NoteResume(time.Now()) })
	for i := 0; i < 100; i++ {
		sendTick(t, ticks)
	}
	wg.Wait()
	d.cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the resume loop did not stop on cancel")
	}
}

// TestOnlyReconcileLoopCallsReconcile pins the invariant that reconcile has
// one caller at a time: the resume code asks reconcileLoop for a pass through
// resumeCh and never calls reconcile itself. The one other caller is the
// SIGHUP handler, which predates the resume loop.
func TestOnlyReconcileLoopCallsReconcile(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"reconcileLoop": true, "handleSIGHUP": true}
	called := map[string]bool{}
	parsed := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "reconcile" {
						called[fn.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	if parsed == 0 || !called["reconcileLoop"] {
		t.Fatalf("parsed %d files and found reconcileLoop calling reconcile = %v: the walk sees nothing", parsed, called["reconcileLoop"])
	}
	for fn := range called {
		if !allowed[fn] {
			t.Errorf("%s calls reconcile: only reconcileLoop (and handleSIGHUP) may", fn)
		}
	}
}
