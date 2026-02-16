# Specification: Issue #103

## Classification
fix

## Deliverables
code

## Problem Analysis

The centralized mode's WireGuard configuration handling (used for SSH-based mesh deployment) completely ignores PresharedKey (PSK) fields, creating a significant gap in security and configuration management.

### Current State

**The problem exists in `pkg/wireguard/config.go`:**

1. **Missing PSK field in Peer struct** (lines 21-27):
   - The `Peer` struct has no `PresharedKey` field
   - Only tracks: PublicKey, Endpoint, AllowedIPs, PersistentKeepalive

2. **Parser skips PSK column** (line 69):
   - `wg show dump` output format for peers: `public-key preshared-key endpoint allowed-ips ... persistent-keepalive`
   - Parser reads: `parts[0]` (public key), `parts[2]` (endpoint), `parts[3]` (allowed-ips), `parts[4]` (keepalive)
   - **Skips `parts[1]`** (preshared-key)

3. **peersEqual ignores PSK** (lines 120-144):
   - Compares Endpoint, PersistentKeepalive, AllowedIPs
   - No PSK comparison means identical peers with different PSKs are considered equal

4. **addOrUpdatePeer cannot set PSK** (lines 177-196):
   - Builds `wg set` command with endpoint, allowed-ips, persistent-keepalive
   - No PSK parameter passed

### Contrast with Decentralized Mode

The decentralized daemon path (`pkg/wireguard/apply.go:86-120`) **correctly** handles PSK:
- `SetPeer` function accepts `psk [32]byte` parameter
- Checks if PSK is non-zero (lines 93-99)
- Passes PSK via stdin: `args = append(args, "preshared-key", "/dev/stdin")`
- Base64-encodes PSK for WireGuard consumption

### Impact

1. **Lost security configuration**: If a peer was manually configured with PSK, centralized deploy will silently remove it
2. **Configuration drift**: Cannot manage PSK through centralized deployment
3. **Inconsistent behavior**: Decentralized mode supports PSK, centralized mode does not
4. **Silent failures**: No warning when PSK is present in current config but not in desired state

### Why This Matters

PresharedKey provides post-quantum security ("defense in depth") for WireGuard connections. Losing PSK configuration during updates is a security regression.

## Proposed Approach

Add complete PSK support to the centralized config handling pipeline, mirroring the decentralized mode's implementation.

### Step-by-Step Changes

#### 1. Add PresharedKey field to Peer struct
**File**: `pkg/wireguard/config.go`, line 21

```go
type Peer struct {
    PublicKey           string
    PresharedKey        string  // Add this field (base64-encoded)
    Endpoint            string
    AllowedIPs          []string
    PersistentKeepalive int
}
```

#### 2. Parse PSK in GetCurrentConfig
**File**: `pkg/wireguard/config.go`, line 69

Update peer parsing to read `parts[1]`:

```go
publicKey := parts[0]
presharedKey := parts[1]  // Add this line
endpoint := parts[2]
// ... rest of parsing

peer := Peer{
    PublicKey:           publicKey,
    PresharedKey:        presharedKey,  // Add this field
    Endpoint:            endpoint,
    AllowedIPs:          allowedIPs,
    PersistentKeepalive: keepalive,
}
```

Notes:
- `wg show dump` outputs `(none)` when no PSK is set
- Should store `(none)` as-is or empty string for consistency

#### 3. Include PSK in peersEqual comparison
**File**: `pkg/wireguard/config.go`, line 120

Add PSK comparison:

```go
func peersEqual(a, b Peer) bool {
    if a.PresharedKey != b.PresharedKey {  // Add this check
        return false
    }
    
    if a.Endpoint != b.Endpoint {
        return false
    }
    // ... rest of comparisons
}
```

#### 4. Set PSK in addOrUpdatePeer
**File**: `pkg/wireguard/config.go`, line 177

Mirror the decentralized mode's approach:

```go
func addOrUpdatePeer(client *ssh.Client, iface string, pubKey string, peer Peer) error {
    cmd := fmt.Sprintf("wg set %s peer %s", iface, pubKey)
    
    // Handle PSK if present
    var stdinContent string
    if peer.PresharedKey != "" && peer.PresharedKey != "(none)" {
        cmd += " preshared-key /dev/stdin"
        stdinContent = peer.PresharedKey + "\n"
    }
    
    // ... endpoint, allowed-ips, keepalive as before
    
    // Execute command with PSK via stdin if needed
    if stdinContent != "" {
        if err := client.RunWithStdin(cmd, stdinContent); err != nil {
            return fmt.Errorf("failed to configure peer: %w", err)
        }
    } else {
        if _, err := client.Run(cmd); err != nil {
            return fmt.Errorf("failed to configure peer: %w", err)
        }
    }
    
    return nil
}
```

**Note**: Requires verifying that `ssh.Client` has a `RunWithStdin` method or similar. If not, must implement it in `pkg/ssh/client.go`.

#### 5. Update ApplyFullConfiguration (if needed)
**File**: `pkg/wireguard/apply.go`, lines 60-80

Check if `ApplyFullConfiguration` needs PSK support for consistency. Currently it uses `WGPeer` struct which may need a PSK field too.

### Alternative Approach (if SSH stdin is complex)

If implementing stdin via SSH is problematic:
1. Write PSK to temporary file on remote host
2. Use `preshared-key /tmp/wg-psk-<random>`
3. Delete temp file after command

Similar to how `ApplyFullConfiguration` handles private keys (lines 40-44).

## Affected Files

### Code Changes Required

1. **`pkg/wireguard/config.go`**:
   - Line 21: Add `PresharedKey string` to `Peer` struct
   - Line 69: Parse `parts[1]` as preshared-key
   - Line 75: Add `PresharedKey: presharedKey` to peer initialization
   - Line 121: Add PSK comparison in `peersEqual`
   - Lines 177-196: Update `addOrUpdatePeer` to handle PSK via stdin or temp file

2. **`pkg/ssh/client.go`** (if needed):
   - Add `RunWithStdin(cmd, stdin string) (string, error)` method if it doesn't exist
   - Or: Extend existing `Run` method to accept optional stdin

3. **`pkg/wireguard/apply.go`** (potentially):
   - Lines 23-28: Consider adding PSK to `WGPeer` struct for consistency
   - Lines 60-80: Update `ApplyFullConfiguration` peer loop to handle PSK

### Documentation

4. **README.md** or similar:
   - Document that centralized mode now supports PSK
   - Explain how PSK is preserved during updates

## Test Strategy

### Manual Testing

Since there are no existing tests for `pkg/wireguard/config.go`, manual testing is required:

1. **Baseline**: Create a test mesh with PSK manually set on one peer
   ```bash
   wg set wg0 peer <pubkey> preshared-key <(echo <base64-psk>)
   ```

2. **Deploy without changes**: Run centralized deploy, verify PSK is preserved
   ```bash
   wg show wg0 dump | grep <pubkey>  # Check PSK column
   ```

3. **Deploy with peer update**: Change endpoint/allowed-ips, verify PSK remains
   
4. **Remove PSK scenario**: Set desired config without PSK, verify it's removed

5. **Add PSK scenario**: Add PSK to desired config, verify it's applied

### Edge Cases to Test

1. **PSK = "(none)"**: Ensure parser handles this correctly
2. **Empty vs "(none)"**: Decide on canonical representation
3. **Invalid PSK format**: Ensure errors are caught
4. **Large mesh**: Verify no performance impact from PSK handling

### Verification Commands

After each test:
```bash
wg show wg0 dump                    # Raw output
wg show wg0 preshared-keys          # Readable PSK list
wg show wg0 peer <pubkey> preshared-key  # Specific peer
```

### Unit Tests (Optional Enhancement)

While not required for the fix, could add:
- `TestParsePresharedKey`: Verify parsing of `(none)` vs base64 values
- `TestPeersEqualWithPSK`: Verify PSK is compared correctly
- `TestAddPeerWithPSK`: Mock test for PSK command generation

## Estimated Complexity

**medium** (2-4 hours)

**Reasoning**:
- Code changes are straightforward (add field, parse column, compare, set)
- Complexity comes from:
  - SSH stdin handling (may need new method in ssh.Client)
  - Testing requires actual WireGuard interface
  - Need to verify interaction with existing deployment flows
- Medium risk: Changes touch config diffing logic used in production deployments
- Requires careful manual testing to avoid breaking existing configs

**Effort breakdown**:
- Code changes: 1 hour
- SSH stdin implementation: 1 hour (if needed)
- Manual testing: 1-2 hours
- Documentation: 30 minutes
