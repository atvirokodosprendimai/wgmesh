package node

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func legacyOffer(ps *PeerStore, info *PeerInfo, method string) Decision {
	return ps.Offer(legacyCandidate(info, method))
}

// The merged rules (d50c51e, 7414656) give the same stored fields through
// Offer as they did through Update.
func TestOfferMatchesLegacyUpdate(t *testing.T) {
	t.Parallel()

	const key = "k"
	direct := func() *PeerInfo {
		return &PeerInfo{WGPubKey: key, MeshIP: "10.1.2.3", MeshIPv6: "fd00::3", RoutableNetworks: []string{"192.168.5.0/24"}, Introducer: true}
	}
	claim := func() *PeerInfo {
		return &PeerInfo{WGPubKey: key, MeshIP: "10.9.9.9", MeshIPv6: "fd00::9", RoutableNetworks: []string{"0.0.0.0/0"}}
	}

	for _, method := range []string{"dht-transitive", "gossip-transitive"} {
		for _, via := range []string{"offer", "update"} {
			ps := NewPeerStore()
			apply := func(info *PeerInfo, m string) {
				if via == "offer" {
					legacyOffer(ps, info, m)
				} else {
					ps.Update(info, m)
				}
			}
			apply(direct(), "dht")
			apply(claim(), method)
			got, _ := ps.Get(key)
			if got == nil || got.MeshIP != "10.1.2.3" || got.MeshIPv6 != "fd00::3" || !got.Introducer ||
				!reflect.DeepEqual(got.RoutableNetworks, []string{"192.168.5.0/24"}) {
				t.Errorf("%s/%s: transitive rewrote first-hand fields: %+v", via, method, got)
			}
		}
	}

	ps := NewPeerStore()
	legacyOffer(ps, &PeerInfo{WGPubKey: key}, "dht")
	legacyOffer(ps, &PeerInfo{WGPubKey: key, MeshIP: "10.3.3.3", MeshIPv6: "fd00::33", RoutableNetworks: []string{"10.7.0.0/16"}}, "gossip-transitive")
	got, _ := ps.Get(key)
	if got == nil || got.MeshIP != "10.3.3.3" || got.MeshIPv6 != "fd00::33" || !reflect.DeepEqual(got.RoutableNetworks, []string{"10.7.0.0/16"}) {
		t.Errorf("transitive did not fill empty fields: %+v", got)
	}

	for _, tt := range []struct {
		method   string
		withdraw bool
	}{
		{"dht", true}, {"lan", true}, {"gossip", true}, {"dht-rendezvous", true},
		{"cache", false}, {"dht-transitive", false}, {"gossip-transitive", false},
	} {
		ps := NewPeerStore()
		legacyOffer(ps, &PeerInfo{WGPubKey: key, RoutableNetworks: []string{"10.5.0.0/16"}}, "dht")
		legacyOffer(ps, &PeerInfo{WGPubKey: key}, tt.method)
		got, _ := ps.Get(key)
		if got == nil {
			t.Fatalf("%s: peer vanished", tt.method)
		}
		if tt.withdraw && len(got.RoutableNetworks) != 0 {
			t.Errorf("%s: empty announcement did not withdraw routes: %v", tt.method, got.RoutableNetworks)
		}
		if !tt.withdraw && !reflect.DeepEqual(got.RoutableNetworks, []string{"10.5.0.0/16"}) {
			t.Errorf("%s: routes wrongly withdrawn: %v", tt.method, got.RoutableNetworks)
		}
	}

	ps = NewPeerStore()
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: key, MeshIP: "10.1.1.1", Endpoint: "1.2.3.4:5"}, Source: "dht-ipv6-sync", Hand: FirstHand})
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: key}, Source: "dht", Hand: SecondHand})
	got, _ = ps.Get(key)
	if want := []string{"dht-ipv6-sync", "dht-transitive"}; got == nil || !reflect.DeepEqual(got.DiscoveredVia, want) || got.EndpointMethod != "dht-ipv6-sync" {
		t.Errorf("method strings changed: %+v", got)
	}
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "c", Endpoint: "1.2.3.4:6"}, Source: "cache", Hand: Restored})
	got, _ = ps.Get("c")
	if got == nil || !reflect.DeepEqual(got.DiscoveredVia, []string{"cache"}) || got.EndpointMethod != "cache" {
		t.Errorf("restored method string changed: %+v", got)
	}
}

// Trust is read from Hand, not from the method string.
func TestOfferHandIsExplicitNotSubstring(t *testing.T) {
	t.Parallel()

	setup := func() *PeerStore {
		ps := NewPeerStore()
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k", MeshIP: "10.1.1.1", Introducer: true}, Source: "dht", Hand: FirstHand})
		return ps
	}
	claim := func() *PeerInfo { return &PeerInfo{WGPubKey: "k", MeshIP: "10.2.2.2"} }

	ps := setup()
	ps.Offer(Candidate{Info: claim(), Source: "dht-transitive", Hand: FirstHand})
	if got, _ := ps.Get("k"); got == nil || got.MeshIP != "10.2.2.2" || got.Introducer {
		t.Errorf("first-hand with a transitive-looking label did not rewrite: %+v", got)
	}

	ps = setup()
	ps.Offer(Candidate{Info: claim(), Source: "dht", Hand: SecondHand})
	if got, _ := ps.Get("k"); got == nil || got.MeshIP != "10.1.1.1" || !got.Introducer {
		t.Errorf("second-hand rewrote first-hand fields: %+v", got)
	}
}

func TestOfferRefusesAtPeerCapWithReason(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	for i := 0; i < DefaultMaxPeers; i++ {
		d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: fmt.Sprintf("key-%d", i)}, Source: "dht", Hand: FirstHand})
		if d.Verdict != Admit {
			t.Fatalf("peer %d refused: %+v", i, d)
		}
	}
	d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "one-too-many"}, Source: "dht", Hand: FirstHand})
	if d.Verdict != Refuse || d.Reason != "peer-cap" {
		t.Errorf("decision = %+v, want Refuse peer-cap", d)
	}
	if n := ps.Count(); n != DefaultMaxPeers {
		t.Errorf("count = %d, want %d", n, DefaultMaxPeers)
	}
	if d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "key-0", Hostname: "h"}, Source: "dht", Hand: FirstHand}); d.Verdict != Admit {
		t.Errorf("known key refused at cap: %+v", d)
	}
}

// A store with no policy admits what the old Update admitted.
func TestOfferNilPolicyIsPermissive(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	for _, h := range []Hand{FirstHand, SecondHand, Restored} {
		key := fmt.Sprintf("k%d", h)
		d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: key, MeshIP: "10.0.0.1"}, Source: "dht", Hand: h})
		if d.Verdict != Admit {
			t.Errorf("hand %d: %+v", h, d)
		}
		if _, ok := ps.Get(key); !ok {
			t.Errorf("hand %d: peer not stored", h)
		}
	}
}

func TestUpdateGoesThroughOffer(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	ps.Update(&PeerInfo{WGPubKey: "k"}, "dht")
	if ps.offers != 1 {
		t.Errorf("offers = %d after one Update, want 1", ps.offers)
	}
}

func BenchmarkOfferAtCap(b *testing.B) {
	ps := NewPeerStore()
	for i := 0; i < DefaultMaxPeers; i++ {
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: fmt.Sprintf("key-%d", i), MeshIP: "10.0.0.1"}, Source: "dht", Hand: FirstHand})
	}
	c := Candidate{Info: &PeerInfo{WGPubKey: "key-500", MeshIP: "10.0.0.2"}, Source: "dht", Hand: FirstHand}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Offer(c)
	}
}

// Only a first-hand claim moves LastSeen to now; a second-hand or restored
// claim leaves the older stamp.
func TestOfferLastSeenFollowsHand(t *testing.T) {
	t.Parallel()

	old := time.Now().Add(-time.Hour)
	for _, tt := range []struct {
		hand    Hand
		refresh bool
	}{{FirstHand, true}, {SecondHand, false}, {Restored, false}} {
		ps := NewPeerStore()
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k", LastSeen: old}, Source: "dht", Hand: FirstHand})
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k"}, Source: "dht", Hand: tt.hand})
		got, _ := ps.Get("k")
		if got == nil || got.LastSeen.After(old) != tt.refresh {
			t.Errorf("hand %d: LastSeen = %v, refreshed want %v", tt.hand, got, tt.refresh)
		}
	}
}

// A store with a nil or an empty policy admits what the old Update admitted.
func TestOfferAdmitsWithNilAndEmptyPolicy(t *testing.T) {
	t.Parallel()

	for _, p := range []*Policy{nil, {}} {
		ps := NewPeerStore()
		ps.SetPolicy(p)
		d := ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k", MeshIP: "10.0.0.1"}, Source: "dht", Hand: FirstHand})
		if _, ok := ps.Get("k"); d.Verdict != Admit || !ok {
			t.Errorf("policy %+v: decision %+v, stored %v", p, d, ok)
		}
	}
}

func BenchmarkOfferRefusedAtCap(b *testing.B) {
	ps := NewPeerStore()
	for i := 0; i < DefaultMaxPeers; i++ {
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: fmt.Sprintf("key-%d", i), MeshIP: "10.0.0.1"}, Source: "dht", Hand: FirstHand})
	}
	c := Candidate{Info: &PeerInfo{WGPubKey: "one-too-many"}, Source: "dht", Hand: FirstHand}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Offer(c)
	}
}

// clockedStore returns a store whose clock the test owns: every reading
// advances it by step, and the test moves it further through the pointer.
func clockedStore(step time.Duration) (*PeerStore, *time.Time) {
	now := time.Unix(1_700_000_000, 0)
	ps := NewPeerStore()
	ps.now = func() time.Time {
		now = now.Add(step)
		return now
	}
	return ps, &now
}

func offerHand(ps *PeerStore, key string, h Hand, introducer bool) Decision {
	return ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: key, Introducer: introducer}, Source: "dht", Hand: h})
}

func wantTombstoned(t *testing.T, d Decision, what string) {
	t.Helper()
	if d.Verdict != Refuse || d.Reason != "tombstoned" {
		t.Errorf("%s: decision = %+v, want Refuse tombstoned", what, d)
	}
}

func TestTombstoneBlocksSecondHandReadd(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	offerHand(ps, "k", FirstHand, false)
	ps.Withdraw("k", "goodbye")

	for _, h := range []Hand{SecondHand, Restored} {
		wantTombstoned(t, offerHand(ps, "k", h, true), fmt.Sprintf("hand %d", h))
		if got, ok := ps.Get("k"); ok {
			t.Errorf("hand %d: refused offer left an entry: %+v", h, got)
		}
	}
	if d := offerHand(ps, "other", SecondHand, true); d.Verdict != Admit {
		t.Errorf("a key that was never withdrawn was refused: %+v", d)
	}
}

func TestFirstHandClearsTombstone(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	offerHand(ps, "k", FirstHand, false)
	ps.Withdraw("k", "goodbye")

	if d := offerHand(ps, "k", FirstHand, false); d.Verdict != Admit {
		t.Fatalf("the peer's own first-hand offer was refused: %+v", d)
	}
	if len(ps.tombstones) != 0 {
		t.Errorf("first-hand offer left tombstones: %v", ps.tombstones)
	}
	// Drop the entry without a tombstone, so only a cleared tombstone can
	// let the next second-hand mention in.
	ps.mu.Lock()
	delete(ps.peers, "k")
	ps.mu.Unlock()
	if d := offerHand(ps, "k", SecondHand, false); d.Verdict != Admit {
		t.Errorf("second-hand offer after the clear: %+v, want Admit", d)
	}
}

func TestTombstoneExpiresAfterTombstoneTTL(t *testing.T) {
	t.Parallel()

	ps, now := clockedStore(0)
	offerHand(ps, "k", FirstHand, false)
	ps.Withdraw("k", "evicted")

	*now = now.Add(TombstoneTTL - time.Second)
	wantTombstoned(t, offerHand(ps, "k", SecondHand, false), "just before the TTL")

	*now = now.Add(2 * time.Second)
	if d := offerHand(ps, "k", SecondHand, false); d.Verdict != Admit {
		t.Errorf("just after the TTL: %+v, want Admit", d)
	}
}

func TestTombstoneTTLIsFiveMinutes(t *testing.T) {
	t.Parallel()

	ttl, remove := TombstoneTTL, PeerRemoveTimeout
	if ttl != 5*time.Minute {
		t.Errorf("TombstoneTTL = %v, want 5m", ttl)
	}
	if ttl == remove {
		t.Errorf("TombstoneTTL equals PeerRemoveTimeout (%v): it must be a constant of its own", remove)
	}
}

func TestCleanupStaleLeavesTombstone(t *testing.T) {
	t.Parallel()

	ps := NewPeerStore()
	old := time.Now().Add(-PeerRemoveTimeout - time.Minute)
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "heard", LastSeen: old}, Source: "dht", Hand: FirstHand})
	ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "mentioned", LastSeen: old}, Source: "dht", Hand: SecondHand})
	if removed := ps.CleanupStale(); len(removed) != 2 {
		t.Fatalf("CleanupStale removed %v, want both peers", removed)
	}

	wantTombstoned(t, offerHand(ps, "heard", SecondHand, false), "a peer once heard first-hand")
	if d := offerHand(ps, "mentioned", SecondHand, false); d.Verdict != Admit {
		t.Errorf("a mention-only peer was hidden by cleanup: %+v", d)
	}
}

func TestTombstoneSetIsBounded(t *testing.T) {
	t.Parallel()

	const n = 2000
	ps, _ := clockedStore(time.Millisecond)
	for i := 0; i < n; i++ {
		ps.Withdraw(fmt.Sprintf("w-%d", i), "goodbye")
	}

	if got := len(ps.tombstones); got != DefaultMaxPeers {
		t.Errorf("%d withdrawals left %d tombstones, want %d", n, got, DefaultMaxPeers)
	}
	wantTombstoned(t, offerHand(ps, fmt.Sprintf("w-%d", n-1), SecondHand, false), "the newest withdrawal")
	if d := offerHand(ps, "w-0", SecondHand, false); d.Verdict != Admit {
		t.Errorf("the oldest tombstone was kept: %+v", d)
	}
}
