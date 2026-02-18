# Specification: Issue #178

## Classification
fix

## Deliverables
code

## Problem Analysis

The `wgmesh peers list` command displays peer information in a tabular format with truncated public keys. Specifically, in `main.go` lines 872-874, public keys longer than 40 characters are truncated to 37 characters with "..." appended:

```go
if len(pubkey) > 40 {
    pubkey = pubkey[:37] + "..."
}
```

WireGuard public keys are base64-encoded 32-byte values, resulting in 44-character strings (including the trailing '=' padding). By truncating these keys to 40 characters, users cannot copy the full public key from the `peers list` output to use with `wgmesh peers get <pubkey>`.

### Current Behavior
```
$ wgmesh peers list
PUBLIC KEY                               MESH IP         ENDPOINT                  LAST SEEN  DISCOVERED VIA
VXD7qLbBvKgPTpBhUEcYHJ4nGBNKdLlvvNZYs...     10.99.1.5      192.168.1.100:51820       2m         lan,dht
```

The truncated key "VXD7qLbBvKgPTpBhUEcYHJ4nGBNKdLlvvNZYs..." cannot be used with:
```
$ wgmesh peers get VXD7qLbBvKgPTpBhUEcYHJ4nGBNKdLlvvNZYs...
RPC error: peer not found
```

### Root Cause
- File: `main.go`
- Function: `handlePeersList()` (lines 835-899)
- Lines 872-874 implement the truncation logic
- Column width is set to 40 characters (line 859)
- WireGuard public keys are 44 characters long

## Proposed Approach

### Option 1: Display Full Public Keys (Recommended)
Modify the `handlePeersList()` function to display full public keys without truncation. Adjust the column width and table formatting to accommodate 44-character keys.

**Changes:**
1. Remove the truncation logic (lines 872-874)
2. Increase the public key column width from 40 to at least 44 characters
3. Adjust the printf format string (line 859) to use the new width
4. Update the separator line (line 860) to match the new total width

**Pros:**
- Users can copy-paste keys directly for use with `peers get`
- No information loss
- Simple implementation

**Cons:**
- Slightly wider output (4 more characters)
- May require horizontal scrolling on narrow terminals

### Option 2: Add a --full or --no-truncate Flag
Keep truncated output by default but add a command-line flag to show full keys.

**Pros:**
- Backward compatible with existing terminal layouts
- Users can opt-in for full keys when needed

**Cons:**
- More complex implementation
- Requires users to know about the flag
- Adds command-line argument parsing complexity

### Option 3: Show Full Key Only in peers get Output
Keep truncation in list view, but ensure error messages from `peers get` suggest using full keys and potentially list available keys.

**Pros:**
- Compact list view
- Better error messages

**Cons:**
- Doesn't solve the core problem of not being able to copy keys
- Requires users to find keys elsewhere (e.g., via `wg show`)

### Recommendation
**Option 1** is recommended because:
1. WireGuard public keys are essential identifiers that should be displayed in full
2. Modern terminals typically support 120+ character widths
3. The 4-character increase is minimal
4. Eliminates user confusion and workflow friction

## Affected Files

### Code Changes
- `main.go`:
  - Function `handlePeersList()` (lines 835-899)
    - Remove truncation logic (lines 872-874)
    - Update column width constant (line 859): change 40 to 44
    - Update printf format string (line 859): `%-40s` → `%-44s`
    - Update separator line width (line 860): adjust from 120 to 124 total

### Documentation Changes
None required (behavior becomes more intuitive)

## Test Strategy

### Manual Testing
1. **Test with running daemon:**
   ```bash
   # Start daemon with test secret
   wgmesh join --secret "wgmesh://v1/test-secret-here"
   
   # In another terminal, list peers
   wgmesh peers list
   
   # Verify full public keys are shown (44 characters)
   # Copy a public key from the output
   
   # Use the copied key with peers get
   wgmesh peers get <full-pubkey-from-list>
   
   # Verify it returns peer details successfully
   ```

2. **Test table formatting:**
   ```bash
   # Verify columns align properly
   # Verify separator line matches table width
   # Test with 0, 1, 3, and 10+ peers
   ```

3. **Test terminal width compatibility:**
   ```bash
   # Test on 80-column terminal (should wrap but remain readable)
   # Test on 120+ column terminal (should display cleanly)
   ```

### Automated Testing
Consider adding integration test in `main_test.go` or RPC integration tests:
- Mock RPC server with test peers
- Capture `handlePeersList()` output
- Verify no truncation occurs
- Verify keys are exactly 44 characters (or actual key length)

### Regression Testing
- Ensure `peers count` command still works
- Ensure `peers get` command works with full keys
- Ensure table formatting doesn't break with 0 peers
- Ensure other fields (mesh IP, endpoint, etc.) display correctly

## Estimated Complexity
low

**Reasoning:**
- Single function modification
- ~5 lines of code changes (remove 3 lines, update 2 format strings)
- No new dependencies
- No changes to RPC protocol or data structures
- Clear test path
- Low risk of side effects

**Estimated Time:** 30-60 minutes including testing

## Additional Notes

### WireGuard Key Format
- WireGuard public keys are base64-encoded 32-byte (256-bit) values
- Base64 encoding of 32 bytes = 44 characters total (43 data characters + 1 '=' padding character)
- Example: `VXD7qLbBvKgPTpBhUEcYHJ4nGBNKdLlvvNZYs6q1WXo=`

### Current Column Widths (from line 859)
```
%-40s  %-15s  %-25s  %-10s  %s
 ^       ^       ^       ^     ^
 |       |       |       |     +-- Discovered Via (variable)
 |       |       |       +-------- Last Seen (10 chars)
 |       |       +---------------- Endpoint (25 chars)
 |       +------------------------ Mesh IP (15 chars)
 +-------------------------------- Public Key (40 chars, TRUNCATED)
```

### Proposed Column Widths
```
%-44s  %-15s  %-25s  %-10s  %s
 ^       ^       ^       ^     ^
 +-- Change 40 to 44 for full WireGuard keys
```

Total width change: 120 → 124 characters

### Alternative Solutions Not Recommended
1. **Use shorter identifiers:** Would require changing RPC protocol and peer storage
2. **Show only first/last N characters:** Still not copyable for `peers get`
3. **Hash-based short IDs:** Adds complexity, not standard in WireGuard ecosystem
