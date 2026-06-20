package wgmesh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// Mesh represents a running mesh node
type Mesh struct {
	daemon *daemon.Daemon
	cancel context.CancelFunc
}

// Option is a configuration option for Start
type Option func(*options)

// options holds the internal configuration for Start
type options struct {
	ifaceName       string
	listenPort      int
	advertiseRoutes []string
	logLevel        string
	gossip          bool
	privacy         bool
	noLAN           bool
	disableIPv6     bool
	forceRelay      bool
	disablePunching bool
	introducer      bool
	meshSubnet      string
	stateDir        string
	tunFd           int    // 0 = normal mode; >0 = Android fd mode
	tunPrivateKey   []byte // required when tunFd > 0
}

// WithInterface sets the WireGuard interface name
func WithInterface(name string) Option {
	return func(o *options) {
		o.ifaceName = name
	}
}

// WithListenPort sets the WireGuard listen port
func WithListenPort(port int) Option {
	return func(o *options) {
		o.listenPort = port
	}
}

// WithAdvertiseRoutes sets the routes to advertise
func WithAdvertiseRoutes(routes ...string) Option {
	return func(o *options) {
		o.advertiseRoutes = routes
	}
}

// WithLogLevel sets the log level
func WithLogLevel(level string) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

// WithGossip enables gossip discovery
func WithGossip() Option {
	return func(o *options) {
		o.gossip = true
	}
}

// WithPrivacy enables privacy mode
func WithPrivacy() Option {
	return func(o *options) {
		o.privacy = true
	}
}

// WithNoLAN disables LAN discovery
func WithNoLAN() Option {
	return func(o *options) {
		o.noLAN = true
	}
}

// WithDisableIPv6 disables IPv6
func WithDisableIPv6() Option {
	return func(o *options) {
		o.disableIPv6 = true
	}
}

// WithForceRelay forces all traffic through relays
func WithForceRelay() Option {
	return func(o *options) {
		o.forceRelay = true
	}
}

// WithDisablePunching disables NAT punching
func WithDisablePunching() Option {
	return func(o *options) {
		o.disablePunching = true
	}
}

// WithIntroducer enables introducer mode
func WithIntroducer() Option {
	return func(o *options) {
		o.introducer = true
	}
}

// WithMeshSubnet sets a custom mesh subnet
func WithMeshSubnet(subnet string) Option {
	return func(o *options) {
		o.meshSubnet = subnet
	}
}

// WithStateDir sets the state directory
func WithStateDir(dir string) Option {
	return func(o *options) {
		o.stateDir = dir
	}
}

// WithTunFd instructs the daemon to use a pre-created TUN file descriptor
// instead of creating a WireGuard interface. fd must be the value returned by
// Android's VpnService.Builder.establish(). key must be the 32-byte raw
// WireGuard private key that was provisioned for this node.
//
// This option is mutually exclusive with WithInterface.
func WithTunFd(fd int, key []byte) Option {
	return func(o *options) {
		o.tunFd = fd
		o.tunPrivateKey = key
	}
}

// getDefaultStateDir returns the default state directory
func getDefaultStateDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/var/lib/wgmesh"
	}
	return filepath.Join(homeDir, ".wgmesh")
}

// Start starts a mesh node with the given shared secret and options.
// The secret can be in "wgmesh://v1/<base64>" format or a plain passphrase.
// The node runs until ctx is cancelled or Stop is called.
func Start(ctx context.Context, secret string, opts ...Option) (*Mesh, error) {
	o := &options{
		listenPort:    51820,
		logLevel:      "info",
		stateDir:      getDefaultStateDir(),
		ifaceName:     "", // Will use default from daemon
		meshSubnet:    "",
		tunFd:         0,
		tunPrivateKey: nil,
	}

	for _, opt := range opts {
		opt(o)
	}

	// Ensure state directory exists
	if err := os.MkdirAll(o.stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Build daemon options
	daemonOpts := daemon.DaemonOpts{
		Secret:              secret,
		InterfaceName:       o.ifaceName,
		WGListenPort:        o.listenPort,
		AdvertiseRoutes:     o.advertiseRoutes,
		LogLevel:            o.logLevel,
		Privacy:             o.privacy,
		Gossip:              o.gossip,
		DisableLANDiscovery: o.noLAN,
		Introducer:          o.introducer,
		DisableIPv6:         o.disableIPv6,
		ForceRelay:          o.forceRelay,
		DisablePunching:     o.disablePunching,
		MeshSubnet:          o.meshSubnet,
		TunFd:               o.tunFd,
		TunPrivateKey:       o.tunPrivateKey,
	}

	cfg, err := daemon.NewConfig(daemonOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	d, err := daemon.NewDaemon(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create daemon: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	// Start daemon in background
	runErr := make(chan error, 1)
	go func() {
		runErr <- d.RunWithDHTDiscovery()
	}()

	// Wait a bit for daemon to start
	select {
	case err := <-runErr:
		cancel()
		return nil, fmt.Errorf("daemon failed to start: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Daemon started successfully
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		d.Shutdown()
	}()

	return &Mesh{
		daemon: d,
		cancel: cancel,
	}, nil
}

// StartWithFd starts a mesh node using a pre-created TUN file descriptor,
// as returned by Android's VpnService.Builder.establish().
//
// key must be a 32-byte raw WireGuard private key.
// fd must be the file descriptor returned by the Android VPN API.
// token must be the shared mesh secret in "wgmesh://v1/<base64>" format or a
// plain passphrase.
//
// The node runs until ctx is cancelled or Stop is called.
func StartWithFd(ctx context.Context, token string, key []byte, fd int, opts ...Option) (*Mesh, error) {
	opts = append([]Option{WithTunFd(fd, key)}, opts...)
	return Start(ctx, token, opts...)
}

// Stop stops the mesh node
func (m *Mesh) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.daemon != nil {
		m.daemon.Shutdown()
	}
}

// GetDaemon returns the underlying daemon instance
func (m *Mesh) GetDaemon() *daemon.Daemon {
	return m.daemon
}
