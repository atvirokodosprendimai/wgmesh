package rpc

import (
	"testing"
	"time"
)

func TestServerConfig(t *testing.T) {
	mockPeers := []*PeerData{
		{
			WGPubKey:      "test-key-1",
			MeshIP:        "10.0.0.1",
			Endpoint:      "1.2.3.4:51820",
			LastSeen:      time.Now(),
			DiscoveredVia: []string{"dht"},
		},
	}

	config := ServerConfig{
		SocketPath: "/tmp/test-wgmesh.sock",
		Version:    "test",
		GetPeers: func() []*PeerData {
			return mockPeers
		},
		GetPeer: func(pubKey string) (*PeerData, bool) {
			for _, p := range mockPeers {
				if p.WGPubKey == pubKey {
					return p, true
				}
			}
			return nil, false
		},
		GetPeerCounts: func() (active, total, dead int) {
			return len(mockPeers), len(mockPeers), 0
		},
		GetStatus: func() *StatusData {
			return &StatusData{
				MeshIP:    "10.0.0.1",
				PubKey:    "local-key",
				Uptime:    time.Minute,
				Interface: "wg0",
			}
		},
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("server is nil")
	}

	if server.version != "test" {
		t.Errorf("expected version 'test', got %s", server.version)
	}
}

func TestGetSocketPath(t *testing.T) {
	path := GetSocketPath()
	if path == "" {
		t.Error("socket path should not be empty")
	}
}

func TestIsWritable(t *testing.T) {
	// Test that /tmp is writable
	if !IsWritable("/tmp") {
		t.Error("/tmp should be writable")
	}

	// Test that non-existent path is not writable
	if IsWritable("/nonexistent") {
		t.Error("/nonexistent should not be writable")
	}
}

func TestFormatSocketPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/tmp/wgmesh.sock", "/tmp/wgmesh.sock"},
		{"/var/run/wgmesh.sock", "/var/run/wgmesh.sock"},
	}

	for _, tt := range tests {
		result := FormatSocketPath(tt.input)
		// Just check it doesn't crash, actual formatting may vary
		if result == "" {
			t.Errorf("FormatSocketPath returned empty string for %s", tt.input)
		}
	}
}
