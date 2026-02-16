package daemon

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

const (
	URIPrefix        = "wgmesh://"
	URIVersion       = "v1"
	DefaultWGPort    = 51820
	DefaultInterface = "wg0"
)

// Config holds all derived configuration for the mesh daemon
type Config struct {
	Secret          string
	Keys            *crypto.DerivedKeys
	InterfaceName   string
	WGListenPort    int
	AdvertiseRoutes []string
	LogLevel        string
	Privacy         bool
	Gossip          bool
}

// DaemonOpts holds options for the daemon
type DaemonOpts struct {
	Secret          string
	InterfaceName   string
	WGListenPort    int
	AdvertiseRoutes []string
	LogLevel        string
	Privacy         bool
	Gossip          bool
}

// NewConfig creates a new daemon configuration from options
func NewConfig(opts DaemonOpts) (*Config, error) {
	// Parse secret from URI format if needed
	secret := parseSecret(opts.Secret)

	// Derive all keys
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	// Set defaults
	ifaceName := opts.InterfaceName
	if ifaceName == "" {
		ifaceName = DefaultInterface
	}

	listenPort := opts.WGListenPort
	if listenPort == 0 {
		listenPort = DefaultWGPort
	}

	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	return &Config{
		Secret:          secret,
		Keys:            keys,
		InterfaceName:   ifaceName,
		WGListenPort:    listenPort,
		AdvertiseRoutes: opts.AdvertiseRoutes,
		LogLevel:        logLevel,
		Privacy:         opts.Privacy,
		Gossip:          opts.Gossip,
	}, nil
}

// GenerateSecret generates a new random mesh secret
func GenerateSecret() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url (no padding for cleaner URLs)
	secret := base64.RawURLEncoding.EncodeToString(b)

	return secret, nil
}

// FormatSecretURI formats a secret as a wgmesh:// URI
func FormatSecretURI(secret string) string {
	return fmt.Sprintf("%s%s/%s", URIPrefix, URIVersion, secret)
}

// parseSecret extracts the raw secret from various input formats
func parseSecret(input string) string {
	input = strings.TrimSpace(input)

	// Handle wgmesh://v1/secret format
	if strings.HasPrefix(input, URIPrefix) {
		input = strings.TrimPrefix(input, URIPrefix)
		parts := strings.SplitN(input, "/", 2)
		if len(parts) == 2 {
			// Remove query params if present
			secret := parts[1]
			if idx := strings.Index(secret, "?"); idx != -1 {
				secret = secret[:idx]
			}
			return secret
		}
		return parts[0]
	}

	return input
}

// GetConfigPath returns the path to the config file for a given interface
func GetConfigPath(interfaceName string) string {
	return filepath.Join("/var/lib/wgmesh", interfaceName+".conf")
}

// LoadConfigFile loads and parses a config file with key=value pairs
func LoadConfigFile(path string) (map[string]string, error) {
	config := make(map[string]string)

	// If file doesn't exist, return empty config (not an error)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, nil
	}

	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Read and parse key=value pairs
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Log warning but continue parsing
			fmt.Printf("Warning: invalid config line %d in %s: %s\n", lineNum, path, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			continue
		}

		// Remove quotes if present
		if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
			value = value[1 : len(value)-1]
		}

		config[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return config, nil
}

// ParseAdvertiseRoutes parses comma-separated network addresses
func ParseAdvertiseRoutes(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	routes := make([]string, 0, len(parts))

	for _, part := range parts {
		network := strings.TrimSpace(part)
		if network != "" {
			routes = append(routes, network)
		}
	}

	return routes, nil
}
