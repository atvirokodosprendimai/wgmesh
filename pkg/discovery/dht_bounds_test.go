package discovery

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// newBoundsDHT returns a DHTDiscovery whose clock is fixed at the returned
// time, and its store.
func newBoundsDHT(t *testing.T) (*DHTDiscovery, *daemon.PeerStore, time.Time) {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-secret-bounds-1"})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	ps := daemon.NewPeerStore()
	d, err := NewDHTDiscovery(context.Background(), cfg, &daemon.LocalNode{WGPubKey: "local"}, ps)
	if err != nil {
		t.Fatalf("NewDHTDiscovery: %v", err)
	}
	base := time.Now()
	clock := func() time.Time { return base }
	d.now.Store(&clock)
	return d, ps, base
}

func TestMarkContactedSweepsEntriesOlderThanWindow(t *testing.T) {
	d, _, base := newBoundsDHT(t)
	// Literal ages: a test that derives them from ContactDedupWindow would
	// follow the constant when it is raised.
	old := base.Add(-61 * time.Second)
	for i := 0; i < 300; i++ {
		d.contactedPeers[fmt.Sprintf("10.0.%d.%d:1", i/200, i%200)] = old
	}

	if !d.markContacted("new:1", 20*time.Second) {
		t.Fatal("new address must be contacted")
	}
	if n := len(d.contactedPeers); n != 1 {
		t.Fatalf("map holds %d entries after the sweep, want 1", n)
	}

	// The sweep runs at most once per window: expired entries refilled at the
	// same instant stay until the window has passed.
	for i := 0; i < 300; i++ {
		d.contactedPeers[fmt.Sprintf("10.1.%d.%d:1", i/200, i%200)] = old
	}
	d.markContacted("new:2", 20*time.Second)
	if n := len(d.contactedPeers); n != 302 {
		t.Fatalf("map holds %d entries after a second call at the same instant, want 302: the sweep ran twice in one window", n)
	}
}

func TestMarkContactedKeepsEntriesInsideWindow(t *testing.T) {
	d, _, base := newBoundsDHT(t)
	recent := base.Add(-59 * time.Second)
	for i := 0; i < 300; i++ {
		d.contactedPeers[fmt.Sprintf("10.0.%d.%d:1", i/200, i%200)] = recent
	}

	if !d.markContacted("new:1", 20*time.Second) {
		t.Fatal("new address must be contacted")
	}
	if n := len(d.contactedPeers); n != 301 {
		t.Fatalf("map holds %d entries, want 301: entries inside the window must survive", n)
	}
	if d.markContacted("10.0.0.0:1", 60*time.Second) {
		t.Fatal("an entry inside its interval must still block a second contact")
	}
}

func TestMarkContactedFailsClosedAtCap(t *testing.T) {
	d, _, base := newBoundsDHT(t)
	for i := 0; i < MaxContactedPeers; i++ {
		d.contactedPeers[fmt.Sprintf("a%d", i)] = base
	}
	d.contactedPeers["known"] = base.Add(-30 * time.Second)
	delete(d.contactedPeers, "a0")

	if d.markContacted("new:1", 20*time.Second) {
		t.Fatal("a new address at the cap must be refused")
	}
	if n := len(d.contactedPeers); n != MaxContactedPeers {
		t.Fatalf("map grew to %d at the cap, want %d", n, MaxContactedPeers)
	}
	if !d.markContacted("known", 20*time.Second) {
		t.Fatal("a known address past its interval is still decided by the interval")
	}
	if d.markContacted("known", 20*time.Second) {
		t.Fatal("a known address inside its interval is still refused")
	}
	if n := len(d.contactedPeers); n != MaxContactedPeers {
		t.Fatalf("map holds %d after known-address calls, want %d", n, MaxContactedPeers)
	}
}

// intervalNanos evaluates the interval argument of a markContacted call:
// a time unit selector, an integer literal times one, or a constant of this
// package that is itself one of those. ok is false for anything else.
func intervalNanos(e ast.Expr, consts map[string]ast.Expr) (int64, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.INT {
			return 0, false
		}
		n, err := strconv.ParseInt(v.Value, 0, 64)
		return n, err == nil
	case *ast.SelectorExpr:
		pkg, isIdent := v.X.(*ast.Ident)
		if !isIdent || pkg.Name != "time" {
			return 0, false
		}
		units := map[string]time.Duration{
			"Nanosecond": time.Nanosecond, "Microsecond": time.Microsecond,
			"Millisecond": time.Millisecond, "Second": time.Second,
			"Minute": time.Minute, "Hour": time.Hour,
		}
		u, known := units[v.Sel.Name]
		return int64(u), known
	case *ast.Ident:
		c, known := consts[v.Name]
		if !known {
			return 0, false
		}
		return intervalNanos(c, consts)
	case *ast.ParenExpr:
		return intervalNanos(v.X, consts)
	case *ast.BinaryExpr:
		if v.Op != token.MUL {
			return 0, false
		}
		l, lok := intervalNanos(v.X, consts)
		r, rok := intervalNanos(v.Y, consts)
		return l * r, lok && rok
	}
	return 0, false
}

func TestEveryCallerIntervalFitsDedupWindow(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	consts := map[string]ast.Expr{}
	var calls []*ast.CallExpr
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				for i, id := range v.Names {
					if i < len(v.Values) {
						consts[id.Name] = v.Values[i]
					}
				}
			case *ast.CallExpr:
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "markContacted" && len(v.Args) == 2 {
					calls = append(calls, v)
				}
			}
			return true
		})
	}
	if len(calls) == 0 {
		t.Fatal("no markContacted call found: the parse is looking in the wrong place")
	}
	for _, c := range calls {
		pos := fset.Position(c.Pos())
		ns, ok := intervalNanos(c.Args[1], consts)
		if !ok {
			t.Errorf("%s: markContacted interval is not a constant or literal duration", pos)
			continue
		}
		if time.Duration(ns) > ContactDedupWindow {
			t.Errorf("%s: markContacted interval %v is above ContactDedupWindow %v", pos, time.Duration(ns), ContactDedupWindow)
		}
	}
}

func TestSweepPeerKeyedDropsKeysNotInStore(t *testing.T) {
	d, ps, base := newBoundsDHT(t)
	d.controlPeers["gone"] = "1.2.3.4:5"
	d.rendezvousBackoff["gone"] = backoffEntry{nextAttempt: base, duration: RendezvousMinBackoff}
	// A peer withdrawn through the store leaves a tombstone, not a peer.
	ps.SetPeerDirectly("withdrawn", &daemon.PeerInfo{WGPubKey: "withdrawn"})
	ps.Withdraw("withdrawn", "goodbye")
	d.controlPeers["withdrawn"] = "1.2.3.4:6"
	d.rendezvousBackoff["withdrawn"] = backoffEntry{nextAttempt: base, duration: RendezvousMinBackoff}

	d.sweepPeerKeyed()

	for _, k := range []string{"gone", "withdrawn"} {
		if _, ok := d.controlPeers[k]; ok {
			t.Errorf("controlPeers keeps %q, which the store lacks", k)
		}
		if _, ok := d.rendezvousBackoff[k]; ok {
			t.Errorf("rendezvousBackoff keeps %q, which the store lacks", k)
		}
	}
}

func TestSweepPeerKeyedKeepsKeysInStore(t *testing.T) {
	d, ps, base := newBoundsDHT(t)
	ps.SetPeerDirectly("kept", &daemon.PeerInfo{WGPubKey: "kept"})
	d.controlPeers["kept"] = "1.2.3.4:5"
	d.rendezvousBackoff["kept"] = backoffEntry{nextAttempt: base, duration: RendezvousMinBackoff}

	d.sweepPeerKeyed()

	if _, ok := d.controlPeers["kept"]; !ok {
		t.Error("controlPeers lost a key the store holds")
	}
	if _, ok := d.rendezvousBackoff["kept"]; !ok {
		t.Error("rendezvousBackoff lost a key the store holds")
	}
}

func TestCheckStaleHandshakesRunsPeerKeyedSweep(t *testing.T) {
	d, _, base := newBoundsDHT(t)
	d.controlPeers["stale"] = "1.2.3.4:5"
	d.rendezvousBackoff["stale"] = backoffEntry{nextAttempt: base, duration: RendezvousMinBackoff}

	d.checkStaleHandshakes()

	if _, ok := d.controlPeers["stale"]; ok {
		t.Error("checkStaleHandshakes left a controlPeers key the store lacks")
	}
	if _, ok := d.rendezvousBackoff["stale"]; ok {
		t.Error("checkStaleHandshakes left a rendezvousBackoff key the store lacks")
	}
}
