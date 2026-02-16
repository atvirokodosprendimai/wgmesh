package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Validation patterns for shell-safe values
var (
	// safePathRe matches safe file paths (alphanumeric, slashes, dots, hyphens, underscores, percent for wg-quick %i)
	safePathRe = regexp.MustCompile(`^[a-zA-Z0-9/._%-]+$`)
	// safeIfaceRe matches safe interface names
	safeIfaceRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	// safeBase64Re matches WireGuard base64-encoded keys
	safeBase64Re = regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)
	// safeEndpointRe matches IP:port or hostname:port endpoints
	safeEndpointRe = regexp.MustCompile(`^[a-zA-Z0-9.:[\]-]+$`)
	// safeCIDRRe matches CIDR network notation
	safeCIDRRe = regexp.MustCompile(`^[0-9a-fA-F.:]+(/[0-9]+)?$`)
)

// ValidatePath returns an error if the path contains shell-unsafe characters
func ValidatePath(path string) error {
	if !safePathRe.MatchString(path) {
		return fmt.Errorf("unsafe path: %q contains shell metacharacters", path)
	}
	return nil
}

// ValidateIface returns an error if the interface name is unsafe
func ValidateIface(name string) error {
	if !safeIfaceRe.MatchString(name) {
		return fmt.Errorf("unsafe interface name: %q", name)
	}
	return nil
}

// ValidateBase64Key returns an error if the key is not valid base64
func ValidateBase64Key(key string) error {
	if key == "" || !safeBase64Re.MatchString(key) {
		return fmt.Errorf("unsafe key value: not valid base64")
	}
	return nil
}

// ValidateEndpoint returns an error if the endpoint contains unsafe characters
func ValidateEndpoint(endpoint string) error {
	if endpoint == "" || endpoint == "(none)" {
		return nil
	}
	if !safeEndpointRe.MatchString(endpoint) {
		return fmt.Errorf("unsafe endpoint: %q", endpoint)
	}
	return nil
}

// ValidateCIDR returns an error if the CIDR contains unsafe characters
func ValidateCIDR(cidr string) error {
	if !safeCIDRRe.MatchString(cidr) {
		return fmt.Errorf("unsafe CIDR: %q", cidr)
	}
	return nil
}

type Client struct {
	conn *ssh.Client
}

// knownHostsPath returns the default path to the known_hosts file
func knownHostsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".ssh", "known_hosts")
}

func NewClient(host string, port int) (*Client, error) {
	var authMethods []ssh.AuthMethod

	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers))
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		keyPaths := []string{
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		}

		for _, keyPath := range keyPaths {
			if key, err := os.ReadFile(keyPath); err == nil {
				if signer, err := ssh.ParsePrivateKey(key); err == nil {
					authMethods = append(authMethods, ssh.PublicKeys(signer))
				}
			}
		}
	}

	// Use known_hosts for host key verification to prevent MITM attacks.
	// Falls back to warning-only if known_hosts doesn't exist.
	var hostKeyCallback ssh.HostKeyCallback
	khPath := knownHostsPath()
	if khPath != "" {
		if _, err := os.Stat(khPath); err == nil {
			hostKeyCallback, err = knownhosts.New(khPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load known_hosts from %s: %w", khPath, err)
			}
		}
	}
	if hostKeyCallback == nil {
		// No known_hosts file found — accept any key but log a warning.
		// This enables first-time use but is not silent about the risk.
		fmt.Fprintf(os.Stderr, "Warning: no known_hosts file found at %s; accepting host keys without verification (TOFU)\n", khPath)
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (c *Client) RunQuiet(cmd string) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	return session.Run(cmd)
}

func (c *Client) WriteFile(path string, content []byte, mode os.FileMode) error {
	// Validate path to prevent command injection
	if err := ValidatePath(path); err != nil {
		return fmt.Errorf("WriteFile rejected: %w", err)
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin: %w", err)
	}

	if err := session.Start(fmt.Sprintf("cat > %s && chmod %o %s", path, mode, path)); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	if _, err := io.Copy(stdin, strings.NewReader(string(content))); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to complete write: %w", err)
	}

	return nil
}
