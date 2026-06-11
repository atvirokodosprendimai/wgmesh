package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ifname"
)

// SysDevice implements WGDevice for traditional system-managed interfaces.
// This wraps the existing command-line based operations.
type SysDevice struct {
	ifaceName  string
	privateKey string
	listenPort int
	mu         sync.RWMutex
	running    bool
	closeOnce  sync.Once
}

// NewSysDevice creates a new SysDevice for the specified interface.
func NewSysDevice(ifaceName string, privateKey string, listenPort int) (*SysDevice, error) {
	if ifaceName == "" {
		return nil, fmt.Errorf("interface name cannot be empty")
	}
	if err := ifname.Validate(ifaceName); err != nil {
		return nil, fmt.Errorf("invalid interface name: %w", err)
	}
	if privateKey == "" {
		return nil, fmt.Errorf("private key cannot be empty")
	}
	if listenPort < 0 || listenPort > 65535 {
		return nil, fmt.Errorf("invalid listen port: %d", listenPort)
	}

	return &SysDevice{
		ifaceName:  ifaceName,
		privateKey: privateKey,
		listenPort: listenPort,
	}, nil
}

// Start activates the WireGuard device.
// For system interfaces, this ensures the interface is up and configured.
func (d *SysDevice) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	var err error

	// Check if interface exists
	if interfaceExists(d.ifaceName) {
		// Reset existing interface
		err = resetInterface(d.ifaceName)
		if err != nil {
			return fmt.Errorf("failed to reset interface: %w", err)
		}
	} else {
		// Create interface
		err = createInterface(d.ifaceName)
		if err != nil {
			return fmt.Errorf("failed to create interface: %w", err)
		}
	}

	// Configure interface with private key and listen port
	err = configureInterface(d.ifaceName, d.privateKey, d.listenPort)
	if err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	// Bring interface up
	err = setInterfaceUp(d.ifaceName)
	if err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	d.running = true
	return nil
}

// Stop deactivates the WireGuard device.
// For system interfaces, this brings the interface down.
func (d *SysDevice) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	err := setInterfaceDown(d.ifaceName)
	d.running = false
	return err
}

// SetPeer configures a peer on the device using the wg set command.
func (d *SysDevice) SetPeer(pubKey string, endpoint string, allowedIPs []string, persistentKeepalive int) error {
	if pubKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Use existing SetPeer function from apply.go
	// Note: SetPeer in apply.go takes a PSK parameter, but we pass zero key
	var zeroKey [32]byte
	allowedIPsStr := ""
	if len(allowedIPs) > 0 {
		// Join allowed IPs with comma
		for i, ip := range allowedIPs {
			if i > 0 {
				allowedIPsStr += ","
			}
			allowedIPsStr += ip
		}
	}

	return SetPeer(d.ifaceName, pubKey, zeroKey, endpoint, allowedIPsStr)
}

// RemovePeer removes a peer from the device using the wg set command.
func (d *SysDevice) RemovePeer(pubKey string) error {
	if pubKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Use existing RemovePeer function from apply.go
	return RemovePeer(d.ifaceName, pubKey)
}

// GetPeers returns the list of configured peers.
func (d *SysDevice) GetPeers() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Use existing GetPeers function from apply.go
	peers, err := GetPeers(d.ifaceName)
	if err != nil {
		return nil, err
	}

	pubKeys := make([]string, 0, len(peers))
	for _, peer := range peers {
		pubKeys = append(pubKeys, peer.PublicKey)
	}
	return pubKeys, nil
}

// Close performs final cleanup and resource release.
func (d *SysDevice) Close() error {
	var err error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		if d.running {
			d.running = false
			// Bring interface down and delete it
			setInterfaceDown(d.ifaceName)
			err = deleteInterface(d.ifaceName)
		}
	})
	return err
}

// Helper functions for system interface management
// These are adapted from pkg/daemon/helpers.go to avoid circular dependencies

// wgBinPath is the absolute path to the wg binary, resolved once at package init.
// Falls back to "wg" (PATH lookup at exec time) if LookPath fails.
var wgBinPath = "wg"

// wireguardGoBinPath is the absolute path to wireguard-go, resolved once at package init.
// Falls back to "wireguard-go" if LookPath fails.
var wireguardGoBinPath = "wireguard-go"

func init() {
	if p, err := exec.LookPath("wg"); err == nil {
		wgBinPath = p
	}
	if p, err := exec.LookPath("wireguard-go"); err == nil {
		wireguardGoBinPath = p
	}
}

// interfaceExists checks if a network interface exists
func interfaceExists(name string) bool {
	switch runtime.GOOS {
	case "linux":
		_, err := os.Stat("/sys/class/net/" + name)
		return err == nil
	case "darwin":
		cmd := exec.Command("ifconfig", name)
		return cmd.Run() == nil
	default:
		return false
	}
}

// createInterface creates a WireGuard interface
func createInterface(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("ip", "link", "add", "dev", name, "type", "wireguard")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create interface: %s: %w", string(output), err)
		}
		return nil
	case "darwin":
		if wireguardGoBinPath == "wireguard-go" {
			if _, err := exec.LookPath("wireguard-go"); err != nil {
				return fmt.Errorf("wireguard-go not found in PATH (required on macOS): %w", err)
			}
		}

		cmd := exec.Command(wireguardGoBinPath, name)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start wireguard-go: %w", err)
		}

		// Wait for the process in a goroutine to prevent zombie processes
		go func() {
			cmd.Wait()
		}()

		// Give macOS a moment to materialize the utun interface
		for i := 0; i < 20; i++ {
			if interfaceExists(name) {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}

		return fmt.Errorf("wireguard interface %s was not created on macOS", name)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// configureInterface configures a WireGuard interface with private key and port
func configureInterface(name, privateKey string, listenPort int) error {
	args := []string{"set", name, "private-key", "/dev/stdin", "listen-port", fmt.Sprintf("%d", listenPort)}
	cmd := exec.Command(wgBinPath, args...)
	cmd.Stdin = strings.NewReader(privateKey + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure interface: %s: %w", string(output), err)
	}

	return nil
}

// setInterfaceUp brings an interface up
func setInterfaceUp(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("ip", "link", "set", "dev", name, "up")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to bring interface up: %s: %w", string(output), err)
		}
		return nil
	case "darwin":
		cmd := exec.Command("ifconfig", name, "up")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to bring interface up: %s: %w", string(output), err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// setInterfaceDown brings an interface down
func setInterfaceDown(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("ip", "link", "set", "dev", name, "down")
		cmd.Run() // Ignore errors - interface might not be up
		return nil
	case "darwin":
		cmd := exec.Command("ifconfig", name, "down")
		cmd.Run() // Ignore errors
		return nil
	default:
		return nil
	}
}

// deleteInterface removes the WireGuard interface from the system
func deleteInterface(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("ip", "link", "del", "dev", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			out := string(output)
			if strings.Contains(out, "Cannot find device") || strings.Contains(out, "does not exist") {
				return nil
			}
			return fmt.Errorf("failed to delete interface: %s: %w", out, err)
		}
		return nil
	case "darwin":
		cmd := exec.Command("ifconfig", name, "destroy")
		if output, err := cmd.CombinedOutput(); err != nil {
			out := string(output)
			if strings.Contains(strings.ToLower(out), "does not exist") || strings.Contains(strings.ToLower(out), "no such") {
				return nil
			}
			return fmt.Errorf("failed to delete interface: %s: %w", out, err)
		}
		return nil
	default:
		return nil
	}
}

// resetInterface resets an existing interface for reconfiguration
func resetInterface(name string) error {
	// Bring interface down first
	setInterfaceDown(name)

	switch runtime.GOOS {
	case "linux":
		// Flush all addresses
		exec.Command("ip", "addr", "flush", "dev", name).Run()
		// Remove all peers
		exec.Command(wgBinPath, "set", name, "peer", "remove").Run()
		return nil
	case "darwin":
		return nil
	default:
		return nil
	}
}
