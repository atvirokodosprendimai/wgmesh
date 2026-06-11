package wireguard

import (
	"sync"
	"testing"
)

func TestNewFDDevice(t *testing.T) {
	tests := []struct {
		name       string
		fd         int
		privateKey []byte
		listenPort int
		wantErr    bool
	}{
		{
			name:       "valid device",
			fd:         100,
			privateKey: make([]byte, 32),
			listenPort: 51820,
			wantErr:    false,
		},
		{
			name:       "invalid fd - negative",
			fd:         -1,
			privateKey: make([]byte, 32),
			listenPort: 51820,
			wantErr:    true,
		},
		{
			name:       "empty private key",
			fd:         100,
			privateKey: []byte{},
			listenPort: 51820,
			wantErr:    true,
		},
		{
			name:       "invalid private key length - too short",
			fd:         100,
			privateKey: make([]byte, 16),
			listenPort: 51820,
			wantErr:    true,
		},
		{
			name:       "invalid private key length - too long",
			fd:         100,
			privateKey: make([]byte, 64),
			listenPort: 51820,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, err := NewFDDevice(tt.fd, tt.privateKey, tt.listenPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFDDevice() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if device == nil {
					t.Errorf("NewFDDevice() returned nil device")
				}
			}
		})
	}
}

func TestFDDevice_StartStop(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	// Test start
	if err := device.Start(); err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	// Test start again (should be idempotent)
	if err := device.Start(); err != nil {
		t.Errorf("Start() again failed: %v", err)
	}

	// Test stop
	if err := device.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Test stop again (should be idempotent)
	if err := device.Stop(); err != nil {
		t.Errorf("Stop() again failed: %v", err)
	}
}

func TestFDDevice_SetPeer(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	if err := device.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	tests := []struct {
		name                 string
		pubKey               string
		endpoint             string
		allowedIPs           []string
		persistentKeepalive  int
		wantErr              bool
	}{
		{
			name:                "valid peer",
			pubKey:              "test-public-key-1",
			endpoint:            "192.168.1.1:51820",
			allowedIPs:          []string{"10.0.0.1/32"},
			persistentKeepalive: 25,
			wantErr:             false,
		},
		{
			name:                "empty public key",
			pubKey:              "",
			endpoint:            "192.168.1.1:51820",
			allowedIPs:          []string{"10.0.0.1/32"},
			persistentKeepalive: 25,
			wantErr:             true,
		},
		{
			name:                "peer with multiple allowed IPs",
			pubKey:              "test-public-key-2",
			endpoint:            "192.168.1.2:51820",
			allowedIPs:          []string{"10.0.0.2/32", "fd00::2/128"},
			persistentKeepalive: 25,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := device.SetPeer(tt.pubKey, tt.endpoint, tt.allowedIPs, tt.persistentKeepalive)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetPeer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFDDevice_RemovePeer(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	if err := device.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Add a peer
	pubKey := "test-public-key-1"
	device.SetPeer(pubKey, "192.168.1.1:51820", []string{"10.0.0.1/32"}, 25)

	// Verify it exists
	peers, err := device.GetPeers()
	if err != nil {
		t.Fatalf("GetPeers() failed: %v", err)
	}
	if len(peers) != 1 {
		t.Errorf("peer count before remove: got %d, want 1", len(peers))
	}

	// Remove the peer
	if err := device.RemovePeer(pubKey); err != nil {
		t.Errorf("RemovePeer() failed: %v", err)
	}

	// Verify it's gone
	peers, err = device.GetPeers()
	if err != nil {
		t.Fatalf("GetPeers() after remove failed: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("peer count after remove: got %d, want 0", len(peers))
	}

	// Test removing non-existent peer (should not error)
	if err := device.RemovePeer("non-existent-key"); err != nil {
		t.Errorf("RemovePeer() non-existent failed: %v", err)
	}

	// Test removing empty key (should error)
	if err := device.RemovePeer(""); err == nil {
		t.Errorf("RemovePeer() empty key expected error, got nil")
	}
}

func TestFDDevice_GetPeers(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	if err := device.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Initially no peers
	peers, err := device.GetPeers()
	if err != nil {
		t.Fatalf("GetPeers() failed: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("initial peer count: got %d, want 0", len(peers))
	}

	// Add some peers
	device.SetPeer("peer1", "endpoint1", []string{"10.0.0.1/32"}, 25)
	device.SetPeer("peer2", "endpoint2", []string{"10.0.0.2/32"}, 25)

	// Get peers
	peers, err = device.GetPeers()
	if err != nil {
		t.Fatalf("GetPeers() failed: %v", err)
	}
	if len(peers) != 2 {
		t.Errorf("peer count after adding: got %d, want 2", len(peers))
	}
}

func TestFDDevice_Close(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	if err := device.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Add a peer
	device.SetPeer("peer1", "endpoint1", []string{"10.0.0.1/32"}, 25)

	// Close the device
	if err := device.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Verify peers are cleared
	peers, err := device.GetPeers()
	if err != nil {
		t.Fatalf("GetPeers() after Close failed: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("peer count after Close: got %d, want 0", len(peers))
	}

	// Close again (should be idempotent)
	if err := device.Close(); err != nil {
		t.Errorf("Close() again failed: %v", err)
	}
}

func TestFDDevice_ConcurrentAccess(t *testing.T) {
	device, err := NewFDDevice(100, make([]byte, 32), 51820)
	if err != nil {
		t.Fatalf("NewFDDevice() failed: %v", err)
	}

	if err := device.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	var wg sync.WaitGroup
	// Launch concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func(idx int) {
			defer wg.Done()
			device.SetPeer("peer"+string(rune(idx)), "endpoint", []string{"10.0.0.1/32"}, 25)
		}(i)
		go func(idx int) {
			defer wg.Done()
			device.GetPeers()
		}(i)
		go func(idx int) {
			defer wg.Done()
			device.RemovePeer("peer" + string(rune(idx)))
		}(i)
	}
	wg.Wait()
}
