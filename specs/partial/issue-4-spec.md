# Specification: Issue #4

## Classification
feature

## Problem Analysis

The current `--gossip` flag implementation adds in-mesh gossip discovery **on top of** DHT discovery. When a user runs `wgmesh join --secret <SECRET> --gossip`, both discovery mechanisms run simultaneously:

1. **DHT Discovery** (always runs): Uses BitTorrent Mainline DHT to find peers
2. **In-mesh Gossip** (when `--gossip` is enabled): Uses UDP gossip over WireGuard tunnels to propagate peer information

From the code analysis:
- In `pkg/discovery/dht.go` lines 95-117: When `config.Gossip` is true, `MeshGossip` is created alongside DHT
- In `main.go` lines 252-259: The daemon always starts with `RunWithDHTDiscovery()`, then optionally enables gossip
- DHT is always initialized (lines 363-379), gossip is only added when the flag is set

The question is: **Should `--gossip` enable gossip in addition to DHT, or replace DHT entirely?**

### Current Behavior
- `wgmesh join --secret <SECRET>` → DHT only
- `wgmesh join --secret <SECRET> --gossip` → DHT + Gossip

### Alternative Behavior Options
1. **Option A (Current)**: Keep current behavior - gossip supplements DHT
2. **Option B**: Make gossip replace DHT - `--gossip` means gossip-only mode
3. **Option C**: Add separate flags for explicit control

## Proposed Approach

**Recommendation: Keep Option A (current behavior) but clarify documentation**

### Rationale

1. **Layered Discovery Design**: The architecture is explicitly designed with multiple discovery layers (see custom instructions):
   - Layer 0: GitHub Issues-based registry
   - Layer 1: LAN multicast
   - Layer 2: BitTorrent DHT
   - Layer 3: In-mesh gossip

2. **Complementary Mechanisms**: DHT and gossip serve different purposes:
   - **DHT**: Bootstrap initial connections when no peers are known
   - **Gossip**: Propagate peers transitively through established mesh connections
   - Removing DHT when enabling gossip would break cold-start scenarios

3. **User Experience**: Most users want "more discovery" not "different discovery"
   - `--gossip` naturally means "add gossip capability"
   - Replacing DHT would be surprising behavior

4. **No Conflict**: DHT and gossip don't interfere with each other:
   - They use different ports (DHT uses random port, gossip uses derived port)
   - Gossip adds value without negating DHT's benefits

### If Gossip-Only Mode is Needed (Future)

If users want gossip-only mode (e.g., for privacy or network restrictions), add explicit flags:
- `--discovery-methods=gossip` or `--discovery=gossip`
- `--no-dht` to disable DHT explicitly

### Documentation Improvements

Clarify in help text and documentation:
1. Without `--gossip`: Uses DHT discovery only
2. With `--gossip`: Uses DHT **and** in-mesh gossip for enhanced peer propagation
3. Add note that gossip requires at least one DHT-discovered peer to bootstrap

## Affected Files

### Documentation Only (Recommended)
- `main.go` (lines 166-167): Update help text for `--gossip` flag
- `README.md`: Add section explaining discovery layers
- `GOSSIP_TESTING.md`: Clarify that gossip supplements DHT

### If New Flags Added (Optional Future Enhancement)
- `main.go`: Add `--discovery-methods` or `--no-dht` flag
- `pkg/daemon/config.go`: Add configuration options
- `pkg/daemon/daemon.go`: Conditional DHT initialization

## Test Strategy

### For Documentation Updates (Recommended)
1. Review updated help text for clarity
2. Test both modes manually:
   - `wgmesh join --secret <SECRET>` (DHT only)
   - `wgmesh join --secret <SECRET> --gossip` (DHT + gossip)
3. Verify log output correctly shows which discovery methods are active

### For New Flags (If Implemented)
1. Unit tests for config parsing
2. Integration tests for gossip-only mode
3. Verify gossip-only fails gracefully without initial DHT peers
4. Test mixed environments (some nodes with DHT, some without)

## Estimated Complexity

**Documentation-only approach**: **low** (30 minutes)
- Update help text
- Clarify existing documentation
- No code changes required

**Full implementation with new flags**: **medium** (2-4 hours)
- Add new configuration options
- Conditional DHT initialization
- Testing across discovery modes
- Comprehensive documentation

## Recommendation

**Proceed with documentation-only approach**: The current behavior is correct by design. The `--gossip` flag appropriately adds gossip on top of DHT, enabling a more robust discovery system. Simply clarify this in the documentation to prevent user confusion.

If gossip-only mode becomes a requested feature, it should be added as a separate, explicit flag rather than changing the meaning of `--gossip`.
