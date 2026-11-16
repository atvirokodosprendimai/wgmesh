package node

import (
	"sync"
	"testing"
	"time"
)

// livenessNow has no monotonic reading, so every age in these tests is wall time.
var livenessNow = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)

// enforceLiveness sets the process-wide switch for one test and restores it
// off. Tests that set it do not call t.Parallel: parallel tests resume only
// after every serial test has finished.
func enforceLiveness(t *testing.T, on bool) {
	t.Helper()
	EnforceLiveness.Store(on)
	t.Cleanup(func() { EnforceLiveness.Store(false) })
}

// clockStore is a store whose clock is fixed at livenessNow.
func clockStore() *PeerStore {
	ps := NewPeerStore()
	ps.now = func() time.Time { return livenessNow }
	return ps
}

func seedPeer(ps *PeerStore, key string, lastSeen time.Time) {
	ps.SetPeerDirectly(key, &PeerInfo{WGPubKey: key, LastSeen: lastSeen})
}

func hasKey(peers []*PeerInfo, key string) bool {
	for _, p := range peers {
		if p.WGPubKey == key {
			return true
		}
	}
	return false
}

type siteLog struct {
	mu    sync.Mutex
	sites []string
}

func (l *siteLog) hook(site string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sites = append(l.sites, site)
}

func (l *siteLog) got() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.sites...)
}

func TestAssessThresholds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		age  time.Duration
		want Liveness
	}{
		{"fresh", 0, Alive},
		{"just under dead timeout", 4*time.Minute + 59*time.Second, Alive},
		{"at dead timeout", 5 * time.Minute, Suspect},
		{"at remove timeout", 10 * time.Minute, Suspect},
		{"just over remove timeout", 10*time.Minute + time.Second, Dead},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Assess(Evidence{LastSeen: livenessNow.Add(-tc.age)}, livenessNow)
			if got != tc.want {
				t.Fatalf("age %v: Assess = %v, want %v", tc.age, got, tc.want)
			}
		})
	}
	if got := Assess(Evidence{}, livenessNow); got != Dead {
		t.Fatalf("empty evidence: Assess = %v, want Dead", got)
	}
}

func TestAssessNewestEvidenceWins(t *testing.T) {
	t.Parallel()
	old := livenessNow.Add(-time.Hour)
	fresh := livenessNow.Add(-time.Minute)
	tests := []struct {
		name string
		ev   Evidence
		want Liveness
	}{
		{"data plane", Evidence{LastSeen: old, LastDataPlane: fresh}, Alive},
		{"restored", Evidence{LastSeen: old, RestoredAt: fresh}, Alive},
		{"resumed", Evidence{LastSeen: old, ResumedAt: fresh}, Alive},
		{"newest is a suspect age", Evidence{LastSeen: old, LastDataPlane: livenessNow.Add(-7 * time.Minute), RestoredAt: old}, Suspect},
		{"fresh last seen beats old others", Evidence{LastSeen: fresh, LastDataPlane: old, RestoredAt: old, ResumedAt: old}, Alive},
		{"all old", Evidence{LastSeen: old, LastDataPlane: old, RestoredAt: old, ResumedAt: old}, Dead},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Assess(tc.ev, livenessNow); got != tc.want {
				t.Fatalf("Assess = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAssessIgnoresMonotonicReading(t *testing.T) {
	t.Parallel()
	withMono := time.Now()
	wallOnly := time.Unix(0, withMono.UnixNano())
	if withMono.Equal(wallOnly) == false {
		t.Fatal("test setup: the two instants must be equal")
	}
	for _, age := range []time.Duration{time.Minute, 5 * time.Minute, 7 * time.Minute, 10*time.Minute + time.Second} {
		// now is advanced by wall time only (no monotonic reading).
		now := wallOnly.Add(age)
		a := Assess(Evidence{LastSeen: withMono}, now)
		b := Assess(Evidence{LastSeen: wallOnly}, now)
		if a != b {
			t.Fatalf("age %v: monotonic LastSeen gives %v, wall-only gives %v", age, a, b)
		}
		// and with a monotonic now too.
		a = Assess(Evidence{LastSeen: withMono}, withMono.Add(age))
		if a != b {
			t.Fatalf("age %v: monotonic now gives %v, wall-only gives %v", age, a, b)
		}
	}
}

func TestGetActiveUsesAssessWhenEnforced(t *testing.T) {
	enforceLiveness(t, true)
	ps := clockStore()
	seedPeer(ps, "k1", livenessNow.Add(-time.Hour))
	ps.ObserveDataPlane("k1", livenessNow.Add(-time.Minute))

	if !hasKey(ps.GetActive(), "k1") {
		t.Fatal("GetActive dropped a peer with a fresh LastDataPlane")
	}
	if removed := ps.CleanupStale(); len(removed) != 0 {
		t.Fatalf("CleanupStale removed %v, want nothing", removed)
	}
	if ps.Count() != 1 {
		t.Fatalf("Count = %d, want 1", ps.Count())
	}
	if ps.IsDead("k1") {
		t.Fatal("IsDead is true for a peer with a fresh LastDataPlane")
	}
	if !ps.IsDead("unknown") {
		t.Fatal("IsDead is false for an unknown key")
	}
}

func TestLegacyAgeStillActsAndReportsDisagreement(t *testing.T) {
	enforceLiveness(t, false)
	ps := clockStore()
	var log siteLog
	ps.SetLivenessDisagreeHook(log.hook)
	other := clockStore() // no hook: unaffected by the first store's hook
	for _, s := range []*PeerStore{ps, other} {
		seedPeer(s, "k1", livenessNow.Add(-time.Hour))
		s.ObserveDataPlane("k1", livenessNow.Add(-time.Minute))
	}

	if hasKey(ps.GetActive(), "k1") {
		t.Fatal("legacy age test did not drop the peer from GetActive")
	}
	if got := log.got(); len(got) != 1 || got[0] != "get-active" {
		t.Fatalf("hook after GetActive = %v, want [get-active]", got)
	}
	if hasKey(other.GetActive(), "k1") {
		t.Fatal("legacy age test did not drop the peer in the store without a hook")
	}
	if got := log.got(); len(got) != 1 {
		t.Fatalf("a second store reached the first store's hook: %v", got)
	}

	if !ps.IsDead("k1") {
		t.Fatal("legacy age test: IsDead is false for an hour-old LastSeen")
	}
	if removed := ps.CleanupStale(); len(removed) != 1 || removed[0] != "k1" {
		t.Fatalf("legacy age test: CleanupStale removed %v, want [k1]", removed)
	}
	want := []string{"get-active", "is-dead", "cleanup"}
	got := log.got()
	if len(got) != len(want) {
		t.Fatalf("hook sites = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hook sites = %v, want %v", got, want)
		}
	}
}

func TestObserveDataPlaneNeverMovesBack(t *testing.T) {
	t.Parallel()
	ps := clockStore()
	seen := livenessNow.Add(-time.Hour)
	seedPeer(ps, "k1", seen)
	ch := ps.Subscribe()

	newer := livenessNow.Add(-time.Minute)
	ps.ObserveDataPlane("k1", newer)
	ps.ObserveDataPlane("k1", newer.Add(-time.Minute))
	got, _ := ps.Get("k1")
	if !got.LastDataPlane.Equal(newer) {
		t.Fatalf("LastDataPlane = %v after an older time, want %v", got.LastDataPlane, newer)
	}
	later := newer.Add(30 * time.Second)
	ps.ObserveDataPlane("k1", later)
	got, _ = ps.Get("k1")
	if !got.LastDataPlane.Equal(later) {
		t.Fatalf("LastDataPlane = %v after a newer time, want %v", got.LastDataPlane, later)
	}
	if !got.LastSeen.Equal(seen) {
		t.Fatalf("ObserveDataPlane wrote LastSeen: %v, want %v", got.LastSeen, seen)
	}

	ps.ObserveDataPlane("unknown", later)
	if ps.Count() != 1 {
		t.Fatalf("an unknown key changed the store: Count = %d", ps.Count())
	}
	select {
	case ev := <-ch:
		t.Fatalf("ObserveDataPlane sent an event: %+v", ev)
	default:
	}
}

func TestGetProbeableIncludesSuspect(t *testing.T) {
	enforceLiveness(t, false)
	ps := clockStore()
	seedPeer(ps, "alive", livenessNow.Add(-time.Minute))
	seedPeer(ps, "suspect", livenessNow.Add(-7*time.Minute))
	seedPeer(ps, "dead", livenessNow.Add(-11*time.Minute))

	active := ps.GetActive()
	if hasKey(active, "suspect") || !hasKey(active, "alive") {
		t.Fatalf("GetActive keys wrong: suspect in = %v, alive in = %v", hasKey(active, "suspect"), hasKey(active, "alive"))
	}
	probeable := ps.GetProbeable()
	if !hasKey(probeable, "suspect") || !hasKey(probeable, "alive") {
		t.Fatal("GetProbeable lacks the alive or the suspect peer")
	}
	if hasKey(probeable, "dead") {
		t.Fatal("GetProbeable contains a dead peer")
	}
}

func TestResumedAtIsStampedIntoCopies(t *testing.T) {
	enforceLiveness(t, true)
	ps := clockStore()
	seedPeer(ps, "k1", livenessNow.Add(-3*time.Hour))
	if p, _ := ps.Get("k1"); p.Alive(livenessNow) != Dead {
		t.Fatal("setup: the peer should be Dead before a resume")
	}

	woke := livenessNow.Add(-time.Minute)
	ps.NoteResume(woke)
	if got := ps.ResumedAt(); !got.Equal(woke) {
		t.Fatalf("ResumedAt = %v, want %v", got, woke)
	}

	one, _ := ps.Get("k1")
	copies := map[string]*PeerInfo{"Get": one}
	for name, list := range map[string][]*PeerInfo{"GetAll": ps.GetAll(), "GetActive": ps.GetActive(), "GetProbeable": ps.GetProbeable()} {
		if len(list) != 1 {
			t.Fatalf("%s returned %d peers, want 1", name, len(list))
		}
		copies[name] = list[0]
	}
	for name, p := range copies {
		if got := p.Alive(livenessNow); got != Alive {
			t.Errorf("%s copy: Alive = %v, want Alive", name, got)
		}
	}
}

func TestLastAliveIsNewestEvidence(t *testing.T) {
	t.Parallel()
	a := livenessNow.Add(-4 * time.Hour)
	b := livenessNow.Add(-3 * time.Hour)
	c := livenessNow.Add(-2 * time.Hour)
	d := livenessNow.Add(-time.Hour)
	tests := []struct {
		name string
		p    PeerInfo
		want time.Time
	}{
		{"last seen", PeerInfo{LastSeen: d, LastDataPlane: c, RestoredAt: b, resumedAt: a}, d},
		{"data plane", PeerInfo{LastSeen: a, LastDataPlane: d, RestoredAt: c, resumedAt: b}, d},
		{"restored", PeerInfo{LastSeen: a, LastDataPlane: b, RestoredAt: d, resumedAt: c}, d},
		{"resumed", PeerInfo{LastSeen: a, LastDataPlane: b, RestoredAt: c, resumedAt: d}, d},
		{"one only", PeerInfo{RestoredAt: c}, c},
		{"none", PeerInfo{}, time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.LastAlive(livenessNow); !got.Equal(tc.want) {
				t.Fatalf("LastAlive = %v, want %v", got, tc.want)
			}
		})
	}

	for _, age := range []time.Duration{4*time.Minute + 59*time.Second, 5 * time.Minute, 10 * time.Minute, 10*time.Minute + time.Second} {
		p := PeerInfo{LastSeen: livenessNow.Add(-time.Hour), LastDataPlane: livenessNow.Add(-age)}
		want := Assess(Evidence{LastSeen: p.LastSeen, LastDataPlane: p.LastDataPlane}, livenessNow)
		if got := p.Alive(livenessNow); got != want {
			t.Fatalf("age %v: Alive = %v, Assess = %v", age, got, want)
		}
		if got := livenessNow.Sub(p.LastAlive(livenessNow)); got != age {
			t.Fatalf("age %v: now - LastAlive = %v", age, got)
		}
	}
}

func TestLivenessUsesStoreClock(t *testing.T) {
	enforceLiveness(t, false)
	// livenessNow is years before the real clock: a reader that calls
	// time.Now() sees every peer here as long dead.
	ps := clockStore()
	seedPeer(ps, "k1", livenessNow.Add(-time.Minute))
	if !hasKey(ps.GetActive(), "k1") {
		t.Fatal("GetActive ignored the store clock")
	}
	if ps.IsDead("k1") {
		t.Fatal("IsDead ignored the store clock")
	}

	ps.Update(&PeerInfo{WGPubKey: "k2"}, "dht")
	got, ok := ps.Get("k2")
	if !ok {
		t.Fatal("Update did not add k2")
	}
	if !got.LastSeen.Equal(livenessNow) {
		t.Fatalf("merge stamped LastSeen %v, want the store clock %v", got.LastSeen, livenessNow)
	}
}

func TestLivenessStateIsRaceFree(t *testing.T) {
	enforceLiveness(t, false)
	ps := clockStore()
	seedPeer(ps, "k1", livenessNow.Add(-7*time.Minute))
	seedPeer(ps, "k2", livenessNow.Add(-time.Minute))

	var log siteLog
	var wg sync.WaitGroup
	run := func(f func(i int)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				f(i)
			}
		}()
	}
	run(func(i int) { ps.ObserveDataPlane("k1", livenessNow.Add(time.Duration(i)*time.Second)) })
	run(func(i int) { ps.NoteResume(livenessNow.Add(time.Duration(i) * time.Second)) })
	run(func(i int) { ps.SetLivenessDisagreeHook(log.hook) })
	run(func(i int) { ps.SetLivenessDisagreeHook(nil) })
	run(func(i int) { _ = ps.GetActive() })
	run(func(i int) { _ = ps.GetProbeable() })
	run(func(i int) { _ = ps.IsDead("k1"); _ = ps.ResumedAt() })
	run(func(i int) { EnforceLiveness.Store(i%2 == 0) })
	run(func(i int) { _ = EnforceLiveness.Load() })
	wg.Wait()
}
