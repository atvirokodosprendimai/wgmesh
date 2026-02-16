package mesh

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

var encryptionPassword string

func SetEncryptionPassword(password string) {
	encryptionPassword = password
}

func Initialize(stateFile string) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	m := &Mesh{
		InterfaceName: "wg0",
		Network:       "10.99.0.0/16",
		ListenPort:    51820,
		Nodes:         make(map[string]*Node),
		LocalHostname: hostname,
	}

	return m.Save(stateFile)
}

func Load(stateFile string) (*Mesh, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Check if file is encrypted (base64 encoded data)
	if encryptionPassword != "" {
		// Decrypt the data
		decrypted, err := crypto.Decrypt(string(data), encryptionPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt state file: %w", err)
		}
		data = decrypted
	}

	var m Mesh
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &m, nil
}

func (m *Mesh) Save(stateFile string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Encrypt if password is set
	if encryptionPassword != "" {
		encrypted, err := crypto.Encrypt(data, encryptionPassword)
		if err != nil {
			return fmt.Errorf("failed to encrypt state: %w", err)
		}
		data = []byte(encrypted)
	}

	// Ensure directory exists
	dir := filepath.Dir(stateFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(stateFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func (m *Mesh) AddNode(nodeSpec string) error {
	parts := strings.Split(nodeSpec, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid node spec, expected hostname:mesh_ip:ssh_host[:ssh_port]")
	}

	hostname := parts[0]
	meshIP := net.ParseIP(parts[1])
	if meshIP == nil {
		return fmt.Errorf("invalid mesh IP: %s", parts[1])
	}

	sshHost := parts[2]
	sshPort := 22
	if len(parts) >= 4 {
		if _, err := fmt.Sscanf(parts[3], "%d", &sshPort); err != nil {
			return fmt.Errorf("invalid SSH port: %s", parts[3])
		}
	}

	if _, exists := m.Nodes[hostname]; exists {
		return fmt.Errorf("node %s already exists", hostname)
	}

	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}

	isLocal := hostname == m.LocalHostname

	node := &Node{
		Hostname:   hostname,
		MeshIP:     meshIP,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		SSHHost:    sshHost,
		SSHPort:    sshPort,
		ListenPort: m.ListenPort,
		IsLocal:    isLocal,
	}

	m.Nodes[hostname] = node

	fmt.Printf("Added node: %s (%s)\n", hostname, meshIP)
	return nil
}

func (m *Mesh) RemoveNode(hostname string) error {
	if _, exists := m.Nodes[hostname]; !exists {
		return fmt.Errorf("node %s not found", hostname)
	}

	delete(m.Nodes, hostname)
	fmt.Printf("Removed node: %s\n", hostname)
	return nil
}

func (m *Mesh) List() {
	fmt.Printf("Mesh Network: %s\n", m.Network)
	fmt.Printf("Interface: %s\n", m.InterfaceName)
	fmt.Printf("Listen Port: %d\n\n", m.ListenPort)
	fmt.Printf("Nodes:\n")

	for hostname, node := range m.Nodes {
		localMarker := ""
		if node.IsLocal {
			localMarker = " (local)"
		}
		natMarker := ""
		if node.BehindNAT {
			natMarker = " [NAT]"
		}

		fmt.Printf("  %s%s%s:\n", hostname, localMarker, natMarker)
		fmt.Printf("    Mesh IP: %s\n", node.MeshIP)
		fmt.Printf("    SSH: %s:%d\n", node.SSHHost, node.SSHPort)
		fmt.Printf("    Public Key: %s\n", node.PublicKey)
		if node.PublicEndpoint != "" {
			fmt.Printf("    Endpoint: %s\n", node.PublicEndpoint)
		}
		if len(node.RoutableNetworks) > 0 {
			fmt.Printf("    Routable Networks: %v\n", node.RoutableNetworks)
		}
		fmt.Println()
	}
}
