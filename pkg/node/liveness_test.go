package node

import (
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// livenessNow has no monotonic reading, so every age in these tests is wall
// time. It lies years before the real clock: a reader that calls time.Now()
// instead of the store clock sees every peer seeded here as decades old, so
// TestLivenessUsesStoreClock fails for it.
var livenessNow = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)

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
	if !strings.Contains(withMono.String(), "m=") {
		t.Fatal("test setup: time.Now() should carry a monotonic reading")
	}
	if s := (Evidence{LastSeen: withMono}).newest().String(); strings.Contains(s, "m=") {
		t.Fatalf("newest kept the monotonic reading: %s", s)
	}
	wallOnly := time.Unix(0, withMono.UnixNano())
	if !withMono.Equal(wallOnly) {
		t.Fatal("test setup: the two instants must be equal")
	}
	tests := []struct {
		age  time.Duration
		want Liveness
	}{
		{time.Minute, Alive},
		{5 * time.Minute, Suspect},
		{7 * time.Minute, Suspect},
		{10*time.Minute + time.Second, Dead},
	}
	for _, tc := range tests {
		// now is advanced by wall time only (no monotonic reading).
		now := wallOnly.Add(tc.age)
		if got := Assess(Evidence{LastSeen: withMono}, now); got != tc.want {
			t.Fatalf("age %v: monotonic LastSeen gives %v, want %v", tc.age, got, tc.want)
		}
		if got := Assess(Evidence{LastSeen: wallOnly}, now); got != tc.want {
			t.Fatalf("age %v: wall-only LastSeen gives %v, want %v", tc.age, got, tc.want)
		}
		// and with a monotonic now too.
		if got := Assess(Evidence{LastSeen: withMono}, withMono.Add(tc.age)); got != tc.want {
			t.Fatalf("age %v: monotonic now gives %v, want %v", tc.age, got, tc.want)
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

	for _, tc := range []struct {
		age  time.Duration
		want Liveness
	}{
		{4*time.Minute + 59*time.Second, Alive},
		{5 * time.Minute, Suspect},
		{10 * time.Minute, Suspect},
		{10*time.Minute + time.Second, Dead},
	} {
		p := PeerInfo{LastSeen: livenessNow.Add(-time.Hour), LastDataPlane: livenessNow.Add(-tc.age)}
		if got := p.Alive(livenessNow); got != tc.want {
			t.Fatalf("age %v: Alive = %v, want %v", tc.age, got, tc.want)
		}
		if got := livenessNow.Sub(p.LastAlive(livenessNow)); got != tc.age {
			t.Fatalf("age %v: now - LastAlive = %v", tc.age, got)
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
	// Times are in the past of the store clock (they would be clamped otherwise);
	// each writer is sequential, so its last write is the maximum.
	run(func(i int) { ps.ObserveDataPlane("k1", livenessNow.Add(-time.Duration(300-i)*time.Second)) })
	run(func(i int) { ps.NoteResume(livenessNow.Add(-time.Duration(300-i) * time.Second)) })
	run(func(i int) { ps.SetLivenessDisagreeHook(log.hook) })
	run(func(i int) { ps.SetLivenessDisagreeHook(nil) })
	run(func(i int) { _ = ps.GetActive() })
	run(func(i int) { _ = ps.GetProbeable() })
	run(func(i int) { _ = ps.IsDead("k1"); _ = ps.ResumedAt() })
	run(func(i int) { EnforceLiveness.Store(i%2 == 0) })
	run(func(i int) { _ = EnforceLiveness.Load() })
	wg.Wait()
	last := livenessNow.Add(-time.Second)
	if got := ps.ResumedAt(); !got.Equal(last) {
		t.Fatalf("ResumedAt = %v after concurrent NoteResume, want the maximum %v (lost update)", got, last)
	}
	if p, _ := ps.Get("k1"); !p.LastDataPlane.Equal(last) {
		t.Fatalf("LastDataPlane = %v after concurrent ObserveDataPlane, want the maximum %v (lost update)", p.LastDataPlane, last)
	}
}

// movableStore is a store whose clock is *cur; the test moves it.
func movableStore(cur *time.Time) *PeerStore {
	ps := NewPeerStore()
	ps.now = func() time.Time { return *cur }
	return ps
}

func TestNoteResumeFutureTimeIsClamped(t *testing.T) {
	enforceLiveness(t, true)
	cur := livenessNow
	ps := movableStore(&cur)
	seedPeer(ps, "k1", cur.Add(-11*time.Minute))

	ps.NoteResume(cur.Add(time.Hour))
	if got := ps.ResumedAt(); got.After(cur) {
		t.Fatalf("ResumedAt = %v, a time after the store clock %v", got, cur)
	}
	cur = cur.Add(11 * time.Minute)
	if hasKey(ps.GetProbeable(), "k1") {
		t.Fatal("a future NoteResume time kept the peer probeable 11 minutes later")
	}
	if removed := ps.CleanupStale(); len(removed) != 1 || removed[0] != "k1" {
		t.Fatalf("CleanupStale removed %v, want [k1]", removed)
	}
}

func TestObserveDataPlaneFutureTimeIsClamped(t *testing.T) {
	enforceLiveness(t, true)
	cur := livenessNow
	ps := movableStore(&cur)
	seedPeer(ps, "k1", cur.Add(-time.Hour))

	ps.ObserveDataPlane("k1", cur.Add(time.Hour))
	ps.ObserveDataPlane("k1", cur.Add(-time.Minute))
	cur = cur.Add(30 * time.Minute)
	if hasKey(ps.GetActive(), "k1") {
		t.Fatal("a future ObserveDataPlane time pinned the peer Alive 30 minutes later")
	}
}

func TestNoteResumeNeverMovesBack(t *testing.T) {
	t.Parallel()
	ps := clockStore()
	newer := livenessNow.Add(-time.Minute)
	ps.NoteResume(newer)
	ps.NoteResume(newer.Add(-time.Minute))
	if got := ps.ResumedAt(); !got.Equal(newer) {
		t.Fatalf("ResumedAt = %v after an older time, want %v", got, newer)
	}
	later := newer.Add(30 * time.Second)
	ps.NoteResume(later)
	if got := ps.ResumedAt(); !got.Equal(later) {
		t.Fatalf("ResumedAt = %v after a newer time, want %v", got, later)
	}
}

func TestOfferedEvidenceIsNotTrusted(t *testing.T) {
	enforceLiveness(t, true)
	ps := clockStore()
	ps.Offer(Candidate{
		Info: &PeerInfo{
			WGPubKey:      "k1",
			LastSeen:      livenessNow.Add(-time.Hour),
			LastDataPlane: livenessNow,
			RestoredAt:    livenessNow,
			resumedAt:     livenessNow,
		},
		Source: "dht",
		Hand:   SecondHand,
	})
	if ps.Count() != 1 {
		t.Fatalf("setup: Count = %d, want 1", ps.Count())
	}
	if hasKey(ps.GetActive(), "k1") {
		t.Fatal("a second-hand claim carrying LastDataPlane/RestoredAt/resumedAt made the peer Alive")
	}
	got, _ := ps.Get("k1")
	if !got.LastDataPlane.IsZero() || !got.RestoredAt.IsZero() {
		t.Fatalf("stored LastDataPlane %v, RestoredAt %v, want zero (measured here only)", got.LastDataPlane, got.RestoredAt)
	}
}

func TestEnforcedSuspectPeerIsNotActiveNotRemoved(t *testing.T) {
	enforceLiveness(t, true)
	ps := clockStore()
	seedPeer(ps, "k1", livenessNow.Add(-7*time.Minute))

	if hasKey(ps.GetActive(), "k1") {
		t.Fatal("GetActive contains a Suspect peer")
	}
	if removed := ps.CleanupStale(); len(removed) != 0 {
		t.Fatalf("CleanupStale removed %v, want nothing for a Suspect peer", removed)
	}
	if ps.IsDead("k1") {
		t.Fatal("IsDead is true for a Suspect peer")
	}
	if ps.Count() != 1 {
		t.Fatalf("Count = %d, want 1", ps.Count())
	}
}

func TestEnforcedDeadPeerIsRemovedAndTombstoned(t *testing.T) {
	enforceLiveness(t, true)
	ps := clockStore()
	ps.SetPeerDirectly("first", &PeerInfo{WGPubKey: "first", LastSeen: livenessNow.Add(-3 * time.Hour), heardFirstHand: true})
	ps.SetPeerDirectly("second", &PeerInfo{WGPubKey: "second", LastSeen: livenessNow.Add(-3 * time.Hour)})

	for _, k := range []string{"first", "second"} {
		if !ps.IsDead(k) {
			t.Fatalf("IsDead(%s) is false for a 3h old peer", k)
		}
	}
	removed := ps.CleanupStale()
	sort.Strings(removed)
	if len(removed) != 2 || removed[0] != "first" || removed[1] != "second" {
		t.Fatalf("CleanupStale removed %v, want [first second]", removed)
	}
	if _, ok := ps.tombstones["first"]; !ok {
		t.Fatal("a first-hand peer removed as Dead left no tombstone")
	}
	if _, ok := ps.tombstones["second"]; ok {
		t.Fatal("a second-hand peer removed as Dead was tombstoned")
	}
}

func TestDisagreementHookIsSilentWhenAssessAgrees(t *testing.T) {
	enforceLiveness(t, false)
	ps := clockStore()
	var log siteLog
	ps.SetLivenessDisagreeHook(log.hook)
	seedPeer(ps, "k1", livenessNow.Add(-time.Hour))

	if hasKey(ps.GetActive(), "k1") {
		t.Fatal("legacy age test kept an hour old peer in GetActive")
	}
	if !ps.IsDead("k1") {
		t.Fatal("legacy age test: IsDead is false for an hour old peer")
	}
	if removed := ps.CleanupStale(); len(removed) != 1 {
		t.Fatalf("CleanupStale removed %v, want [k1]", removed)
	}
	if got := log.got(); len(got) != 0 {
		t.Fatalf("hook heard %v where Assess agrees with the legacy test, want silence", got)
	}
}

func TestDisagreementHookIsCalledPerDisagreeingPeer(t *testing.T) {
	enforceLiveness(t, false)
	ps := clockStore()
	var log siteLog
	ps.SetLivenessDisagreeHook(log.hook)
	for _, k := range []string{"k1", "k2"} {
		seedPeer(ps, k, livenessNow.Add(-time.Hour))
		ps.ObserveDataPlane(k, livenessNow.Add(-time.Minute))
	}

	_ = ps.GetActive()
	if got := log.got(); len(got) != 2 || got[0] != "get-active" || got[1] != "get-active" {
		t.Fatalf("hook after GetActive = %v, want two get-active", got)
	}
	if removed := ps.CleanupStale(); len(removed) != 2 {
		t.Fatalf("CleanupStale removed %v, want both peers", removed)
	}
	if got := log.got(); len(got) != 4 || got[2] != "cleanup" || got[3] != "cleanup" {
		t.Fatalf("hook after CleanupStale = %v, want two more cleanup", got)
	}
}

// restoredOffer offers a cache entry for key to ps the way the daemon's
// restore does.
func restoredOffer(ps *PeerStore, key string, lastSeen time.Time) {
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: key, MeshIP: "10.0.0.2", LastSeen: lastSeen}, Source: "cache", Hand: Restored})
}

func TestRestoredAtIsSetOnlyForNewKeys(t *testing.T) {
	cur := livenessNow
	ps := movableStore(&cur)
	seen := cur.Add(-2 * time.Minute)
	seedPeer(ps, "known", seen)

	cur = cur.Add(time.Minute)
	restoredOffer(ps, "known", cur.Add(-time.Hour))
	restoredOffer(ps, "fresh", cur.Add(-time.Hour))
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "second", MeshIP: "10.0.0.3"}, Source: "gossip", Hand: SecondHand})
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "first", MeshIP: "10.0.0.4"}, Source: "dht", Hand: FirstHand})

	known, _ := ps.Get("known")
	if !known.RestoredAt.IsZero() || !known.LastSeen.Equal(seen) || !known.LastDataPlane.IsZero() {
		t.Fatalf("a Restored offer changed a known key's evidence: RestoredAt %v LastSeen %v (want zero and %v) LastDataPlane %v", known.RestoredAt, known.LastSeen, seen, known.LastDataPlane)
	}
	fresh, _ := ps.Get("fresh")
	if !fresh.RestoredAt.Equal(cur) {
		t.Fatalf("a new Restored key has RestoredAt %v, want the store clock %v", fresh.RestoredAt, cur)
	}
	if !fresh.LastSeen.Equal(cur.Add(-time.Hour)) {
		t.Fatalf("a new Restored key has LastSeen %v, want the cache time %v untouched", fresh.LastSeen, cur.Add(-time.Hour))
	}
	for _, k := range []string{"second", "first"} {
		if p, _ := ps.Get(k); !p.RestoredAt.IsZero() {
			t.Fatalf("a %s new key has RestoredAt %v, want zero: only a Restored candidate sets it", k, p.RestoredAt)
		}
	}
}

func TestRestoredNeverHeardLeavesAfterRemoveTimeout(t *testing.T) {
	enforceLiveness(t, true)
	cur := livenessNow
	ps := movableStore(&cur)
	restoredOffer(ps, "k1", cur.Add(-time.Hour))

	steps := []struct {
		name    string
		after   time.Duration
		active  bool
		removed bool
	}{
		{"just restored", 0, true, false},
		{"before PeerDeadTimeout", PeerDeadTimeout - time.Second, true, false},
		{"at PeerDeadTimeout", PeerDeadTimeout, false, false},
		{"at PeerRemoveTimeout", PeerRemoveTimeout, false, false},
		{"after PeerRemoveTimeout", PeerRemoveTimeout + time.Second, false, true},
	}
	for _, s := range steps {
		cur = livenessNow.Add(s.after)
		if got := hasKey(ps.GetActive(), "k1"); got != s.active {
			t.Fatalf("%s: in GetActive = %v, want %v", s.name, got, s.active)
		}
		removed := ps.CleanupStale()
		if (len(removed) == 1) != s.removed {
			t.Fatalf("%s: CleanupStale removed %v, want removed=%v", s.name, removed, s.removed)
		}
	}
	if ps.Count() != 0 {
		t.Fatalf("Count = %d, want the restored peer gone", ps.Count())
	}
}

func TestRestoredPeerIsNotFirstHandAndLeavesNoTombstone(t *testing.T) {
	enforceLiveness(t, true)
	cur := livenessNow
	ps := movableStore(&cur)
	restoredOffer(ps, "k1", cur.Add(-time.Hour))

	ps.mu.RLock()
	first := ps.peers["k1"].heardFirstHand
	ps.mu.RUnlock()
	if first {
		t.Fatal("a restored peer is first-hand")
	}
	cur = livenessNow.Add(PeerRemoveTimeout + time.Second)
	if removed := ps.CleanupStale(); len(removed) != 1 {
		t.Fatalf("CleanupStale removed %v, want the restored peer", removed)
	}
	if _, ok := ps.tombstones["k1"]; ok {
		t.Fatal("CleanupStale of a restored, never heard peer left a tombstone")
	}
	if d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k1", MeshIP: "10.0.0.2"}, Source: "cache", Hand: Restored}); d.Verdict == Refuse {
		t.Fatalf("a restore after the removal was refused: %v", d.Reason)
	}
}
