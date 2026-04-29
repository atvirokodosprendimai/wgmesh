# Specification: Issue #540

## Classification
fix

## Deliverables
code

## Problem Analysis

When a node joins the mesh with a given secret (`wgmesh join --secret <TOKEN>`), its mesh IP
address is derived as:

```
meshIP = SHA256(wgPubKey + secret)
```

This is computed in `pkg/crypto/derive.go` by `DeriveMeshIP`, `DeriveMeshIPv6`, and
`DeriveMeshIPInSubnet`. The derivation uses **both** the WireGuard public key **and** the
secret, so changing the secret changes the IP even when the same WireGuard keypair is reused.

The WireGuard keypair (private + public key) is persisted in
`/var/lib/wgmesh/{interface}.json` via `localNodeState` in `pkg/daemon/helpers.go`.
However, `localNodeState` only stores `wg_pubkey` and `wg_private_key`; the mesh IP and
IPv6 address are **not** stored.

On every startup (see `initLocalNode` in `pkg/daemon/daemon.go` lines 303–367), the mesh IP
is re-derived using the current secret. Consequently, rotating the secret causes the IP to
change even though the WireGuard identity (keypair) is unchanged — breaking reachability for
every peer that knew the old IP.

**Root cause:** `localNodeState` does not persist `mesh_ip` and `mesh_ipv6`. Every start
re-derives the IP from the current secret, so a secret rotation silently changes the node's
address.

## Implementation Tasks

### Task 1: Add `MeshIP` and `MeshIPv6` fields to `localNodeState` in `pkg/daemon/helpers.go`

Locate the `localNodeState` struct (lines 45–49 of `pkg/daemon/helpers.go`):

```go
type localNodeState struct {
	WGPubKey     string `json:"wg_pubkey"`
	WGPrivateKey string `json:"wg_private_key"`
}
```

Replace it with:

```go
type localNodeState struct {
	WGPubKey     string `json:"wg_pubkey"`
	WGPrivateKey string `json:"wg_private_key"`
	MeshIP       string `json:"mesh_ip,omitempty"`
	MeshIPv6     string `json:"mesh_ipv6,omitempty"`
}
```

The `omitempty` tags ensure the new fields are backward-compatible: older state files that
lack these fields deserialise with empty strings, which triggers the fallback derivation in
Task 2.

### Task 2: Restore persisted mesh IP in `loadLocalNode` in `pkg/daemon/helpers.go`

Locate `loadLocalNode` (lines 51–67 of `pkg/daemon/helpers.go`):

```go
func loadLocalNode(path string) (*LocalNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state localNodeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &LocalNode{
		WGPubKey:     state.WGPubKey,
		WGPrivateKey: state.WGPrivateKey,
	}, nil
}
```

Replace it with:

```go
func loadLocalNode(path string) (*LocalNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state localNodeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &LocalNode{
		WGPubKey:     state.WGPubKey,
		WGPrivateKey: state.WGPrivateKey,
		MeshIP:       state.MeshIP,
		MeshIPv6:     state.MeshIPv6,
	}, nil
}
```

### Task 3: Persist mesh IP in `saveLocalNode` in `pkg/daemon/helpers.go`

Locate `saveLocalNode` (lines 69–89 of `pkg/daemon/helpers.go`).

Replace:

```go
state := localNodeState{
	WGPubKey:     node.WGPubKey,
	WGPrivateKey: node.WGPrivateKey,
}
```

With:

```go
state := localNodeState{
	WGPubKey:     node.WGPubKey,
	WGPrivateKey: node.WGPrivateKey,
	MeshIP:       node.MeshIP,
	MeshIPv6:     node.MeshIPv6,
}
```

### Task 4: Use persisted mesh IP in `initLocalNode` in `pkg/daemon/daemon.go`

Locate `initLocalNode` (lines 303–367 of `pkg/daemon/daemon.go`). The current logic for the
"node already exists" branch (lines 313–330) unconditionally re-derives the mesh IP regardless
of what was persisted. Replace the entire "loaded existing node" block so that the persisted
IP is used when present and valid, and re-derived only when the persisted IP is absent or
belongs to a different subnet.

Current code (lines 313–330):

```go
node, err := loadLocalNode(stateFile)
if err == nil && node != nil {
	d.localNode = node
	// Derive mesh IP from pubkey
	if d.config.CustomSubnet != nil {
		ip, err := crypto.DeriveMeshIPInSubnet(d.config.CustomSubnet, d.localNode.WGPubKey, d.config.Secret)
		if err != nil {
			return fmt.Errorf("failed to derive mesh IP in custom subnet: %w", err)
		}
		d.localNode.MeshIP = ip
	} else {
		d.localNode.MeshIP = crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret)
	}
	d.localNode.MeshIPv6 = crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, d.localNode.WGPubKey, d.config.Secret)
	d.localNode.RoutableNetworks = d.config.AdvertiseRoutes
	d.localNode.Introducer = d.config.Introducer
	d.localNode.Hostname = hostname
	return nil
}
```

Replace it with:

```go
node, err := loadLocalNode(stateFile)
if err == nil && node != nil {
	d.localNode = node

	// Use the persisted mesh IP when it is present and falls within the
	// expected subnet. Re-derive only when the field is absent (old state
	// file) or the configured subnet has changed.
	if node.MeshIP != "" && meshIPInSubnet(node.MeshIP, d.config) {
		// Persisted IP is valid — keep it so that secret rotation does not
		// change the node's address.
	} else {
		if d.config.CustomSubnet != nil {
			ip, err := crypto.DeriveMeshIPInSubnet(d.config.CustomSubnet, d.localNode.WGPubKey, d.config.Secret)
			if err != nil {
				return fmt.Errorf("failed to derive mesh IP in custom subnet: %w", err)
			}
			d.localNode.MeshIP = ip
		} else {
			d.localNode.MeshIP = crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret)
		}
		// Persist the newly derived IP so subsequent starts reuse it.
		if err := saveLocalNode(stateFile, d.localNode); err != nil {
			log.Printf("Warning: failed to save local node state: %v", err)
		}
	}

	// Always re-derive IPv6; IPv6 addresses are not globally routable and
	// subnet pinning is not applicable for the /64 ULA prefix.
	if node.MeshIPv6 == "" {
		d.localNode.MeshIPv6 = crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, d.localNode.WGPubKey, d.config.Secret)
		if err := saveLocalNode(stateFile, d.localNode); err != nil {
			log.Printf("Warning: failed to save local node state: %v", err)
		}
	}

	d.localNode.RoutableNetworks = d.config.AdvertiseRoutes
	d.localNode.Introducer = d.config.Introducer
	d.localNode.Hostname = hostname
	return nil
}
```

### Task 5: Add helper `meshIPInSubnet` in `pkg/daemon/daemon.go`

Add the following unexported helper function anywhere in `pkg/daemon/daemon.go` (for example
just before `initLocalNode`):

```go
// meshIPInSubnet returns true when the given IP string falls within the mesh
// subnet implied by cfg. This is used to detect when a persisted mesh IP is no
// longer valid because the operator changed --mesh-subnet.
func meshIPInSubnet(meshIP string, cfg *Config) bool {
	ip := net.ParseIP(meshIP)
	if ip == nil {
		return false
	}
	if cfg.CustomSubnet != nil {
		return cfg.CustomSubnet.Contains(ip)
	}
	// Legacy derivation: 10.<meshSubnet[0]>.x.y — check the /8 prefix only.
	subnet := &net.IPNet{
		IP:   net.IP{10, cfg.Keys.MeshSubnet[0], 0, 0},
		Mask: net.CIDRMask(16, 32),
	}
	return subnet.Contains(ip)
}
```

The `net` package is already imported in `pkg/daemon/daemon.go`.

### Task 6: Add tests in `pkg/daemon/helpers_test.go`

Add table-driven unit tests for the updated `loadLocalNode` / `saveLocalNode` cycle. The tests
must be added to `pkg/daemon/helpers_test.go`.

Add the following test function:

```go
func TestLocalNodeStatePersistsMeshIP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.json")

	original := &LocalNode{
		WGPubKey:     "pub-key-abc",
		WGPrivateKey: "priv-key-xyz",
		MeshIP:       "10.42.7.33",
		MeshIPv6:     "fd12:3456:789a:0001::1",
	}

	if err := saveLocalNode(path, original); err != nil {
		t.Fatalf("saveLocalNode: %v", err)
	}

	loaded, err := loadLocalNode(path)
	if err != nil {
		t.Fatalf("loadLocalNode: %v", err)
	}

	if loaded.MeshIP != original.MeshIP {
		t.Errorf("MeshIP: got %q, want %q", loaded.MeshIP, original.MeshIP)
	}
	if loaded.MeshIPv6 != original.MeshIPv6 {
		t.Errorf("MeshIPv6: got %q, want %q", loaded.MeshIPv6, original.MeshIPv6)
	}
}

func TestLocalNodeStateBackwardCompatibility(t *testing.T) {
	t.Parallel()

	// Simulate an old state file that has no mesh_ip / mesh_ipv6 fields.
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0-old.json")
	oldJSON := `{"wg_pubkey":"pub-key-abc","wg_private_key":"priv-key-xyz"}`
	if err := os.WriteFile(path, []byte(oldJSON), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := loadLocalNode(path)
	if err != nil {
		t.Fatalf("loadLocalNode: %v", err)
	}

	if loaded.MeshIP != "" {
		t.Errorf("expected empty MeshIP for old state file, got %q", loaded.MeshIP)
	}
	if loaded.MeshIPv6 != "" {
		t.Errorf("expected empty MeshIPv6 for old state file, got %q", loaded.MeshIPv6)
	}
}
```

Import `"os"` and `"path/filepath"` if they are not already present in the test file's import
block.

### Task 7: Add test for `meshIPInSubnet` in `pkg/daemon/daemon_test.go`

Add the following table-driven test to `pkg/daemon/daemon_test.go`:

```go
func TestMeshIPInSubnet(t *testing.T) {
	t.Parallel()

	_, customNet, _ := net.ParseCIDR("192.168.100.0/24")

	tests := []struct {
		name   string
		meshIP string
		cfg    *Config
		want   bool
	}{
		{
			name:   "legacy subnet match",
			meshIP: "10.42.7.33",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: true,
		},
		{
			name:   "legacy subnet mismatch",
			meshIP: "10.99.1.1",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
		{
			name:   "custom subnet match",
			meshIP: "192.168.100.55",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{},
				CustomSubnet: customNet,
			},
			want: true,
		},
		{
			name:   "custom subnet mismatch",
			meshIP: "10.42.7.33",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{},
				CustomSubnet: customNet,
			},
			want: false,
		},
		{
			name:   "invalid IP",
			meshIP: "not-an-ip",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
		{
			name:   "empty IP",
			meshIP: "",
			cfg: &Config{
				Keys:         &crypto.DerivedKeys{MeshSubnet: [2]byte{42, 0}},
				CustomSubnet: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := meshIPInSubnet(tt.meshIP, tt.cfg)
			if got != tt.want {
				t.Errorf("meshIPInSubnet(%q) = %v, want %v", tt.meshIP, got, tt.want)
			}
		})
	}
}
```

The test file already imports `"net"`. Add `"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"`
to the import block of `pkg/daemon/daemon_test.go` if it is not already present.

## Affected Files

- **Modified:** `pkg/daemon/helpers.go` — extend `localNodeState`, `loadLocalNode`, `saveLocalNode`
- **Modified:** `pkg/daemon/daemon.go` — update `initLocalNode` to use persisted IP; add `meshIPInSubnet` helper
- **Modified:** `pkg/daemon/helpers_test.go` — add `TestLocalNodeStatePersistsMeshIP` and `TestLocalNodeStateBackwardCompatibility`
- **Modified:** `pkg/daemon/daemon_test.go` — add `TestMeshIPInSubnet`

No other files need to change.

## Test Strategy

1. **Unit tests** (Tasks 6 and 7) verify the serialisation round-trip and the subnet validation helper in isolation without requiring WireGuard binaries.
2. **Manual integration test:**
   - `wgmesh join --secret <SECRET_A>` — note the assigned mesh IP.
   - Stop the daemon.
   - `wgmesh join --secret <SECRET_B>` — confirm the mesh IP is unchanged.
   - Inspect `/var/lib/wgmesh/<iface>.json` — confirm `mesh_ip` field is present and matches.
3. **Backward compatibility test:** Delete `mesh_ip` from the state file manually and restart; the daemon should derive the IP from the current secret (same as existing behaviour) and persist it for the next restart.
4. **Subnet change test:** Start with `--mesh-subnet 10.42.0.0/16`, note IP, then restart with `--mesh-subnet 192.168.100.0/24`. The persisted IP (`10.42.x.x`) is outside the new subnet, so a new IP must be re-derived within `192.168.100.0/24`.

Run all tests with:

```bash
go test ./pkg/daemon/... -run 'TestLocalNodeState|TestMeshIPInSubnet' -v
go test -race ./pkg/daemon/...
```

## Estimated Complexity
low
