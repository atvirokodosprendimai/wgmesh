package wireguard

import (
	"fmt"
	"sync"
)

// peerConfig holds the configuration for a single peer
type peerConfig struct {
	publicKey           string
	endpoint            string
	allowedIPs          []string
	persistentKeepalive int
}

// FDDevice implements WGDevice for Android VPN API integration.
// It uses a file descriptor provided by the Android VPN system.
type FDDevice struct {
	fd         int
	privateKey []byte
	listenPort int
	peers      map[string]*peerConfig
	mu         sync.RWMutex
	running    bool
	closeOnce  sync.Once
}

// NewFDDevice creates a new FDDevice with the provided file descriptor and private key.
func NewFDDevice(fd int, privateKey []byte, listenPort int) (*FDDevice, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid file descriptor: %d", fd)
	}
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("private key cannot be empty")
	}
	if len(privateKey) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(privateKey))
	}

	return &FDDevice{
		fd:         fd,
		privateKey: make([]byte, 32),
		listenPort: listenPort,
		peers:      make(map[string]*peerConfig),
	}, nil
}

// Start activates the WireGuard device.
// For FD-based devices, this begins processing traffic on the file descriptor.
func (d *FDDevice) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	// TODO: Integrate with go.zedro/go/wireguard library for actual FD operation
	// For now, we just mark the device as running
	d.running = true
	return nil
}

// ConfigureNetwork is a no-op for VPN FD devices because the host OS VPN API
// owns interface addressing and link state.
func (d *FDDevice) ConfigureNetwork(addresses []string) error {
	return nil
}

// Stop deactivates the WireGuard device.
// For FD-based devices, this stops processing and may close the file descriptor.
func (d *FDDevice) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	// TODO: Stop processing on the file descriptor
	d.running = false
	return nil
}

// SetPeer configures a peer on the device.
func (d *FDDevice) SetPeer(pubKey string, endpoint string, allowedIPs []string, persistentKeepalive int) error {
	if pubKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.peers[pubKey] = &peerConfig{
		publicKey:           pubKey,
		endpoint:            endpoint,
		allowedIPs:          allowedIPs,
		persistentKeepalive: persistentKeepalive,
	}

	// TODO: Apply peer configuration to the actual WireGuard device via FD
	return nil
}

// RemovePeer removes a peer from the device.
func (d *FDDevice) RemovePeer(pubKey string) error {
	if pubKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.peers, pubKey)

	// TODO: Remove peer from the actual WireGuard device via FD
	return nil
}

// GetPeers returns the list of configured peers.
func (d *FDDevice) GetPeers() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	peers := make([]string, 0, len(d.peers))
	for pubKey := range d.peers {
		peers = append(peers, pubKey)
	}
	return peers, nil
}

// Close performs final cleanup and resource release.
func (d *FDDevice) Close() error {
	var err error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		if d.running {
			d.running = false
			// TODO: Close file descriptor if owned by this device
		}

		d.peers = make(map[string]*peerConfig)
	})
	return err
}
