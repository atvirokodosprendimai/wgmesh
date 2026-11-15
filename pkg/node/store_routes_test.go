package node

import "testing"

// A peer that stops advertising routes announces an empty list; its own
// announcement must withdraw the old routes. A transitive mention or a stale
// cache entry with no routes says nothing about withdrawal and must not.
func TestUpdateEmptyRoutesWithdrawOnlyOnFirstHandAnnouncement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method   string
		withdraw bool
	}{
		{"dht", true},
		{"lan", true},
		{"gossip", true},
		{"dht-rendezvous", true},
		{"cache", false},
		{"dht-transitive", false},
		{"gossip-transitive", false},
	}
	for _, tt := range tests {
		ps := NewPeerStore()
		ps.Update(&PeerInfo{WGPubKey: "k", RoutableNetworks: []string{"10.5.0.0/16"}}, "dht")
		ps.Update(&PeerInfo{WGPubKey: "k"}, tt.method)

		got, _ := ps.Get("k")
		if tt.withdraw && len(got.RoutableNetworks) != 0 {
			t.Errorf("%s: empty announcement did not withdraw routes: %v", tt.method, got.RoutableNetworks)
		}
		if !tt.withdraw && len(got.RoutableNetworks) != 1 {
			t.Errorf("%s: routes wrongly withdrawn: %v", tt.method, got.RoutableNetworks)
		}
	}
}
