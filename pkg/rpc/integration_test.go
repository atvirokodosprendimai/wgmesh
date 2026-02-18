package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientServerIntegration(t *testing.T) {
	// Unix socket paths are limited to ~104 chars on macOS. Use /tmp directly
	// with a short unique name rather than t.TempDir() which produces long paths.
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("wg-rpc-%d.sock", os.Getpid()))
	t.Cleanup(func() { os.Remove(socketPath) })

	// Mock peer data
	mockPeer := &PeerData{
		WGPubKey:         "test-pubkey-abc123",
		MeshIP:           "10.42.0.5",
		Endpoint:         "203.0.113.10:51820",
		LastSeen:         time.Now(),
		DiscoveredVia:    []string{"dht", "gossip"},
		RoutableNetworks: []string{"192.168.1.0/24"},
	}

	mockStatus := &StatusData{
		MeshIP:    "10.42.0.1",
		PubKey:    "local-pubkey-xyz789",
		Uptime:    5 * time.Minute,
		Interface: "wg0",
	}

	// Create server
	config := ServerConfig{
		SocketPath: socketPath,
		Version:    "test-v1.0",
		GetPeers: func() []*PeerData {
			return []*PeerData{mockPeer}
		},
		GetPeer: func(pubKey string) (*PeerData, bool) {
			if pubKey == mockPeer.WGPubKey {
				return mockPeer, true
			}
			return nil, false
		},
		GetPeerCounts: func() (active, total, dead int) {
			return 1, 1, 0
		},
		GetStatus: func() *StatusData {
			return mockStatus
		},
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	// Create client (retry logic with timeout to handle server startup)
	var client *Client
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		client, err = NewClient(socketPath)
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			t.Fatalf("failed to create client after %d retries: %v", maxRetries, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer client.Close()

	// Test daemon.ping
	t.Run("daemon.ping", func(t *testing.T) {
		result, err := client.Call("daemon.ping", nil)
		if err != nil {
			t.Fatalf("daemon.ping failed: %v", err)
		}

		resultMap := result.(map[string]interface{})
		if resultMap["pong"] != true {
			t.Error("expected pong to be true")
		}
		if resultMap["version"] != "test-v1.0" {
			t.Errorf("expected version test-v1.0, got %v", resultMap["version"])
		}
	})

	// Test peers.list
	t.Run("peers.list", func(t *testing.T) {
		result, err := client.Call("peers.list", nil)
		if err != nil {
			t.Fatalf("peers.list failed: %v", err)
		}

		resultMap := result.(map[string]interface{})
		peers := resultMap["peers"].([]interface{})
		if len(peers) != 1 {
			t.Fatalf("expected 1 peer, got %d", len(peers))
		}

		peer := peers[0].(map[string]interface{})
		if peer["pubkey"] != mockPeer.WGPubKey {
			t.Errorf("expected pubkey %s, got %v", mockPeer.WGPubKey, peer["pubkey"])
		}
		if peer["mesh_ip"] != mockPeer.MeshIP {
			t.Errorf("expected mesh_ip %s, got %v", mockPeer.MeshIP, peer["mesh_ip"])
		}
	})

	// Test peers.get
	t.Run("peers.get", func(t *testing.T) {
		params := map[string]interface{}{
			"pubkey": mockPeer.WGPubKey,
		}
		result, err := client.Call("peers.get", params)
		if err != nil {
			t.Fatalf("peers.get failed: %v", err)
		}

		peer := result.(map[string]interface{})
		if peer["pubkey"] != mockPeer.WGPubKey {
			t.Errorf("expected pubkey %s, got %v", mockPeer.WGPubKey, peer["pubkey"])
		}
	})

	// Test peers.get with invalid pubkey
	t.Run("peers.get invalid", func(t *testing.T) {
		params := map[string]interface{}{
			"pubkey": "nonexistent-key",
		}
		_, err := client.Call("peers.get", params)
		if err == nil {
			t.Error("expected error for nonexistent peer")
		}
	})

	// Test peers.count
	t.Run("peers.count", func(t *testing.T) {
		result, err := client.Call("peers.count", nil)
		if err != nil {
			t.Fatalf("peers.count failed: %v", err)
		}

		counts := result.(map[string]interface{})
		if int(counts["active"].(float64)) != 1 {
			t.Errorf("expected 1 active peer, got %v", counts["active"])
		}
		if int(counts["total"].(float64)) != 1 {
			t.Errorf("expected 1 total peer, got %v", counts["total"])
		}
		if int(counts["dead"].(float64)) != 0 {
			t.Errorf("expected 0 dead peers, got %v", counts["dead"])
		}
	})

	// Test daemon.status
	t.Run("daemon.status", func(t *testing.T) {
		result, err := client.Call("daemon.status", nil)
		if err != nil {
			t.Fatalf("daemon.status failed: %v", err)
		}

		status := result.(map[string]interface{})
		if status["mesh_ip"] != mockStatus.MeshIP {
			t.Errorf("expected mesh_ip %s, got %v", mockStatus.MeshIP, status["mesh_ip"])
		}
		if status["pubkey"] != mockStatus.PubKey {
			t.Errorf("expected pubkey %s, got %v", mockStatus.PubKey, status["pubkey"])
		}
	})

	// Test invalid method
	t.Run("invalid method", func(t *testing.T) {
		_, err := client.Call("invalid.method", nil)
		if err == nil {
			t.Error("expected error for invalid method")
		}
	})
}
