package daemon

import (
	"errors"
	"go/ast"
	"strings"
	"sync"
	"testing"
	"time"
)

// ADR-0003 T8: a graceful exit undoes only what this run did to forwarding.

var (
	wantDelete  = "iptables -D " + strings.Join(forwardRuleArgs("wgmesh-t3-none"), " ")
	wantRestore = "sysctl -w net.ipv4.ip_forward=0"
)

// enforceRevert sets the switch for one test and restores it.
func enforceRevert(t *testing.T, v bool) {
	t.Helper()
	old := EnforceRevertOnExit.Load()
	EnforceRevertOnExit.Store(v)
	t.Cleanup(func() { EnforceRevertOnExit.Store(old) })
}

// A run that found both absent and wrote both removes both at exit, the rule
// first; a second call at the same exit issues nothing.
func TestShutdownRemovesRuleWeAdded(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 2)

	d.revertKernel()
	if got, want := h.writes(), []string{wantSysctl, wantRule, wantDelete, wantRestore}; !sameLines(got, want) {
		t.Fatalf("writes = %q, want %q", got, want)
	}
	if h.ruleInstalled || h.ipForward {
		t.Errorf("host ends with rule=%v ip_forward=%v, want both undone", h.ruleInstalled, h.ipForward)
	}
	d.revertKernel()
	if got := len(h.writes()); got != 4 {
		t.Errorf("a second revert wrote again: %q", h.writes())
	}
}

// A rule `-C` found at the first tick is not ours, also when someone deletes it
// later and the next tick appends it again.
func TestShutdownKeepsPreexistingRule(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true, ruleInstalled: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 1)
	h.set(func(h *fakeHost) { h.ruleInstalled = false })
	tick(t, d, 1)
	if !h.ruleInstalled {
		t.Fatal("setup: the drift tick did not append the rule again")
	}

	d.revertKernel()
	if n := h.count("iptables -D"); n != 0 {
		t.Errorf("a rule this run found was deleted at exit (%d iptables -D)", n)
	}
	if !h.ruleInstalled {
		t.Error("the rule is gone after exit")
	}
}

// ip_forward is restored only when this run read 0 and wrote 1; a value found
// as 1 stays, also when someone sets 0 later and a tick writes 1 again.
func TestShutdownRestoresIPForwardOnlyIfWeChangedIt(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)

	t.Run("read 0 and written 1", func(t *testing.T) {
		h := &fakeHost{iptablesPresent: true, ruleInstalled: true}
		d := newForwardDaemon(t, h, &logSink{})
		tick(t, d, 2)
		d.revertKernel()
		if got, want := h.count(wantRestore), 1; got != want {
			t.Errorf("%q ran %d times, want %d", wantRestore, got, want)
		}
		if h.ipForward {
			t.Error("ip_forward is still 1 after exit")
		}
	})
	t.Run("read 1", func(t *testing.T) {
		h := &fakeHost{iptablesPresent: true, ruleInstalled: true, ipForward: true}
		d := newForwardDaemon(t, h, &logSink{})
		tick(t, d, 2)
		d.revertKernel()
		if n := h.count("sysctl -w"); n != 0 {
			t.Errorf("ip_forward was 1 at start, yet sysctl ran %d times", n)
		}
		if !h.ipForward {
			t.Error("ip_forward was set to 0")
		}
	})
	t.Run("read 1, drifted to 0, written 1 again", func(t *testing.T) {
		h := &fakeHost{iptablesPresent: true, ruleInstalled: true, ipForward: true}
		d := newForwardDaemon(t, h, &logSink{})
		tick(t, d, 1)
		h.set(func(h *fakeHost) { h.ipForward = false })
		tick(t, d, 1)
		if got := h.count(wantSysctl); got != 1 {
			t.Fatalf("setup: the drift tick wrote ip_forward %d times, want 1", got)
		}
		d.revertKernel()
		if n := h.count(wantRestore); n != 0 {
			t.Errorf("a value this run found as 1 was restored to 0 (%d times)", n)
		}
	})
}

// At release N the exit logs one line and issues no command.
func TestShutdownRevertIsWarnOnlyAtN(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, false)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	tick(t, d, 2)
	before := len(h.writes())

	d.revertKernel()
	d.revertKernel()
	if got := len(h.writes()); got != before {
		t.Errorf("exit at N wrote: %q", h.writes()[before:])
	}
	if !h.ruleInstalled || !h.ipForward {
		t.Errorf("host ends with rule=%v ip_forward=%v, want both left", h.ruleInstalled, h.ipForward)
	}
	if got := ls.count("revert-on-exit"); got != 1 {
		t.Errorf("revert-on-exit logged %d times, want 1; lines %q", got, ls.lines)
	}
	if got := ls.count("EnforceRevertOnExit"); got != 1 {
		t.Errorf("the line does not name EnforceRevertOnExit: %q", ls.lines)
	}
}

// A rule we added and someone deleted is appended again by a drift tick; the
// record of the first write stands, so exit still deletes it.
func TestShutdownRecordSurvivesDriftReapply(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 1)
	h.set(func(h *fakeHost) { h.ruleInstalled = false; h.ipForward = false })
	tick(t, d, 1)
	if got := h.count("iptables -A"); got != 2 {
		t.Fatalf("setup: the rule was appended %d times, want 2", got)
	}

	d.revertKernel()
	if h.ruleInstalled || h.ipForward {
		t.Errorf("host ends with rule=%v ip_forward=%v, want both undone", h.ruleInstalled, h.ipForward)
	}
	if h.count(wantDelete) != 1 || h.count(wantRestore) != 1 {
		t.Errorf("writes = %q, want one delete and one restore", h.writes())
	}
}

// A node whose forwarding was never wanted has no record: exit issues nothing
// and says nothing.
func TestRevertDoesNotRunWhenNoForwardingWanted(t *testing.T) {
	enforceForwardScope(t, true)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	tick(t, d, 3)

	d.revertKernel()
	if got := len(h.writes()); got != 0 {
		t.Errorf("writes = %q, want none", h.writes())
	}
	if got := ls.count("revert-on-exit"); got != 0 {
		t.Errorf("a node with nothing to undo logged %q", ls.lines)
	}
}

// revertKernel takes convergeMu: while a tick holds it inside a kernel write,
// the exit does not run. No sleep decides the outcome: the write is held on a
// channel inside the fake host, the exit gets a bounded wait to (wrongly)
// return, then the write is released.
func TestRevertKernelWaitsForConvergeLock(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 1)
	gate := &execGate{entered: make(chan struct{}), release: make(chan struct{})}
	h.set(func(h *fakeHost) { h.ipForward = false; h.gate = gate })

	tickDone := make(chan struct{})
	go func() { d.converge(nil); close(tickDone) }()
	<-gate.entered // the tick holds convergeMu, inside its ip_forward write

	revertDone := make(chan struct{})
	go func() { d.revertKernel(); close(revertDone) }()
	select {
	case <-revertDone:
		t.Fatal("revertKernel returned while a tick held convergeMu inside a kernel write")
	case <-time.After(50 * time.Millisecond):
	}
	close(gate.release)
	<-tickDone
	<-revertDone
	if h.ipForward || h.ruleInstalled {
		t.Errorf("host ends with rule=%v ip_forward=%v, want the exit to have run after the tick and undone both", h.ruleInstalled, h.ipForward)
	}
}

// revertKernel is race-free against ticks and a health path that write under
// convergeMu (run with -race), and the host ends where one more tick puts it.
func TestRevertKernelRaceFree(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			d.converge(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = d.removePeerFromKernel("no-such-peer")
		}
	}()
	for i := 0; i < 50; i++ {
		d.revertKernel()
	}
	wg.Wait()

	tick(t, d, 1)
	if !h.ipForward || !h.ruleInstalled {
		t.Errorf("after a final tick the host has rule=%v ip_forward=%v, want both forwarding", h.ruleInstalled, h.ipForward)
	}
	for _, w := range h.writes() {
		switch w {
		case wantSysctl, wantRule, wantDelete, wantRestore:
		default:
			t.Errorf("unexpected write %q", w)
		}
	}
}

// A node with nothing to undo says nothing at release N: no record because
// forwarding was never wanted, or because both items were found in place.
func TestRevertSilentAtNWithNothingToUndo(t *testing.T) {
	enforceRevert(t, false)
	t.Run("forwarding not wanted", func(t *testing.T) {
		enforceForwardScope(t, true)
		h := &fakeHost{iptablesPresent: true}
		ls := &logSink{}
		d := newForwardDaemon(t, h, ls)
		tick(t, d, 3)
		d.revertKernel()
		if got := ls.count("revert-on-exit"); got != 0 || len(h.writes()) != 0 {
			t.Errorf("revert-on-exit lines %d, writes %q; want none", got, h.writes())
		}
	})
	t.Run("both found in place", func(t *testing.T) {
		enforceForwardScope(t, false)
		h := &fakeHost{iptablesPresent: true, ipForward: true, ruleInstalled: true}
		ls := &logSink{}
		d := newForwardDaemon(t, h, ls)
		tick(t, d, 3)
		d.revertKernel()
		if got := ls.count("revert-on-exit"); got != 0 || len(h.writes()) != 0 {
			t.Errorf("revert-on-exit lines %d, writes %q; want none", got, h.writes())
		}
	})
}

// A write that failed does not make the item ours: only a later success does,
// and only that one is undone. A write that fails on every tick is never undone.
func TestRevertOwnershipFollowsSuccessfulWrite(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	cases := []struct {
		name      string
		fail      func(h *fakeHost, on bool)
		undo      string
		wantAfter int // undo commands after a failing tick then a succeeding one
	}{
		{"sysctl", func(h *fakeHost, on bool) {
			if on {
				h.sysctlErr = errors.New("denied")
			} else {
				h.sysctlErr = nil
			}
		}, wantRestore, 1},
		{"rule", func(h *fakeHost, on bool) {
			if on {
				h.appendErr = errors.New("denied")
			} else {
				h.appendErr = nil
			}
		}, wantDelete, 1},
	}
	for _, c := range cases {
		t.Run(c.name+" fails once, then succeeds", func(t *testing.T) {
			h := &fakeHost{iptablesPresent: true}
			d := newForwardDaemon(t, h, &logSink{})
			h.set(func(h *fakeHost) { c.fail(h, true) })
			tick(t, d, 1)
			h.set(func(h *fakeHost) { c.fail(h, false) })
			tick(t, d, 1)
			d.revertKernel()
			if got := h.count(c.undo); got != c.wantAfter {
				t.Errorf("%q ran %d times, want %d; writes %q", c.undo, got, c.wantAfter, h.writes())
			}
		})
		t.Run(c.name+" fails on every tick", func(t *testing.T) {
			h := &fakeHost{iptablesPresent: true}
			d := newForwardDaemon(t, h, &logSink{})
			h.set(func(h *fakeHost) { c.fail(h, true) })
			tick(t, d, 3)
			d.revertKernel()
			if got := h.count(c.undo); got != 0 {
				t.Errorf("%q ran %d times for an item this run never wrote; writes %q", c.undo, got, h.writes())
			}
		})
	}
}

// A failed -D or restore is a logged result; the records are cleared, so a
// second call does not retry it.
func TestRevertFailedUndoIsLoggedAndNotRetried(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	tick(t, d, 1)
	h.set(func(h *fakeHost) { h.deleteErr = errors.New("denied"); h.sysctlErr = errors.New("denied") })

	d.revertKernel()
	d.revertKernel()
	if h.count(wantDelete) != 1 || h.count(wantRestore) != 1 {
		t.Errorf("-D ran %d times and the restore %d times over two exits, want 1 and 1", h.count(wantDelete), h.count(wantRestore))
	}
	if ls.count("Failed to delete forward FORWARD") != 1 || ls.count("Failed to restore forward ip_forward") != 1 {
		t.Errorf("failures not logged once each: %q", ls.lines)
	}
}

// A host without iptables: the rule is unknown, only ip_forward is ours.
func TestRevertWithoutIptablesRestoresOnlyIPForward(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 2)
	d.revertKernel()
	if got, want := h.writes(), []string{wantSysctl, wantRestore}; !sameLines(got, want) {
		t.Errorf("writes = %q, want %q", got, want)
	}
}

// Both entry points stop the goroutines and then revert, in that order; the
// deferred teardown of the interface runs after. Deleting the call, or moving
// it before the wait, fails here.
func TestRunRevertsKernelAfterWait(t *testing.T) {
	files := parseProduction(t)
	f, ok := files["daemon.go"]
	if !ok {
		t.Fatal("daemon.go does not exist")
	}
	for _, name := range []string{"Run", "RunWithDHTDiscovery"} {
		var body *ast.BlockStmt
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == name {
				body = fn.Body
			}
		}
		if body == nil {
			t.Fatalf("%s not found in daemon.go", name)
		}
		wait, revert := -1, -1
		for i, st := range body.List {
			if _, deferred := st.(*ast.DeferStmt); deferred {
				continue
			}
			es, ok := st.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			switch sel.Sel.Name {
			case "Wait":
				if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "wg" {
					wait = i
				}
			case "revertKernel":
				revert = i
			}
		}
		if wait < 0 || revert < 0 || revert < wait {
			t.Errorf("%s: d.wg.Wait() at statement %d, d.revertKernel() at %d; the revert must be a plain call after the wait", name, wait, revert)
		}
	}
}

// Before it restores ip_forward the exit reads it again and restores only a
// value that still reads 1; a read error or another value is skipped and
// logged once. The rule is deleted either way.
func TestRevertRereadsIPForwardBeforeRestore(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	cases := []struct {
		name string
		do   func(h *fakeHost)
	}{
		{"reads 0 now", func(h *fakeHost) { h.ipForward = false }},
		{"read error", func(h *fakeHost) { h.readErr = errors.New("read denied") }},
		{"reads another value", func(h *fakeHost) { v := "2\n"; h.procValue = &v }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &fakeHost{iptablesPresent: true}
			ls := &logSink{}
			d := newForwardDaemon(t, h, ls)
			tick(t, d, 2)
			h.set(c.do)

			d.revertKernel()
			if n := h.count(wantRestore); n != 0 {
				t.Errorf("%q ran %d times, want 0 when the re-read is not 1", wantRestore, n)
			}
			if n := h.count(wantDelete); n != 1 {
				t.Errorf("the rule was deleted %d times, want 1", n)
			}
			if got := ls.count("revert-on-exit: ip_forward is not restored"); got != 1 {
				t.Errorf("skip logged %d times, want 1; lines %q", got, ls.lines)
			}
		})
	}
}

// Two daemons (wg0, wg1) share the global ip_forward. A wrote 1 (ours), B read
// 1 (found). A's exit still restores 0 (the known gap in docs/kernel-state.md):
// B's next tick writes 1 again, and B's own exit never restores a value it
// found.
func TestRevertTwoDaemonsShareIPForward(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{} // no iptables: only ip_forward is shared state here
	a := newForwardDaemon(t, h, &logSink{})
	b := newForwardDaemon(t, h, &logSink{})
	b.config.InterfaceName = "wg1"

	tick(t, a, 1)
	tick(t, b, 1)
	if got := h.count(wantSysctl); got != 1 {
		t.Fatalf("setup: ip_forward written %d times, want 1 (by A)", got)
	}

	a.revertKernel()
	if h.ipForward {
		t.Fatal("A's exit left ip_forward at 1; the documented gap (B loses forwarding until its next tick) is gone")
	}
	tick(t, b, 1)
	if !h.ipForward {
		t.Error("B's next tick did not write ip_forward=1 again")
	}
	b.revertKernel()
	if !h.ipForward || h.count(wantRestore) != 1 {
		t.Errorf("B's exit restored a value it found: ip_forward=%v writes=%q", h.ipForward, h.writes())
	}
}
