package daemon

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

const (
	URIPrefix              = "wgmesh://"
	URIVersion             = "v1"
	DefaultWGPort          = 51820
	DefaultInterface       = "wg0"
	DefaultInterfaceDarwin = "utun20"
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
		if runtime.GOOS == "darwin" {
			ifaceName = DefaultInterfaceDarwin
		} else {
			ifaceName = DefaultInterface
		}
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

// ReloadConfigPath returns the path of the reload config file for the given
// interface name.  The file is written by the operator (or systemd service)
// and contains lines of the form KEY=VALUE.  Currently supported keys:
//
//	advertise-routes   comma-separated CIDR list
//	log-level          debug|info|warn|error
func ReloadConfigPath(ifaceName string) string {
	return fmt.Sprintf("/var/lib/wgmesh/%s.reload", ifaceName)
}

// LoadReloadFile parses a reload config file and returns a DaemonOpts with
// only the reloadable fields populated.  Missing or malformed keys are
// silently skipped so that a partial file is still useful.
func LoadReloadFile(path string) (DaemonOpts, error) {
	f, err := os.Open(path)
	if err != nil {
		return DaemonOpts{}, fmt.Errorf("open reload file: %w", err)
	}
	defer f.Close()

	var opts DaemonOpts
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(val)
		switch key {
		case "advertise-routes":
			if val == "" {
				opts.AdvertiseRoutes = []string{}
			} else {
				parts := strings.Split(val, ",")
				routes := make([]string, 0, len(parts))
				for _, p := range parts {
					if r := strings.TrimSpace(p); r != "" {
						routes = append(routes, r)
					}
				}
				opts.AdvertiseRoutes = routes
			}
		case "log-level":
			opts.LogLevel = val
		}
	}
	if err := sc.Err(); err != nil {
		return DaemonOpts{}, fmt.Errorf("read reload file: %w", err)
	}
	return opts, nil
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
