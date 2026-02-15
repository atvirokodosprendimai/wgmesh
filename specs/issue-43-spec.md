# Specification: Issue #43

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, the wgmesh application supports displaying the version only through a subcommand:
```bash
wgmesh version    # works
```

However, the conventional `--version` flag and its shorthand `-v` are not supported:
```bash
wgmesh --version  # does NOT work (unrecognized flag)
wgmesh -v         # does NOT work (unrecognized flag)
```

This violates user expectations, as most CLI tools support both forms. The standard convention is to support `--version` (and often `-v`) as global flags that work from any context.

From code analysis of `main.go`:
- Lines 23-54: Subcommands are checked first before flag parsing
- Line 26-28: The `version` subcommand is handled, printing `"wgmesh " + version` and returning
- Line 67: `flag.Parse()` is called only after subcommand checks
- The standard Go `flag` package doesn't automatically handle `--version` or `-v`

The issue affects user experience because:
1. Users expect `--version` to work (standard CLI convention)
2. Scripts and automation often rely on `--version` for version checks
3. The shorthand `-v` is a common expectation
4. Current behavior leads to "flag provided but not defined" errors

## Proposed Approach

Add support for `--version` and `-v` flags by checking for them **before** subcommand processing and standard flag parsing. This ensures they work in all contexts and take priority.

### Implementation Strategy

1. **Early flag detection** (before subcommand check):
   - Check `os.Args` for `--version` or `-v` before line 24 (subcommand switch)
   - If found, print version and exit immediately
   - This ensures `--version` works regardless of other arguments

2. **Maintain existing behavior**:
   - The `version` subcommand continues to work (line 26-28)
   - All other subcommands and flags remain unchanged
   - No breaking changes to existing functionality

3. **Output format consistency**:
   - Use the same format as the `version` subcommand: `"wgmesh " + version`
   - Exit with code 0 (success)

### Implementation Steps

1. Add version flag check at the very beginning of `main()`, before subcommand processing:
   ```go
   // Check for version flags first (before subcommands)
   for _, arg := range os.Args[1:] {
       if arg == "--version" || arg == "-v" {
           fmt.Println("wgmesh " + version)
           return
       }
   }
   ```

2. Keep the existing `version` subcommand unchanged (backward compatibility)

3. Update help text in `printUsage()` to document both forms

### Why This Approach

- **Priority**: Checking before subcommands ensures `--version` always works
- **Simplicity**: No changes to flag parsing logic or subcommand handling
- **Compatibility**: Existing `version` subcommand continues to work
- **Convention**: Matches standard CLI tool behavior (e.g., `git --version`, `docker --version`)

## Affected Files

### Code Changes Required

1. **`main.go`**:
   - Lines 22-24: Add version flag check before subcommand switch
   - Lines 147-179 (printUsage): Add documentation for `--version` and `-v` flags

### Documentation Changes

1. **`main.go`** (printUsage function):
   - Add to the top of usage output:
     ```
     VERSION:
       wgmesh version           Show version information
       wgmesh --version         Show version information
       wgmesh -v                Show version information (shorthand)
     ```

2. **`README.md`** (if version documentation exists):
   - Add note that `--version` and `-v` are supported

## Test Strategy

### Manual Testing

1. **Test version flag variants**:
   ```bash
   wgmesh version        # Should print: wgmesh <version>
   wgmesh --version      # Should print: wgmesh <version>
   wgmesh -v             # Should print: wgmesh <version>
   ```

2. **Test version flag priority**:
   ```bash
   wgmesh --version --other-flags    # Should print version and ignore other flags
   wgmesh join --version             # Should print version, not try to run join
   ```

3. **Test existing functionality unchanged**:
   ```bash
   wgmesh join --help         # Should show join help (not affected)
   wgmesh --help              # Should show main help (not affected)
   wgmesh init --secret       # Should work as before
   ```

### Automated Testing (if test infrastructure is added)

Since there is no `main_test.go`, and the custom instructions say "If there is not existing test infrastructure, you can skip adding tests as part of your instructions to make minimal modifications", testing should focus on manual verification.

However, if tests are desired:
1. Create `main_test.go` with table-driven tests
2. Test version output for all three forms
3. Test that version flag takes priority over other arguments
4. Mock `os.Args` and capture stdout

### Edge Cases to Verify

1. `wgmesh -v --other-flags` → prints version, ignores others
2. `wgmesh --version --secret xyz` → prints version, ignores others
3. Version output format matches existing: `wgmesh <version>`
4. Exit code is 0 (success) for version display

## Estimated Complexity

**low** (15-30 minutes)

- Simple argument scanning before subcommand processing
- No changes to flag parsing or subcommand logic
- Minimal code addition (3-5 lines for version check)
- Documentation updates are straightforward
- Manual testing is quick and comprehensive
