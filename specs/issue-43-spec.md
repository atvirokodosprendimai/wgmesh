# Specification: Issue #43

## Classification
feature

## Deliverables
code + tests + documentation

## Problem Analysis

Currently `wgmesh version` works (added in PR #42), but conventional `--version` and `-v` flags do not. Users expect CLI tools to support both forms. The version subcommand prints in format: `wgmesh <version>`.

**Current behavior:**
- `wgmesh version` ✅ works
- `wgmesh --version` ❌ not recognized  
- `wgmesh -v` ❌ not recognized

**Root cause:** In `main.go`, subcommand routing happens before flag parsing, so `--version` is never checked.

## Proposed Approach

Add early argument scanning for version flags before subcommand processing:

1. **Before line 23 in main.go**, scan `os.Args[1:]` for `--version` or `-v`
2. If found, print `wgmesh <version>` and exit immediately
3. Otherwise, continue with existing subcommand switch logic

**Why this approach:**
- Non-invasive: doesn't modify existing code paths
- Priority-based: version flags override all other operations  
- Simple: ~6 lines of code using standard library
- Compatible: preserves `version` subcommand for backward compatibility

**Implementation snippet:**
```go
// Check for version flags first (before subcommand check)
for _, arg := range os.Args[1:] {
    if arg == "--version" || arg == "-v" {
        fmt.Println("wgmesh " + version)
        return
    }
}
```

**Why not use flag package:** Would require calling `flag.Parse()` before subcommand routing, interfering with subcommands that use their own `flag.NewFlagSet()`.

## Affected Files

1. **main.go** (line ~22): Add version flag check loop (6-8 lines)
2. **main.go** (printUsage, line ~148): Update help text to include `version` subcommand (currently not listed) and document `--version`/`-v` as aliases
3. **main_test.go** (new file): Create automated tests for all three version forms
4. **README.md** (optional): Update CLI usage examples if present

## Test Strategy

**Manual verification:**
```bash
# Basic functionality
wgmesh --version    # Should print: wgmesh <version>
wgmesh -v           # Should print: wgmesh <version>  
wgmesh version      # Should still work

# Priority verification
wgmesh --version --help    # Should print version, not help
wgmesh -v join             # Should print version, not start join

# Build-time version
go build -ldflags "-X main.version=1.2.3"
./wgmesh --version         # Should print: wgmesh 1.2.3
```

**Regression checks:**
- All existing subcommands (join, init, status) still work
- Centralized mode flags (-state, -add, -list) still work  
- Help text displays correctly

**Automated testing:** 

Create `main_test.go` with unit tests covering:
- `--version` flag prints correct output and exits
- `-v` flag prints correct output and exits  
- `version` subcommand continues to work
- Version flags take priority over other arguments (e.g., `--version --help`)
- Output format matches `wgmesh <version>`

This aligns with the issue acceptance criteria requiring tests for all three forms and ensures the repository maintains test coverage consistent with other packages.

## Estimated Complexity

**low** (30-45 minutes)

**Rationale:**
- Minimal code change (6-8 lines)
- No architectural modifications
- No external dependencies  
- Straightforward logic (simple loop)
- Low risk (doesn't touch existing paths)
- Quick to test and verify
