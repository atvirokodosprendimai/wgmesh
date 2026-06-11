# Issue #539 Spec: Start/Stop with Key and FD for Android VPN API Integration

## Classification
feature

## Problem Analysis

### Current State

The wgmesh daemon currently manages WireGuard interfaces by:

1. Creating network interfaces directly via OS commands (`ip link add`, `wireguard-go`, etc.)
2. Generating WireGuard keypairs using the `wg genkey` command-line tool
3. Configuring interfaces with the `wg set` command

This approach works on Linux and macOS but is incompatible with Android's VPN API, which:

- Requires the VPN application to use a file descriptor (FD) provided by the Android system
- Does not allow direct creation of WireGuard interfaces via network commands
- Expects the application to generate and manage cryptographic keys programmatically
- Requires explicit Start() and Stop() lifecycle management for the VPN tunnel

### Requirements

Android VPN API constraints necessitate:

1. **FD-based interface creation**: Accept a pre-configured file descriptor from the Android VPN system
2. **Key-based initialization**: Accept a pre-generated WireGuard private key as a byte slice
3. **Explicit lifecycle management**: Provide Start() and Stop() methods for VPN tunnel control
4. **Programmatic operation**: No dependency on external command-line tools (`wg`, `wg genkey`, `ip`, etc.)

### Gap Analysis

Missing functionality:
- No interface to accept external file descriptors for WireGuard interface creation
- No method to initialize WireGuard with a provided private key byte slice
- No Start/Stop lifecycle methods for VPN tunnel management
- Dependency on `wg` binary for configuration, which may not be available in Android environments

## Proposed Approach

### 1. New Interface: WGDevice

Create a new `WGDevice` interface in `pkg/wireguard/device.go`:

```go
// WGDevice defines the interface for managing a WireGuard device.
// This abstraction supports both system-managed interfaces (Linux/macOS)
// and application-provided file descriptors (Android VPN API).
type WGDevice interface {
    // Start activates the WireGuard device with the provided configuration.
    // For FD-based devices, this begins processing traffic on the file descriptor.
    // For system interfaces, this ensures the interface is up and configured.
    Start() error

    // Stop deactivates the WireGuard device.
    // For FD-based devices, this stops processing and may close the file descriptor.
    // For system interfaces, this brings the interface down.
    Stop() error

    // SetPeer configures a peer on the device.
    SetPeer(pubKey string, endpoint string, allowedIPs []string, persistentKeepalive int) error

    // RemovePeer removes a peer from the device.
    RemovePeer(pubKey string) error

    // GetPeers returns the list of configured peers.
    GetPeers() ([]string, error)

    // Close performs final cleanup and resource release.
    Close() error
}
```

### 2. FD-based Implementation

Create `pkg/wireguard/fddevice.go` with `FDDevice` implementation:

```go
// FDDevice implements WGDevice for Android VPN API integration.
// It uses a file descriptor provided by the Android VPN system.
type FDDevice struct {
    fd         int
    privateKey []byte
    listenPort int
    peers      map[string]*peerConfig
    mu         sync.RWMutex
    running    bool
    closeOnce  sync.Once
}

// NewFDDevice creates a new FDDevice with the provided file descriptor and private key.
func NewFDDevice(fd int, privateKey []byte, listenPort int) (*FDDevice, error) {
    if fd < 0 {
        return nil, fmt.Errorf("invalid file descriptor: %d", fd)
    }
    if len(privateKey) == 0 {
        return nil, fmt.Errorf("private key cannot be empty")
    }

    return &FDDevice{
        fd:         fd,
        privateKey: privateKey,
        listenPort: listenPort,
        peers:      make(map[string]*peerConfig),
    }, nil
}
```

Key implementation details:
- Use `go.zedro/go/wireguard` userspace library for FD-based operation
- Implement Start() to begin reading/writing to the file descriptor
- Implement Stop() to halt traffic processing
- Manage peer configurations in memory
- Thread-safe with sync.RWMutex

### 3. System Device Implementation

Create `pkg/wireguard/sysdevice.go` with `SysDevice` implementation:

```go
// SysDevice implements WGDevice for traditional system-managed interfaces.
// This wraps the existing command-line based operations.
type SysDevice struct {
    ifaceName string
    privateKey string
    listenPort int
    mu         sync.RWMutex
}

// NewSysDevice creates a new SysDevice for the specified interface.
func NewSysDevice(ifaceName string, privateKey string, listenPort int) (*SysDevice, error) {
    return &SysDevice{
        ifaceName: ifaceName,
        privateKey: privateKey,
        listenPort: listenPort,
    }, nil
}
```

This wraps existing functions from `apply.go` and `helpers.go`:
- Start() calls `createInterface()`, `configureInterface()`, `setInterfaceAddress()`, `setInterfaceUp()`
- Stop() calls `setInterfaceDown()`, `resetInterface()`, `deleteInterface()`
- SetPeer/RemovePeer use existing `wg set` commands

### 4. Key Generation Helper

Create `pkg/wireguard/keys.go` additions:

```go
// GenerateKeyPairBytes generates a WireGuard keypair without external dependencies.
// Returns (privateKey, publicKey, error) as raw byte slices (32 bytes each).
func GenerateKeyPairBytes() ([]byte, []byte, error) {
    // Use golang.org/x/crypto/curve25519 for key generation
    // This provides a drop-in replacement for 'wg genkey'
}

// PrivateKeyToPublicKey derives a public key from a private key byte slice.
func PrivateKeyToPublicKey(privateKey []byte) ([]byte, error) {
    // Curve25519 key derivation
}
```

This eliminates dependency on the `wg` binary for key generation.

### 5. Daemon Integration

Modify `pkg/daemon/daemon.go`:

```go
type Daemon struct {
    // ... existing fields ...

    wgDevice   wireguard.WGDevice  // Replace direct interface management
}

func (d *Daemon) setupWireGuard() error {
    // Use WGDevice abstraction
    if d.config.VPNFD > 0 {
        // Android VPN mode: use FD-based device
        privKeyBytes, err := wireguard.ParseKey(d.localNode.WGPrivateKey)
        if err != nil {
            return fmt.Errorf("parsing private key: %w", err)
        }
        d.wgDevice, err = wireguard.NewFDDevice(d.config.VPNFD, privKeyBytes, d.config.WGListenPort)
    } else {
        // Traditional mode: use system device
        d.wgDevice, err = wireguard.NewSysDevice(d.config.InterfaceName, d.localNode.WGPrivateKey, d.config.WGListenPort)
    }
    if err != nil {
        return err
    }

    return d.wgDevice.Start()
}

func (d *Daemon) teardownWireGuard() error {
    if d.wgDevice != nil {
        d.wgDevice.Stop()
        return d.wgDevice.Close()
    }
    return nil
}
```

### 6. Configuration Extension

Extend `pkg/daemon/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // VPNFD is the file descriptor for VPN operation (Android VPN API).
    // If > 0, the daemon uses FD-based device management.
    // If 0, the daemon uses traditional system interface management.
    VPNFD int `json:"vpn_fd,omitempty"`
}
```

## Acceptance Criteria

1. **WGDevice interface exists** in `pkg/wireguard/device.go` with Start(), Stop(), SetPeer(), RemovePeer(), GetPeers(), Close() methods

2. **FDDevice implementation** in `pkg/wireguard/fddevice.go`:
   - `NewFDDevice(fd int, key []byte, port int) (*FDDevice, error)` constructor
   - Accepts valid file descriptor (> 0)
   - Accepts 32-byte private key slice
   - Start() begins processing traffic on the FD
   - Stop() halts traffic processing
   - SetPeer() configures peers programmatically
   - RemovePeer() removes peers
   - Thread-safe with mutex protection

3. **SysDevice implementation** in `pkg/wireguard/sysdevice.go`:
   - `NewSysDevice(iface string, key string, port int) (*SysDevice, error)` constructor
   - Wraps existing command-line operations
   - Start() creates and configures system interface
   - Stop() tears down system interface
   - SetPeer()/RemovePeer() use `wg set` commands

4. **Key generation without external tools**:
   - `GenerateKeyPairBytes() ([]byte, []byte, error)` in `pkg/wireguard/keys.go`
   - `PrivateKeyToPublicKey(privateKey []byte) ([]byte, error)` in `pkg/wireguard/keys.go`
   - No dependency on `wg genkey` binary

5. **Daemon integration**:
   - `Daemon` struct uses `WGDevice` interface instead of direct operations
   - `setupWireGuard()` creates appropriate device based on `VPNFD` config
   - `teardownWireGuard()` calls Stop() and Close() on device
   - `Config` struct includes `VPNFD int` field

6. **Testing**:
   - Unit tests for FDDevice (mock file operations)
   - Unit tests for SysDevice (using existing test infrastructure)
   - Unit tests for key generation functions
   - Integration test for daemon with both device types

7. **Backward compatibility**:
   - Existing Linux/macOS deployments continue to work unchanged
   - No breaking changes to existing API or CLI

## Out of scope

- Android-specific VPN Builder API integration (Java/Kotlin bindings)
- Android permission handling and VPN intent creation
- Network configuration on Android (routing, DNS)
- Userspace WireGuard implementation from scratch (will use `go.zedro/go/wireguard` library)
- Key rotation and management enhancements
- Windows TUN/TAP support
- iOS NetworkExtension support

## Affected Files

### New Files
- `pkg/wireguard/device.go` - WGDevice interface definition
- `pkg/wireguard/fddevice.go` - FD-based device implementation for Android VPN API
- `pkg/wireguard/sysdevice.go` - System device implementation wrapping existing operations

### Modified Files
- `pkg/wireguard/keys.go` - Add GenerateKeyPairBytes(), PrivateKeyToPublicKey()
- `pkg/daemon/daemon.go` - Integrate WGDevice interface, update setupWireGuard(), teardownWireGuard()
- `pkg/daemon/config.go` - Add VPNFD field to Config struct
- `pkg/wireguard/apply.go` - May need refactoring to support SysDevice (no breaking changes)

## Test Strategy

### Unit Tests

1. **FDDevice tests** (`pkg/wireguard/fddevice_test.go`):
   - Mock file descriptor operations
   - Test NewFDDevice with valid and invalid parameters
   - Test Start() lifecycle
   - Test Stop() lifecycle
   - Test SetPeer() with various configurations
   - Test RemovePeer()
   - Test concurrent operations (race detection)

2. **SysDevice tests** (`pkg/wireguard/sysdevice_test.go`):
   - Use existing test infrastructure from `apply_test.go`
   - Test NewSysDevice constructor
   - Test Start() creates and configures interface
   - Test Stop() tears down interface
   - Test SetPeer()/RemovePeer() operations

3. **Key generation tests** (`pkg/wireguard/keys_test.go`):
   - Extend existing tests
   - Test GenerateKeyPairBytes() produces valid Curve25519 keys
   - Test PrivateKeyToPublicKey() derivation
   - Test key validation and error cases

### Integration Tests

4. **Daemon integration tests** (`pkg/daemon/daemon_test.go`):
   - Test daemon startup with VPNFD=0 (system mode)
   - Test daemon startup with VPNFD>0 (FD mode)
   - Test proper device selection
   - Test peer operations through both device types

### Performance Considerations

- FDDevice should minimize syscalls for efficient mobile operation
- Peer configuration updates should be batched when possible
- Start()/Stop() must complete quickly (< 100ms) for responsive UI

## Estimated Complexity

**Medium** (3-5 days implementation + testing)

- Interface design: 0.5 day
- FDDevice implementation: 1.5 days (including dependency integration)
- SysDevice wrapper: 0.5 day
- Key generation functions: 0.5 day
- Daemon integration: 0.5 day
- Testing and documentation: 1-2 days
