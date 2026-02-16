# Specification: Issue #113

## Classification
fix

## Deliverables
documentation

## Problem Analysis

The `pkg/wireguard` package is missing godoc comments on all of its exported types and functions across multiple files. This is inconsistent with other packages in the project (`pkg/crypto`, `pkg/daemon`) which have good godoc coverage.

### Missing Godoc Comments

**In `pkg/wireguard/config.go`:**
- `Config` (type, line 10) - Main configuration structure
- `Interface` (type, line 15) - WireGuard interface configuration
- `Peer` (type, line 21) - WireGuard peer configuration
- `ConfigDiff` (type, line 28) - Represents configuration differences
- `GetCurrentConfig` (func, line 35) - Retrieves current WireGuard configuration
- `CalculateDiff` (func, line 87) - Computes configuration differences
- `HasChanges` (method, line 116) - Checks if diff has changes
- `ApplyDiff` (func, line 147) - Applies configuration differences

**In `pkg/wireguard/convert.go`:**
- `FullConfigToConfig` (func, line 3) - Converts FullConfig to Config

**In `pkg/wireguard/apply.go`:**
- `FullConfig` (type, line 12) - Full WireGuard configuration structure
- `WGInterface` (type, line 17) - WireGuard interface with full details
- `WGPeer` (type, line 23) - WireGuard peer with full details
- `ApplyFullConfiguration` (func, line 30) - Applies complete WireGuard configuration
- `SetPeer` (func, line 86) - Configures a single peer
- `RemovePeer` (func, line 124) - Removes a peer from interface
- `GetPeers` (func, line 133) - Lists all peers on an interface

**In `pkg/wireguard/keys.go`:**
- `GenerateKeyPair` (func, line 10) - Generates WireGuard key pair
- `ValidatePrivateKey` (func, line 35) - Validates a WireGuard private key

**In `pkg/wireguard/persist.go`:**
- `GenerateWgQuickConfig` (func, line 10) - Generates wg-quick compatible config
- `ApplyPersistentConfig` (func, line 61) - Applies persistent WireGuard configuration
- `UpdatePersistentConfig` (func, line 84) - Updates persistent configuration
- `RemovePersistentConfig` (func, line 113) - Removes persistent configuration

### Impact

Lack of godoc comments:
1. Makes the package harder to understand for new contributors
2. Reduces discoverability via `go doc` and godoc.org
3. Fails to explain the purpose and usage of types and functions
4. Creates inconsistency across the codebase

## Proposed Approach

Add standard godoc comments to all exported types, functions, and methods in the `pkg/wireguard` package following Go documentation conventions and the style used in other packages like `pkg/crypto` and `pkg/daemon`.

### Documentation Style Guidelines

Based on existing godoc in the codebase:

1. **Type comments**: Start with the type name and describe what it represents
   - Example: `// Config represents a WireGuard interface configuration.`

2. **Function comments**: Start with the function name and describe what it does
   - Example: `// CalculateDiff computes the differences between two WireGuard configurations.`

3. **Method comments**: Start with the method name and describe its behavior
   - Example: `// HasChanges returns true if the diff contains any configuration changes.`

4. **Keep comments concise**: One or two sentences is usually sufficient
5. **Focus on "what" not "how"**: Describe the purpose, not implementation details
6. **Use proper punctuation**: Complete sentences with periods

### Implementation Steps

1. **config.go**: Add godoc comments for:
   - Config type: Describe it holds interface and peer configuration
   - Interface type: Describe WireGuard interface settings
   - Peer type: Describe WireGuard peer settings
   - ConfigDiff type: Describe it represents differences between configurations
   - GetCurrentConfig: Describe retrieval of current config via SSH
   - CalculateDiff: Describe diff calculation between configs
   - HasChanges: Describe change detection
   - ApplyDiff: Describe applying changes via SSH

2. **convert.go**: Add godoc comment for:
   - FullConfigToConfig: Describe conversion from FullConfig to Config format

3. **apply.go**: Add godoc comments for:
   - FullConfig type: Describe complete WireGuard configuration structure
   - WGInterface type: Describe interface configuration with all details
   - WGPeer type: Describe peer configuration with all details
   - ApplyFullConfiguration: Describe applying complete configuration
   - SetPeer: Describe configuring a single peer
   - RemovePeer: Describe removing a peer
   - GetPeers: Describe listing peers

4. **keys.go**: Add godoc comments for:
   - GenerateKeyPair: Describe key pair generation
   - ValidatePrivateKey: Describe private key validation

5. **persist.go**: Add godoc comments for:
   - GenerateWgQuickConfig: Describe wg-quick config generation
   - ApplyPersistentConfig: Describe persistent config application
   - UpdatePersistentConfig: Describe persistent config update
   - RemovePersistentConfig: Describe persistent config removal

## Affected Files

All changes are documentation-only (godoc comments):

1. **`pkg/wireguard/config.go`**:
   - Add comments for 4 types: Config, Interface, Peer, ConfigDiff
   - Add comments for 4 functions/methods: GetCurrentConfig, CalculateDiff, HasChanges, ApplyDiff

2. **`pkg/wireguard/convert.go`**:
   - Add comment for 1 function: FullConfigToConfig

3. **`pkg/wireguard/apply.go`**:
   - Add comments for 3 types: FullConfig, WGInterface, WGPeer
   - Add comments for 4 functions: ApplyFullConfiguration, SetPeer, RemovePeer, GetPeers

4. **`pkg/wireguard/keys.go`**:
   - Add comments for 2 functions: GenerateKeyPair, ValidatePrivateKey

5. **`pkg/wireguard/persist.go`**:
   - Add comments for 4 functions: GenerateWgQuickConfig, ApplyPersistentConfig, UpdatePersistentConfig, RemovePersistentConfig

## Test Strategy

### Documentation Verification

1. **Manual review**: Read all added godoc comments for:
   - Correct grammar and punctuation
   - Accurate description of functionality
   - Consistency with existing godoc style in pkg/crypto and pkg/daemon
   - Proper capitalization and formatting

2. **Go doc command**: Verify comments appear correctly:
   ```bash
   go doc github.com/atvirokodosprendimai/wgmesh/pkg/wireguard
   go doc github.com/atvirokodosprendimai/wgmesh/pkg/wireguard.Config
   go doc github.com/atvirokodosprendimai/wgmesh/pkg/wireguard.CalculateDiff
   ```

3. **Package documentation**: Ensure package-level docs are readable:
   ```bash
   go doc -all github.com/atvirokodosprendimai/wgmesh/pkg/wireguard
   ```

### Code Validation

1. **No code changes**: Verify that ONLY comments are added (no functional changes)
2. **Build verification**: Ensure project still builds:
   ```bash
   go build ./...
   ```
3. **Format check**: Run go fmt to ensure proper formatting:
   ```bash
   go fmt ./pkg/wireguard/...
   ```

### No Functional Testing Required

Since this is documentation-only (comments), no functional tests, linting, or runtime testing is needed. The existing test suite will continue to pass unchanged.

## Estimated Complexity

**low** (30-45 minutes)

- Pure documentation change (no code logic affected)
- Straightforward task of adding standard godoc comments
- Clear examples available in pkg/crypto and pkg/daemon
- No testing required beyond verification of comment formatting
- No risk of breaking existing functionality
