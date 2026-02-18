package daemon

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
	LANDiscovery    bool
	Introducer      bool
	DisableIPv6     bool
	ForceRelay      bool
	DisablePunching bool
}

// DaemonOpts holds options for the daemon
type DaemonOpts struct {
	Secret              string
	InterfaceName       string
	WGListenPort        int
	AdvertiseRoutes     []string
	LogLevel            string
	Privacy             bool
	Gossip              bool
	DisableLANDiscovery bool
	Introducer          bool
	DisableIPv6         bool
	ForceRelay          bool
	DisablePunching     bool
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
		LANDiscovery:    !opts.DisableLANDiscovery,
		Introducer:      opts.Introducer,
		DisableIPv6:     opts.DisableIPv6,
		ForceRelay:      opts.ForceRelay,
		DisablePunching: opts.DisablePunching,
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
