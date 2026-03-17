# Specification: Issue #460

## Classification
feature

## Deliverables
code

## Problem Analysis

Issue #460 requests a "basic WireGuard mesh networking core" as the minimum viable product, with these acceptance criteria:

1. CLI can create a new mesh network
2. Two nodes can join the same mesh and establish WireGuard tunnels
3. Basic peer discovery mechanism (file-based or simple API)
4. Mesh connectivity verification command
5. Basic configuration management

### What Is Already Implemented

After auditing the codebase, criteria 1–3 and 5 are fully satisfied:

| Criterion | Status | Evidence |
|---|---|---|
| Create mesh network | ✅ Implemented | `wgmesh init --secret` → `daemon.GenerateSecret()` / `daemon.FormatSecretURI()` in `pkg/daemon/config.go:168–190` |
| Two nodes join same mesh | ✅ Implemented | `wgmesh join --secret <URI>` runs a daemon with 5-second reconciliation loop (`pkg/daemon/daemon.go`, `ReconcileInterval = 5s`) |
| Peer discovery | ✅ Implemented | 4-layer stack: LAN multicast (`pkg/discovery/lan.go`), BitTorrent DHT (`pkg/discovery/dht.go`), GitHub Issue registry (`pkg/discovery/registry.go`), in-mesh gossip (`pkg/discovery/gossip.go`) |
| Configuration management | ✅ Implemented | `pkg/daemon/config.go`: `DaemonOpts` → `Config` with HKDF-derived keys, interface name validation, subnet validation |

### The Gap: Criterion 4 — Mesh Connectivity Verification

`wgmesh status --secret <SECRET>` (implemented in `main.go:504–560`) only displays **derived configuration parameters** (mesh subnet, gossip port, rendezvous ID). It does NOT show live WireGuard peer connectivity. The command ends with:

```
(Run 'wg show' to see connected peers)
```

This is a UX failure for the MVP: a user cannot tell whether their mesh is actually working without leaving `wgmesh` and running a raw `wg` command.

The alternative connectivity query, `wgmesh peers list`, uses JSON-RPC over a Unix socket (`pkg/rpc/`) and **requires the daemon to be running**. It cannot be used as a standalone connectivity check.

The local WireGuard query functions already exist in `pkg/wireguard/apply.go`:
- `GetPeers(iface string) ([]WGPeer, error)` — runs `wg show <iface> peers`
- `GetLatestHandshakes(iface string) (map[string]int64, error)` — runs `wg show <iface> latest-handshakes`
- `GetPeerTransfers(iface string) (map[string]PeerTransfer, error)` — runs `wg show <iface> transfer`

Missing: a function to get endpoints for each peer (`wg show <iface> endpoints`).

The `statusCmd()` in `main.go` does not import or use `pkg/wireguard`, so it cannot call these functions.

## Implementation Tasks

### Task 1: Add `GetEndpoints()` to `pkg/wireguard/apply.go`

Add the following function immediately after `GetPeerTransfers()` (after line 259 in `pkg/wireguard/apply.go`):

```go
// GetEndpoints returns the current endpoint for each WireGuard peer.
// Returns a map of public key → "IP:port" string, or "(none)" when no endpoint
// has been negotiated yet. Requires the wg binary and root/CAP_NET_ADMIN.
func GetEndpoints(iface string) (map[string]string, error) {
	cmd := exec.Command(wgPath, "show", iface, "endpoints")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wg show endpoints failed: %w", err)
	}

	result := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <pubkey>\t<IP:port>  or  <pubkey>\t(none)
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}

	return result, nil
}
```

No new imports are required; `exec` and `strings` are already imported in `pkg/wireguard/apply.go`.

### Task 2: Add `formatBytes()` helper to `main.go`

Add this unexported function at the bottom of `main.go` (after the last existing helper function):

```go
// formatBytes converts a byte count into a human-readable string (KiB, MiB, etc.).
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

### Task 3: Enhance `statusCmd()` in `main.go`

#### 3a: Add `wireguard` package import

In `main.go`, in the import block (lines 3–20), add the wireguard package alongside the existing internal imports:

```go
"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
```

The full import block becomes:
```go
import (
    "flag"
    "fmt"
    "log"
    "net"
    "net/http"
    _ "net/http/pprof"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
    "github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
    "github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
    "github.com/atvirokodosprendimai/wgmesh/pkg/rpc"
    "github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"

    // Import discovery to register the DHT factory via init()
    _ "github.com/atvirokodosprendimai/wgmesh/pkg/discovery"
)
```

#### 3b: Replace the peer hint at end of `statusCmd()`

In `main.go`, locate the `statusCmd()` function (starts at line 504). Find and **remove** this line near the end of the function:

```go
fmt.Println("(Run 'wg show' to see connected peers)")
```

Replace it with the following block that queries the local WireGuard interface and displays a peer connectivity table:

```go
// Show live WireGuard peer connectivity.
peers, err := wireguard.GetPeers(cfg.InterfaceName)
if err != nil {
    fmt.Printf("WireGuard interface %q not running.\n", cfg.InterfaceName)
    fmt.Println("Run 'wgmesh join --secret <SECRET>' to start the mesh.")
} else if len(peers) == 0 {
    fmt.Println("Connected Peers: none")
    fmt.Println("(Peers will appear here once discovered and connected.)")
} else {
    handshakes, _ := wireguard.GetLatestHandshakes(cfg.InterfaceName)
    transfers, _ := wireguard.GetPeerTransfers(cfg.InterfaceName)
    endpoints, _ := wireguard.GetEndpoints(cfg.InterfaceName)

    fmt.Printf("Connected Peers (%d):\n", len(peers))
    fmt.Printf("%-20s  %-25s  %-18s  %10s  %10s\n",
        "PEER KEY (prefix)", "ENDPOINT", "LAST HANDSHAKE", "RX", "TX")
    fmt.Println(strings.Repeat("-", 90))
    for _, p := range peers {
        key := p.PublicKey
        // Display first 16 chars of the 44-char base64 pubkey as a recognizable prefix.
        keyShort := key
        if len(key) > 16 {
            keyShort = key[:16] + "..."
        }

        endpoint := endpoints[key]
        if endpoint == "" || endpoint == "(none)" {
            endpoint = "(none)"
        }

        hsAge := "never"
        if ts, ok := handshakes[key]; ok && ts > 0 {
            age := time.Since(time.Unix(ts, 0)).Round(time.Second)
            hsAge = age.String() + " ago"
        }

        rxStr, txStr := "0 B", "0 B"
        if t, ok := transfers[key]; ok {
            rxStr = formatBytes(t.RxBytes)
            txStr = formatBytes(t.TxBytes)
        }

        fmt.Printf("%-20s  %-25s  %-18s  %10s  %10s\n",
            keyShort, endpoint, hsAge, rxStr, txStr)
    }
}
```

### Task 4: Add `GetEndpoints()` unit test

`pkg/wireguard/apply.go` calls `exec.Command(wgPath, ...)` directly — there is no `CommandExecutor` mock in the wireguard package. The appropriate approach is to extract a `parseEndpointsOutput` helper and test that, keeping the external `exec` call in `GetEndpoints()` untested at unit-test level (covered by integration/smoke tests when the `wg` binary is present).

**Step 4a:** In `pkg/wireguard/apply.go`, refactor `GetEndpoints()` to use a private parse helper:

```go
func GetEndpoints(iface string) (map[string]string, error) {
	cmd := exec.Command(wgPath, "show", iface, "endpoints")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wg show endpoints failed: %w", err)
	}
	return parseEndpointsOutput(string(output)), nil
}

// parseEndpointsOutput parses the tab-separated output of `wg show <iface> endpoints`.
// Each line is "pubkey\tIP:port" or "pubkey\t(none)".
func parseEndpointsOutput(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}
	return result
}
```

**Step 4b:** In `pkg/wireguard/config_test.go`, add a table-driven test for `parseEndpointsOutput`. The test file is in package `wireguard` (not `wireguard_test`) so it can access unexported helpers directly.

Add the following imports to the import block if not already present: `"strings"`.

Add the test function after the last existing test:

```go
func TestParseEndpointsOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   map[string]string
	}{
		{
			name:  "two peers",
			input: "ABC123pubkey1==\t1.2.3.4:51820\nDEF456pubkey2==\t5.6.7.8:51820\n",
			want: map[string]string{
				"ABC123pubkey1==": "1.2.3.4:51820",
				"DEF456pubkey2==": "5.6.7.8:51820",
			},
		},
		{
			name:  "peer with no endpoint",
			input: "ABC123pubkey1==\t(none)\n",
			want:  map[string]string{"ABC123pubkey1==": "(none)"},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "malformed line ignored",
			input: "no-tab-here\nABC123pubkey1==\t1.2.3.4:51820\n",
			want:  map[string]string{"ABC123pubkey1==": "1.2.3.4:51820"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEndpointsOutput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
```

## Affected Files

| File | Change |
|---|---|
| `pkg/wireguard/apply.go` | Add `GetEndpoints()` and private `parseEndpointsOutput()` after `GetPeerTransfers()` |
| `pkg/wireguard/config_test.go` | Add `TestParseEndpointsOutput` test; add `"strings"` import if missing |
| `main.go` | Add `wireguard` import; add `formatBytes()` helper; enhance `statusCmd()` to show live peers |

## Test Strategy

1. **Unit test** (`pkg/wireguard/config_test.go`): `TestGetEndpoints_ParsesOutput` validates tab-delimited parsing logic for normal, "(none)", and empty cases without requiring the `wg` binary.
2. **Build check**: `go build ./...` must pass with the new `wireguard` import in `main.go`.
3. **Race detector**: `go test -race ./pkg/wireguard/... ./pkg/daemon/...` must pass.
4. **Manual smoke test** (requires root + WireGuard kernel module):
   - Run `wgmesh init --secret` → copy the URI
   - Run `wgmesh join --secret <URI>` in one terminal (background or separate shell)
   - Run `wgmesh status --secret <URI>` in another terminal
   - Expected: table with 0 or more peers, no "(Run 'wg show'...)" line

## Estimated Complexity
low
