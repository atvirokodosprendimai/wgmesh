package wireguard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ifname"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

// wgPath is the absolute path to the wg binary, resolved once at package init.
// Falls back to "wg" (PATH lookup at exec time) if LookPath fails. Atomic: a
// test replaces it through SetWGPath while readers run.
var wgPath atomic.Pointer[string]

func init() {
	p := "wg"
	if lp, err := exec.LookPath("wg"); err == nil {
		p = lp
	}
	wgPath.Store(&p)
}

// shortKey safely truncates a key for logging (avoids panic on short/empty keys).
func shortKey(key string) string {
	if len(key) > 16 {
		return key[:16]
	}
	return key
}

// FullConfig is a complete WireGuard configuration used by the centralized
// (SSH) deployment path. It pairs one WGInterface with a list of WGPeers.
type FullConfig struct {
	Interface WGInterface
	Peers     []WGPeer
}

// WGInterface holds the local WireGuard interface parameters for centralized mode.
type WGInterface struct {
	PrivateKey string
	Address    string
	ListenPort int
}

// WGPeer holds the per-peer parameters used by centralized configuration apply.
type WGPeer struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepalive int
}

// PeerTransfer holds the cumulative byte counters for a single WireGuard peer.
type PeerTransfer struct {
	RxBytes uint64
	TxBytes uint64
}

// ApplyFullConfiguration creates a fresh WireGuard interface on the remote host
// and applies the given full configuration over SSH.
func ApplyFullConfiguration(client *ssh.Client, iface string, config *FullConfig) error {
	// Defense-in-depth: validate before interpolating into shell commands.
	if err := ifname.Validate(iface); err != nil {
		return fmt.Errorf("invalid interface name: %w", err)
	}

	fmt.Println("  Creating fresh WireGuard configuration...")

	if _, err := client.Run(ssh.Cmd("ip", "link", "del", iface) + " 2>/dev/null || true"); err != nil {
	}

	if _, err := client.Run(ssh.Cmd("ip", "link", "add", iface, "type", "wireguard")); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	tmpKeyFile := fmt.Sprintf("/tmp/wg-key-%s", iface)
	if err := client.WriteFile(tmpKeyFile, []byte(config.Interface.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	defer client.Run(ssh.Cmd("rm", "-f", tmpKeyFile))

	if _, err := client.Run(ssh.Cmd("wg", "set", iface, "private-key", tmpKeyFile, "listen-port", strconv.Itoa(config.Interface.ListenPort))); err != nil {
		return fmt.Errorf("failed to set interface config: %w", err)
	}

	if _, err := client.Run(ssh.Cmd("ip", "addr", "add", config.Interface.Address, "dev", iface)); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}

	if _, err := client.Run(ssh.Cmd("ip", "link", "set", iface, "up")); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	for _, peer := range config.Peers {
		args := []string{"set", iface, "peer", peer.PublicKey}

		if peer.Endpoint != "" {
			args = append(args, "endpoint", peer.Endpoint)
		}

		if len(peer.AllowedIPs) > 0 {
			args = append(args, "allowed-ips", strings.Join(peer.AllowedIPs, ","))
		}

		if peer.PersistentKeepalive > 0 {
			args = append(args, "persistent-keepalive", strconv.Itoa(peer.PersistentKeepalive))
		}

		if _, err := client.Run(ssh.Cmd("wg", args...)); err != nil {
			return fmt.Errorf("failed to add peer %s: %w", shortKey(peer.PublicKey), err)
		}

		fmt.Printf("    Added peer: %s\n", shortKey(peer.PublicKey))
	}

	return nil
}

// AllowedIPsClear is the allowedIPs argument of SetPeer that removes every
// allowed IP of the peer: wg set keeps the old list when allowed-ips is left
// out, so "no AllowedIPs" has to be sent as an empty value (ADR-0004 T6, a
// relayed peer kept on the interface only to handshake). An empty allowedIPs
// still means "leave the list alone".
const AllowedIPsClear = "-"

// SetPeerArgs builds the arguments of the wg set call SetPeer runs. hasPSK
// adds the preshared-key flag, whose value SetPeer feeds on stdin.
func SetPeerArgs(iface, pubKey string, hasPSK bool, endpoint, allowedIPs string) []string {
	args := []string{"set", iface, "peer", pubKey}

	if hasPSK {
		args = append(args, "preshared-key", "/dev/stdin")
	}

	if endpoint != "" {
		args = append(args, "endpoint", endpoint)
	}

	switch allowedIPs {
	case "":
	case AllowedIPsClear:
		args = append(args, "allowed-ips", "")
	default:
		args = append(args, "allowed-ips", allowedIPs)
	}

	// Add persistent keepalive for NAT traversal
	return append(args, "persistent-keepalive", "25")
}

// SetPeer adds or updates a peer on the local WireGuard interface
func SetPeer(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error {
	var stdin io.Reader

	// Add PSK if non-zero
	// NOTE: /dev/stdin is Linux/macOS only; Windows would need a named pipe or temp file.
	var zeroKey [32]byte
	if psk != zeroKey {
		pskB64 := base64.StdEncoding.EncodeToString(psk[:])
		stdin = strings.NewReader(pskB64 + "\n")
	}

	if stdout, stderr, err := runWG(stdin, SetPeerArgs(iface, pubKey, stdin != nil, endpoint, allowedIPs)...); err != nil {
		return fmt.Errorf("wg set failed: %s: %w", string(append(stdout, stderr...)), err)
	}

	return nil
}

// RemovePeer removes a peer from the local WireGuard interface
func RemovePeer(iface, pubKey string) error {
	if stdout, stderr, err := runWG(nil, "set", iface, "peer", pubKey, "remove"); err != nil {
		return fmt.Errorf("wg set peer remove failed: %s: %w", string(append(stdout, stderr...)), err)
	}
	return nil
}

// GetPeers returns the list of peers on the local WireGuard interface.
// It reads through ReadState(iface, 0).
func GetPeers(iface string) ([]WGPeer, error) {
	st, err := ReadState(iface, 0)
	if err != nil {
		return nil, err
	}

	var peers []WGPeer
	for _, p := range st.Peers {
		peers = append(peers, WGPeer{PublicKey: p.PublicKey})
	}

	return peers, nil
}

// GetLatestHandshakes returns the most recent handshake time for each WG peer.
// Returns a map of public key → Unix timestamp (0 means no handshake yet).
// It reads through ReadState(iface, 0).
func GetLatestHandshakes(iface string) (map[string]int64, error) {
	st, err := ReadState(iface, 0)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(st.Peers))
	for _, p := range st.Peers {
		result[p.PublicKey] = p.LatestHandshake
	}

	return result, nil
}

// GetPeerTransfers returns per-peer transfer counters from WireGuard.
// Map key is peer public key and values are cumulative rx/tx bytes.
// It reads through ReadState(iface, 0).
func GetPeerTransfers(iface string) (map[string]PeerTransfer, error) {
	st, err := ReadState(iface, 0)
	if err != nil {
		return nil, err
	}

	result := make(map[string]PeerTransfer, len(st.Peers))
	for _, p := range st.Peers {
		result[p.PublicKey] = PeerTransfer{RxBytes: p.RxBytes, TxBytes: p.TxBytes}
	}

	return result, nil
}
