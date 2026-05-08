# Specification: Issue #539

## Classification
feature

## Deliverables
code

## Problem Analysis

Android's VPN Service API creates a TUN device and hands the application a raw file descriptor
(`int`) for it. The application does **not** create the interface — the OS does — and the
application has no interface name to pass to `wg` or `ip`. As a result the existing
`setupWireGuard()` flow in `pkg/daemon/daemon.go` (which calls `createInterface`,
`configureInterface`, `setInterfaceAddress`, `setInterfaceUp`) cannot be used on Android.

What is needed is a way to start the mesh daemon when the caller already holds a TUN fd and
a raw WireGuard private key, so that:

1. The daemon **skips** `createInterface` / `setInterfaceAddress` / `setInterfaceUp` (those
   steps were done by the Android VPN Service before handing us the fd).
2. The daemon **uses** the fd to configure WireGuard peers via the userspace WireGuard
   control interface (e.g. `wireguard-go` unix socket) or directly via
   `golang.zx2c4.com/wireguard`.
3. On `Stop()` the daemon tears down gracefully without trying to `deleteInterface` (Android
   closes the fd itself when the VPN Service is revoked).

The change must be **additive**: existing Linux/macOS users who never pass an fd must
experience zero behaviour change.

The embeddable API lives in (or will live in, per spec #536) `pkg/wgmesh`. Callers there
call:

```go
m, err := wgmesh.Start(ctx, "wgmesh://v1/<base64>", wgmesh.WithTunFd(fd))
```

On Android the VPN Service calls:

```go
m, err := wgmesh.StartWithFd(ctx, key, fd)
```

Both entry points converge on the same daemon path.

## Implementation Tasks

### Task 1: Add `TunFd` and `TunPrivateKey` to `DaemonOpts` in `pkg/daemon/config.go`

In `pkg/daemon/config.go`, add two new fields to the `DaemonOpts` struct:

```go
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
	MeshSubnet          string // Custom mesh subnet CIDR (e.g. "192.168.100.0/24")
	TunFd               int    // Pre-created TUN file descriptor (Android VPN API). 0 = unused.
	TunPrivateKey       []byte // Raw 32-byte WireGuard private key for TunFd mode.
}
```

Add two new fields to the `Config` struct (keep all existing fields unchanged):

```go
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
	CustomSubnet    *net.IPNet
	TunFd           int    // 0 means "not set / create interface the normal way"
	TunPrivateKey   []byte // non-nil in TunFd mode; 32 raw WG private key bytes
}
```

In `NewConfig`, add the following validation block **before** the `return &Config{...}` literal
(i.e. after the `customSubnet` validation block and before the `return` statement), and then
add the two fields to the returned struct literal:

```go
	// Validate TunFd options before building the Config.
	if opts.TunFd != 0 {
		if len(opts.TunPrivateKey) != 32 {
			return nil, fmt.Errorf("TunPrivateKey must be exactly 32 bytes when TunFd is set")
		}
	}

	return &Config{
		// ... all existing fields unchanged ...
		TunFd:         opts.TunFd,
		TunPrivateKey: opts.TunPrivateKey,
	}, nil
```

Replace the existing `return &Config{...}, nil` statement with the version above that
includes the two new fields. All other fields in the literal remain unchanged.

### Task 2: Add `isTunFdMode()` helper and modify `setupWireGuard` in `pkg/daemon/daemon.go`

Add a private helper on `*Daemon`:

```go
// isTunFdMode returns true when the daemon was started with a pre-created TUN fd
// (Android VPN API). In this mode the WireGuard interface is managed externally.
func (d *Daemon) isTunFdMode() bool {
	return d.config.TunFd != 0
}
```

Modify `setupWireGuard` so that the entire body executes only in normal (non-fd) mode.
Specifically, change the function as follows — **replace** the existing `setupWireGuard`:

```go
// setupWireGuard creates and configures the WireGuard interface.
// In TunFd mode (Android) the interface was already created by the OS; only
// configure WireGuard keys and peers via the fd-based backend.
func (d *Daemon) setupWireGuard() error {
	if d.isTunFdMode() {
		return d.setupWireGuardFromFd()
	}

	log.Printf("Setting up WireGuard interface %s...", d.config.InterfaceName)

	// Check if interface exists
	if interfaceExists(d.config.InterfaceName) {
		existingPort := getWGInterfacePort(d.config.InterfaceName)
		if existingPort == d.config.WGListenPort {
			log.Printf("Interface %s exists with same port, resetting...", d.config.InterfaceName)
		} else {
			log.Printf("Interface %s exists, resetting...", d.config.InterfaceName)
		}
		if err := resetInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to reset interface: %w", err)
		}
	} else {
		if err := createInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to create interface: %w", err)
		}
	}

	listenPort := d.config.WGListenPort
	if isPortInUse(listenPort) {
		availablePort := findAvailablePort(listenPort + 1)
		if availablePort == 0 {
			return fmt.Errorf("port %d is in use and no available ports found (try --listen-port with a different port)", listenPort)
		}
		log.Printf("Port %d is in use, using port %d instead", listenPort, availablePort)
		listenPort = availablePort
		d.config.WGListenPort = availablePort
	}

	if err := configureInterface(d.config.InterfaceName, d.localNode.WGPrivateKey, listenPort); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	if err := setInterfaceAddress(d.config.InterfaceName, fmt.Sprintf("%s/%d", d.localNode.MeshIP, d.config.PrefixLen())); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}
	if d.localNode.MeshIPv6 != "" {
		if err := setInterfaceAddress(d.config.InterfaceName, d.localNode.MeshIPv6+"/64"); err != nil {
			return fmt.Errorf("failed to set IPv6 address: %w", err)
		}
	}

	if err := setInterfaceUp(d.config.InterfaceName); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	log.Printf("WireGuard interface %s ready on port %d", d.config.InterfaceName, listenPort)
	return nil
}
```

Add the new `setupWireGuardFromFd` method to `pkg/daemon/daemon.go`:

```go
// setupWireGuardFromFd configures the WireGuard stack using a pre-existing TUN fd
// (returned by Android VPN Service). It stores the supplied private key in
// localNode so the reconcile loop can configure peers normally.
// Must be called only from the daemon goroutine before the reconcile loop
// starts (no concurrent localNode access at that point).
func (d *Daemon) setupWireGuardFromFd() error {
	if len(d.config.TunPrivateKey) != 32 {
		return fmt.Errorf("setupWireGuardFromFd: TunPrivateKey must be 32 bytes")
	}

	// Encode the raw key as base64 (WireGuard standard encoding).
	privKeyB64 := base64.StdEncoding.EncodeToString(d.config.TunPrivateKey)

	// localNode is initialised by initLocalNode before setupWireGuard is called.
	// If for any reason it is nil (e.g. in unit tests), create a minimal struct.
	// No mutex is needed here because setupWireGuard is called sequentially
	// during daemon startup, before the reconcile goroutines are launched.
	if d.localNode == nil {
		d.localNode = &LocalNode{}
	}
	d.localNode.WGPrivateKey = privKeyB64

	log.Printf("WireGuard configured from pre-created TUN fd %d", d.config.TunFd)
	return nil
}
```

This method must import `"encoding/base64"` — add it to the imports in `daemon.go` if not
already present.

### Task 3: Skip interface teardown in TunFd mode in `pkg/daemon/daemon.go`

Modify `teardownWireGuard` to skip `setInterfaceDown` / `deleteInterface` when in fd mode
(Android controls the fd lifetime):

```go
func (d *Daemon) teardownWireGuard() {
	if d == nil || d.config == nil || d.config.InterfaceName == "" {
		return
	}
	if d.isTunFdMode() {
		log.Printf("[Shutdown] TunFd mode: skipping interface teardown (fd %d)", d.config.TunFd)
		return
	}
	if err := setInterfaceDown(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to bring down interface %s: %v", d.config.InterfaceName, err)
	}
	if err := deleteInterface(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to delete interface %s: %v", d.config.InterfaceName, err)
		return
	}
	log.Printf("[Shutdown] WireGuard interface %s removed", d.config.InterfaceName)
}
```

### Task 4: Add `WithTunFd` option to the embeddable API in `pkg/wgmesh/wgmesh.go`

The `pkg/wgmesh` package is introduced by spec #536. This task **adds** two items to that
file (or creates the file if #536 has not yet been merged):

1. A new field in the internal `options` struct:

```go
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
```

2. Two new exported option functions:

```go
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
```

3. Wire the new fields through `Start`:

Inside the `Start` function, in the `daemon.DaemonOpts{...}` literal, add:

```go
		TunFd:         o.tunFd,
		TunPrivateKey: o.tunPrivateKey,
```

### Task 5: Add `StartWithFd` convenience function in `pkg/wgmesh/wgmesh.go`

Add the following function at the end of `pkg/wgmesh/wgmesh.go`. This is the primary
Android-targeted entry point that matches the signature requested in the issue:

```go
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
```

### Task 6: Add unit tests in `pkg/daemon/daemon_fd_test.go`

Create the new file `pkg/daemon/daemon_fd_test.go`:

```go
package daemon

import (
	"testing"
)

// TestIsTunFdMode verifies the isTunFdMode helper.
func TestIsTunFdMode(t *testing.T) {
	tests := []struct {
		name  string
		tunFd int
		want  bool
	}{
		{"zero fd = normal mode", 0, false},
		{"positive fd = tun mode", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{config: &Config{TunFd: tt.tunFd}}
			if got := d.isTunFdMode(); got != tt.want {
				t.Errorf("isTunFdMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSetupWireGuardFromFd verifies that setupWireGuardFromFd stores the
// base64-encoded private key in localNode and does not attempt to create or
// configure a system WireGuard interface.
func TestSetupWireGuardFromFd(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: key,
		},
	}

	if err := d.setupWireGuardFromFd(); err != nil {
		t.Fatalf("setupWireGuardFromFd() error: %v", err)
	}

	if d.localNode == nil {
		t.Fatal("localNode is nil after setupWireGuardFromFd")
	}
	if d.localNode.WGPrivateKey == "" {
		t.Error("localNode.WGPrivateKey is empty after setupWireGuardFromFd")
	}
}

// TestSetupWireGuardFromFd_BadKey verifies that an invalid key length is rejected.
func TestSetupWireGuardFromFd_BadKey(t *testing.T) {
	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: []byte{1, 2, 3}, // too short
		},
	}
	if err := d.setupWireGuardFromFd(); err == nil {
		t.Error("expected error for short TunPrivateKey, got nil")
	}
}

// TestTeardownWireGuardTunFdMode verifies that teardownWireGuard is a no-op
// (does not call deleteInterface) when in TunFd mode.
func TestTeardownWireGuardTunFdMode(t *testing.T) {
	// Replace cmdExecutor with a recorder to assert no wg/ip commands are run.
	orig := cmdExecutor
	defer func() { cmdExecutor = orig }()
	rec := &recordingExecutor{}
	cmdExecutor = rec

	d := &Daemon{
		config: &Config{
			InterfaceName: "wg0",
			TunFd:         5,
		},
	}
	d.teardownWireGuard()

	if len(rec.commands) != 0 {
		t.Errorf("teardownWireGuard in TunFd mode ran unexpected commands: %v", rec.commands)
	}
}

// recordingExecutor captures all commands issued through cmdExecutor.
type recordingExecutor struct {
	commands [][]string
}

func (r *recordingExecutor) Command(name string, args ...string) Cmd {
	r.commands = append(r.commands, append([]string{name}, args...))
	return &noopCmd{}
}

// noopCmd is a Cmd that does nothing.
type noopCmd struct{}

func (n *noopCmd) Output() ([]byte, error)                 { return nil, nil }
func (n *noopCmd) CombinedOutput() ([]byte, error)         { return nil, nil }
func (n *noopCmd) Run() error                              { return nil }
func (n *noopCmd) Start() error                            { return nil }
func (n *noopCmd) Wait() error                             { return nil }
func (n *noopCmd) SetStdin(_ interface{})                  {}
func (n *noopCmd) SetStdout(_ interface{})                 {}
func (n *noopCmd) SetStderr(_ interface{})                 {}
```

**Note:** Check the existing `CommandExecutor` and `Cmd` interface definitions in
`pkg/daemon/helpers.go` or a `_test.go` file. Adapt the `recordingExecutor` to match the
exact interface methods already defined. Do not introduce a new interface — reuse the
existing one.

## Affected Files

| File | Change |
|------|--------|
| `pkg/daemon/config.go` | Add `TunFd int` and `TunPrivateKey []byte` to `DaemonOpts` and `Config`; validate in `NewConfig` |
| `pkg/daemon/daemon.go` | Add `isTunFdMode()`, `setupWireGuardFromFd()`; modify `setupWireGuard()` and `teardownWireGuard()` |
| `pkg/wgmesh/wgmesh.go` | Add `tunFd`/`tunPrivateKey` to `options`; add `WithTunFd()` option; add `StartWithFd()` function; wire through `Start()` |
| `pkg/daemon/daemon_fd_test.go` | New file: unit tests for fd mode helpers |

## Test Strategy

1. Run `go test ./pkg/daemon/... -run TestIsTunFdMode` — must pass.
2. Run `go test ./pkg/daemon/... -run TestSetupWireGuardFromFd` — must pass.
3. Run `go test ./pkg/daemon/... -run TestSetupWireGuardFromFd_BadKey` — must pass.
4. Run `go test ./pkg/daemon/... -run TestTeardownWireGuardTunFdMode` — must pass.
5. Run `go build ./...` — must compile without errors.
6. Run `go test -race ./pkg/daemon/...` — must pass with no race conditions.
7. Existing test suite `go test ./...` must remain green (no regressions).

## Estimated Complexity
low
