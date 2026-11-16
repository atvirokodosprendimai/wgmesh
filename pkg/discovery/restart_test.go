package discovery

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// oneLoopGroup is the WaitGroup a test passes to a hand-started listenLoop,
// which calls Done once.
func oneLoopGroup() *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return wg
}

// ADR-0005 T10: PeerExchange and DHTDiscovery restart after Stop.
//
// Words: after Stop then Start the exchange has exactly one listen loop and
// the DHT layer's loops see a live context that still ends with the parent.
// Why: Start replaced stopCh and conn while the old loop read them (a data
// race, and the old loop then never ended), and Stop cancelled d.ctx for good.
// Invariants: Stop waits for its loop; running is set only after the
// sub-layers started; a startup error leaves running false and the started
// layers stopped.

func newRestartExchange(t *testing.T, secret string) *PeerExchange {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: secret})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return NewPeerExchange(cfg, &daemon.LocalNode{WGPubKey: "a"}, daemon.NewPeerStore())
}

func newRestartDHT(t *testing.T, parent context.Context, secret string, opts daemon.DaemonOpts) *DHTDiscovery {
	t.Helper()
	opts.Secret = secret
	cfg, err := daemon.NewConfig(opts)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	d, err := NewDHTDiscovery(parent, cfg, &daemon.LocalNode{WGPubKey: "a"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery: %v", err)
	}
	return d
}

// waitLoops polls pe.loops until it equals want or 3 s pass, and returns the
// last value read.
func waitLoops(pe *PeerExchange, want int32) int32 {
	deadline := time.Now().Add(3 * time.Second)
	for pe.loops.Load() != want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return pe.loops.Load()
}

func TestPeerExchangeRestartKeepsOneLoop(t *testing.T) {
	pe := newRestartExchange(t, "wgmesh-test-restart-one-loop-1")
	if err := pe.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	pe.Stop()
	if err := pe.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	t.Cleanup(pe.Stop)

	if got := waitLoops(pe, 1); got != 1 {
		t.Fatalf("live listen loops after Start, Stop, Start = %d, want 1", got)
	}
}

func TestPeerExchangeStopEndsTheLoop(t *testing.T) {
	pe := newRestartExchange(t, "wgmesh-test-restart-ends-loop-1")
	if err := pe.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	pe.Stop()
	if err := pe.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	pe.Stop()

	if got := waitLoops(pe, 0); got != 0 {
		t.Fatalf("live listen loops after the second Stop = %d, want 0", got)
	}
	pe.Stop() // idempotent
}

func TestPeerExchangeStopWaitsForLoop(t *testing.T) {
	pe := newRestartExchange(t, "wgmesh-test-restart-waits-1")
	for i := 0; i < 20; i++ {
		if err := pe.Start(); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		pe.Stop()
		// No polling: Stop must have waited for the loop.
		if got := pe.loops.Load(); got != 0 {
			t.Fatalf("iteration %d: loops = %d right after Stop, want 0", i, got)
		}
	}
}

func TestDHTBeginRunGivesLiveContext(t *testing.T) {
	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-beginrun-1", daemon.DaemonOpts{})
	if err := d.beginRun(); err != nil {
		t.Fatalf("first beginRun: %v", err)
	}
	d.setRunning()
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := d.beginRun(); err != nil {
		t.Fatalf("beginRun after Stop: %v", err)
	}
	if err := d.runCtx().Err(); err != nil {
		t.Fatalf("context after a restart is already done: %v", err)
	}
	d.setRunning()
	if err := d.beginRun(); err == nil {
		t.Fatal("second beginRun while running returned nil, want the already-running error")
	}
}

func TestDHTRestartContextFollowsParent(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	d := newRestartDHT(t, parent, "wgmesh-test-restart-parent-1", daemon.DaemonOpts{})

	if err := d.beginRun(); err != nil {
		t.Fatalf("first beginRun: %v", err)
	}
	d.setRunning()
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := d.beginRun(); err != nil {
		t.Fatalf("beginRun after Stop: %v", err)
	}
	d.setRunning()

	ctx := d.runCtx()
	if err := ctx.Err(); err != nil {
		t.Fatalf("restarted context is already done: %v", err)
	}
	cancelParent()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelling the parent did not end the restarted context")
	}
}

// TestDHTStartFailureLeavesNotRunning makes the DHT server fail without a
// network: an empty bootstrap list gives "no bootstrap nodes resolved" in
// initDHTServer, after the exchange, gossip and LAN layers already started.
// STUN is emptied so Start does not touch the network. Package variables are
// set by this non-parallel test only and restored on cleanup.
func TestDHTStartFailureLeavesNotRunning(t *testing.T) {
	oldNodes, oldSTUN := DHTBootstrapNodes, DefaultSTUNServers
	DHTBootstrapNodes, DefaultSTUNServers = nil, nil
	t.Cleanup(func() { DHTBootstrapNodes, DefaultSTUNServers = oldNodes, oldSTUN })

	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-failure-1",
		daemon.DaemonOpts{Gossip: true, DisableIPv6: true})
	if err := d.Start(); err == nil {
		_ = d.Stop()
		t.Fatal("Start succeeded, want the DHT server error")
	}

	d.mu.RLock()
	running, lan := d.running, d.lan
	d.mu.RUnlock()
	if running {
		t.Error("running is true after a failed Start")
	}
	if lan != nil {
		t.Error("LAN layer still held after a failed Start (leaked)")
	}
	if d.exchange.isRunning() {
		t.Error("exchange still running after a failed Start")
	}
}

// hermeticDHT makes initDHTServer succeed without the internet: one loopback
// bootstrap node nobody listens on, and no STUN servers. Package variables are
// set by non-parallel tests only and restored on cleanup.
func hermeticDHT(t *testing.T) {
	t.Helper()
	oldNodes, oldSTUN := DHTBootstrapNodes, DefaultSTUNServers
	DHTBootstrapNodes, DefaultSTUNServers = []string{"127.0.0.1:1"}, nil
	t.Cleanup(func() { DHTBootstrapNodes, DefaultSTUNServers = oldNodes, oldSTUN })
}

// TestDHTStopStartWithLiveLoops runs Stop and Start at the same time while
// the loops of the first run are live. Words: server, dhtPort, lan and gossip
// are written by Start and read by the loops and Stop, so every access goes
// through d.mu. Why: a restart while an old loop winds down raced on them.
// Invariant: -race stays silent and the layer ends stopped. It is a -race
// test: without -race it only checks that nothing hangs.
func TestDHTStopStartWithLiveLoops(t *testing.T) {
	hermeticDHT(t)
	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-race-1",
		daemon.DaemonOpts{Gossip: true, DisableIPv6: true})
	t.Cleanup(func() { _ = d.Stop() })

	for i := 0; i < 3; i++ {
		if err := d.Start(); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		time.Sleep(100 * time.Millisecond) // let the loops run their first pass

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Stop()
		}()
		// Start is refused until Stop released the run, then restarts at once,
		// while the loops of the stopped run are still winding down.
		deadline := time.Now().Add(5 * time.Second)
		for d.Start() != nil {
			if time.Now().After(deadline) {
				t.Fatalf("iteration %d: Start still refused 5 s after Stop", i)
			}
			time.Sleep(time.Millisecond)
		}
		wg.Wait()
		_ = d.Stop()
	}
}

// TestDHTStopWaitsForLoops: Stop returns only after the DHT loops ended, so a
// restart never overlaps an old loop. The loops counter is the seam: it is
// above 0 while the layer runs and exactly 0 right after Stop, no polling.
func TestDHTStopWaitsForLoops(t *testing.T) {
	hermeticDHT(t)
	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-loops-1",
		daemon.DaemonOpts{Gossip: true, DisableIPv6: true})
	t.Cleanup(func() { _ = d.Stop() })

	for i := 0; i < 3; i++ {
		if err := d.Start(); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		if got := d.loops.Load(); got < 1 {
			t.Fatalf("iteration %d: loops = %d while running, want at least 1", i, got)
		}
		if err := d.Stop(); err != nil {
			t.Fatalf("iteration %d: Stop: %v", i, err)
		}
		if got := d.loops.Load(); got != 0 {
			t.Fatalf("iteration %d: loops = %d right after Stop, want 0", i, got)
		}
	}
}

// TestPeerExchangeWithPeerDuringRestart runs ExchangeWithPeer while the
// exchange is stopped and started again. Words: port is written by Start under
// pe.mu, so every read (also the log lines of ExchangeWithPeer) goes through
// pe.Port. It is a -race test; without -race it only checks nothing hangs.
func TestPeerExchangeWithPeerDuringRestart(t *testing.T) {
	pe := newRestartExchange(t, "wgmesh-test-restart-port-race-1")
	t.Cleanup(pe.Stop)
	if err := pe.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	pe.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = pe.ExchangeWithPeer("127.0.0.1:1") // ends fast on the closed connection, or at ExchangeTimeout
	}()
	if err := pe.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	pe.Stop()
	<-done
}

// dhtRunning reads d.running under d.mu.
func dhtRunning(d *DHTDiscovery) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// TestDHTStopDuringStartStopsTheRun: a Stop issued while a Start is in the
// middle waits for that Start and then stops its run.
// Why: Stop returned at once (running was still false) and the run then went
// live with nobody left to stop it. Invariant: after both calls return,
// nothing runs. The seam startHook holds Start between beginRun and
// setRunning; Stop must not return while it is held (a 200 ms window to see it
// return early; it never fires when Stop waits).
func TestDHTStopDuringStartStopsTheRun(t *testing.T) {
	hermeticDHT(t)
	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-stopduring-1",
		daemon.DaemonOpts{Gossip: true, DisableIPv6: true})
	t.Cleanup(func() { _ = d.Stop() })

	reached := make(chan struct{})
	release := make(chan struct{})
	startHook = func() {
		close(reached)
		<-release
	}
	t.Cleanup(func() { startHook = nil })

	startErr := make(chan error, 1)
	go func() { startErr <- d.Start() }()
	<-reached

	stopErr := make(chan error, 1)
	go func() { stopErr <- d.Stop() }()
	stoppedEarly := false
	select {
	case <-stopErr:
		stoppedEarly = true
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-startErr; err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !stoppedEarly {
		if err := <-stopErr; err != nil {
			t.Fatalf("Stop: %v", err)
		}
	}
	if stoppedEarly {
		t.Error("Stop returned while a Start was still in progress")
	}
	if dhtRunning(d) {
		t.Error("DHT layer still running after Stop issued during Start")
	}
	if d.exchange.isRunning() {
		t.Error("exchange still running after Stop issued during Start")
	}
}

// TestDHTStopThenStartRace runs `go Stop(); Start()` on a running layer 20
// times. Words: Stop and Start are serialized by runMu, so one wins and the
// other follows whole. Invariant: when Start returned nil it ran after Stop
// and everything runs; when it returned "already running" Stop came last and
// nothing runs; the exchange always agrees with the DHT layer. It is a -race
// test.
func TestDHTStopThenStartRace(t *testing.T) {
	hermeticDHT(t)
	d := newRestartDHT(t, context.Background(), "wgmesh-test-restart-stopstart-1",
		daemon.DaemonOpts{Gossip: true, DisableIPv6: true})
	t.Cleanup(func() { _ = d.Stop() })

	if err := d.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	for i := 0; i < 20; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Stop()
		}()
		err := d.Start()
		wg.Wait()
		if err != nil && !strings.Contains(err.Error(), "already running") {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		want := err == nil
		if got := dhtRunning(d); got != want {
			t.Fatalf("iteration %d: running = %v, want %v (Start err %v)", i, got, want, err)
		}
		if got := d.exchange.isRunning(); got != want {
			t.Fatalf("iteration %d: exchange running = %v, want %v", i, got, want)
		}
		if !want {
			if err := d.Start(); err != nil {
				t.Fatalf("iteration %d: restart: %v", i, err)
			}
		}
	}
}
