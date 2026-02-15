# Specification: Issue #24

## Classification
feature

## Deliverables
code

## Problem Analysis

Currently, the GitHub Actions binary build workflow (`.github/workflows/binary-build.yml`) only builds binaries for Linux platforms:
- linux/amd64
- linux/arm64
- linux/armv7

There are no macOS builds available, which prevents users on Mac systems (both Intel and Apple Silicon) from using pre-built binaries. Users on macOS cannot download and run wgmesh without building from source.

The issue is classified as "Critical / Blocking" because:
1. macOS is a major desktop platform with significant developer adoption
2. Apple Silicon (ARM-based) Macs are now the primary Mac platform
3. Users need both Intel (x86_64) and ARM (arm64) builds for different Mac models

## Proposed Approach

Add macOS builds to the existing GitHub Actions `binary-build.yml` workflow by extending the build matrix to include Darwin (macOS) targets:

1. **Add Darwin/AMD64 target** for Intel Macs (x86_64)
2. **Add Darwin/ARM64 target** for Apple Silicon Macs (M1/M2/M3)

### Implementation Steps

1. **Extend the build matrix** in `.github/workflows/binary-build.yml`:
   - Add entry for `goos: darwin`, `goarch: amd64` (Intel Macs)
   - Add entry for `goos: darwin`, `goarch: arm64` (Apple Silicon Macs)
   - Set appropriate output filenames: `wgmesh-darwin-amd64` and `wgmesh-darwin-arm64`

2. **Update release job** to include macOS binaries:
   - Add `wgmesh-darwin-amd64` to the release files
   - Add `wgmesh-darwin-arm64` to the release files

3. **Verify CGO settings**:
   - Confirm `CGO_ENABLED: 0` is appropriate for Darwin builds
   - WireGuard operations use kernel APIs, so static linking should work

### Technical Considerations

- **Cross-compilation**: Go supports cross-compilation to Darwin targets from Linux runners
- **No code signing**: Initial implementation won't include Apple code signing (can be added later if needed)
- **Testing**: Cannot test Darwin binaries on Linux runners, but compilation will verify syntax
- **Naming convention**: Follow existing pattern: `wgmesh-{os}-{arch}`

## Affected Files

### Code Changes Required

1. **`.github/workflows/binary-build.yml`**:
   - Lines 24-34: Add two new matrix entries for Darwin builds
   - Lines 86-90: Add Darwin binaries to release files list

## Test Strategy

### Build Verification
1. Trigger workflow manually or via PR to verify builds complete successfully
2. Check that artifacts are uploaded for all platforms (verify 5 artifacts: 3 Linux + 2 Darwin)
3. Download Darwin artifacts and verify they are Mach-O executables:
   ```bash
   file wgmesh-darwin-amd64  # Should show: Mach-O 64-bit executable x86_64
   file wgmesh-darwin-arm64  # Should show: Mach-O 64-bit executable arm64
   ```

### Release Verification
1. Create a test tag (e.g., `v0.0.0-test`) to trigger release workflow
2. Verify GitHub release includes all 5 binaries
3. Download and inspect release assets

### Manual Testing (Optional, if Mac available)
1. Download `wgmesh-darwin-amd64` or `wgmesh-darwin-arm64` on appropriate Mac
2. Make executable: `chmod +x wgmesh-darwin-*`
3. Run basic commands to verify functionality:
   ```bash
   ./wgmesh-darwin-arm64 --help
   ./wgmesh-darwin-arm64 version
   ```

### Risk Assessment
- **Low risk**: Only extending existing working workflow pattern
- **No code changes**: Only CI/CD configuration changes
- **Reversible**: Can remove Darwin entries if issues arise
- **No impact on Linux builds**: Existing builds continue unchanged

## Estimated Complexity

**low** (15-30 minutes)

- Simple matrix extension in existing workflow
- No code changes to application logic
- Following established pattern for Linux builds
- Straightforward testing via GitHub Actions logs
