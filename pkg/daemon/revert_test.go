package daemon

import (
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

// revertKernel takes convergeMu, and is race-free against ticks and a health
// path that write under it.
func TestRevertKernelRaceFree(t *testing.T) {
	enforceForwardScope(t, false)
	enforceRevert(t, true)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 1)

	d.convergeMu.Lock()
	done := make(chan struct{})
	go func() { d.revertKernel(); close(done) }()
	select {
	case <-done:
		t.Fatal("revertKernel returned while convergeMu was held")
	case <-time.After(50 * time.Millisecond):
	}
	d.convergeMu.Unlock()
	<-done

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
