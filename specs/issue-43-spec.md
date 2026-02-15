# Specification: Issue #43

## Classification
feature

## Deliverables
code + tests + documentation

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

### Test Files (REQUIRED)

2. **`main_test.go`** (NEW FILE - REQUIRED):
   - Create integration tests for all three version forms
   - Test version flag priority behavior
   - Verify output format and exit codes
   - Approximately 80-100 lines of test code
   - **This is a required deliverable per Issue #43 acceptance criteria**

### Documentation Changes

3. **`main.go`** (printUsage function, around line 148):
   - Add `--version` and `-v` to the help text
   - Include in the "FLAGS" or "OPTIONS" section

4. **`README.md`** (if there's a CLI reference section):
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

### Automated Testing (REQUIRED)

**This is a required deliverable.** Issue #43 acceptance criteria explicitly requires: "Tests verify all three forms". The repository has existing Go test infrastructure (`pkg/*_test.go`), so we must add automated tests for this feature.

#### Test Implementation Approach

Create `main_test.go` with integration tests that build and execute the binary to verify CLI behavior:

```go
package main

import (
    "os"
    "os/exec"
    "strings"
    "testing"
)

// TestVersionCommands verifies all three version invocation methods work
func TestVersionCommands(t *testing.T) {
    // Build the binary for testing
    buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
    if err := buildCmd.Run(); err != nil {
        t.Fatalf("Failed to build test binary: %v", err)
    }
    defer os.Remove("/tmp/wgmesh-test")

    tests := []struct {
        name string
        args []string
    }{
        {"version subcommand", []string{"version"}},
        {"--version flag", []string{"--version"}},
        {"-v flag", []string{"-v"}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
            output, err := cmd.CombinedOutput()
            if err != nil {
                t.Fatalf("Command failed: %v, output: %s", err, output)
            }
            
            result := strings.TrimSpace(string(output))
            if !strings.HasPrefix(result, "wgmesh ") {
                t.Errorf("Expected output to start with 'wgmesh ', got: %s", result)
            }
        })
    }
}

// TestVersionFlagPriority verifies --version takes priority over other flags
func TestVersionFlagPriority(t *testing.T) {
    // Build the binary for testing
    buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
    if err := buildCmd.Run(); err != nil {
        t.Fatalf("Failed to build test binary: %v", err)
    }
    defer os.Remove("/tmp/wgmesh-test")

    tests := []struct {
        name string
        args []string
    }{
        {"version with other flags", []string{"--version", "--help"}},
        {"version with subcommand", []string{"-v", "join"}},
        {"version with init flag", []string{"--version", "-init"}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
            output, err := cmd.CombinedOutput()
            if err != nil {
                t.Fatalf("Command failed: %v, output: %s", err, output)
            }
            
            result := strings.TrimSpace(string(output))
            if !strings.HasPrefix(result, "wgmesh ") {
                t.Errorf("Expected version output, got: %s", result)
            }
            // Ensure it doesn't show help or try to run other commands
            if strings.Contains(result, "SUBCOMMANDS") || strings.Contains(result, "FLAGS") {
                t.Errorf("Version flag should not show help, got: %s", result)
            }
        })
    }
}
```

#### Test Coverage Requirements (REQUIRED)

These tests MUST be implemented to satisfy Issue #43 acceptance criteria:

1. **All three forms work** (`version`, `--version`, `-v`):
   - Each prints `wgmesh <version>`
   - Each exits with code 0
   - Output format is identical across all forms

2. **Version flag priority**:
   - `--version` with other flags → prints version only
   - `-v` with subcommands → prints version only
   - Version flags override all other arguments

3. **Exit behavior**:
   - All forms exit immediately after printing
   - No other operations are performed
   - No error messages or warnings

### Regression Testing

Verify that existing functionality is **not** affected:
- All subcommands still work (join, init, status, etc.)
- Centralized mode flags still work (-state, -add, -list, etc.)
- Help text still displays correctly
- Other flag combinations continue to function

## Estimated Complexity

**low-medium** (1-1.5 hours)

### Rationale

- **Simple code change**: 6-8 lines added to main.go
- **Integration tests required**: 80-100 lines of test code in new main_test.go
- **No architectural changes**: Just an early check before existing logic
- **No external dependencies**: Uses only standard library
- **Clear implementation**: Straightforward loop over os.Args
- **Low risk**: Doesn't modify existing code paths
- **Testing overhead**: Integration tests require building binary and validating output

### Time Breakdown

- Code implementation: 10 minutes
- Test implementation: 30-40 minutes (REQUIRED)
- Manual testing: 10 minutes
- Documentation updates: 10 minutes
- Verification and cleanup: 10 minutes
- Total: ~70-80 minutes
