package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

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
