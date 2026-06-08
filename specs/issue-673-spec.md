# Issue #673 - spec: Issue #672 - spec: Issue #671 - spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline

## Summary

Implement the fix to resolve the CI failure blocking the merge pipeline by removing the non-existent "CodeQL" workflow reference from the auto-merge workflow trigger configuration. This is the final implementation phase of a five-issue chain: #652 (investigation), #667 (proposed fix), #671 (meta-spec), #672 (implementation spec), and #673 (implementation).

## Context

### Complete Issue Chain

1. **Issue #652** (specs/issue-652-spec.md): "Investigate and fix CI failure blocking merge pipeline"
   - ✅ Identified that auto-merge.yml references non-existent "CodeQL" workflow
   - ✅ Spec created documenting the investigation findings
   - Status: Investigation complete

2. **Issue #667** (specs/issue-667-spec.md): "Implement Fix for CI Failure Blocking Merge Pipeline"
   - ✅ Proposed implementation approach
   - ✅ Spec created with detailed requirements
   - Status: Specification complete

3. **Issue #671** (specs/issue-671-spec.md): "spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline"
   - ✅ Created meta-spec tracing the issue chain
   - ✅ Documented why "Status Check" should NOT be added to trigger
   - ✅ Explained that merge script already validates ALL checks via GitHub Checks API
   - Status: Meta-specification complete

4. **Issue #672** (specs/issue-672-spec.md): "spec: Issue #671 - spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline"
   - ✅ Created detailed implementation spec
   - ✅ Documented acceptance criteria and verification steps
   - Status: Implementation specification complete

5. **Issue #673** (this issue): "spec: Issue #672 - spec: Issue #671 - spec: Issue #667 - spec: Issue #652 - Investigate and fix CI failure blocking merge pipeline"
   - 🔄 **Final implementation phase**
   - Will actually modify `.github/workflows/auto-merge.yml`
   - Status: Awaiting implementation (this spec)

### Problem Analysis

The auto-merge workflow at `.github/workflows/auto-merge.yml` line 13 contains:

```yaml
on:
  workflow_run:
    workflows: ["Build and Push Docker Images", "CodeQL"]
    types: [completed]
```

**Issue**: "CodeQL" does not exist in the repository. The actual CI workflows are:
- `docker-build.yml` → "Build and Push Docker Images" ✅ (exists, verified)
- `status-check.yml` → "Status Check" ✅ (exists but intentionally NOT in trigger)
- ~~"CodeQL"~~ ❌ (does not exist)

### Current Impact

1. **Trigger dependency broken**: The `workflow_run` trigger waits for both workflows to complete, but "CodeQL" never completes because it doesn't exist
2. **Fallback mechanisms mitigate but don't fix**: The workflow has three fallback triggers:
   - `schedule: - cron: '*/10 * * * *'` - checks every 10 minutes
   - `pull_request: types: [ready_for_review]` - fires when draft PRs are marked ready
   - `workflow_dispatch` - allows manual triggering
3. **Delayed merges**: PRs may not merge as quickly as intended when workflow_run trigger fails
4. **Misleading configuration**: The trigger references a workflow that doesn't exist, causing confusion

### Why "Status Check" Is NOT in the Trigger

The auto-merge script already verifies ALL check runs via the GitHub Checks API (lines 107-135 in the workflow):

```javascript
const { data: checkRuns } = await github.rest.checks.listForRef({
  owner, repo,
  ref: pr.head.sha,
});

const relevant = checkRuns.check_runs.filter(
  cr => cr.name !== 'auto-merge' &&
        cr.name !== 'Auto-merge on CI pass' &&
        cr.name !== 'sync' &&
        cr.name !== 'Cloud Integration Tests' &&
        cr.name !== 'Cleanup Orphaned VMs'
);
```

This includes the "Status Check" workflow. The design is:
- **workflow_run trigger**: Fires when the primary CI workflow (docker-build) completes
- **GitHub Checks API verification**: Script validates ALL checks (including Status Check) before merging
- **Result**: Only docker-build needs to be in the workflow_run trigger; everything else is verified via API

Adding "Status Check" to the workflow_run trigger would create a duplicate dependency and is unnecessary.

### Root Cause

The "CodeQL" reference is legacy code from when the project may have planned to use GitHub's CodeQL security scanning. A CodeQL workflow was never created, but the reference remained in the auto-merge trigger configuration. This is a common pattern where planned features are referenced in configuration but never implemented.

## Requirements

### Must Implement

1. **Remove CodeQL dependency**
   - Delete "CodeQL" from the `workflow_run.workflows` array in `.github/workflows/auto-merge.yml` line 13
   - Keep only "Build and Push Docker Images" as the workflow_run trigger
   - Verify the workflow name exactly matches `docker-build.yml` line 1

2. **Add inline documentation**
   - Add comment at line 13 explaining the trigger logic
   - Document why only "Build and Push Docker Images" is in the workflow_run trigger
   - Explain that ALL checks are verified via GitHub Checks API in the script
   - Include reference to the full issue chain: #652, #667, #671, #672, #673

3. **Add historical comment**
   - Add comment explaining that "CodeQL" was removed because the workflow never existed
   - Note the legacy nature of the reference and the issue chain that identified it

4. **Validate YAML syntax**
   - Ensure modified YAML is valid
   - No syntax errors that would prevent workflow loading
   - Confirm workflow loads successfully in GitHub Actions UI

### Should Implement

1. **Update header comments**
   - Review lines 1-10 header comments
   - Ensure they accurately describe the trigger mechanism
   - Consider adding brief explanation of why workflow_run only triggers on docker-build

2. **Document fallback mechanisms**
   - Add comment explaining why schedule and pull_request triggers exist
   - Document that they catch edge cases where workflow_run timing causes misses

### Nice to Have

1. **Add workflow reference validation**
   - Consider adding a CI step or script to validate all workflow_run references point to existing workflows
   - This would prevent similar issues in the future

2. **Document in project docs**
   - Briefly mention the CI workflow dependencies in project documentation
   - Explain which workflows represent the full CI gate

## Acceptance Criteria

1. ✅ Auto-merge workflow YAML is syntactically valid (no YAML errors)
2. ✅ Workflow_run trigger references only existing workflows
3. ✅ No references to "CodeQL" remain in the trigger configuration or anywhere in the repository
4. ✅ Inline comments document the trigger dependencies and rationale
5. ✅ Historical comment references the full issue chain (#652, #667, #671, #672, #673)
6. ✅ Auto-merge workflow triggers correctly when "Build and Push Docker Images" completes successfully
7. ✅ PRs auto-merge after passing all required CI checks (with proper approvals)
8. ✅ No regressions in existing merge behavior (fallback triggers still work)

### Verification Steps

1. **Syntax validation**
   - Use GitHub Actions workflow editor to validate YAML syntax
   - Confirm no YAML errors before committing
   - Verify workflow loads successfully in GitHub Actions UI

2. **Workflow name verification**
   - Run: `head -1 .github/workflows/docker-build.yml` to verify name is "Build and Push Docker Images"
   - Confirm exact match with auto-merge.yml trigger (case-sensitive)
   - Ensure no typos in workflow name

3. **CodeQL reference cleanup**
   - Run: `grep -r "CodeQL" .github/workflows/` to verify no other references
   - Confirm "CodeQL" is only removed from auto-merge.yml
   - Verify no other workflows reference the non-existent CodeQL workflow

4. **Manual test** (if possible)
   - Create a test PR or monitor an existing non-critical PR
   - Verify auto-merge workflow runs when docker-build completes
   - Check workflow logs for successful trigger
   - Confirm no errors related to missing workflows

5. **Log verification**
   - Check workflow logs show no errors related to missing workflows
   - Confirm trigger fires correctly on workflow completion
   - Verify no references to "CodeQL" in log output
   - Monitor for any unexpected behavior

6. **Regression testing**
   - Verify schedule fallback still works (every 10 minutes)
   - Confirm ready_for_review trigger still works when draft PRs are marked ready
   - Ensure workflow_dispatch trigger still works for manual runs
   - Check that all three fallback mechanisms continue to function correctly

7. **Integration testing**
   - Monitor auto-merge behavior for several PR cycles
   - Confirm PRs auto-merge after passing all CI checks
   - Verify no PRs are stuck waiting for "CodeQL"
   - Check that merge timing improves (no more delays from missing trigger)

## Out of Scope

1. **Adding CodeQL security scanning** - Not implementing CodeQL; only removing the reference
2. **Adding "Status Check" to trigger** - Current design (verify all checks via API) is correct and documented
3. **Modifying branch protection rules** - Not changing required checks or protection rules
4. **Changing approval mechanism** - Not modifying how approvals work in the script
5. **Modifying Docker build workflow** - Not changing docker-build.yml itself
6. **Changing E2E test workflows** - Not modifying e2e-verifier.yml or hetzner-integration.yml
7. **Modifying other bot workflows** - Not changing approve-build.yml, goose-triage.yml, etc.
8. **Creating new CI workflows** - Only fixing existing configuration, not adding new workflows
9. **Modifying merge script logic** - Not changing the JavaScript merge logic, only the trigger
10. **Adding new triggers** - Not modifying schedule, pull_request, or workflow_dispatch triggers

## Affected Files

### Code Changes Required

1. **`.github/workflows/auto-merge.yml`**:
   - **Line 13**: Remove "CodeQL" from workflows array
     - Before: `workflows: ["Build and Push Docker Images", "CodeQL"]`
     - After: `workflows: ["Build and Push Docker Images"]`
   - **Lines 13-14**: Add inline comments documenting trigger logic
   - **Lines 1-10**: Review and update header comments if needed
   - Add historical comment section referencing the issue chain

### Suggested Code Change

```yaml
# BEFORE (lines 12-14):
on:
  workflow_run:
    workflows: ["Build and Push Docker Images", "CodeQL"]
    types: [completed]

# AFTER (lines 12-22):
on:
  # Trigger on docker-build completion (the primary CI workflow).
  # The merge script verifies ALL check runs via GitHub Checks API,
  # including Status Check, so no other workflows need to be listed here.
  # "CodeQL" was removed (issues #652, #667, #671, #672, #673) — workflow never existed.
  workflow_run:
    workflows: ["Build and Push Docker Images"]
    types: [completed]
```

### Optional Header Comment Update

Consider updating lines 1-10 to clarify the trigger mechanism:

```yaml
# Auto-approves and squash-merges non-draft PRs to main when all CI checks pass.
# Uses a GitHub App (wgmesh-bot) so the approval comes from a distinct identity,
# bypassing the "cannot approve your own PR" restriction.
#
# Trigger: Fires when docker-build workflow completes (the primary CI gate).
# The merge script then verifies ALL check runs via GitHub Checks API before merging.
# Fallback triggers (schedule, pull_request, workflow_dispatch) handle edge cases.
#
# When a PR branch is behind main, the workflow updates it automatically.
# After a branch update, CI re-runs and the next cycle merges the PR.
```

## Test Strategy

### Syntax Testing
1. Use GitHub Actions workflow editor to validate YAML syntax
2. Verify no syntax errors before committing
3. Confirm workflow loads successfully in GitHub Actions UI
4. Check for YAML indentation errors

### Trigger Testing
1. Monitor workflow_run events in GitHub Actions logs
2. Verify auto-merge workflow triggers when docker-build completes
3. Check that the workflow no longer references "CodeQL"
4. Confirm trigger fires on successful docker-build completion (not on failure)

### CodeQL Reference Cleanup
1. Search entire repository for "CodeQL" references: `grep -r "CodeQL" .github/`
2. Verify no other workflows reference the non-existent CodeQL workflow
3. Confirm cleanup is complete

### Integration Testing
1. Monitor auto-merge behavior for several PR cycles
2. Confirm PRs auto-merge after passing all CI checks
3. Verify no PRs are stuck waiting for "CodeQL"
4. Check that merge timing improves (no more delays from missing trigger)
5. Verify the script still validates ALL checks via GitHub Checks API

### Regression Testing
1. Verify schedule fallback still works (every 10 minutes)
2. Confirm ready_for_review trigger still works when draft PRs are marked ready
3. Ensure workflow_dispatch trigger still works for manual runs
4. Check that all three fallback mechanisms continue to function correctly
5. Verify no changes to merge script behavior or approval logic

### Log Analysis
1. Check workflow logs for errors related to missing workflows
2. Verify trigger fires correctly on workflow completion
3. Confirm no references to "CodeQL" in any log output
4. Monitor for any unexpected behavior in the merge script
5. Verify the GitHub Checks API verification still works correctly

### Risk Assessment
- **Very low risk**: Only removing an invalid reference that never worked
- **No logic changes**: Not modifying the merge script logic or behavior
- **Backwards compatible**: The "CodeQL" reference was never valid (workflow doesn't exist)
- **Easily revertible**: Can add back "CodeQL" if unexpected issues arise (though unlikely)
- **No impact on other workflows**: Only modifying auto-merge.yml
- **Fallbacks remain intact**: Schedule, pull_request, and workflow_dispatch triggers unchanged
- **Well-documented**: Five specs (#652, #667, #671, #672, #673) document the issue and solution

## Estimated Complexity

**Trivial** (5-10 minutes implementation, 15-30 minutes verification)

### Breakdown
- **Code change**: 2 minutes
  - Single-line change to remove "CodeQL" from array
  - Adding inline documentation comments (3-4 lines)
  - Optional header comment update
- **Validation**: 3 minutes
  - YAML syntax validation
  - Workflow name verification
  - CodeQL reference cleanup check
- **Testing**: 10-25 minutes
  - Monitor workflow execution on next PR
  - Verify trigger fires correctly
  - Check logs for errors
  - Confirm fallback mechanisms still work

### Why Trivial
1. **Single-line change**: Only removing "CodeQL" from one array
2. **No logic changes**: Not modifying any JavaScript code or workflow behavior
3. **Well-understood problem**: Root cause and fix clearly documented across five specs
4. **Low risk**: Removing invalid reference that never worked
5. **Existing fallbacks**: Three other triggers already handle edge cases
6. **Extensive documentation**: Five specs (#652, #667, #671, #672, #673) document the issue and solution
7. **Verification is straightforward**: YAML syntax check and workflow observation

### Success Metrics
1. Auto-merge workflow triggers correctly when docker-build completes
2. No references to "CodeQL" remain in repository (verified via grep)
3. Workflow logs show no errors related to missing workflows
4. PR merge timing improves (no more delays from broken trigger)
5. All fallback mechanisms continue to work correctly
6. Merge script still validates ALL checks via GitHub Checks API

## Related Documentation

- **Issue #652** (specs/issue-652-spec.md): Initial investigation identifying the CodeQL reference issue
- **Issue #667** (specs/issue-667-spec.md): Proposed implementation approach
- **Issue #671** (specs/issue-671-spec.md): Meta-spec explaining why Status Check is not in trigger
- **Issue #672** (specs/issue-672-spec.md): Detailed implementation specification
- **Issue #673** (this spec): Final implementation specification

## Implementation Notes

### Before Making Changes
1. Verify the current state of auto-merge.yml matches what's documented
2. Confirm docker-build.yml workflow name is "Build and Push Docker Images"
3. Check for any other CodeQL references in the repository

### Making Changes
1. Remove "CodeQL" from line 13 workflows array
2. Add inline comments explaining the trigger logic
3. Add historical comment referencing the issue chain
4. Optionally update header comments

### After Making Changes
1. Validate YAML syntax
2. Verify workflow loads in GitHub Actions UI
3. Test trigger on next PR or manual workflow_dispatch
4. Monitor for several PR cycles to confirm fix
5. Update relevant specs (mark #652, #667, #671, #672, #673 as implemented)
