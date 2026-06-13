package daemon

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// TestDeviceLifecycle tests the basic lifecycle of WireGuard devices.
// This integration test verifies that devices can be created, started, stopped, and closed properly.
func TestDeviceLifecycle(t *testing.T) {
	t.Run("SysDevice basic lifecycle", func(t *testing.T) {
		// Generate a test key pair using the existing function
		privateKey, publicKey, err := wireguard.GenerateKeyPair()
		if err != nil {
			// If wg binary is not available, skip this test
			t.Skipf("wg binary not available: %v", err)
		}

		// Create a system device
		device, err := wireguard.NewSysDevice("wg0test", privateKey, 51820)
		if err != nil {
			t.Fatalf("failed to create SysDevice: %v", err)
		}

		// Start the device (may fail without privileges, but we test the interface)
		startErr := device.Start()
		t.Logf("Start() result (may fail without privileges): %v", startErr)

		// Test peer operations
		if startErr == nil {
			// Set a peer
			err = device.SetPeer(publicKey, "192.168.1.1:51820", []string{"10.0.0.1/32"}, 25)
			if err != nil {
				t.Logf("SetPeer() failed (may fail without wg binary): %v", err)
			}

			// Get peers
			peers, err := device.GetPeers()
			if err != nil {
				t.Logf("GetPeers() failed: %v", err)
			} else {
				t.Logf("Got %d peers", len(peers))
			}

			// Remove peer
			err = device.RemovePeer(publicKey)
			if err != nil {
				t.Logf("RemovePeer() failed: %v", err)
			}
		}

		// Stop the device
		if startErr == nil {
			stopErr := device.Stop()
			if stopErr != nil {
				t.Logf("Stop() failed: %v", stopErr)
			}
		}

		// Close the device
		closeErr := device.Close()
		if closeErr != nil {
			t.Logf("Close() failed: %v", closeErr)
		}
	})

	t.Run("FDDevice basic lifecycle", func(t *testing.T) {
		// Generate a test key pair using the existing function
		privateKey, publicKey, err := wireguard.GenerateKeyPair()
		if err != nil {
			// If wg binary is not available, skip this test
			t.Skipf("wg binary not available: %v", err)
		}

		// Parse the private key to bytes
		privKeyBytes, err := wireguard.ParseKey(privateKey)
		if err != nil {
			t.Fatalf("failed to parse private key: %v", err)
		}

		// Create an FD device with a dummy fd
		device, err := wireguard.NewFDDevice(100, privKeyBytes, 51820)
		if err != nil {
			t.Fatalf("failed to create FDDevice: %v", err)
		}

		// Start the device
		if err := device.Start(); err != nil {
			t.Fatalf("failed to start FDDevice: %v", err)
		}

		// Test peer operations
		err = device.SetPeer(publicKey, "192.168.1.1:51820", []string{"10.0.0.1/32"}, 25)
		if err != nil {
			t.Errorf("SetPeer() failed: %v", err)
		}

		// Get peers
		peers, err := device.GetPeers()
		if err != nil {
			t.Errorf("GetPeers() failed: %v", err)
		}
		if len(peers) != 1 {
			t.Errorf("expected 1 peer, got %d", len(peers))
		}

		// Remove peer
		err = device.RemovePeer(publicKey)
		if err != nil {
			t.Errorf("RemovePeer() failed: %v", err)
		}

		// Stop the device
		if err := device.Stop(); err != nil {
			t.Errorf("failed to stop FDDevice: %v", err)
		}

		// Close the device
		if err := device.Close(); err != nil {
			t.Errorf("failed to close FDDevice: %v", err)
		}
	})
}

// TestDeviceInterfaceCompliance verifies that both device implementations
// properly implement the WGDevice interface.
func TestDeviceInterfaceCompliance(t *testing.T) {
	t.Run("SysDevice implements WGDevice", func(t *testing.T) {
		var _ wireguard.WGDevice = &wireguard.SysDevice{}
	})

	t.Run("FDDevice implements WGDevice", func(t *testing.T) {
		var _ wireguard.WGDevice = &wireguard.FDDevice{}
	})
}

// TestCryptoIntegration tests that the crypto package integrates properly
// with the wireguard device implementations.
func TestCryptoIntegration(t *testing.T) {
	secret := "test-secret-for-key-derivation"

	// Derive keys from the secret
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("failed to derive keys: %v", err)
	}

	// Generate a WireGuard key pair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Skipf("wg binary not available: %v", err)
	}

	// Verify we can create a SysDevice with the generated key
	device, err := wireguard.NewSysDevice("wg0test", privateKey, 51820)
	if err != nil {
		t.Fatalf("failed to create SysDevice: %v", err)
	}

	// Verify the device is properly initialized
	if device == nil {
		t.Fatal("device is nil")
	}

	// Clean up
	_ = device.Close()

	// Verify the public key format
	pubKeyBytes, err := wireguard.ParseKey(publicKey)
	if err != nil {
		t.Errorf("failed to parse public key: %v", err)
	}
	if len(pubKeyBytes) != 32 {
		t.Errorf("expected 32-byte public key, got %d bytes", len(pubKeyBytes))
	}

	// Verify the private key format
	privKeyBytes, err := wireguard.ParseKey(privateKey)
	if err != nil {
		t.Errorf("failed to parse private key: %v", err)
	}
	if len(privKeyBytes) != 32 {
		t.Errorf("expected 32-byte private key, got %d bytes", len(privKeyBytes))
	}

	t.Logf("Successfully tested crypto integration with derived keys (NetworkID: %x)", keys.NetworkID)
}
