package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakeHost is the kernel and the command line of one Linux node: ip_forward,
// the FORWARD rule, whether iptables is installed, and a log of every command
// that ran. It is reached through MockCommandExecutor and Daemon.readFileFn.
type fakeHost struct {
	mu              sync.Mutex
	ipForward       bool
	ruleInstalled   bool
	iptablesPresent bool
	cmds            []string
	sysctlErr       error
	checkErr        error // what `iptables -C` fails with, whatever the rule
	lookPathErr     error // what LookPath fails with, whatever the binary
	readErr         error // what reading ip_forward fails with
	procValue       *string
	reads           int // reads of ip_forward
}

// exitError is a command that ended with an exit status, as exec.ExitError.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e exitError) ExitCode() int { return e.code }

func (h *fakeHost) readFile(path string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reads++
	if path != "/proc/sys/net/ipv4/ip_forward" {
		return nil, fmt.Errorf("fake host: unexpected read of %s", path)
	}
	if h.readErr != nil {
		return nil, h.readErr
	}
	if h.procValue != nil {
		return []byte(*h.procValue), nil
	}
	if h.ipForward {
		return []byte("1\n"), nil
	}
	return []byte("0\n"), nil
}

func (h *fakeHost) lookPath(file string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lookPathErr != nil {
		return "", h.lookPathErr
	}
	if file == "iptables" && h.iptablesPresent {
		return "/usr/sbin/iptables", nil
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

func (h *fakeHost) exec(name string, args ...string) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	line := name + " " + strings.Join(args, " ")
	h.cmds = append(h.cmds, line)
	switch {
	case name == "sysctl" && line == "sysctl -w net.ipv4.ip_forward=1":
		if h.sysctlErr != nil {
			return []byte("sysctl: denied"), h.sysctlErr
		}
		h.ipForward = true
	case name == "iptables" && !h.iptablesPresent:
		return nil, errors.New("executable file not found")
	case strings.HasPrefix(line, "iptables -C FORWARD "):
		if h.checkErr != nil {
			return []byte("Another app is currently holding the xtables lock"), h.checkErr
		}
		if !h.ruleInstalled {
			return []byte("iptables: Bad rule"), exitError{1}
		}
	case strings.HasPrefix(line, "iptables -A FORWARD "):
		h.ruleInstalled = true
	default:
		return nil, fmt.Errorf("fake host: unexpected command %q", line)
	}
	return nil, nil
}

func (h *fakeHost) command(name string, args ...string) Command {
	return &MockCommand{
		runFunc:            func() error { _, err := h.exec(name, args...); return err },
		combinedOutputFunc: func() ([]byte, error) { return h.exec(name, args...) },
		outputFunc:         func() ([]byte, error) { return h.exec(name, args...) },
	}
}

// writes are the commands that change the kernel.
func (h *fakeHost) writes() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, c := range h.cmds {
		if strings.HasPrefix(c, "sysctl -w") || strings.HasPrefix(c, "iptables -A") {
			out = append(out, c)
		}
	}
	return out
}

func (h *fakeHost) count(prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.cmds {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (h *fakeHost) set(f func(h *fakeHost)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f(h)
}

const wantSysctl = "sysctl -w net.ipv4.ip_forward=1"

// wantRule is built from the same helper as the code; its text is pinned by
// TestForwardRuleText.
var wantRule = "iptables -A " + strings.Join(forwardRuleArgs("wgmesh-t3-none"), " ")

func TestForwardRuleText(t *testing.T) {
	if got, want := strings.Join(forwardRuleArgs("wg0"), " "), "FORWARD -i wg0 -o wg0 -j ACCEPT"; got != want {
		t.Errorf("forwardRuleArgs = %q, want %q", got, want)
	}
}

// newForwardDaemon is a Linux daemon on host h. The tests that use it swap the
// package executor and so never run in parallel.
func newForwardDaemon(t *testing.T, h *fakeHost, ls *logSink) *Daemon {
	t.Helper()
	return newForwardDaemonOn(t, h, ls, newFakeKernel())
}

// newForwardDaemonOn is newForwardDaemon on a given WireGuard kernel.
func newForwardDaemonOn(t *testing.T, h *fakeHost, ls *logSink, k *fakeKernel) *Daemon {
	t.Helper()
	d := newConvergeDaemon(t, k, ls)
	d.goos = "linux"
	d.readFileFn = h.readFile
	old := cmdExecutor
	cmdExecutor = &MockCommandExecutor{lookPathFunc: h.lookPath, commandFunc: h.command}
	t.Cleanup(func() { cmdExecutor = old })
	return d
}

// enforceForwardScope sets the switch for one test and restores it.
func enforceForwardScope(t *testing.T, v bool) {
	t.Helper()
	old := EnforceForwardScope.Load()
	EnforceForwardScope.Store(v)
	t.Cleanup(func() { EnforceForwardScope.Store(old) })
}

func tick(t *testing.T, d *Daemon, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if d.converge(nil) == nil {
			t.Fatalf("tick %d: converge skipped the tick", i+1)
		}
	}
}

func sameLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestForwardAppliedOnceThenQuiet(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})

	tick(t, d, 1)
	if got, want := h.writes(), []string{wantSysctl, wantRule}; !sameLines(got, want) {
		t.Fatalf("tick 1 writes = %q, want %q", got, want)
	}
	tick(t, d, 5)
	if got := h.writes(); len(got) != 2 {
		t.Errorf("ticks 2 to 6 wrote again: %q", got)
	}
	if got := h.count("iptables -C"); got != 6 {
		t.Errorf("iptables -C ran %d times in 6 ticks, want 6 (one read per tick)", got)
	}
	if !h.ipForward || !h.ruleInstalled {
		t.Errorf("host ends with ip_forward=%v rule=%v, want both", h.ipForward, h.ruleInstalled)
	}
}

func TestForwardReappliedWhenKernelDrifts(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	tick(t, d, 2)

	h.set(func(h *fakeHost) { h.ipForward = false })
	tick(t, d, 1)
	if got, want := h.writes(), []string{wantSysctl, wantRule, wantSysctl}; !sameLines(got, want) {
		t.Fatalf("after ip_forward drifted to 0 writes = %q, want %q", got, want)
	}

	h.set(func(h *fakeHost) { h.ruleInstalled = false })
	tick(t, d, 1)
	if got, want := h.writes(), []string{wantSysctl, wantRule, wantSysctl, wantRule}; !sameLines(got, want) {
		t.Fatalf("after the rule was dropped writes = %q, want %q", got, want)
	}
	tick(t, d, 3)
	if got := len(h.writes()); got != 4 {
		t.Errorf("a settled host was written to again: %d writes", got)
	}
}

func TestForwardPreexistingValueNotRewritten(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{ipForward: true, ruleInstalled: true, iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})

	tick(t, d, 5)
	if got := h.writes(); len(got) != 0 {
		t.Errorf("ip_forward 1 and the rule present, yet writes = %q", got)
	}
	if got := h.count("iptables -C"); got != 5 {
		t.Errorf("the rule was read %d times, want once per tick (5)", got)
	}
}

func TestMissingIptablesLoggedOnce(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: false}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)

	tick(t, d, 10)
	if got := ls.count("iptables"); got != 1 {
		t.Errorf("a missing iptables was logged %d times in 10 ticks, want 1; log: %q", got, ls.lines)
	}
	if got := h.count("iptables"); got != 0 {
		t.Errorf("iptables was run %d times without a binary (no -C, no -A): %q", got, h.cmds)
	}
	if got, want := h.writes(), []string{wantSysctl}; !sameLines(got, want) {
		t.Errorf("writes = %q, want only the sysctl %q", got, want)
	}
	if got := ls.count("Failed"); got != 0 {
		t.Errorf("a missing iptables is an outcome, not a failure; log: %q", ls.lines)
	}
}

func TestForwardNotNeededWarnsOnceAtN(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("forward-scope")) }
	before := warnings()

	tick(t, d, 5)
	if got, want := h.writes(), []string{wantSysctl, wantRule}; !sameLines(got, want) {
		t.Errorf("at N a node that is not needed is still forced: writes = %q, want %q", got, want)
	}
	if got := ls.count("forward-scope"); got != 1 {
		t.Errorf("forward-scope logged %d times in 5 ticks, want 1; log: %q", got, ls.lines)
	}
	if got := warnings() - before; got != 1 {
		t.Errorf("forward-scope counted %v times in 5 ticks, want 1", got)
	}

	// A transition to needed and back is a new transition: one more warning.
	d.configMu.Lock()
	d.config.AdvertiseRoutes = []string{"192.168.50.0/24"}
	d.configMu.Unlock()
	tick(t, d, 2)
	if got := warnings() - before; got != 1 {
		t.Errorf("a needed node was counted: %v warnings, want still 1", got)
	}
	d.configMu.Lock()
	d.config.AdvertiseRoutes = nil
	d.configMu.Unlock()
	tick(t, d, 2)
	if got := warnings() - before; got != 2 {
		t.Errorf("after the node stopped advertising %v warnings, want 2", got)
	}
	if got := ls.count("forward-scope"); got != 2 {
		t.Errorf("forward-scope logged %d times over two transitions, want 2", got)
	}
}

func TestForwardScopeEnforcedStopsForcing(t *testing.T) {
	enforceForwardScope(t, true)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	before := testutil.ToFloat64(kernelWarnings.WithLabelValues("forward-scope"))

	tick(t, d, 5)
	if h.reads != 0 || h.count("iptables") != 0 {
		t.Errorf("a node that is not wanted was read: %d reads of ip_forward, commands %q", h.reads, h.cmds)
	}
	if got := h.writes(); len(got) != 0 {
		t.Errorf("EnforceForwardScope on, node not needed, yet writes = %q", got)
	}
	if h.ipForward || h.ruleInstalled {
		t.Errorf("host changed: ip_forward=%v rule=%v", h.ipForward, h.ruleInstalled)
	}
	if got := testutil.ToFloat64(kernelWarnings.WithLabelValues("forward-scope")) - before; got != 0 {
		t.Errorf("an enforced rule is not a warning: counted %v", got)
	}
}

func TestForwardNeededForIntroducerAndAdvertiser(t *testing.T) {
	enforceForwardScope(t, true)
	cases := []struct {
		name   string
		setup  func(d *Daemon)
		needed bool
	}{
		{"introducer", func(d *Daemon) { d.config.Introducer = true }, true},
		{"advertiser", func(d *Daemon) { d.config.AdvertiseRoutes = []string{"192.168.50.0/24"} }, true},
		{"neither", func(d *Daemon) {}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &fakeHost{iptablesPresent: true}
			d := newForwardDaemon(t, h, &logSink{})
			c.setup(d)
			ds := d.converge(nil)
			if ds == nil {
				t.Fatal("converge skipped the tick")
			}
			if ds.Forward.Needed != c.needed || ds.Forward.Wanted != c.needed {
				t.Errorf("Forward = %+v, want Needed and Wanted both %v with the switch on", ds.Forward, c.needed)
			}
			want := []string(nil)
			if c.needed {
				want = []string{wantSysctl, wantRule}
			}
			if got := h.writes(); !sameLines(got, want) {
				t.Errorf("writes = %q, want %q", got, want)
			}
		})
	}
}

// A failed write is a result of its item: logged once however many ticks it
// repeats, the other item is still written, the tick is not skipped.
func TestForwardWriteFailureIsAResult(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true, sysctlErr: errors.New("exit status 255")}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)

	tick(t, d, 5)
	if got := ls.count("Failed to set forward ip_forward"); got != 1 {
		t.Errorf("a failing sysctl was logged %d times in 5 ticks, want 1; log: %q", got, ls.lines)
	}
	if !h.ruleInstalled {
		t.Error("the rule was not appended after the sysctl failed")
	}
	if got := h.count("sysctl -w"); got != 5 {
		t.Errorf("sysctl was retried %d times in 5 ticks, want 5", got)
	}
	h.set(func(h *fakeHost) { h.sysctlErr = nil })
	tick(t, d, 1)
	if got := ls.count("set forward ip_forward... recovered"); got != 1 {
		t.Errorf("recovery logged %d times, want 1; log: %q", got, ls.lines)
	}
}

// The tick, the switch and a SIGHUP reload run on different goroutines.
func TestForwardTickRaceFree(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true}
	d := newForwardDaemon(t, h, &logSink{})
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			EnforceForwardScope.Store(i%2 == 0)
			d.configMu.Lock()
			if i%3 == 0 {
				d.config.AdvertiseRoutes = []string{"192.168.50.0/24"}
			} else {
				d.config.AdvertiseRoutes = nil
			}
			d.configMu.Unlock()
		}
		close(done)
	}()
	for {
		select {
		case <-done:
			wg.Wait()
			return
		default:
			d.converge(nil)
		}
	}
}

// `iptables -C` exits 1 for an absent rule; any other failure (the xtables
// lock is exit 4, an exec error has no status) says nothing about the rule and
// is an observe error for the forward target: no -A, however many ticks.
func TestForwardRuleCheckFailureIsNotAbsent(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"xtables lock exit 4", exitError{4}},
		{"exec failure", errors.New("fork/exec /usr/sbin/iptables: permission denied")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enforceForwardScope(t, false)
			h := &fakeHost{iptablesPresent: true, checkErr: c.err}
			ls := &logSink{}
			d := newForwardDaemon(t, h, ls)

			tick(t, d, 3)
			if got := h.count("iptables -A"); got != 0 {
				t.Errorf("a failed -C was read as 'rule absent': %d x iptables -A in 3 ticks: %q", got, h.cmds)
			}
			if got := ls.count("Cannot read the forwarding state"); got != 1 {
				t.Errorf("the observe error was logged %d times in 3 ticks, want 1; log: %q", got, ls.lines)
			}

			h.set(func(h *fakeHost) { h.checkErr = nil })
			tick(t, d, 2)
			if got, want := h.writes(), []string{wantSysctl, wantRule}; !sameLines(got, want) {
				t.Errorf("after -C recovered writes = %q, want %q", got, want)
			}
			if got := ls.count("works again"); got != 1 {
				t.Errorf("recovery logged %d times, want 1; log: %q", got, ls.lines)
			}
		})
	}
}

// A LookPath error other than "not found" is an observe error, not a missing
// iptables.
func TestForwardLookPathErrorIsObserveError(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true, lookPathErr: &exec.Error{Name: "iptables", Err: fs.ErrPermission}}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)

	tick(t, d, 3)
	if got := ls.count("iptables not found"); got != 0 {
		t.Errorf("a permission error was reported as a missing iptables: %q", ls.lines)
	}
	if got := ls.count("Cannot read the forwarding state"); got != 1 {
		t.Errorf("the observe error was logged %d times in 3 ticks, want 1; log: %q", got, ls.lines)
	}
	if got := h.writes(); len(got) != 0 {
		t.Errorf("an unobserved forward target was written: %q", got)
	}
}

// The "iptables not found" line is logged once per stretch of ticks in which
// the target was observed: an unobserved tick (the /proc read failed) does not
// reset it.
func TestForwardNoIptablesLogOnceAcrossReadError(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: false}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)

	tick(t, d, 1)
	h.set(func(h *fakeHost) { h.readErr = errors.New("permission denied") })
	tick(t, d, 1)
	h.set(func(h *fakeHost) { h.readErr = nil })
	tick(t, d, 1)
	if got := ls.count("iptables not found"); got != 1 {
		t.Errorf("a missing iptables was logged %d times over 3 ticks with a read error on tick 2, want 1; log: %q", got, ls.lines)
	}
}

// A target that was not observed (not wanted) is not "recovered".
func TestForwardRecoveryNotLoggedWhileNotWanted(t *testing.T) {
	enforceForwardScope(t, true)
	h := &fakeHost{iptablesPresent: true, readErr: errors.New("permission denied")}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	advertise := func(routes []string) {
		d.configMu.Lock()
		d.config.AdvertiseRoutes = routes
		d.configMu.Unlock()
	}

	advertise([]string{"192.168.50.0/24"})
	tick(t, d, 1)
	if got := ls.count("Cannot read the forwarding state"); got != 1 {
		t.Fatalf("the read error was logged %d times, want 1; log: %q", got, ls.lines)
	}
	advertise(nil)
	tick(t, d, 2)
	if got := ls.count("works again"); got != 0 {
		t.Errorf("'works again' logged while the target was not observed: %q", ls.lines)
	}
	h.set(func(h *fakeHost) { h.readErr = nil })
	advertise([]string{"192.168.50.0/24"})
	tick(t, d, 1)
	if got := ls.count("works again"); got != 1 {
		t.Errorf("recovery logged %d times once the read worked, want 1; log: %q", got, ls.lines)
	}
}

// The forward-scope line says which roles keep forwarding, which interface and
// what to do about it, on one line.
func TestForwardScopeLogNamesRoleAndRemedy(t *testing.T) {
	enforceForwardScope(t, false)
	h := &fakeHost{iptablesPresent: true}
	ls := &logSink{}
	d := newForwardDaemon(t, h, ls)
	tick(t, d, 1)

	var line string
	ls.mu.Lock()
	for _, l := range ls.lines {
		if strings.Contains(l, "forward-scope") {
			line = l
		}
	}
	ls.mu.Unlock()
	for _, part := range []string{"introducer", "advertise-routes", "--introducer", "wgmesh-t3-none", "EnforceForwardScope", "a later release"} {
		if !strings.Contains(line, part) {
			t.Errorf("forward-scope line lacks %q: %q", part, line)
		}
	}
	if strings.Contains(line, "\n") {
		t.Errorf("forward-scope line is not one line: %q", line)
	}
}

// An unreadable forwarding state (a read error, "2", an empty file) skips only
// the forward target: no sysctl, no iptables write, while the peer writes of
// the same tick still run. Logged once, recovery once.
func TestForwardReadErrorSkipsOnlyForward(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name string
		fail func(h *fakeHost)
	}{
		{"read error", func(h *fakeHost) { h.readErr = errors.New("permission denied") }},
		{"value 2", func(h *fakeHost) { h.procValue = str("2\n") }},
		{"empty file", func(h *fakeHost) { h.procValue = str("") }},
	}
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enforceForwardScope(t, false)
			h := &fakeHost{iptablesPresent: true}
			c.fail(h)
			ls := &logSink{}
			k := newFakeKernel()
			d := newForwardDaemonOn(t, h, ls, k)

			for i := 0; i < 3; i++ {
				if d.converge(peers) == nil {
					t.Fatalf("tick %d: converge skipped the whole tick", i+1)
				}
			}
			if got := h.writes(); len(got) != 0 {
				t.Errorf("an unreadable forwarding state was written to: %q", got)
			}
			if got := len(k.setCalls()); got != 1 {
				t.Errorf("peer sets = %d, want 1: a forward observe error must not skip the peers", got)
			}
			if got := ls.count("Cannot read the forwarding state"); got != 1 {
				t.Errorf("observe error logged %d times in 3 ticks, want 1; log: %q", got, ls.lines)
			}

			h.set(func(h *fakeHost) { h.readErr, h.procValue = nil, nil })
			for i := 0; i < 2; i++ {
				d.converge(peers)
			}
			if got, want := h.writes(), []string{wantSysctl, wantRule}; !sameLines(got, want) {
				t.Errorf("after recovery writes = %q, want %q", got, want)
			}
			if got := ls.count("works again"); got != 1 {
				t.Errorf("recovery logged %d times, want 1; log: %q", got, ls.lines)
			}
		})
	}
}
