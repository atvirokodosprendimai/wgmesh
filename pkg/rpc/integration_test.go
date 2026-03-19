package rpc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/atvirokodosprendimai/wgmesh/pkg/rpc/proto"
)

func TestClientServerIntegration(t *testing.T) {
	// Unix socket paths are limited to ~104 chars on macOS. Use /tmp directly
	// with a short unique name rather than t.TempDir() which produces long paths.
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("wg-rpc-%d.sock", os.Getpid()))
	t.Cleanup(func() { os.Remove(socketPath) })

	// Mock peer data using generated proto types
	mockPeer := &pb.PeerInfo{
		Pubkey:           "test-pubkey-abc123",
		Hostname:         "node-test-1",
		MeshIp:           "10.42.0.5",
		Endpoint:         "203.0.113.10:51820",
		LastSeen:         time.Now().Format(time.RFC3339),
		DiscoveredVia:    []string{"dht", "gossip"},
		RoutableNetworks: []string{"192.168.1.0/24"},
	}

	// Mock peer without hostname (to test fallback behaviour)
	mockPeerNoHostname := &pb.PeerInfo{
		Pubkey:        "test-pubkey-nohostname",
		MeshIp:        "10.42.0.6",
		Endpoint:      "203.0.113.11:51820",
		LastSeen:      time.Now().Format(time.RFC3339),
		DiscoveredVia: []string{"lan"},
	}

	mockStatus := &pb.StatusData{
		MeshIp:    "10.42.0.1",
		Pubkey:    "local-pubkey-xyz789",
		Uptime:    int64(5 * time.Minute),
		Interface: "wg0",
	}

	// Create server
	config := ServerConfig{
		SocketPath: socketPath,
		Version:    "test-v1.0",
		GetPeers: func() []*pb.PeerInfo {
			return []*pb.PeerInfo{mockPeer, mockPeerNoHostname}
		},
		GetPeer: func(pubKey string) (*pb.PeerInfo, bool) {
			switch pubKey {
			case mockPeer.Pubkey:
				return mockPeer, true
			case mockPeerNoHostname.Pubkey:
				return mockPeerNoHostname, true
			}
			return nil, false
		},
		GetPeerCounts: func() (active, total, dead int) {
			return 2, 2, 0
		},
		GetStatus: func() *pb.StatusData {
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

	// unmarshalResult re-encodes the interface{} result from client.Call and
	// unmarshals it into the given proto-generated target struct.
	unmarshalResult := func(t *testing.T, result interface{}, target interface{}) {
		t.Helper()
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to encode RPC result: %v", err)
		}
		if err := json.Unmarshal(b, target); err != nil {
			t.Fatalf("failed to decode RPC result: %v", err)
		}
	}

	// Test daemon.ping
	t.Run("daemon.ping", func(t *testing.T) {
		result, err := client.Call("daemon.ping", nil)
		if err != nil {
			t.Fatalf("daemon.ping failed: %v", err)
		}

		var ping pb.DaemonPingResult
		unmarshalResult(t, result, &ping)
		if !ping.Pong {
			t.Error("expected pong to be true")
		}
		if ping.Version != "test-v1.0" {
			t.Errorf("expected version test-v1.0, got %v", ping.Version)
		}
	})

	// Test peers.list
	t.Run("peers.list", func(t *testing.T) {
		result, err := client.Call("peers.list", nil)
		if err != nil {
			t.Fatalf("peers.list failed: %v", err)
		}

		var list pb.PeersListResult
		unmarshalResult(t, result, &list)
		if len(list.Peers) != 2 {
			t.Fatalf("expected 2 peers, got %d", len(list.Peers))
		}

		peer := list.Peers[0]
		if peer.Pubkey != mockPeer.Pubkey {
			t.Errorf("expected pubkey %s, got %v", mockPeer.Pubkey, peer.Pubkey)
		}
		if peer.MeshIp != mockPeer.MeshIp {
			t.Errorf("expected mesh_ip %s, got %v", mockPeer.MeshIp, peer.MeshIp)
		}
		// Hostname must be present and correct when set
		if peer.Hostname != mockPeer.Hostname {
			t.Errorf("expected hostname %s, got %v", mockPeer.Hostname, peer.Hostname)
		}

		// Second peer has no hostname — field must be absent or empty string
		peerNoHostname := list.Peers[1]
		if peerNoHostname.Pubkey != mockPeerNoHostname.Pubkey {
			t.Errorf("expected pubkey %s, got %v", mockPeerNoHostname.Pubkey, peerNoHostname.Pubkey)
		}
		if peerNoHostname.Hostname != "" {
			t.Errorf("expected empty hostname for peer without hostname, got %v", peerNoHostname.Hostname)
		}
	})

	// Test peers.get
	t.Run("peers.get", func(t *testing.T) {
		params := map[string]interface{}{
			"pubkey": mockPeer.Pubkey,
		}
		result, err := client.Call("peers.get", params)
		if err != nil {
			t.Fatalf("peers.get failed: %v", err)
		}

		var peer pb.PeerInfo
		unmarshalResult(t, result, &peer)
		if peer.Pubkey != mockPeer.Pubkey {
			t.Errorf("expected pubkey %s, got %v", mockPeer.Pubkey, peer.Pubkey)
		}
		// Hostname must flow through peers.get as well
		if peer.Hostname != mockPeer.Hostname {
			t.Errorf("expected hostname %s, got %v", mockPeer.Hostname, peer.Hostname)
		}
	})

	// Test peers.get for peer without hostname
	t.Run("peers.get no hostname", func(t *testing.T) {
		params := map[string]interface{}{
			"pubkey": mockPeerNoHostname.Pubkey,
		}
		result, err := client.Call("peers.get", params)
		if err != nil {
			t.Fatalf("peers.get failed: %v", err)
		}
		var peer pb.PeerInfo
		unmarshalResult(t, result, &peer)
		if peer.Hostname != "" {
			t.Errorf("expected empty hostname for peer without hostname, got %v", peer.Hostname)
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

		var counts pb.PeersCountResult
		unmarshalResult(t, result, &counts)
		if counts.Active != 2 {
			t.Errorf("expected 2 active peers, got %v", counts.Active)
		}
		if counts.Total != 2 {
			t.Errorf("expected 2 total peers, got %v", counts.Total)
		}
		if counts.Dead != 0 {
			t.Errorf("expected 0 dead peers, got %v", counts.Dead)
		}
	})

	// Test daemon.status
	t.Run("daemon.status", func(t *testing.T) {
		result, err := client.Call("daemon.status", nil)
		if err != nil {
			t.Fatalf("daemon.status failed: %v", err)
		}

		var status pb.DaemonStatusResult
		unmarshalResult(t, result, &status)
		if status.MeshIp != mockStatus.MeshIp {
			t.Errorf("expected mesh_ip %s, got %v", mockStatus.MeshIp, status.MeshIp)
		}
		if status.Pubkey != mockStatus.Pubkey {
			t.Errorf("expected pubkey %s, got %v", mockStatus.Pubkey, status.Pubkey)
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
