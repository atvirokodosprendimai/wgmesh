package node

import (
	"fmt"
	"reflect"
	"testing"
)

// fakeDerive stands in for the daemon's closure: the address a key derives
// for a nonce.
func fakeDerive(key string, nonce int) string {
	return fmt.Sprintf("10.%s.0.%d", key, nonce)
}

func setEnforce(t *testing.T, on bool) {
	t.Helper()
	old := EnforceIPBinding
	EnforceIPBinding = on
	t.Cleanup(func() { EnforceIPBinding = old })
}

func TestClaimOrderBoundBeatsUnbound(t *testing.T) {
	setEnforce(t, true)

	rotated := Claim{Key: "aaa", IP: "10.9.9.9"}
	for _, bound := range []Claim{
		{Key: "zzz", IP: fakeDerive("zzz", 0)},
		{Key: "zzz", IP: fakeDerive("zzz", 3)},
	} {
		var warned []string
		warn := func(rule string) { warned = append(warned, rule) }
		for _, pair := range [][2]Claim{{bound, rotated}, {rotated, bound}} {
			w, l := Winner(pair[0], pair[1], fakeDerive, warn)
			if w != bound || l != rotated {
				t.Errorf("Winner(%v, %v) = %v, %v; want the bound claim %v first", pair[0], pair[1], w, l, bound)
			}
		}
		if len(warned) != 0 {
			t.Errorf("enforced order warned %v; want nothing", warned)
		}
	}

	// A nil derive binds nothing, so the lower key decides.
	bound := Claim{Key: "zzz", IP: fakeDerive("zzz", 0)}
	if w, _ := Winner(bound, rotated, nil, nil); w != rotated {
		t.Errorf("nil derive: winner = %v, want the lower key %v", w, rotated)
	}
}

func TestClaimOrderFallsToLowerKey(t *testing.T) {
	cases := []struct {
		name string
		a, b Claim
	}{
		{"both bound", Claim{Key: "aaa", IP: fakeDerive("aaa", 0)}, Claim{Key: "bbb", IP: fakeDerive("bbb", 2)}},
		{"both unbound", Claim{Key: "aaa", IP: "10.1.1.1"}, Claim{Key: "bbb", IP: "10.1.1.1"}},
	}
	for _, enforce := range []bool{true, false} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s enforce=%v", tc.name, enforce), func(t *testing.T) {
				setEnforce(t, enforce)
				for _, pair := range [][2]Claim{{tc.a, tc.b}, {tc.b, tc.a}} {
					w, l := Winner(pair[0], pair[1], fakeDerive, nil)
					if w != tc.a || l != tc.b {
						t.Errorf("Winner(%v, %v) = %v, %v; want the lower key %v", pair[0], pair[1], w, l, tc.a)
					}
				}
			})
		}
	}
}

func TestWinnerLegacyOrderWarnsWhenBoundOrderDiffers(t *testing.T) {
	setEnforce(t, false)

	lowerUnbound := Claim{Key: "aaa", IP: "10.9.9.9"}
	higherBound := Claim{Key: "zzz", IP: fakeDerive("zzz", 0)}
	var warned []string
	warn := func(rule string) { warned = append(warned, rule) }

	w, l := Winner(higherBound, lowerUnbound, fakeDerive, warn)
	if w != lowerUnbound || l != higherBound {
		t.Errorf("legacy order: winner = %v, loser = %v; want the lower key to win", w, l)
	}
	if !reflect.DeepEqual(warned, []string{"winner-differs"}) {
		t.Errorf("warnings = %v, want [winner-differs]", warned)
	}

	warned = nil
	Winner(Claim{Key: "aaa", IP: "10.1.1.1"}, Claim{Key: "bbb", IP: "10.1.1.1"}, fakeDerive, warn)
	Winner(lowerUnbound, higherBound, nil, warn)
	if len(warned) != 0 {
		t.Errorf("warned %v where both orders agree; want nothing", warned)
	}
}

func TestResolveClaimsIncludesLocal(t *testing.T) {
	local := Claim{Key: "k-local", IP: "10.1.1.1"}
	peers := []*PeerInfo{
		{WGPubKey: "k-remote", MeshIP: "10.1.1.1"},
		{WGPubKey: "k-other", MeshIP: "10.2.2.2"},
	}

	got := ResolveClaims(local, peers, fakeDerive)
	if len(got) != 1 {
		t.Fatalf("collisions = %v, want exactly one", got)
	}
	if got[0].IP != "10.1.1.1" || got[0].A != local || got[0].B != (Claim{Key: "k-remote", IP: "10.1.1.1"}) {
		t.Errorf("collision = %v, want the lower key (local) first against k-remote on 10.1.1.1", got[0])
	}

	// The list does not depend on the order the peers were stored in.
	three := []*PeerInfo{
		{WGPubKey: "k-c", MeshIP: "10.3.3.3"},
		{WGPubKey: "k-a", MeshIP: "10.3.3.3"},
		{WGPubKey: "k-b", MeshIP: "10.3.3.3"},
		{WGPubKey: "k-x", MeshIP: "10.0.0.5"},
		{WGPubKey: "k-y", MeshIP: "10.0.0.5"},
	}
	reversed := make([]*PeerInfo, len(three))
	for i, p := range three {
		reversed[len(three)-1-i] = p
	}
	forward := ResolveClaims(local, three, nil)
	if !reflect.DeepEqual(forward, ResolveClaims(local, reversed, nil)) {
		t.Errorf("ResolveClaims depends on peer order: %v", forward)
	}
	wantForward := []Collision{
		{IP: "10.0.0.5", A: Claim{Key: "k-x", IP: "10.0.0.5"}, B: Claim{Key: "k-y", IP: "10.0.0.5"}},
		{IP: "10.3.3.3", A: Claim{Key: "k-a", IP: "10.3.3.3"}, B: Claim{Key: "k-b", IP: "10.3.3.3"}},
		{IP: "10.3.3.3", A: Claim{Key: "k-a", IP: "10.3.3.3"}, B: Claim{Key: "k-c", IP: "10.3.3.3"}},
	}
	if !reflect.DeepEqual(forward, wantForward) {
		t.Errorf("collisions = %v, want %v: the lower key first, each other claim against it, sorted by IP then key", forward, wantForward)
	}

	if got := ResolveClaims(local, []*PeerInfo{{WGPubKey: "k-local", MeshIP: "10.1.1.1"}}, nil); len(got) != 0 {
		t.Errorf("the local key stored as a peer collided with itself: %v", got)
	}
}

func TestBoundAcceptsNonceOneToTen(t *testing.T) {
	const key = "kkk"
	for nonce := 0; nonce <= 10; nonce++ {
		if !Bound(Claim{Key: key, IP: fakeDerive(key, nonce)}, fakeDerive, MaxNonce) {
			t.Errorf("an address derived with nonce %d is not bound", nonce)
		}
	}
	if Bound(Claim{Key: key, IP: fakeDerive(key, 11)}, fakeDerive, MaxNonce) {
		t.Error("an address derived with nonce 11 is bound")
	}
	if Bound(Claim{Key: key, IP: fakeDerive("other", 0)}, fakeDerive, MaxNonce) {
		t.Error("an address derived from another key is bound")
	}
	if Bound(Claim{Key: key, IP: fakeDerive(key, 0)}, nil, MaxNonce) {
		t.Error("a nil derive bound a claim")
	}
	if Bound(Claim{Key: key, IP: ""}, func(string, int) string { return "" }, MaxNonce) {
		t.Error("an empty address is bound to a derivation that gives an empty address")
	}
}
