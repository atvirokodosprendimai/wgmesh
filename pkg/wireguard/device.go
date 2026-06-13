package wireguard

// WGDevice defines the interface for managing a WireGuard device.
// This abstraction supports both system-managed interfaces (Linux/macOS)
// and application-provided file descriptors (Android VPN API).
type WGDevice interface {
	// Start activates the WireGuard device.
	// For FD-based devices, this begins processing traffic on the file descriptor.
	// For system interfaces, this ensures the interface is up and configured.
	Start() error

	// ConfigureNetwork applies device-local network configuration.
	// System-managed interfaces use this to assign addresses; mobile VPN-backed
	// implementations can no-op because the OS VPN API owns addressing/state.
	ConfigureNetwork(addresses []string) error

	// Stop deactivates the WireGuard device.
	// For FD-based devices, this stops processing and may close the file descriptor.
	// For system interfaces, this brings the interface down.
	Stop() error

	// SetPeer configures a peer on the device.
	SetPeer(pubKey string, endpoint string, allowedIPs []string, persistentKeepalive int) error

	// RemovePeer removes a peer from the device.
	RemovePeer(pubKey string) error

	// GetPeers returns the list of configured peers.
	GetPeers() ([]string, error)

	// Close performs final cleanup and resource release.
	Close() error
}
