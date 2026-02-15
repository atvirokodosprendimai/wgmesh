# Specification: Issue #43

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, `wgmesh version` prints the version information (added in PR #42), but the conventional `--version` and `-v` flags do not work. This creates an inconsistent user experience, as most CLI tools support both forms:

```bash
wgmesh version      # ✅ works (prints: wgmesh <version>)
wgmesh --version    # ❌ does NOT work
wgmesh -v           # ❌ does NOT work
```

Users expect command-line tools to follow standard conventions where `--version` (and its short form `-v`) display version information and exit immediately, taking priority over other flags and subcommands.

### Current Implementation

From `main.go` lines 19-28:
- Version is stored in `var version = "dev"` (set at build time via ldflags)
- The `version` subcommand is handled in the initial switch statement (lines 24-28)
- Format: `fmt.Println("wgmesh " + version)`
- Standard flags are parsed later (line 67) using the `flag` package

### The Problem

The current implementation checks for subcommands before parsing flags:
1. Lines 23-54: Subcommand switch statement checks `os.Args[1]`
2. Lines 57-67: Flag definitions and `flag.Parse()` happen after subcommand routing
3. This means `--version` is never checked if it's not a subcommand

## Proposed Approach

Add special handling for `--version` and `-v` flags **before** the subcommand switch, ensuring they take priority over all other operations.

### Implementation Strategy

1. **Check for version flags early** (before line 23):
   - Check if `--version` or `-v` appears in `os.Args`
   - If found, print version and exit immediately
   - This ensures version flags work even with other arguments

2. **Preserve existing behavior**:
   - Keep `wgmesh version` subcommand working (no changes to lines 26-28)
   - Maintain current flag parsing for centralized mode
   - Don't affect any other subcommand behavior

3. **Handle edge cases**:
   - `wgmesh --version --other-flags` → print version, ignore other flags
   - `wgmesh -v subcommand` → print version, ignore subcommand
   - `wgmesh version` → continue to work as before

### Pseudocode

```go
func main() {
    // Check for version flags first (before subcommand check)
    for _, arg := range os.Args[1:] {
        if arg == "--version" || arg == "-v" {
            fmt.Println("wgmesh " + version)
            return
        }
    }
    
    // Existing subcommand handling (lines 23-54)
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "version":
            // existing implementation
        // ... other cases
        }
    }
    
    // Existing flag parsing (lines 57-67)
    // ...
}
```

### Why Not Use the flag Package?

Using `flag.Bool("version", ...)` would require calling `flag.Parse()` before the subcommand switch, which would interfere with:
1. Subcommands that use their own `flag.NewFlagSet()`
2. The existing centralized mode flag definitions
3. Help text generation and error handling

A simple loop check is cleaner and doesn't affect existing behavior.

## Affected Files

### Code Changes

1. **`main.go`** (lines 22-23):
   - Add version flag check loop before the subcommand switch
   - Approximately 6-8 new lines of code
   - No changes to existing subcommand or flag handling

### Documentation Changes

2. **`main.go`** (printUsage function, around line 148):
   - Add `--version` and `-v` to the help text
   - Include in the "FLAGS" or "OPTIONS" section

3. **`README.md`** (if there's a CLI reference section):
   - Document that both `wgmesh version` and `wgmesh --version` work
   - Optional: Add to usage examples

## Test Strategy

### Manual Testing

1. **Basic functionality**:
   ```bash
   wgmesh --version    # Should print: wgmesh <version>
   wgmesh -v           # Should print: wgmesh <version>
   wgmesh version      # Should still work (backward compatibility)
   ```

2. **Priority verification**:
   ```bash
   wgmesh --version --help    # Should print version, not help
   wgmesh -v join             # Should print version, not start join
   wgmesh --version -init     # Should print version, not init
   ```

3. **Build-time version**:
   ```bash
   go build -ldflags "-X main.version=1.2.3"
   ./wgmesh --version         # Should print: wgmesh 1.2.3
   ```

### Automated Testing

Since there are no existing tests for `main.go` CLI behavior (no `main_test.go` file found in the repository), automated tests are **optional** but recommended for future robustness:

1. **If adding tests** (create `main_test.go`):
   ```go
   func TestVersionFlag(t *testing.T) {
       // Test --version flag
       // Test -v flag
       // Test version subcommand
       // Test version priority over other flags
   }
   ```

2. **Integration test approach**:
   - Build the binary
   - Execute with different flag combinations
   - Verify output matches expected format
   - Verify exit code is 0

3. **Test coverage targets**:
   - All three forms work (`version`, `--version`, `-v`)
   - Version flag takes priority
   - Output format is consistent: "wgmesh <version>"

### Regression Testing

Verify that existing functionality is **not** affected:
- All subcommands still work (join, init, status, etc.)
- Centralized mode flags still work (-state, -add, -list, etc.)
- Help text still displays correctly
- Other flag combinations continue to function

## Estimated Complexity

**low** (30-45 minutes)

### Rationale

- **Simple code change**: 6-8 lines added to main.go
- **No architectural changes**: Just an early check before existing logic
- **No external dependencies**: Uses only standard library
- **Clear implementation**: Straightforward loop over os.Args
- **Low risk**: Doesn't modify existing code paths
- **Easy to test**: Manual testing is quick and comprehensive
- **Easy to verify**: Build and run a few commands to confirm

### Time Breakdown

- Code implementation: 10 minutes
- Manual testing: 10 minutes
- Documentation updates: 10 minutes
- Verification and cleanup: 10 minutes
- Total: ~40 minutes
