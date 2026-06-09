# Issue #652: Investigate and fix CI failure blocking merge pipeline

## Classification
bug

## Problem Analysis

The merge pipeline for wgmesh is blocked by a CI failure that prevents automatic merging of approved implementations. Based on the codebase analysis, the CI pipeline involves multiple interconnected workflows:

1. **Spec Auto-Approve** (`.github/workflows/spec-auto-approve.yml`) - Validates and auto-approves specification PRs
2. **Goose Implementation** (`.github/workflows/goose-build.yml`) - Runs Goose AI to implement specs
3. **Auto-Merge** (`.github/workflows/auto-merge.yml`) - Automatically merges implementation PRs

The pipeline failure could stem from several potential root causes:

### Potential Failure Points:

1. **Goose Build Timeout/Failure**
   - The Goose implementation workflow has a 40-minute timeout
   - Uses external AI API (ZAI_API_KEY) which may fail with auth/network issues
   - Runs complex build/test/vet checks that may fail on new Go 1.25.5

2. **Missing Dependencies**
   - mem0ai Python package installation may fail silently
   - HuggingFace model cache misses
   - Goose CLI installation failures

3. **Memory System Issues**
   - mem0 database encryption/decryption failures
   - QDrant vector store initialization failures
   - Memory retrieval/save script errors

4. **Secret/Configuration Issues**
   - `ZAI_API_KEY` secret missing or invalid
   - `MEM0_ENCRYPTION_KEY` secret missing
   - `APP_ID`/`APP_PRIVATE_KEY` GitHub App configuration issues
   - `MENTISDB_URL`/`POSTHOG_PROJECT_KEY` external service failures

5. **Workflow Logic Issues**
   - Spec auto-approve validation logic may be too strict
   - Branch naming conflicts (`goose/issue-N` branches)
   - Git checkout/commit/push failures

6. **Go Build/Test Failures**
   - Codebase may have existing test failures
   - Go 1.25.5 compatibility issues with dependencies
   - `go vet` finding issues in existing code

### Investigation Strategy:

The spec needs to identify the exact failure point by:
1. Checking recent workflow run logs for the specific failure
2. Examining the auto-merge workflow status checks
3. Verifying all required secrets are configured
4. Testing the build/test/vet commands locally
5. Checking for any recent changes that broke compatibility

## Proposed Approach

### Phase 1: Diagnostic Investigation

1. **Identify the failing workflow**
   - Check GitHub Actions tab for recent failed runs
   - Determine which specific workflow is failing (goose-build, spec-auto-approve, or auto-merge)
   - Extract the exact error message and failure step

2. **Verify CI prerequisites**
   - Confirm all required secrets are configured in repository settings
   - Verify GitHub App (APP_ID) has correct permissions
   - Check that the Goose recipe file exists and is valid YAML

3. **Test build locally**
   - Run `go build ./...` to check for compilation errors
   - Run `go test ./...` to check for test failures
   - Run `go vet ./...` to check for vet issues
   - Run `gofmt -w .` to check formatting

### Phase 2: Fix Implementation

Based on investigation findings, implement fixes:

**If build/test failures:**
- Fix the specific test failures or compilation errors
- Ensure all tests pass locally before CI changes
- Add any missing dependencies to `go.mod`

**If secret/configuration issues:**
- Document which secrets need to be configured
- Add better error messages in workflows for missing secrets
- Update workflow validation to provide clearer guidance

**If workflow logic issues:**
- Fix branch naming conflicts or Git operation failures
- Improve error handling and retry logic
- Add better logging for debugging

**If external dependency issues:**
- Make mem0/HuggingFace/cache failures truly non-fatal with better warnings
- Add fallback behavior when external services are unavailable
- Improve timeout handling

**If Go version compatibility:**
- Pin Go version in workflows to a known working version
- Update dependencies for Go 1.25.5 compatibility
- Add compatibility tests

### Phase 3: Validation

1. **Test the fix**
   - Create a test spec PR to verify the full pipeline works
   - Monitor the workflow runs to completion
   - Verify auto-merge works correctly

2. **Add safeguards**
   - Add workflow status checks to prevent merging broken code
   - Improve error messages in workflows
   - Add notifications for CI failures

3. **Document the solution**
   - Document the root cause in `docs/solutions/test-failures/` or `docs/solutions/integration-issues/`
   - Update troubleshooting docs if needed

## Acceptance Criteria

1. **CI Pipeline Works**
   - All workflows (spec-auto-approve, goose-build, auto-merge) complete successfully
   - Implementation PRs are automatically merged after approval
   - No manual intervention required for valid specs

2. **Build/Test Pass**
   - `go build ./...` succeeds with no errors
   - `go test ./...` passes all tests
   - `go vet ./...` reports no issues
   - Code is properly formatted with `gofmt`

3. **Error Handling**
   - Missing secrets produce clear error messages
   - Transient failures (network, external services) are handled gracefully
   - Workflow logs provide sufficient debugging information

4. **Documentation**
   - Root cause and fix documented in appropriate solutions directory
   - Any new requirements or configuration steps documented
   - Troubleshooting guide updated if applicable

## Out of Scope

- Changing the overall CI/CD architecture (e.g., replacing Goose with a different system)
- Modifying the spec approval workflow logic
- Adding new CI features or metrics
- Performance optimizations of the pipeline (unless the failure is performance-related)
- Fixing unrelated code issues discovered during investigation (unless they block the CI)
