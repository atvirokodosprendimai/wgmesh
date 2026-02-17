# Specification: Issue #76

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, the `-state` flag in centralized mode defaults to `mesh-state.json` in the current working directory. This behavior has several drawbacks:

1. **Inconsistency**: The decentralized mode stores its data in `/var/lib/wgmesh/`, but centralized mode stores data in the current directory
2. **Unpredictable location**: The state file location depends on where the command is run from, making it harder to find and manage
3. **Not following Linux conventions**: Standard practice for application state is to use `/var/lib/<appname>/`
4. **Service management**: When running as a system service, storing state in `/var/lib/wgmesh/` is more appropriate than the current directory

The issue requests changing the default path from the current directory (`mesh-state.json`) to a consistent location (`/var/lib/wgmesh/mesh-state.json`).

## Proposed Approach

Change the default value of the `-state` flag from `mesh-state.json` to `/var/lib/wgmesh/mesh-state.json`. This will:

1. Align centralized mode with decentralized mode's storage location
2. Follow Linux filesystem conventions for application state
3. Make the state file location predictable and consistent
4. Simplify system service deployment

### Implementation Steps

1. **Update the default flag value** in `main.go`:
   - Change line 66 from: `stateFile = flag.String("state", "mesh-state.json", "Path to mesh state file")`
   - To: `stateFile = flag.String("state", "/var/lib/wgmesh/mesh-state.json", "Path to mesh state file")`

2. **Ensure directory exists**:
   - Add code to create `/var/lib/wgmesh/` directory if it doesn't exist before reading/writing state file
   - This should be done in the `mesh.Initialize()` and `mesh.Load()` functions
   - Use appropriate permissions (0755 for directory, 0600 for state file)

3. **Update documentation**:
   - Update README.md to reflect the new default path
   - Update any examples that reference `mesh-state.json` to use the new path or explicitly show the `-state` flag
   - Update help text/usage information if needed (line 160 in main.go)

4. **Update auxiliary files**:
   - Update `.gitignore` if needed (currently has `mesh-state.json`, may need to keep it for backward compatibility)
   - Update example scripts in `example.sh`, `test-route-cleanup.sh`, `test-encryption.sh` to show explicit `-state` usage
   - Update Docker documentation that references the state file location

### Backward Compatibility

To maintain backward compatibility:
- Users can still specify `-state mesh-state.json` explicitly to use the old behavior
- Existing documentation examples that show explicit `-state` flags will continue to work
- The change only affects the default when `-state` is not specified

### Migration Path

Users upgrading will need to either:
1. Move their existing `mesh-state.json` to `/var/lib/wgmesh/mesh-state.json`, or
2. Continue using `-state mesh-state.json` to specify the current directory location

A migration note should be added to release notes when this change is deployed.

## Affected Files

### Code Changes Required

1. **`main.go`** (line 66):
   - Change default value of `-state` flag to `/var/lib/wgmesh/mesh-state.json`
   - Update help text (line 160) to reflect new default

2. **`pkg/mesh/mesh.go`**:
   - Add directory creation in `Initialize()` function (around line 20)
   - Add directory creation check in `Load()` function (around line 37)
   - Add directory creation in `Save()` function (around line 61)

### Documentation Changes Required

1. **`README.md`**:
   - Update references to state file default location (line 11, 160, 384, 413, 515)
   - Update example commands to be explicit about state file location where relevant

2. **`ENCRYPTION.md`**:
   - Update examples that reference `mesh-state.json` to show explicit path

3. **`DOCKER.md`**:
   - Update volume mount examples if relevant

4. **`example.sh`**:
   - Update to show explicit `-state` flag usage

5. **`test-route-cleanup.sh`**:
   - Update to create state file in appropriate location for testing

6. **`test-encryption.sh`**:
   - Update to show explicit `-state` flag usage for clarity

7. **`.gitignore`**:
   - Consider whether to keep `mesh-state.json` for backward compatibility with users still using old location

## Test Strategy

### Manual Testing

1. **Fresh installation test**:
   - Run `wgmesh -init` without `-state` flag
   - Verify state file is created at `/var/lib/wgmesh/mesh-state.json`
   - Verify directory has correct permissions (0755)
   - Verify file has correct permissions (0600)

2. **Operations test**:
   - Run `wgmesh -add node1:10.99.0.1:192.168.1.10` without `-state` flag
   - Verify it uses `/var/lib/wgmesh/mesh-state.json`
   - Run `wgmesh -list` without `-state` flag
   - Verify it reads from the correct location

3. **Backward compatibility test**:
   - Create a state file at `./mesh-state.json`
   - Run `wgmesh -state mesh-state.json -list`
   - Verify it still works with explicit path

4. **Encryption test**:
   - Run `wgmesh -init -encrypt` without `-state` flag
   - Verify encrypted state file is created in new location

5. **Permissions test**:
   - Verify `/var/lib/wgmesh/` directory creation works
   - Verify appropriate error messages if directory cannot be created

### Automated Testing

1. **Unit tests**:
   - Add tests for directory creation in `pkg/mesh/mesh_test.go`
   - Test that `Initialize()`, `Load()`, and `Save()` handle missing directories correctly

2. **Integration tests**:
   - Add test that verifies default state path is used when `-state` is not specified
   - Add test that verifies explicit `-state` flag overrides default

### Docker Testing

1. Verify Docker examples in `DOCKER.md` and `DOCKER-COMPOSE.md` still work correctly
2. Test volume mount behavior with new default path

### Risk Assessment

- **Medium risk**: Changes default behavior, which could surprise existing users
- **Mitigation**: Clear documentation in release notes, backward compatibility maintained with explicit `-state` flag
- **Benefit**: Improved consistency and alignment with Linux conventions
- **Directory creation**: Need to handle permission errors gracefully when `/var/lib/wgmesh/` cannot be created

## Estimated Complexity

**low** (1-2 hours)

- Simple default value change
- Directory creation logic is straightforward
- Main effort is in updating documentation and ensuring consistency across all examples
- Testing is straightforward with clear success/failure criteria
