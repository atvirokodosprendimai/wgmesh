# Specification: Issue #212

## Classification
fix

## Deliverables
code

## Problem Analysis

In `pkg/discovery/exchange.go` lines 260-274, GOODBYE messages are processed without validating the `Timestamp` field. This creates a replay attack vector where an attacker who captures a valid GOODBYE envelope can replay it hours or days later to forcibly evict a peer from the mesh.

### Current Vulnerable Code
```go
case crypto.MessageTypeGoodbye:
    var bye goodbyeMessage
    if err := json.Unmarshal(plaintext, &bye); err != nil {
        log.Printf("[Exchange] Invalid GOODBYE payload from %s: %v", remoteAddr.String(), err)
        return
    }
    if bye.WGPubKey == "" || bye.WGPubKey == pe.localNode.WGPubKey {
        return
    }
    pe.peerStore.Remove(bye.WGPubKey)  // <-- No timestamp validation!
    name := bye.WGPubKey
    if len(name) > 8 {
        name = name[:8] + "..."
    }
    log.Printf("[Exchange] Peer %s reported shutdown, removed from active set", name)
```

### Attack Scenario
1. Attacker captures a legitimate GOODBYE message from peer A
2. Peer A rejoins the mesh normally
3. Hours/days later, attacker replays the captured GOODBYE message
4. Victim node removes peer A from its active peer set
5. Mesh connectivity is disrupted for the victim

### Security Context
- **goodbyeMessage structure** (lines 54-58): Contains a `Timestamp` field (int64, Unix seconds) that is populated when sent (line 1078) but never validated when received
- **Other message types** (HELLO, REPLY): Are session-based and don't have this vulnerability
- **Existing validation pattern**: `pkg/crypto/rotation.go` lines 42-48 demonstrates the correct timestamp validation approach for rotation announcements (1-hour window)

### Why GOODBYE is Vulnerable
Unlike HELLO/REPLY which establish sessions with handshakes, GOODBYE is a one-way termination message. Without timestamp validation, any captured GOODBYE can be replayed indefinitely.

## Proposed Approach

Add timestamp validation before processing GOODBYE messages, following the pattern established in `rotation.go:42-48`.

### Implementation Steps

1. **Add timestamp validation in exchange.go**
   - Location: `pkg/discovery/exchange.go` lines 260-274 (GOODBYE case handler)
   - Add validation after unmarshaling but before `peerStore.Remove()`
   - Use 60-second window as specified in the issue
   - Return error for stale messages (don't process them)

2. **Validation logic** (following rotation.go pattern):
   ```go
   // After unmarshaling bye message:
   msgTime := time.Unix(bye.Timestamp, 0)
   if time.Since(msgTime) > 60*time.Second {
       log.Printf("[Exchange] Rejected stale GOODBYE from %s (age: %v)", 
           remoteAddr.String(), time.Since(msgTime))
       return
   }
   // Also reject future timestamps (clock skew protection)
   if msgTime.After(time.Now().Add(60*time.Second)) {
       log.Printf("[Exchange] Rejected GOODBYE with future timestamp from %s", 
           remoteAddr.String())
       return
   }
   ```

3. **No changes to goodbyeMessage structure**
   - The `Timestamp` field already exists and is populated
   - The field is set in `SendGoodbye()` at line 1078
   - Only the validation logic needs to be added

### Alternative Considered: Adjust Window Size
- **60 seconds**: Specified in issue, sufficient for normal network delays
- **Longer window (5+ minutes)**: More tolerant of clock skew but weaker security
- **Shorter window (10 seconds)**: Stronger security but may reject legitimate messages in high-latency networks

**Decision**: Use 60 seconds as specified in the issue. This balances security with practical network conditions.

## Affected Files

### Code Changes
- `pkg/discovery/exchange.go`:
  - Lines 260-274: GOODBYE message handler
    - Add timestamp validation after unmarshaling (2-3 new validation checks)
    - Add log messages for rejected messages

### Test Changes
- `pkg/discovery/exchange_test.go`:
  - Add test case: `TestHandleGoodbye_RejectsStaleMessage`
    - Create GOODBYE message with old timestamp (>60s)
    - Verify peer is NOT removed from peerStore
  - Add test case: `TestHandleGoodbye_AcceptsFreshMessage`
    - Create GOODBYE message with current timestamp
    - Verify peer IS removed from peerStore
  - Add test case: `TestHandleGoodbye_RejectsFutureTimestamp`
    - Create GOODBYE message with future timestamp
    - Verify peer is NOT removed from peerStore

## Test Strategy

### Unit Tests (Primary Verification)

1. **Test: Stale GOODBYE rejection**
   ```go
   func TestHandleGoodbye_RejectsStaleMessage(t *testing.T) {
       // Setup peer exchange with test peer in peerStore
       // Create GOODBYE with timestamp 120 seconds in the past
       // Send message to exchange handler
       // Verify peer still exists in peerStore
       // Verify rejection logged
   }
   ```

2. **Test: Fresh GOODBYE acceptance**
   ```go
   func TestHandleGoodbye_AcceptsFreshMessage(t *testing.T) {
       // Setup peer exchange with test peer in peerStore
       // Create GOODBYE with current timestamp
       // Send message to exchange handler
       // Verify peer removed from peerStore
       // Verify removal logged
   }
   ```

3. **Test: Future timestamp rejection**
   ```go
   func TestHandleGoodbye_RejectsFutureTimestamp(t *testing.T) {
       // Setup peer exchange with test peer in peerStore
       // Create GOODBYE with timestamp 120 seconds in the future
       // Send message to exchange handler
       // Verify peer still exists in peerStore
       // Verify rejection logged
   }
   ```

4. **Test: Boundary conditions**
   - GOODBYE at exactly 60 seconds old (should pass)
   - GOODBYE at 61 seconds old (should reject)
   - GOODBYE at 59 seconds old (should pass)

### Manual Testing

1. **Replay attack simulation**
   ```bash
   # Terminal 1: Start mesh node
   wgmesh join --secret "wgmesh://v1/test-secret"
   
   # Terminal 2: Capture GOODBYE with packet capture
   tcpdump -i any -w goodbye.pcap udp port 51821
   
   # Terminal 1: Shutdown gracefully (sends GOODBYE)
   # ctrl-c
   
   # Wait 90 seconds
   
   # Replay captured GOODBYE
   # Verify it's rejected in logs
   ```

2. **Normal shutdown behavior**
   ```bash
   # Start two nodes
   # Verify they discover each other
   # Shutdown one node gracefully
   # Verify the other node removes it from peers list
   # Verify clean GOODBYE processing in logs
   ```

### Integration Testing

Test with existing discovery integration tests to ensure:
- Normal peer shutdown still works
- LAN discovery not affected
- DHT discovery not affected
- Gossip propagation not affected

## Estimated Complexity
low

**Reasoning:**
- Single function modification (GOODBYE handler in exchange.go)
- ~10-15 lines of new validation code
- 3 new unit tests (~60-90 lines)
- Follows existing validation pattern from rotation.go
- No protocol changes (Timestamp field already exists)
- No changes to message serialization
- No changes to encryption/decryption
- Clear acceptance criteria

**Estimated Time:** 1-2 hours including testing

## Additional Notes

### Timestamp Format
- GOODBYE messages use `Timestamp int64` (Unix seconds)
- Set at creation time: `time.Now().Unix()` (line 1078)
- Validated using: `time.Unix(bye.Timestamp, 0)`

### Comparison with Other Validations

**Rotation announcements** (`pkg/crypto/rotation.go:42-48`):
- Window: 1 hour (past and future)
- More tolerant due to administrative nature
- Used as the validation pattern reference

**GOODBYE messages** (this fix):
- Window: 60 seconds (past and future)
- Stricter due to security-critical nature
- Prevents replay attacks

### Security Impact

**Before fix:**
- Severity: **Medium** (requires packet capture but easy to replay)
- Impact: Denial of service (peer removal)
- Exploitability: High (no crypto required, just replay)

**After fix:**
- Replay window reduced from infinite to 60 seconds
- Attacker must capture AND replay within 60 seconds
- Clock skew tolerance: ±60 seconds (reasonable for NTP-synced systems)

### Backward Compatibility
- No protocol version change required
- `Timestamp` field already exists in all GOODBYE messages
- Older nodes sending valid recent GOODBYE messages work fine
- Only stale replays are rejected (desired behavior)

### Future Enhancements (Out of Scope)
- Add nonce/sequence numbers to prevent replay within the 60s window
- Implement per-peer last-seen-timestamp tracking
- Add rate limiting for GOODBYE messages from same source
