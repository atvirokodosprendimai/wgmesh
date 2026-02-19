package wireguard

import (
	"testing"
)

func TestCalculateDiff(t *testing.T) {
	tests := []struct {
		name     string
		current  *Config
		desired  *Config
		expected *ConfigDiff
	}{
		{
			name: "identical configs",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:           "peer1-key",
						PresharedKey:        "psk1",
						Endpoint:            "192.168.1.10:51820",
						AllowedIPs:          []string{"10.99.0.2/32"},
						PersistentKeepalive: 25,
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:           "peer1-key",
						PresharedKey:        "psk1",
						Endpoint:            "192.168.1.10:51820",
						AllowedIPs:          []string{"10.99.0.2/32"},
						PersistentKeepalive: 25,
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers:    map[string]Peer{},
			},
		},
		{
			name: "added peer",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
					"peer2-key": {
						PublicKey:  "peer2-key",
						Endpoint:   "192.168.1.11:51820",
						AllowedIPs: []string{"10.99.0.3/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers: map[string]Peer{
					"peer2-key": {
						PublicKey:  "peer2-key",
						Endpoint:   "192.168.1.11:51820",
						AllowedIPs: []string{"10.99.0.3/32"},
					},
				},
				RemovedPeers:  []string{},
				ModifiedPeers: map[string]Peer{},
			},
		},
		{
			name: "removed peer",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
					"peer2-key": {
						PublicKey:  "peer2-key",
						Endpoint:   "192.168.1.11:51820",
						AllowedIPs: []string{"10.99.0.3/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{"peer2-key"},
				ModifiedPeers:    map[string]Peer{},
			},
		},
		{
			name: "modified peer - endpoint changed",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "203.0.113.50:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "203.0.113.50:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
		},
		{
			name: "modified peer - AllowedIPs changed",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32", "fd00::2/128"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32", "fd00::2/128"},
					},
				},
			},
		},
		{
			name: "interface port changed",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51821,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: true,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers:    map[string]Peer{},
			},
		},
		{
			name: "multiple changes at once",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
					"peer2-key": {
						PublicKey:  "peer2-key",
						Endpoint:   "192.168.1.11:51820",
						AllowedIPs: []string{"10.99.0.3/32"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51821,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "203.0.113.50:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
					"peer3-key": {
						PublicKey:  "peer3-key",
						Endpoint:   "192.168.1.12:51820",
						AllowedIPs: []string{"10.99.0.4/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: true,
				AddedPeers: map[string]Peer{
					"peer3-key": {
						PublicKey:  "peer3-key",
						Endpoint:   "192.168.1.12:51820",
						AllowedIPs: []string{"10.99.0.4/32"},
					},
				},
				RemovedPeers: []string{"peer2-key"},
				ModifiedPeers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "203.0.113.50:51820",
						AllowedIPs: []string{"10.99.0.2/32"},
					},
				},
			},
		},
		{
			name: "AllowedIPs order changed (but same IPs)",
			current: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"10.99.0.2/32", "fd00::2/128"},
					},
				},
			},
			desired: &Config{
				Interface: Interface{
					PrivateKey: "private-key-1",
					ListenPort: 51820,
				},
				Peers: map[string]Peer{
					"peer1-key": {
						PublicKey:  "peer1-key",
						Endpoint:   "192.168.1.10:51820",
						AllowedIPs: []string{"fd00::2/128", "10.99.0.2/32"},
					},
				},
			},
			expected: &ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers:    map[string]Peer{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CalculateDiff(tt.current, tt.desired)

			if got.InterfaceChanged != tt.expected.InterfaceChanged {
				t.Errorf("InterfaceChanged: expected %v, got %v", tt.expected.InterfaceChanged, got.InterfaceChanged)
			}

			if len(got.AddedPeers) != len(tt.expected.AddedPeers) {
				t.Errorf("AddedPeers count: expected %d, got %d", len(tt.expected.AddedPeers), len(got.AddedPeers))
			}
			for k, v := range tt.expected.AddedPeers {
				gotPeer := got.AddedPeers[k]
				if gotPeer.PublicKey != v.PublicKey ||
					gotPeer.PresharedKey != v.PresharedKey ||
					gotPeer.Endpoint != v.Endpoint ||
					gotPeer.PersistentKeepalive != v.PersistentKeepalive {
					t.Errorf("AddedPeers[%s]: expected %+v, got %+v", k, v, gotPeer)
				}
				// Compare AllowedIPs
				if len(gotPeer.AllowedIPs) != len(v.AllowedIPs) {
					t.Errorf("AddedPeers[%s]: AllowedIPs length mismatch: expected %d, got %d", k, len(v.AllowedIPs), len(gotPeer.AllowedIPs))
				} else {
					for i, ip := range v.AllowedIPs {
						if gotPeer.AllowedIPs[i] != ip {
							t.Errorf("AddedPeers[%s]: AllowedIPs[%d]: expected %s, got %s", k, i, ip, gotPeer.AllowedIPs[i])
						}
					}
				}
			}

			if len(got.RemovedPeers) != len(tt.expected.RemovedPeers) {
				t.Errorf("RemovedPeers count: expected %d, got %d", len(tt.expected.RemovedPeers), len(got.RemovedPeers))
			}
			for i, pk := range tt.expected.RemovedPeers {
				if i >= len(got.RemovedPeers) || got.RemovedPeers[i] != pk {
					t.Errorf("RemovedPeers[%d]: expected %s, got %s", i, pk, got.RemovedPeers[i])
				}
			}

			if len(got.ModifiedPeers) != len(tt.expected.ModifiedPeers) {
				t.Errorf("ModifiedPeers count: expected %d, got %d", len(tt.expected.ModifiedPeers), len(got.ModifiedPeers))
			}
			for k, v := range tt.expected.ModifiedPeers {
				gotPeer := got.ModifiedPeers[k]
				if gotPeer.PublicKey != v.PublicKey ||
					gotPeer.PresharedKey != v.PresharedKey ||
					gotPeer.Endpoint != v.Endpoint ||
					gotPeer.PersistentKeepalive != v.PersistentKeepalive {
					t.Errorf("ModifiedPeers[%s]: expected %+v, got %+v", k, v, gotPeer)
				}
				// Compare AllowedIPs
				if len(gotPeer.AllowedIPs) != len(v.AllowedIPs) {
					t.Errorf("ModifiedPeers[%s]: AllowedIPs length mismatch: expected %d, got %d", k, len(v.AllowedIPs), len(gotPeer.AllowedIPs))
				} else {
					for i, ip := range v.AllowedIPs {
						if gotPeer.AllowedIPs[i] != ip {
							t.Errorf("ModifiedPeers[%s]: AllowedIPs[%d]: expected %s, got %s", k, i, ip, gotPeer.AllowedIPs[i])
						}
					}
				}
			}
		})
	}
}

func TestPeersEqual(t *testing.T) {
	tests := []struct {
		name     string
		peerA    Peer
		peerB    Peer
		expected bool
	}{
		{
			name: "identical peers",
			peerA: Peer{
				PublicKey:           "peer1-key",
				PresharedKey:        "psk1",
				Endpoint:            "192.168.1.10:51820",
				AllowedIPs:          []string{"10.99.0.2/32"},
				PersistentKeepalive: 25,
			},
			peerB: Peer{
				PublicKey:           "peer1-key",
				PresharedKey:        "psk1",
				Endpoint:            "192.168.1.10:51820",
				AllowedIPs:          []string{"10.99.0.2/32"},
				PersistentKeepalive: 25,
			},
			expected: true,
		},
		{
			name: "different PresharedKey",
			peerA: Peer{
				PublicKey:    "peer1-key",
				PresharedKey: "psk1",
				Endpoint:     "192.168.1.10:51820",
				AllowedIPs:   []string{"10.99.0.2/32"},
			},
			peerB: Peer{
				PublicKey:    "peer1-key",
				PresharedKey: "psk2",
				Endpoint:     "192.168.1.10:51820",
				AllowedIPs:   []string{"10.99.0.2/32"},
			},
			expected: false,
		},
		{
			name: "different Endpoint",
			peerA: Peer{
				PublicKey:  "peer1-key",
				Endpoint:   "192.168.1.10:51820",
				AllowedIPs: []string{"10.99.0.2/32"},
			},
			peerB: Peer{
				PublicKey:  "peer1-key",
				Endpoint:   "203.0.113.50:51820",
				AllowedIPs: []string{"10.99.0.2/32"},
			},
			expected: false,
		},
		{
			name: "different PersistentKeepalive",
			peerA: Peer{
				PublicKey:           "peer1-key",
				PersistentKeepalive: 25,
			},
			peerB: Peer{
				PublicKey:           "peer1-key",
				PersistentKeepalive: 30,
			},
			expected: false,
		},
		{
			name: "different AllowedIPs (different IPs)",
			peerA: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.2/32"},
			},
			peerB: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.3/32"},
			},
			expected: false,
		},
		{
			name: "different AllowedIPs (different count)",
			peerA: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.2/32"},
			},
			peerB: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.2/32", "fd00::2/128"},
			},
			expected: false,
		},
		{
			name: "same AllowedIPs in different order",
			peerA: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.2/32", "fd00::2/128"},
			},
			peerB: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"fd00::2/128", "10.99.0.2/32"},
			},
			expected: true,
		},
		{
			name: "empty vs populated AllowedIPs",
			peerA: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{},
			},
			peerB: Peer{
				PublicKey:  "peer1-key",
				AllowedIPs: []string{"10.99.0.2/32"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := peersEqual(tt.peerA, tt.peerB)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestHasChanges(t *testing.T) {
	tests := []struct {
		name     string
		diff     ConfigDiff
		expected bool
	}{
		{
			name: "empty diff",
			diff: ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers:    map[string]Peer{},
			},
			expected: false,
		},
		{
			name: "InterfaceChanged only",
			diff: ConfigDiff{
				InterfaceChanged: true,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers:    map[string]Peer{},
			},
			expected: true,
		},
		{
			name: "AddedPeers only",
			diff: ConfigDiff{
				InterfaceChanged: false,
				AddedPeers: map[string]Peer{
					"peer1": {PublicKey: "peer1"},
				},
				RemovedPeers:  []string{},
				ModifiedPeers: map[string]Peer{},
			},
			expected: true,
		},
		{
			name: "RemovedPeers only",
			diff: ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{"peer1"},
				ModifiedPeers:    map[string]Peer{},
			},
			expected: true,
		},
		{
			name: "ModifiedPeers only",
			diff: ConfigDiff{
				InterfaceChanged: false,
				AddedPeers:       map[string]Peer{},
				RemovedPeers:     []string{},
				ModifiedPeers: map[string]Peer{
					"peer1": {PublicKey: "peer1"},
				},
			},
			expected: true,
		},
		{
			name: "multiple change types",
			diff: ConfigDiff{
				InterfaceChanged: true,
				AddedPeers: map[string]Peer{
					"peer1": {PublicKey: "peer1"},
				},
				RemovedPeers: []string{"peer2"},
				ModifiedPeers: map[string]Peer{
					"peer3": {PublicKey: "peer3"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.diff.HasChanges()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
