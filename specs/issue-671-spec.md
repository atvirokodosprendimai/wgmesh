# Issue #671 - spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline

## Summary

Create implementation specifications for issues #652, #667, and #671 to investigate and fix CI failures blocking the merge pipeline. This is a meta-spec that traces through the chain of specs: issue #652 identified that the auto-merge workflow references a non-existent "CodeQL" workflow, issue #667 proposed the fix, and this issue (#671) will implement the fix by removing the CodeQL dependency from the auto-merge workflow trigger.

## Context

### Issue Chain

1. **Issue #652**: "Investigate and fix CI failure blocking merge pipeline"
   - Identified that auto-merge.yml references a non-existent "CodeQL" workflow
   - Spec created at specs/issue-652-spec.md
   - Status: Spec implemented, awaiting implementation

2. **Issue #667**: "Implement Fix for CI Failure Blocking Merge Pipeline"
   - Proposed implementation approach for fixing issue #652
   - Spec created at specs/issue-667-spec.md
   - Status: Spec implemented, awaiting implementation

3. **Issue #671** (this issue): "spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline"
   - This is the implementation phase
   - Will actually modify .github/workflows/auto-merge.yml
   - Status: Awaiting specification (this document)

### Problem Analysis

The auto-merge workflow (`.github/workflows/auto-merge.yml`) is configured to trigger on completion of two workflows:

```yaml
on:
  workflow_run:
    workflows: ["Build and Push Docker Images", "CodeQL"]
    types: [completed]
```

However, **"CodeQL" does not exist** in the repository. The actual CI workflows are:
- `docker-build.yml` (named "Build and Push Docker Images")
- `status-check.yml` (named "Status Check")

### Current Impact

1. **Trigger dependency issue**: The `workflow_run` trigger waits for both workflows to complete, but "CodeQL" never completes because it doesn't exist
2. **Timing-based fallback**: The workflow has a `schedule: - cron: '*/10 * * * *'` fallback that catches PRs where timing caused a miss, but this is a workaround, not a solution
3. **Manual intervention**: PRs may remain unmerged despite passing all actual CI checks
4. **Pipeline reliability**: The merge pipeline cannot function as designed

### Why Status Check Should Not Be Added

While investigating whether to add "Status Check" to the workflow_run trigger:
- The `status-check.yml` workflow runs `make lint-eidos`, `make status`, and checks `STATUS.md` diff
- These are repository-level status checks, not PR-specific CI checks
- The auto-merge workflow already checks ALL check runs via the GitHub Checks API in the script
- Adding "Status Check" to the trigger would create a duplicate dependency
- The current approach (wait for docker-build only, then verify all checks) is correct

### Root Cause

The "CodeQL" reference is legacy code from when the project may have planned to use CodeQL security scanning. A CodeQL workflow was never created, but the reference remained in the auto-merge trigger.

## Requirements

### Must Implement

1. **Remove CodeQL dependency**
   - Delete "CodeQL" from the `workflow_run.workflows` array in `auto-merge.yml`
   - Keep only "Build and Push Docker Images" as the trigger workflow

2. **Verify workflow names**
   - Confirm "Build and Push Docker Images" exactly matches the name in `docker-build.yml`
   - Ensure no other non-existent workflows are referenced

3. **Add inline documentation**
   - Add comments explaining the trigger logic
   - Document why "Build and Push Docker Images" is the only workflow_run trigger
   - Explain that ALL checks are verified via the GitHub Checks API in the script
   - Include reference to issues #652, #667, #671 for historical context

4. **Add historical comment**
   - Document that "CodeQL" was removed because it never existed
   - Explain the legacy nature of the reference

5. **Validate YAML syntax**
   - Ensure modified YAML is valid
   - No syntax errors that would prevent workflow loading

### Should Implement

1. **Update header comments**
   - Review and update the workflow's header comments to reflect current behavior
   - Ensure the description accurately describes the trigger mechanism

2. **Document the fallback mechanism**
   - Add comments explaining why the schedule fallback exists (catch timing misses)
   - Document that the periodic check is safe because all checks are verified before merge

### Nice to Have

1. **Add workflow reference validation**
   - Consider adding a linting step or script to validate all workflow_run references
   - This would prevent similar issues in the future

2. **Document CI architecture**
   - Briefly document the CI workflow dependencies in a comment
   - Explain which workflows represent the CI gate

## Acceptance Criteria

1. ✅ Auto-merge workflow YAML is syntactically valid
2. ✅ Workflow_run trigger references only existing workflows
3. ✅ No references to "CodeQL" remain in the trigger configuration
4. ✅ Inline comments document the trigger dependencies and rationale
5. ✅ Historical comment references issues #652, #667, #671
6. ✅ Auto-merge workflow triggers correctly when "Build and Push Docker Images" completes
7. ✅ PRs auto-merge after passing all required CI checks (with proper approvals)
8. ✅ No regressions in existing merge behavior

### Verification Steps

1. **Syntax validation**
   - Use GitHub's workflow editor or `yamllint` to validate syntax
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
   - Ensure the schedule fallback still works as intended

## Out of Scope

1. **Adding CodeQL security scanning** - Not implementing CodeQL; only removing the reference
2. **Adding "Status Check" to trigger** - Current design (verify all checks via API) is correct
3. **Modifying branch protection rules** - Not changing required checks or protection rules
4. **Changing approval mechanism** - Not modifying how approvals work
5. **Modifying Docker build workflow** - Not changing docker-build.yml itself
6. **Changing E2E test workflows** - Not modifying e2e-verifier.yml or hetzner-integration.yml
7. **Modifying other bot workflows** - Not changing approve-build.yml, goose-triage.yml, etc.
8. **Creating new CI workflows** - Only fixing existing configuration, not adding new workflows
9. **Modifying the merge logic** - Not changing the auto-merge script logic, only the trigger

## Affected Files

### Code Changes Required

1. **`.github/workflows/auto-merge.yml`**:
   - Line 13: Remove "CodeQL" from workflows array
   - Lines 1-10: Update header comments if needed
   - Add inline comments documenting trigger logic and historical context

## Test Strategy

### Syntax Testing
1. Use GitHub Actions workflow editor to validate YAML syntax
2. Verify no syntax errors before committing

### Trigger Testing
1. Create a test PR to verify auto-merge workflow triggers on docker-build completion
2. Check workflow logs to confirm trigger fires correctly
3. Verify the workflow no longer references "CodeQL"

### Integration Testing
1. Monitor auto-merge behavior for several PR cycles
2. Confirm PRs auto-merge after passing all CI checks
3. Verify no PRs are stuck waiting for "CodeQL"

### Regression Testing
1. Verify the schedule fallback still works (every 10 minutes)
2. Confirm the ready_for_review trigger still works
3. Ensure the workflow_dispatch trigger still works

### Risk Assessment
- **Very low risk**: Only removing an invalid reference that never worked
- **No logic changes**: Not modifying the merge script logic
- **Backwards compatible**: The "CodeQL" reference was never valid
- **Easily revertible**: Can add back "CodeQL" if unexpected issues arise
- **No impact on other workflows**: Only modifying auto-merge.yml

## Estimated Complexity

**trivial** (5-10 minutes)

- Single-line change to remove "CodeQL" from array
- Adding inline documentation comments
- Validating YAML syntax
- No code logic changes
- Following established pattern for workflow triggers

The change is straightforward because:
1. The "CodeQL" reference was never valid (workflow doesn't exist)
2. Removing it fixes the trigger to match reality
3. The fallback mechanisms (schedule, pull_request, workflow_dispatch) already handle edge cases
4. The merge script already validates ALL checks via the GitHub Checks API
