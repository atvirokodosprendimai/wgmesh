// Package wgmesh provides a minimal embeddable API for the wgmesh mesh daemon.
//
// Usage:
//
//	import (
//	    "github.com/atvirokodosprendimai/wgmesh/pkg/wgmesh"
//	    _ "github.com/atvirokodosprendimai/wgmesh/pkg/discovery" // register DHT factory
//	)
//
//	m, err := wgmesh.Start(ctx, "wgmesh://v1/<base64>")
//	if err != nil { log.Fatal(err) }
//	defer m.Stop()
package wgmesh

import (
	"context"
	"fmt"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// Option is a functional option for Start.
type Option func(*options)

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

// WithInterface overrides the WireGuard interface name (default: "wg0").
func WithInterface(name string) Option {
	return func(o *options) { o.ifaceName = name }
}

// WithListenPort overrides the WireGuard listen port (default: 51820).
func WithListenPort(port int) Option {
	return func(o *options) { o.listenPort = port }
}

// WithAdvertiseRoutes sets the subnets to advertise to mesh peers.
func WithAdvertiseRoutes(routes []string) Option {
	return func(o *options) { o.advertiseRoutes = routes }
}

// WithLogLevel sets the log verbosity ("debug", "info", "warn", "error").
func WithLogLevel(level string) Option {
	return func(o *options) { o.logLevel = level }
}

// WithGossip enables in-mesh gossip discovery (Layer 3).
func WithGossip() Option {
	return func(o *options) { o.gossip = true }
}

// WithPrivacy enables Dandelion++ announcement relay.
func WithPrivacy() Option {
	return func(o *options) { o.privacy = true }
}

// WithNoLAN disables LAN multicast discovery (Layer 1).
func WithNoLAN() Option {
	return func(o *options) { o.noLAN = true }
}

// WithDisableIPv6 disables IPv6 mesh addressing.
func WithDisableIPv6() Option {
	return func(o *options) { o.disableIPv6 = true }
}

// WithForceRelay forces all traffic to go through relay peers.
func WithForceRelay() Option {
	return func(o *options) { o.forceRelay = true }
}

// WithDisablePunching disables UDP hole-punching.
func WithDisablePunching() Option {
	return func(o *options) { o.disablePunching = true }
}

// WithIntroducer marks this node as an introducer/relay.
func WithIntroducer() Option {
	return func(o *options) { o.introducer = true }
}

// WithMeshSubnet overrides the mesh IP subnet (e.g. "10.99.0.0/16").
func WithMeshSubnet(cidr string) Option {
	return func(o *options) { o.meshSubnet = cidr }
}

// WithStateDir overrides the directory used to persist node keypair state.
// Defaults to "/var/lib/wgmesh".
func WithStateDir(dir string) Option {
	return func(o *options) { o.stateDir = dir }
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

// Mesh is a running wgmesh node. Call Stop to shut it down cleanly.
type Mesh struct {
	d      *daemon.Daemon
	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan error
}

// Start creates and starts a mesh node identified by token.
// token must be in "wgmesh://v1/<base64url>" format or a plain passphrase.
// The node runs until ctx is cancelled or Stop is called.
// Start blocks only until the daemon is initialised; the reconcile loop runs
// in background goroutines.
func Start(ctx context.Context, token string, opts ...Option) (*Mesh, error) {
	o := &options{
		logLevel: "info",
	}
	for _, fn := range opts {
		fn(o)
	}

	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:              token,
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
	})
	if err != nil {
		return nil, fmt.Errorf("wgmesh.Start: build config: %w", err)
	}

	d, err := daemon.NewDaemon(cfg)
	if err != nil {
		return nil, fmt.Errorf("wgmesh.Start: new daemon: %w", err)
	}

	mctx, cancel := context.WithCancel(ctx)
	m := &Mesh{
		d:      d,
		ctx:    mctx,
		cancel: cancel,
		doneCh: make(chan error, 1),
	}

	go func() {
		m.doneCh <- d.RunWithContext(mctx)
	}()

	return m, nil
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

// Stop shuts down the mesh node, tears down the WireGuard interface, and
// waits for all background goroutines to exit.
func (m *Mesh) Stop() error {
	m.cancel()
	return <-m.doneCh
}

// LocalIP returns the mesh IPv4 address assigned to this node.
// Returns an empty string if the daemon has not yet initialised.
func (m *Mesh) LocalIP() string {
	n := m.d.GetLocalNode()
	if n == nil {
		return ""
	}
	return n.MeshIP
}

// Peers returns a snapshot of all currently known mesh peers.
func (m *Mesh) Peers() []*node.PeerInfo {
	ps := m.d.GetPeerStore()
	if ps == nil {
		return nil
	}
	return ps.List()
}
