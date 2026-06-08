# Issue #667 - Implement Fix for CI Failure Blocking Merge Pipeline

## Summary

Implement the fix specified in issue #652 to resolve the CI failure blocking the merge pipeline. The auto-merge workflow (`.github/workflows/auto-merge.yml`) references a non-existent "CodeQL" workflow in its trigger configuration, causing the auto-merge functionality to wait indefinitely for a workflow that does not exist. This implementation will remove the CodeQL dependency and ensure the auto-merge workflow triggers correctly on actual CI completion.

## Context

### Problem Statement

From issue #652, the auto-merge workflow is configured with:

```yaml
on:
  workflow_run:
    workflows: ["Build and Push Docker Images", "CodeQL"]
    types: [completed]
```

The repository has 28 workflow files but **no CodeQL workflow exists**. The actual CI workflows are:
- `docker-build.yml` (named "Build and Push Docker Images")
- `status-check.yml` (runs lint and status checks)

### Current Impact

1. **Auto-merge trigger failure**: The workflow_run trigger requires both workflows to complete, but "CodeQL" never completes because it doesn't exist
2. **Manual intervention required**: PRs that pass all actual CI checks remain unmerged
3. **Pipeline blocking**: The merge pipeline cannot automatically approve and merge PRs

### Solution Approach

The fix is straightforward:
1. Remove "CodeQL" from the workflow_run trigger list
2. Verify all referenced workflows exist
3. Add inline documentation explaining trigger dependencies
4. Test the fix to ensure auto-merge works correctly

### Files Involved

- **Primary**: `.github/workflows/auto-merge.yml` - modify workflow_run trigger
- **Related workflows** (for reference):
  - `.github/workflows/docker-build.yml` - "Build and Push Docker Images"
  - `.github/workflows/status-check.yml` - "Status Check"

## Requirements

### Must Implement

1. **Remove CodeQL dependency**
   - Delete "CodeQL" from the `workflow_run.workflows` array in `auto-merge.yml`
   - Keep only "Build and Push Docker Images" as the trigger workflow

2. **Verify workflow names**
   - Confirm "Build and Push Docker Images" matches the actual workflow name in `docker-build.yml`
   - Ensure no other non-existent workflows are referenced

3. **Add inline documentation**
   - Add comments explaining which workflows trigger auto-merge
   - Document why these workflows are required (they represent the full CI gate)
   - Include note about the removed CodeQL reference and why it was incorrect

4. **Validate YAML syntax**
   - Ensure the modified YAML is valid
   - No syntax errors that would prevent workflow loading

### Should Implement

1. **Consider status-check workflow**
   - Evaluate if "Status Check" workflow should be added to the trigger list
   - If yes, add it to the `workflows` array
   - Document why it's required (represents lint and status validation)

2. **Add verification comment**
   - Add a comment at the top of the workflow explaining the trigger logic
   - Include reference to issue #652/#667 for context

### Nice to Have

1. **Add workflow trigger validation**
   - Consider adding a workflow that validates all workflow_run references point to existing workflows
   - This would prevent similar issues in the future

2. **Document CI workflow dependencies**
   - Create a brief document explaining the CI workflow dependencies
   - List all workflows that must complete before auto-merge can proceed

## Acceptance Criteria

1. ✅ Auto-merge workflow YAML is syntactically valid (no YAML errors)
2. ✅ Workflow_run trigger references only existing workflows
3. ✅ No references to "CodeQL" remain in the trigger configuration
4. ✅ Inline comments document the trigger dependencies
5. ✅ Auto-merge workflow triggers when "Build and Push Docker Images" completes successfully
6. ✅ PRs auto-merge after passing all required CI checks (with proper approvals)
7. ✅ No regressions in existing merge behavior

### Verification Steps

1. **Syntax validation**
   - Use `yamllint` or GitHub's workflow syntax validation
   - Confirm no YAML errors

2. **Workflow name verification**
   - Check that "Build and Push Docker Images" matches exactly in `docker-build.yml`
   - Verify no typos in workflow names

3. **Manual test** (if possible)
   - Create a test PR or use an existing non-critical PR
   - Verify auto-merge workflow runs when docker-build completes
   - Confirm auto-merge approves and merges eligible PRs

4. **Log verification**
   - Check workflow logs show no errors related to missing workflows
   - Confirm trigger fires correctly on workflow completion

5. **Regression testing**
   - Verify existing merge behavior still works
   - Confirm no unintended side effects from the change

## Out of Scope

1. **Adding CodeQL security scanning** - Not implementing CodeQL; only removing the reference
2. **Modifying branch protection rules** - Not changing required checks or protection rules
3. **Changing approval mechanism** - Not modifying how approvals work
4. **Modifying Docker build workflow** - Not changing docker-build.yml itself
5. **Changing E2E test workflows** - Not modifying e2e-verifier.yml or hetzner-integration.yml
6. **Modifying other bot workflows** - Not changing approve-build.yml, goose-triage.yml, etc.
7. **Creating new CI workflows** - Only fixing existing configuration, not adding new workflows

### Code Safety

This is a low-risk change:
- Only modifying workflow trigger configuration
- Not changing any logic or behavior
- Removing a reference that was never valid
- The change is backwards compatible (the CodeQL reference never worked)

### Testing Requirements

While automated tests are difficult for GitHub Actions workflows, the following should be verified:
1. YAML syntax is valid
2. Workflow names match existing workflows
3. Manual verification that auto-merge triggers correctly
4. No workflow errors in GitHub Actions logs after the change
