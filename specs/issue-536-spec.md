# Specification: Issue #536

## Classification
feature

## Deliverables
code

## Problem Analysis

`pkg/discovery` directly imports `pkg/daemon` concrete types (`Config`, `LocalNode`, `PeerStore`,
`DiscoveryLayer`) throughout its four files (`dht.go`, `exchange.go`, `gossip.go`, `lan.go`,
`init.go`). This coupling has three concrete consequences:

1. **No embeddability.** An external Go binary cannot import only the discovery or crypto
   subsystems without pulling in the full daemon (WireGuard CLI subprocess invocations, state
   files under `/var/lib/wgmesh`, signal handling, etc.).

2. **No library API.** There is no `Start(token)` / `Stop()` function a caller can invoke
   programmatically. The only entry point is `(*Daemon).RunWithDHTDiscovery()`, which blocks
   on OS signals and owns the process lifecycle.

3. **Hard wg-CLI coupling.** All WireGuard interface operations (`createInterface`,
   `configureInterface`, `setInterfaceAddress`, `addPeer`, `removePeer`, `wg show dump`) in
   `pkg/daemon/helpers.go` execute `wg` or `ip` as child processes. An embedding application
   that already carries `golang.zx2c4.com/wireguard` (wireguard-go) in-process cannot reuse any
   of this code.

The fix is threefold:

- Extract the shared peer/node types that both `pkg/daemon` and `pkg/discovery` need into a
  new neutral package `pkg/node` so that `pkg/discovery` no longer imports `pkg/daemon`.
- Introduce a thin public embeddable API in a new `pkg/wgmesh` package: `Start(ctx, token,
  ...Option) (*Mesh, error)` and `(*Mesh) Stop() error`.
- Define a `TunnelBackend` interface in `pkg/node` so that future callers can plug in a
  wireguard-go in-process backend while the current CLI-based backend continues to work as the
  default.

## Implementation Tasks

### Task 1: Create `pkg/node/types.go` — shared types package

Create the file `pkg/node/types.go` with the following exact content:

```go
// Package node contains the shared types used by both pkg/daemon and
// pkg/discovery. Neither package may import the other; both import pkg/node.
package node

import (
	"net"
	"sync"
	"time"
)

// DiscoveryLayer is the interface implemented by every discovery backend.
type DiscoveryLayer interface {
	Start() error
	Stop() error
}

// PeerInfo represents a discovered mesh peer.
type PeerInfo struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	MeshIPv6         string
	Endpoint         string // best known endpoint (ip:port)
	Introducer       bool
	RoutableNetworks []string
	LastSeen         time.Time
	DiscoveredVia    []string       // ["lan", "dht", "gossip"]
	Latency          *time.Duration // measured via WG handshake
	NATType          string         // "cone", "symmetric", or "unknown"
	EndpointMethod   string
}

// LocalNode represents the local WireGuard node.
// Endpoint access is thread-safe via GetEndpoint / SetEndpoint.
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	MeshIPv6         string
	RoutableNetworks []string
	Introducer       bool
	NATType          string
	Hostname         string

	endpointMu sync.RWMutex
	wgEndpoint string
}

// GetEndpoint returns the current WireGuard endpoint (thread-safe).
func (n *LocalNode) GetEndpoint() string {
	n.endpointMu.RLock()
	defer n.endpointMu.RUnlock()
	return n.wgEndpoint
}

// SetEndpoint updates the WireGuard endpoint (thread-safe).
func (n *LocalNode) SetEndpoint(ep string) {
	n.endpointMu.Lock()
	defer n.endpointMu.Unlock()
	n.wgEndpoint = ep
}

// DiscoveryConfig holds the subset of daemon.Config that discovery layers
// actually need. It contains no daemon-specific fields (no StateDir,
// no CommandExecutor, etc.).
type DiscoveryConfig struct {
	Secret          string
	Keys            interface{ GetKeys() *DerivedKeyRefs } // filled by daemon.Config adapter
	InterfaceName   string
	WGListenPort    int
	AdvertiseRoutes []string
	Privacy         bool
	Gossip          bool
	LANDiscovery    bool
	DisableIPv6     bool
	ForceRelay      bool
	DisablePunching bool
	Introducer      bool
	MeshSubnet      *net.IPNet
}

// TunnelBackend abstracts WireGuard interface management.
// The default implementation (CLITunnelBackend in pkg/daemon) uses the wg
// CLI subprocess. An embedding application may provide an alternative
// implementation backed by golang.zx2c4.com/wireguard (wireguard-go).
type TunnelBackend interface {
	// CreateInterface creates and configures the WireGuard interface.
	CreateInterface(ifaceName, privateKey string, listenPort int) error
	// SetAddress assigns a CIDR address to the interface.
	SetAddress(ifaceName, cidr string) error
	// BringUp brings the interface up.
	BringUp(ifaceName string) error
	// AddPeer installs a peer (pubkey, endpoint, allowedIPs, keepalive).
	AddPeer(ifaceName string, peer PeerConfig) error
	// RemovePeer removes a peer by public key.
	RemovePeer(ifaceName, pubkey string) error
	// ListPeers returns the current peer list and transfer counters.
	ListPeers(ifaceName string) ([]PeerStatus, error)
	// DestroyInterface removes the interface entirely.
	DestroyInterface(ifaceName string) error
}

// PeerConfig carries the parameters needed to install a WireGuard peer.
type PeerConfig struct {
	PublicKey            string
	Endpoint             string
	AllowedIPs           []string
	PersistentKeepalive  int
}

// PeerStatus carries the live status of an installed peer as returned by
// the kernel / userspace WireGuard implementation.
type PeerStatus struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	LastHandshakeTime   time.Time
	RxBytes             uint64
	TxBytes             uint64
}
```

Create `pkg/node/store.go` with the PeerStore implementation (moved verbatim from
`pkg/daemon/peerstore.go`), replacing every `PeerInfo` reference with `node.PeerInfo` (already
in the same package, so no qualifier needed). The package declaration must be `package node`.
Change the constant names to avoid a name collision: rename `LANMethod` → keep it in daemon
(`pkg/daemon/peerstore.go` already exports it, and `node` does not export discovery-layer
constants).

The file `pkg/node/store.go` must contain:

```go
package node

import (
	"log"
	"strings"
	"sync"
	"time"
)

const (
	PeerDeadTimeout   = 5 * time.Minute
	PeerRemoveTimeout = 10 * time.Minute
	PeerEventBufSize  = 16
	DefaultMaxPeers   = 1000
)

type PeerEventKind int

const (
	PeerEventNew     PeerEventKind = iota
	PeerEventUpdated PeerEventKind = iota
)

type PeerEvent struct {
	PubKey string
	Kind   PeerEventKind
}

// PeerStore is a thread-safe store for discovered peers.
type PeerStore struct {
	mu          sync.RWMutex
	peers       map[string]*PeerInfo
	subscribers []chan PeerEvent
}

// NewPeerStore creates a new peer store.
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*PeerInfo),
	}
}
```

Copy the remaining methods of `PeerStore` (`Subscribe`, `Unsubscribe`, `AddOrUpdate`, `Get`,
`List`, `Count`, `Remove`, `SetEndpointMethod`, `notify`, `MaxPeers`) verbatim from
`pkg/daemon/peerstore.go`, updating only the package declaration line.

### Task 2: Remove duplicate types from `pkg/daemon`

After `pkg/node` exists, update `pkg/daemon` so it no longer defines the same types
redundantly:

**`pkg/daemon/peerstore.go`**: Delete the entire file. Add a new file
`pkg/daemon/peerstore_compat.go` that contains only type aliases:

```go
package daemon

import "github.com/atvirokodosprendimai/wgmesh/pkg/node"

// Type aliases so that all existing daemon-internal code continues to compile
// without modification.
type PeerInfo = node.PeerInfo
type PeerStore = node.PeerStore
type PeerEvent = node.PeerEvent
type PeerEventKind = node.PeerEventKind

const (
	PeerDeadTimeout   = node.PeerDeadTimeout
	PeerRemoveTimeout = node.PeerRemoveTimeout
	PeerEventBufSize  = node.PeerEventBufSize
	DefaultMaxPeers   = node.DefaultMaxPeers
	PeerEventNew      = node.PeerEventNew
	PeerEventUpdated  = node.PeerEventUpdated
)

func NewPeerStore() *PeerStore { return node.NewPeerStore() }
```

**`pkg/daemon/daemon.go`**: In `LocalNode`, replace the struct definition with a type alias:

```go
// LocalNode is the shared local-node type from pkg/node.
type LocalNode = node.LocalNode
```

Remove the two methods `GetEndpoint` and `SetEndpoint` from `daemon.go` since they now live on
`node.LocalNode`. Add the import `"github.com/atvirokodosprendimai/wgmesh/pkg/node"` to
`daemon.go`.

**`pkg/daemon/daemon.go`**: Replace the `DiscoveryLayer` interface definition with an alias:

```go
// DiscoveryLayer is the shared discovery interface from pkg/node.
type DiscoveryLayer = node.DiscoveryLayer
```

### Task 3: Add a `Config` adapter method so `pkg/discovery` can read config without importing `pkg/daemon`

`pkg/discovery` needs key material and settings from `daemon.Config`. Instead of importing
`daemon`, `pkg/discovery` will accept a `*daemon.Config` via a narrow interface. Add the
following interface to `pkg/node/types.go` (replace the placeholder `Keys` field in
`DiscoveryConfig` above with this concrete approach):

Delete the `DiscoveryConfig` struct added in Task 1 (it was a placeholder). Instead, add a
`ConfigReader` interface to `pkg/node/types.go`:

```go
// ConfigReader is the narrow interface that pkg/discovery uses to read
// configuration. pkg/daemon.Config implements this interface; callers that
// embed wgmesh may provide their own implementation.
type ConfigReader interface {
	GetSecret() string
	GetNetworkID() [32]byte
	GetGossipKey() [32]byte
	GetGossipPort() uint16
	GetMulticastID() [2]byte
	GetEpochSeed() [32]byte
	GetRendezvousID() [32]byte
	GetMeshSubnet() string
	GetMeshPrefixV6() string
	GetInterfaceName() string
	GetWGListenPort() int
	GetAdvertiseRoutes() []string
	IsPrivacy() bool
	IsGossip() bool
	IsLANDiscovery() bool
	IsDisableIPv6() bool
	IsForceRelay() bool
	IsDisablePunching() bool
	IsIntroducer() bool
}
```

Add the following methods to `pkg/daemon/config.go` so that `*Config` satisfies `node.ConfigReader`:

```go
func (c *Config) GetSecret() string               { return c.Secret }
func (c *Config) GetNetworkID() [32]byte           { return c.Keys.NetworkID }
func (c *Config) GetGossipKey() [32]byte           { return c.Keys.GossipKey }
func (c *Config) GetGossipPort() uint16            { return c.Keys.GossipPort }
func (c *Config) GetMulticastID() [2]byte          { return c.Keys.MulticastID }
func (c *Config) GetEpochSeed() [32]byte           { return c.Keys.EpochSeed }
func (c *Config) GetRendezvousID() [32]byte        { return c.Keys.RendezvousID }
func (c *Config) GetMeshSubnet() string            { return c.Keys.MeshSubnet }
func (c *Config) GetMeshPrefixV6() string          { return c.Keys.MeshPrefixV6 }
func (c *Config) GetInterfaceName() string         { return c.InterfaceName }
func (c *Config) GetWGListenPort() int             { return c.WGListenPort }
func (c *Config) GetAdvertiseRoutes() []string     { return c.AdvertiseRoutes }
func (c *Config) IsPrivacy() bool                  { return c.Privacy }
func (c *Config) IsGossip() bool                   { return c.Gossip }
func (c *Config) IsLANDiscovery() bool             { return c.LANDiscovery }
func (c *Config) IsDisableIPv6() bool              { return c.DisableIPv6 }
func (c *Config) IsForceRelay() bool               { return c.ForceRelay }
func (c *Config) IsDisablePunching() bool          { return c.DisablePunching }
func (c *Config) IsIntroducer() bool               { return c.Introducer }
```

Add `"github.com/atvirokodosprendimai/wgmesh/pkg/node"` to the import block in
`pkg/daemon/config.go` (needed only for a compile-time assertion; the methods use only fields
of `Config`). Add the following blank compile-time check at package scope:

```go
var _ node.ConfigReader = (*Config)(nil)
```

### Task 4: Update `pkg/discovery` to use `pkg/node` instead of `pkg/daemon`

In every file in `pkg/discovery/` that currently imports
`"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"`:

**`dht.go`**: Change the `daemon` import to `node`. Replace every occurrence of:

| Old type / identifier | New type / identifier |
|-----------------------|-----------------------|
| `*daemon.Config` | `node.ConfigReader` |
| `*daemon.LocalNode` | `*node.LocalNode` |
| `*daemon.PeerStore` | `*node.PeerStore` |
| `daemon.PeerInfo` | `node.PeerInfo` |
| `daemon.PeerEventNew` | `node.PeerEventNew` |
| `daemon.PeerEventUpdated` | `node.PeerEventUpdated` |

All calls that previously accessed `config.Keys.GossipKey` must become `config.GetGossipKey()`,
`config.Keys.NetworkID` → `config.GetNetworkID()`, and so on for every field accessed through
`config.Keys.*` or `config.<field>`. Use the `ConfigReader` interface methods defined in
Task 3.

Apply the same substitution table to **`exchange.go`**, **`gossip.go`**, and **`lan.go`**.

**`init.go`**: Change the factory signature. The file currently registers:

```go
func createDHTDiscovery(ctx context.Context, config *daemon.Config, localNode *daemon.LocalNode, peerStore *daemon.PeerStore) (daemon.DiscoveryLayer, error)
```

Change it to:

```go
func createDHTDiscovery(ctx context.Context, config node.ConfigReader, localNode *node.LocalNode, peerStore *node.PeerStore) (node.DiscoveryLayer, error)
```

Update `daemon.SetDHTDiscoveryFactory` in `pkg/daemon/daemon.go` to accept the new signature:

```go
// DHTDiscoveryFactory is the function type for creating a discovery layer.
type DHTDiscoveryFactory func(ctx context.Context, config node.ConfigReader, localNode *node.LocalNode, peerStore *node.PeerStore) (node.DiscoveryLayer, error)
```

Update the call site in `RunWithDHTDiscovery` (it already passes `d.config`, `d.localNode`,
`d.peerStore` — these continue to work because `*Config` satisfies `ConfigReader` and the
other types are already node aliases).

After these changes `pkg/discovery` must have **no import of `pkg/daemon`** anywhere. Verify
with:

```
grep -r '"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"' pkg/discovery/
```

The command must produce no output.

### Task 5: Create `pkg/wgmesh/wgmesh.go` — public embeddable API

Create the new file `pkg/wgmesh/wgmesh.go`:

```go
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
		StateDir:            o.stateDir,
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
```

### Task 6: Add `RunWithContext` to `pkg/daemon/daemon.go`

`pkg/wgmesh` calls `d.RunWithContext(ctx)`, but this method does not yet exist. Add it to
`pkg/daemon/daemon.go`:

```go
// RunWithContext starts the daemon and blocks until ctx is cancelled or an
// unrecoverable error occurs. It is the context-aware variant of
// RunWithDHTDiscovery and is intended for library/embedding use.
func (d *Daemon) RunWithContext(ctx context.Context) error {
	d.ctx = ctx
	// Re-create the cancel func so Stop() can be called independently.
	// When ctx is already cancelled at call time the daemon exits immediately.
	d.ctx, d.cancel = context.WithCancel(ctx)
	return d.RunWithDHTDiscovery()
}
```

`RunWithDHTDiscovery` already selects on `d.ctx.Done()` inside the reconcile loop; no further
changes are needed inside that function.

### Task 7: Add optional `StateDir` field to `DaemonOpts` and wire it through

`WithStateDir` in `pkg/wgmesh` passes `o.stateDir` to `daemon.DaemonOpts.StateDir`, but that
field does not yet exist. Add it:

In `pkg/daemon/config.go`, add `StateDir string` to `DaemonOpts`:

```go
type DaemonOpts struct {
    // ... existing fields ...
    StateDir string // directory for keypair state files; defaults to /var/lib/wgmesh
}
```

In `NewConfig`, after the other field assignments, store it on the `Config`:

```go
cfg.StateDir = opts.StateDir
if cfg.StateDir == "" {
    cfg.StateDir = "/var/lib/wgmesh"
}
```

Add `StateDir string` to `Config`:

```go
type Config struct {
    // ... existing fields ...
    StateDir string
}
```

In `pkg/daemon/daemon.go`, replace the hardcoded path in `initLocalNode`:

```go
// Before:
stateFile := fmt.Sprintf("/var/lib/wgmesh/%s.json", d.config.InterfaceName)
// After:
stateFile := fmt.Sprintf("%s/%s.json", d.config.StateDir, d.config.InterfaceName)
```

### Task 8: Write unit tests for `pkg/wgmesh`

Create `pkg/wgmesh/wgmesh_test.go`:

```go
package wgmesh_test

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wgmesh"
)

// TestOptionDefaults verifies that functional options apply correctly.
// It does NOT start a real daemon (requires root + WG kernel module).
func TestOptionDefaults(t *testing.T) {
	// Start is not called; this test only validates the Option function types compile.
	opts := []wgmesh.Option{
		wgmesh.WithInterface("wg1"),
		wgmesh.WithListenPort(51821),
		wgmesh.WithAdvertiseRoutes([]string{"10.0.0.0/8"}),
		wgmesh.WithLogLevel("debug"),
		wgmesh.WithGossip(),
		wgmesh.WithPrivacy(),
		wgmesh.WithNoLAN(),
		wgmesh.WithDisableIPv6(),
		wgmesh.WithForceRelay(),
		wgmesh.WithDisablePunching(),
		wgmesh.WithIntroducer(),
		wgmesh.WithMeshSubnet("10.50.0.0/16"),
		wgmesh.WithStateDir(t.TempDir()),
	}
	if len(opts) == 0 {
		t.Fatal("no options constructed")
	}
}

// TestStartBadToken checks that Start returns an error for an invalid token
// without attempting to touch any network interfaces.
func TestStartBadToken(t *testing.T) {
	// An empty token must fail at config-build time, before any root action.
	_, err := wgmesh.Start(t.Context(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}
```

### Task 9: Write unit tests for `pkg/node`

Create `pkg/node/store_test.go` containing the same tests that currently exist in
`pkg/daemon/peerstore_test.go`, updated only to change the package import path from
`pkg/daemon` to `pkg/node` and to use `node.PeerStore`, `node.PeerInfo`, etc.

```go
package node_test

import (
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

func TestNewPeerStore(t *testing.T) {
	ps := node.NewPeerStore()
	if ps == nil {
		t.Fatal("NewPeerStore returned nil")
	}
	if count := ps.Count(); count != 0 {
		t.Fatalf("new store has %d peers, want 0", count)
	}
}

func TestPeerStoreAddAndGet(t *testing.T) {
	ps := node.NewPeerStore()
	info := &node.PeerInfo{
		WGPubKey: "testkey",
		MeshIP:   "10.0.0.1",
		LastSeen: time.Now(),
	}
	ps.AddOrUpdate(info)
	got, ok := ps.Get("testkey")
	if !ok {
		t.Fatal("Get returned not-ok for inserted peer")
	}
	if got.MeshIP != "10.0.0.1" {
		t.Errorf("MeshIP: got %q, want 10.0.0.1", got.MeshIP)
	}
}

func TestPeerStoreList(t *testing.T) {
	ps := node.NewPeerStore()
	ps.AddOrUpdate(&node.PeerInfo{WGPubKey: "a", LastSeen: time.Now()})
	ps.AddOrUpdate(&node.PeerInfo{WGPubKey: "b", LastSeen: time.Now()})
	if count := ps.Count(); count != 2 {
		t.Fatalf("Count: got %d, want 2", count)
	}
	list := ps.List()
	if len(list) != 2 {
		t.Fatalf("List: got %d, want 2", len(list))
	}
}

func TestPeerStoreRemove(t *testing.T) {
	ps := node.NewPeerStore()
	ps.AddOrUpdate(&node.PeerInfo{WGPubKey: "key1", LastSeen: time.Now()})
	ps.Remove("key1")
	if _, ok := ps.Get("key1"); ok {
		t.Fatal("Get returned ok after Remove")
	}
}

func TestLocalNodeEndpoint(t *testing.T) {
	n := &node.LocalNode{}
	if ep := n.GetEndpoint(); ep != "" {
		t.Fatalf("initial endpoint: got %q, want empty", ep)
	}
	n.SetEndpoint("1.2.3.4:51820")
	if ep := n.GetEndpoint(); ep != "1.2.3.4:51820" {
		t.Fatalf("after SetEndpoint: got %q, want 1.2.3.4:51820", ep)
	}
}
```

## Affected Files

| File | Change |
|------|--------|
| `pkg/node/types.go` | **New** — `DiscoveryLayer`, `LocalNode`, `TunnelBackend`, `PeerConfig`, `PeerStatus`, `ConfigReader` interfaces and types |
| `pkg/node/store.go` | **New** — `PeerStore`, `PeerInfo`, `PeerEvent` moved from `pkg/daemon/peerstore.go` |
| `pkg/node/store_test.go` | **New** — unit tests for `pkg/node` types |
| `pkg/daemon/peerstore.go` | **Delete** — replaced by `pkg/node/store.go` |
| `pkg/daemon/peerstore_compat.go` | **New** — type aliases pointing at `pkg/node` so all daemon-internal code continues to compile |
| `pkg/daemon/daemon.go` | Add `type LocalNode = node.LocalNode`; add `type DiscoveryLayer = node.DiscoveryLayer`; remove `GetEndpoint`/`SetEndpoint`; update `DHTDiscoveryFactory` signature; add `RunWithContext`; add `"pkg/node"` import |
| `pkg/daemon/config.go` | Add `StateDir` to `DaemonOpts` and `Config`; add `ConfigReader` accessor methods; add compile-time interface check |
| `pkg/daemon/helpers.go` | Replace hardcoded `/var/lib/wgmesh` with `d.config.StateDir` in `initLocalNode` (called from `daemon.go`) |
| `pkg/discovery/dht.go` | Replace `*daemon.Config` with `node.ConfigReader`; replace `*daemon.LocalNode`/`*daemon.PeerStore` with `*node.LocalNode`/`*node.PeerStore`; update all `config.Keys.*` field accesses to use `ConfigReader` methods; remove `pkg/daemon` import |
| `pkg/discovery/exchange.go` | Same substitution as `dht.go` |
| `pkg/discovery/gossip.go` | Same substitution as `dht.go` |
| `pkg/discovery/lan.go` | Same substitution as `dht.go` |
| `pkg/discovery/init.go` | Update factory signature to use `node` types; remove `pkg/daemon` import |
| `pkg/wgmesh/wgmesh.go` | **New** — public embeddable `Start` / `Stop` / `Peers` / `LocalIP` API |
| `pkg/wgmesh/wgmesh_test.go` | **New** — compile-level and bad-token unit tests |

No changes to `go.mod`/`go.sum` — all additions use existing dependencies.

## Test Strategy

Run after implementation:

```bash
# Verify no daemon import in discovery
grep -r '"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"' pkg/discovery/
# Must produce no output

# Build and unit-test
go build ./...
go test ./pkg/node/...
go test ./pkg/wgmesh/...
go test ./pkg/daemon/...
go test ./pkg/discovery/...
go test -race ./...
```

Manual embed smoke-test (requires root + WireGuard kernel module):

```bash
# Build a minimal test binary
cat > /tmp/embed_test/main.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/atvirokodosprendimai/wgmesh/pkg/wgmesh"
    _ "github.com/atvirokodosprendimai/wgmesh/pkg/discovery"
)

func main() {
    secret := "wgmesh://v1/dGVzdHNlY3JldHRlc3RzZWNyZXQ="
    m, err := wgmesh.Start(
        context.Background(),
        secret,
        wgmesh.WithInterface("wg99"),
        wgmesh.WithStateDir("/tmp/wgmesh-embed-test"),
        wgmesh.WithLogLevel("debug"),
    )
    if err != nil {
        log.Fatalf("Start: %v", err)
    }
    fmt.Println("LocalIP:", m.LocalIP())
    fmt.Println("Peers:  ", len(m.Peers()))
    m.Stop()
}
EOF
cd /tmp/embed_test && go mod init embed_test
go mod edit -replace github.com/atvirokodosprendimai/wgmesh=/home/runner/work/wgmesh/wgmesh
go mod tidy
sudo go run .
```

Expected outcome: binary compiles and prints a mesh IP without errors. The `pkg/discovery`
import must not trigger a cyclic-import error.

## Estimated Complexity
high

**Reasoning:** The change touches seven existing files in two packages plus introduces four new
files. The most error-prone step is replacing every `config.Keys.*` field access in
`pkg/discovery/dht.go` (the largest file at ~800 lines) with `ConfigReader` method calls. The
type-alias approach in `pkg/daemon/peerstore_compat.go` minimises churn in the daemon package
but must be verified to compile cleanly with the `-race` flag. Estimated effort: 4–6 hours.
