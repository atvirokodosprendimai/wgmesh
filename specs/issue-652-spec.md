# Issue #652 - Investigate and fix CI failure blocking merge pipeline

## Summary

The auto-merge workflow (`.github/workflows/auto-merge.yml`) is configured to trigger on completion of two workflows: "Build and Push Docker Images" and "CodeQL". However, the CodeQL workflow does not exist in the repository, causing the auto-merge workflow to potentially fail to trigger correctly or wait indefinitely for a non-existent workflow to complete. This blocks the merge pipeline from automatically approving and merging PRs after CI checks pass.

## Context

### Current State

The auto-merge workflow at `.github/workflows/auto-merge.yml` defines its trigger as:

```yaml
on:
  workflow_run:
    workflows: ["Build and Push Docker Images", "CodeQL"]
    types: [completed]
```

### Problem Analysis

1. **Missing CodeQL workflow**: The repository has 26 workflow files but no CodeQL workflow exists
2. **Trigger dependency**: The auto-merge workflow explicitly waits for both "Build and Push Docker Images" and "CodeQL" workflows to complete
3. **Actual CI workflows**: The repository has:
   - `docker-build.yml` (named "Build and Push Docker Images")
   - `status-check.yml` (runs lint and status checks)
   - Various other workflows but no CodeQL analysis workflow

4. **Impact**: This misconfiguration likely causes:
   - Auto-merge may not trigger when expected
   - PRs remain unmerged despite passing all actual CI checks
   - Manual intervention required to merge PRs

### File Structure

The `.github/workflows/` directory contains:
- Agent workflows (goose-*.yml)
- CI workflows (docker-build.yml, status-check.yml)
- Release workflows (release.yml, docker-build.yml for tags)
- E2E workflows (e2e-verifier.yml, hetzner-integration.yml)
- Bot workflows (auto-merge.yml, approve-build.yml, etc.)

## Requirements

### Must Fix

1. **Remove CodeQL dependency**: Delete "CodeQL" from the workflow_run trigger list in `auto-merge.yml`
2. **Verify correct CI workflows**: Ensure the auto-merge trigger references only workflows that:
   - Actually exist in the repository
   - Represent the full set of required CI checks
   - Complete successfully before merge should proceed

3. **Update trigger logic**: The auto-merge workflow should trigger on completion of all actual CI workflows:
   - "Build and Push Docker Images" (docker-build.yml)
   - Any other required CI check workflows

### Should Fix

1. **Add status-check workflow**: Determine if `status-check.yml` should be added to the trigger list
2. **Document trigger dependencies**: Add comments explaining which workflows are required and why
3. **Test auto-merge behavior**: Verify the fix by testing on a non-critical branch or PR

### Nice to Have

1. **Add CodeQL workflow**: If security analysis is desired, create an actual CodeQL workflow
2. **Consolidate CI workflows**: Consider creating a single "CI" workflow that runs all checks
3. **Add monitoring**: Track auto-merge success/failure rates

## Acceptance Criteria

1. ✅ Auto-merge workflow triggers correctly when "Build and Push Docker Images" completes
2. ✅ No references to non-existent workflows remain in the trigger configuration
3. ✅ PRs auto-merge after all required CI checks pass (with approvals, etc.)
4. ✅ No manual merge intervention required for PRs that pass all checks
5. ✅ Workflow syntax is valid (no YAML errors)
6. ✅ Trigger logic is documented with inline comments

### Verification Steps

1. Create a test PR or use existing PR
2. Verify auto-merge workflow runs when docker-build completes
3. Confirm auto-merge approves and merges eligible PRs
4. Check workflow logs show no errors related to missing workflows
5. Verify no regressions in existing merge behavior

## Out of Scope

1. Adding CodeQL security scanning (unless explicitly requested)
2. Modifying branch protection rules or required checks
3. Changing the approval mechanism or merge method
4. Modifying other bot workflows (approve-build, goose-triage, etc.)
5. Changes to Docker build workflow itself
6. Modifying E2E test workflows or release workflows
