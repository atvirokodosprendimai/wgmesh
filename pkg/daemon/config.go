package daemon

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
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
	CustomSubnet    *net.IPNet // User-specified mesh subnet (nil = use derived)
}

// DaemonOpts holds options for the daemon
type DaemonOpts struct {
	Secret              string
	InterfaceName       string
	WGListenPort        int
	AdvertiseRoutes     []string
	AcceptRoutes        []string
	LogLevel            string
	Privacy             bool
	Gossip              bool
	DisableLANDiscovery bool
	Introducer          bool
	DisableIPv6         bool
	ForceRelay          bool
	DisablePunching     bool
	MeshSubnet          string // Custom mesh subnet CIDR (e.g. "192.168.100.0/24")
}

// NewConfig creates a new daemon configuration from options
func NewConfig(opts DaemonOpts) (*Config, error) {
	// Parse secret from URI format if needed
	secret := parseSecret(opts.Secret)

	// Warn if secret looks user-chosen rather than auto-generated.
	if opts.Secret != "" && !strings.HasPrefix(strings.TrimSpace(opts.Secret), "wgmesh://") {
		log.Printf("[WARN] Secret does not use wgmesh:// format — it may have low entropy. " +
			"Use 'wgmesh init --secret' to generate a cryptographically strong secret.")
	}

	// Derive all keys
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	// Validate interface name before applying defaults.
	if err := ValidateInterfaceName(opts.InterfaceName); err != nil {
		return nil, fmt.Errorf("invalid interface name: %w", err)
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

	// Parse and validate custom subnet if provided
	customSubnet, err := crypto.ParseSubnetOrDefault(opts.MeshSubnet)
	if err != nil {
		return nil, fmt.Errorf("invalid mesh subnet: %w", err)
	}
	if customSubnet != nil {
		if customSubnet.IP.To4() == nil {
			return nil, fmt.Errorf("mesh subnet must be an IPv4 CIDR, got %q", customSubnet.String())
		}
		ones, bits := customSubnet.Mask.Size()
		if bits != 32 {
			return nil, fmt.Errorf("mesh subnet must be an IPv4 CIDR, got %q", customSubnet.String())
		}
		if bits-ones < 2 {
			return nil, fmt.Errorf("mesh subnet /%d is too small (need at least /30 for 2 host addresses)", ones)
		}
	}

	advertiseRoutes, err := normalizeRoutes(opts.AdvertiseRoutes)
	if err != nil {
		return nil, fmt.Errorf("invalid advertise-routes: %w", err)
	}

	return &Config{
		Secret:          secret,
		Keys:            keys,
		InterfaceName:   ifaceName,
		WGListenPort:    listenPort,
		AdvertiseRoutes: advertiseRoutes,
		LogLevel:        logLevel,
		Privacy:         opts.Privacy,
		Gossip:          opts.Gossip,
		LANDiscovery:    !opts.DisableLANDiscovery,
		Introducer:      opts.Introducer,
		DisableIPv6:     opts.DisableIPv6,
		ForceRelay:      opts.ForceRelay,
		DisablePunching: opts.DisablePunching,
		CustomSubnet:    customSubnet,
	}, nil
}

// normalizeRoutes trims and validates advertised routes. Empty entries are
// dropped, each entry must be a CIDR (a bare IP is rejected), host bits are
// cleared (192.168.1.10/24 -> 192.168.1.0/24, with a warning) and duplicates
// are removed. Peers drop announcements carrying invalid CIDRs, so a bad entry
// must fail here rather than make this node silently invisible.
func normalizeRoutes(routes []string) ([]string, error) {
	if routes == nil {
		return nil, nil
	}
	out := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(r)
		if err != nil {
			return nil, fmt.Errorf("%q is not a CIDR (expected e.g. 192.168.1.0/24 or 10.0.0.5/32)", r)
		}
		canonical := ipnet.String()
		if canonical != r {
			log.Printf("WARNING: advertise route %q normalized to %q", r, canonical)
		}
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	if len(out) > crypto.MaxRoutableNetworks {
		return nil, fmt.Errorf("too many routes (%d, max %d)", len(out), crypto.MaxRoutableNetworks)
	}
	return out, nil
}

// PrefixLen returns the prefix length for the mesh subnet.
// Uses CustomSubnet mask if set, otherwise defaults to 16.
func (c *Config) PrefixLen() int {
	if c.CustomSubnet != nil {
		ones, _ := c.CustomSubnet.Mask.Size()
		return ones
	}
	return 16
}

// GenerateSecret generates a new random mesh secret
func GenerateSecret() (string, error) {
	// Generate 128 random bytes
	b := make([]byte, 128)
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
// only the reloadable fields populated.  Missing or malformed lines and
// unknown keys are silently skipped so that a partial file is still useful,
// but an advertise-routes value with an invalid CIDR fails the whole load so
// the caller keeps the current configuration.
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
			routes, err := normalizeRoutes(strings.Split(val, ","))
			if err != nil {
				return DaemonOpts{}, fmt.Errorf("invalid advertise-routes: %w", err)
			}
			opts.AdvertiseRoutes = routes
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
