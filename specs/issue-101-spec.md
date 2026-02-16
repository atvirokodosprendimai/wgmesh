# Specification: Issue #101

## Classification
refactor

## Deliverables
code

## Problem Analysis

The `LocalNode` type is defined identically in two packages:
- `pkg/daemon/daemon.go:44-50` 
- `pkg/discovery/dht.go:59-65`

This duplication creates maintenance burden:

1. **Manual field copying**: `pkg/discovery/init.go:16-22` manually copies all fields from `daemon.LocalNode` to `discovery.LocalNode`
2. **Synchronization risk**: Adding a new field (like `Hostname` from issue #87) requires updating:
   - Both struct definitions (2 locations)
   - The manual copy code (1 location)
   - Any tests using either struct
3. **Easy to get out of sync**: No compile-time guarantee that the structs remain identical
4. **Code smell**: Violates DRY (Don't Repeat Yourself) principle

### Current Architecture

The duplication exists due to a **factory pattern used to avoid import cycles**:

```
pkg/daemon/daemon.go
  ├─ Defines LocalNode, DiscoveryLayer interface, DHTDiscoveryFactory
  ├─ Does NOT import pkg/discovery
  └─ Calls dhtDiscoveryFactory(config, localNode, peerStore)

pkg/discovery/init.go
  ├─ Imports pkg/daemon
  ├─ Registers createDHTDiscovery via daemon.SetDHTDiscoveryFactory()
  └─ Converts daemon.LocalNode → discovery.LocalNode (manual copy)

pkg/discovery/*.go (dht.go, lan.go, gossip.go)
  ├─ Import pkg/daemon for Config, PeerStore, PeerInfo
  ├─ Define their own LocalNode type
  └─ Use discovery.LocalNode internally
```

**Why the factory exists**: To prevent an import cycle where:
- `daemon` would import `discovery` to create discovery layers
- `discovery` already imports `daemon` for `Config`, `PeerStore`, `PeerInfo`

### Evidence of Usage

All three main discovery implementations use `discovery.LocalNode`:
- **DHT Discovery** (`dht.go:68`): `func NewDHTDiscovery(config *daemon.Config, localNode *LocalNode, ...)`
- **LAN Discovery** (`lan.go:39`): `func NewLANDiscovery(config *daemon.Config, localNode *LocalNode, ...)`
- **Mesh Gossip** (`gossip.go:39,51`): `func NewMeshGossip(config *daemon.Config, localNode *LocalNode, ...)`

The manual copy in `init.go` is the **only place** that converts between the two types.

## Proposed Approach

**Recommendation: Option B - Move LocalNode to a shared package**

Create a new `pkg/node` package that both `daemon` and `discovery` can import without creating cycles.

### Rationale for Option B

**Why not Option A (discovery uses daemon.LocalNode)?**
- Would create an import cycle: `discovery` already imports `daemon` for `Config`, `PeerStore`, `PeerInfo`
- Daemon importing discovery would close the cycle
- Factory pattern was specifically designed to avoid this

**Why not Option C (interface approach)?**
- Over-engineered for a simple data structure
- `LocalNode` is a pure data type with no behavior
- Interface would provide no benefit over direct type usage
- Still requires defining concrete types somewhere

**Why Option B is best:**
- ✅ No import cycles: `pkg/node` has no dependencies on other packages
- ✅ Single source of truth: One struct definition
- ✅ No manual copying: Both packages use the same type
- ✅ Clear ownership: Node-related types belong in `pkg/node`
- ✅ Extensible: Can add other shared node types (e.g., `RemoteNode`, `NodeMetadata`)
- ✅ Consistent with Go conventions: Shared types in separate package

### Implementation Steps

1. **Create `pkg/node/node.go`**:
   ```go
   package node

   // LocalNode represents our local WireGuard node
   type LocalNode struct {
       WGPubKey         string
       WGPrivateKey     string
       MeshIP           string
       WGEndpoint       string
       RoutableNetworks []string
   }
   ```

2. **Update `pkg/daemon/daemon.go`**:
   - Remove `LocalNode` struct definition (lines 43-50)
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Change all references from `LocalNode` to `node.LocalNode`
   - Update `DHTDiscoveryFactory` signature to use `*node.LocalNode`

3. **Update `pkg/discovery/dht.go`**:
   - Remove `LocalNode` struct definition (lines 58-65)
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Change all references from `LocalNode` to `node.LocalNode`
   - Update `NewDHTDiscovery` signature

4. **Update `pkg/discovery/lan.go`**:
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Change all references from `LocalNode` to `node.LocalNode`
   - Update `NewLANDiscovery` signature

5. **Update `pkg/discovery/gossip.go`**:
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Change all references from `LocalNode` to `node.LocalNode`
   - Update `NewMeshGossip` and `NewMeshGossipWithExchange` signatures

6. **Update `pkg/discovery/init.go`**:
   - Remove the entire manual copy in `createDHTDiscovery` (lines 16-22)
   - Directly pass `localNode` to `NewDHTDiscovery`:
     ```go
     func createDHTDiscovery(config *daemon.Config, localNode *node.LocalNode, peerStore *daemon.PeerStore) (daemon.DiscoveryLayer, error) {
         return NewDHTDiscovery(config, localNode, peerStore)
     }
     ```

7. **Update tests**:
   - `pkg/discovery/gossip_test.go`: Change `&LocalNode{...}` to `&node.LocalNode{...}`
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`

### Verification

After changes:
- No import cycles: `pkg/node` → (none), `pkg/daemon` → `pkg/node`, `pkg/discovery` → `pkg/daemon` + `pkg/node`
- Single definition: Only `pkg/node/node.go` defines `LocalNode`
- Zero manual copying: All packages use the same type

## Affected Files

### New Files
- **`pkg/node/node.go`**: New package containing `LocalNode` definition

### Modified Files

1. **`pkg/daemon/daemon.go`**:
   - Remove: Lines 43-50 (`LocalNode` struct definition)
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Update: Line 27 (`localNode *LocalNode` → `localNode *node.LocalNode`)
   - Update: Line 414 (`DHTDiscoveryFactory` signature)
   - Update: All `LocalNode` references throughout file

2. **`pkg/discovery/dht.go`**:
   - Remove: Lines 58-65 (`LocalNode` struct definition)
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Update: Line 54 (`localNode *LocalNode` → `localNode *node.LocalNode`)
   - Update: Line 68 (`NewDHTDiscovery` signature)
   - Update: All `LocalNode` references throughout file

3. **`pkg/discovery/lan.go`**:
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Update: Line 26 (`localNode *LocalNode` → `localNode *node.LocalNode`)
   - Update: Line 39 (`NewLANDiscovery` signature)
   - Update: All `LocalNode` references throughout file

4. **`pkg/discovery/gossip.go`**:
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Update: Line 25 (`localNode *LocalNode` → `localNode *node.LocalNode`)
   - Update: Lines 39, 51 (constructor signatures)
   - Update: All `LocalNode` references throughout file

5. **`pkg/discovery/exchange.go`**:
   - Review for any `LocalNode` usage (likely none, but verify)

6. **`pkg/discovery/registry.go`**:
   - Review for any `LocalNode` usage (likely none, but verify)

7. **`pkg/discovery/init.go`**:
   - Remove: Lines 16-22 (manual struct copy)
   - Update: Line 14 signature to accept `*node.LocalNode`
   - Simplify: Line 24 to pass `localNode` directly

8. **`pkg/discovery/gossip_test.go`**:
   - Add import: `"github.com/atvirokodosprendimai/wgmesh/pkg/node"`
   - Update: All `&LocalNode{...}` instantiations to `&node.LocalNode{...}`

## Test Strategy

### Build Verification
1. **No import cycles**: `go build ./...` must succeed
2. **No compilation errors**: All references to `LocalNode` resolve correctly

### Unit Tests
1. **Existing tests pass**: `go test ./pkg/daemon ./pkg/discovery`
2. **Gossip tests**: Verify `pkg/discovery/gossip_test.go` works with new import

### Manual Verification
1. **Type compatibility**: Verify that `node.LocalNode` can be passed between packages
2. **Factory pattern**: Ensure the factory still works without manual copying
3. **Field access**: Spot-check that field access patterns remain unchanged

### Integration Tests
1. **Daemon startup**: Test that daemon can create discovery layers using shared type
2. **Field addition test**: After refactor, add a dummy field to verify single-location change works

### Regression Tests
Run full test suite to ensure no behavioral changes:
```bash
go test ./... -race
```

### Future-Proofing Test
To verify the refactor solves the original problem:
1. Add a new field to `pkg/node/node.go` (e.g., `TestField string`)
2. Verify it's immediately available in both `daemon` and `discovery` packages
3. Confirm no manual copying is needed

## Estimated Complexity

**medium** (3-4 hours)

### Breakdown
- **Low complexity**: The change is conceptually simple (move struct to new package)
- **Medium complexity**: Requires careful updates across 8 files
- **Medium risk**: Import path changes affect multiple files
- **Time factors**:
  - Creating new package: 15 minutes
  - Updating imports and references: 90 minutes (5 discovery files + daemon)
  - Test updates: 30 minutes
  - Build/test verification: 45 minutes
  - Documentation review: 30 minutes

### Risk Mitigation
- Compile-time verification: Wrong import paths will cause immediate build failures
- Test coverage: Existing tests will catch behavioral regressions
- Small scope: Change is mechanical and focused on type definitions only
- Reversible: If issues arise, can easily revert (no data migration needed)
