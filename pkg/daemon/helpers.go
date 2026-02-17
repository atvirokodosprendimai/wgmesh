package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// cmdExecutor is the command executor used by helper functions.
// It can be replaced with a mock for testing.
var cmdExecutor CommandExecutor = &RealCommandExecutor{}

// localNodeState is the persisted state for a local node
type localNodeState struct {
	WGPubKey     string `json:"wg_pubkey"`
	WGPrivateKey string `json:"wg_private_key"`
}

// loadLocalNode loads the local node state from a file
func loadLocalNode(path string) (*LocalNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state localNodeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &LocalNode{
		WGPubKey:     state.WGPubKey,
		WGPrivateKey: state.WGPrivateKey,
	}, nil
}

// saveLocalNode saves the local node state to a file
func saveLocalNode(path string, node *LocalNode) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	state := localNodeState{
		WGPubKey:     node.WGPubKey,
		WGPrivateKey: node.WGPrivateKey,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write with secure permissions
	return os.WriteFile(path, data, 0600)
}

// interfaceExists checks if a network interface exists
func interfaceExists(name string) bool {
	switch runtime.GOOS {
	case "linux":
		_, err := os.Stat("/sys/class/net/" + name)
		return err == nil
	case "darwin":
		cmd := cmdExecutor.Command("ifconfig", name)
		return cmd.Run() == nil
	default:
		return false
	}
}

// createInterface creates a WireGuard interface
func createInterface(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := cmdExecutor.Command("ip", "link", "add", "dev", name, "type", "wireguard")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create interface: %s: %w", string(output), err)
		}
		return nil
	case "darwin":
		if _, err := cmdExecutor.LookPath("wireguard-go"); err != nil {
			return fmt.Errorf("wireguard-go not found in PATH (required on macOS): %w", err)
		}

		cmd := cmdExecutor.Command("wireguard-go", name)

		// Capture output for debugging/error messages
		var outBuf, errBuf strings.Builder
		cmd.SetStdout(&outBuf)
		cmd.SetStderr(&errBuf)

		// Start wireguard-go asynchronously since it's a long-running daemon
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start wireguard-go: %w", err)
		}

		// Wait for the process in a goroutine to prevent zombie processes
		// Copy the interface name to avoid capturing the loop variable
		ifaceName := name
		go func() {
			if err := cmd.Wait(); err != nil {
				// Log any errors but don't fail - wireguard-go runs as daemon
				// Read output after Wait() to avoid race conditions
				log.Printf("wireguard-go process for %s exited: %v", ifaceName, err)
				if stderr := errBuf.String(); stderr != "" {
					log.Printf("wireguard-go stderr: %s", stderr)
				}
				if stdout := outBuf.String(); stdout != "" {
					log.Printf("wireguard-go stdout: %s", stdout)
				}
			}
		}()

		// Give macOS a moment to materialize the utun interface.
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
	// Configure interface. Pass key via stdin to avoid filesystem permission issues.
	args := []string{"set", name, "private-key", "/dev/stdin", "listen-port", fmt.Sprintf("%d", listenPort)}
	cmd := cmdExecutor.Command("wg", args...)
	cmd.SetStdin(strings.NewReader(privateKey + "\n"))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure interface: %s: %w", string(output), err)
	}

	return nil
}

// setInterfaceAddress sets the IP address on an interface
func setInterfaceAddress(name, address string) error {
	switch runtime.GOOS {
	case "linux":
		ip, _, err := net.ParseCIDR(address)
		if err != nil {
			return fmt.Errorf("invalid address format: %s: %w", address, err)
		}
		if ip.To4() != nil {
			cmdExecutor.Command("ip", "-4", "addr", "flush", "dev", name).Run()
		} else {
			cmdExecutor.Command("ip", "-6", "addr", "flush", "dev", name).Run()
		}

		cmd := cmdExecutor.Command("ip", "addr", "add", address, "dev", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Ignore "file exists" error (address already set)
			if !strings.Contains(string(output), "File exists") {
				return fmt.Errorf("failed to set address: %s: %w", string(output), err)
			}
		}
		return nil
	case "darwin":
		ip, ipNet, err := net.ParseCIDR(address)
		if err != nil {
			return fmt.Errorf("invalid address format: %s: %w", address, err)
		}

		ipv4 := ip.To4()
		if ipv4 != nil {
			netmask := net.IP(ipNet.Mask).String()
			cmd := cmdExecutor.Command("ifconfig", name, "inet", ipv4.String(), ipv4.String(), "netmask", netmask, "alias")
			if output, err := cmd.CombinedOutput(); err != nil {
				if !strings.Contains(string(output), "File exists") {
					return fmt.Errorf("failed to set address: %s: %w", string(output), err)
				}
			}

			// macOS utun interfaces are point-to-point and may not add a connected
			// route for the CIDR. Ensure the mesh subnet routes via this interface.
			networkCIDR := ipNet.String()
			routeAdd := cmdExecutor.Command("route", "-n", "add", "-net", networkCIDR, "-interface", name)
			if output, err := routeAdd.CombinedOutput(); err != nil {
				out := string(output)
				if strings.Contains(out, "File exists") {
					routeChange := cmdExecutor.Command("route", "-n", "change", "-net", networkCIDR, "-interface", name)
					if changeOutput, changeErr := routeChange.CombinedOutput(); changeErr != nil {
						return fmt.Errorf("failed to update route %s via %s: %s: %w", networkCIDR, name, string(changeOutput), changeErr)
					}
				} else {
					return fmt.Errorf("failed to add route %s via %s: %s: %w", networkCIDR, name, out, err)
				}
			}

			return nil
		}

		cmd := cmdExecutor.Command("ifconfig", name, "inet6", ip.String(), "prefixlen", fmt.Sprintf("%d", maskSize(ipNet.Mask)), "alias")
		if output, err := cmd.CombinedOutput(); err != nil {
			if !strings.Contains(string(output), "File exists") {
				return fmt.Errorf("failed to set address: %s: %w", string(output), err)
			}
		}

		networkCIDR := ipNet.String()
		routeAdd := cmdExecutor.Command("route", "-n", "add", "-inet6", "-net", networkCIDR, "-interface", name)
		if output, err := routeAdd.CombinedOutput(); err != nil {
			out := string(output)
			if strings.Contains(out, "File exists") {
				routeChange := cmdExecutor.Command("route", "-n", "change", "-inet6", "-net", networkCIDR, "-interface", name)
				if changeOutput, changeErr := routeChange.CombinedOutput(); changeErr != nil {
					return fmt.Errorf("failed to update route %s via %s: %s: %w", networkCIDR, name, string(changeOutput), changeErr)
				}
			} else {
				return fmt.Errorf("failed to add route %s via %s: %s: %w", networkCIDR, name, out, err)
			}
		}

		return nil
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func maskSize(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}

// setInterfaceUp brings an interface up
func setInterfaceUp(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := cmdExecutor.Command("ip", "link", "set", "dev", name, "up")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to bring interface up: %s: %w", string(output), err)
		}
		return nil
	case "darwin":
		cmd := cmdExecutor.Command("ifconfig", name, "up")
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
		cmd := cmdExecutor.Command("ip", "link", "set", "dev", name, "down")
		cmd.Run() // Ignore errors - interface might not be up
		return nil
	case "darwin":
		cmd := cmdExecutor.Command("ifconfig", name, "down")
		cmd.Run() // Ignore errors
		return nil
	default:
		return nil
	}
}

// deleteInterface removes the WireGuard interface from the system.
func deleteInterface(name string) error {
	switch runtime.GOOS {
	case "linux":
		cmd := cmdExecutor.Command("ip", "link", "del", "dev", name)
		if output, err := cmd.CombinedOutput(); err != nil {
			out := string(output)
			if strings.Contains(out, "Cannot find device") || strings.Contains(out, "does not exist") {
				return nil
			}
			return fmt.Errorf("failed to delete interface: %s: %w", out, err)
		}
		return nil
	case "darwin":
		cmd := cmdExecutor.Command("ifconfig", name, "destroy")
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
		cmdExecutor.Command("ip", "addr", "flush", "dev", name).Run()
		// Remove all peers
		cmdExecutor.Command("wg", "set", name, "peer", "remove").Run()
		return nil
	case "darwin":
		return nil
	default:
		return nil
	}
}

// isPortInUse checks if a UDP port is already bound
func isPortInUse(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return true // Port is in use
	}
	conn.Close()
	return false
}

// findAvailablePort finds an available UDP port starting from the given port
func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		if !isPortInUse(port) {
			return port
		}
	}
	return 0 // No available port found
}

// getWGInterfacePort gets the listen port of a WireGuard interface (0 if not set)
func getWGInterfacePort(name string) int {
	cmd := cmdExecutor.Command("wg", "show", name, "listen-port")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	var port int
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &port)
	return port
}
