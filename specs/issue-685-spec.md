# Issue #685 - Fix CI: spec-auto-approve approves as github-actions[bot], not self (app token)

## Summary

Fix the spec-auto-approve workflow to approve spec PRs using the GitHub App token identity instead of the github-actions[bot] identity. Currently, the workflow approves PRs with the app token but uses inconsistent authentication, causing the approval to appear as coming from github-actions[bot] instead of the app itself.

## Context

The `.github/workflows/spec-auto-approve.yml` workflow has two critical authentication contexts:

1. **GitHub App token** (`steps.app-token.outputs.token`): Generated using `actions/create-github-app-token@v1` with the app-id and private-key. This token represents the GitHub App as an entity.
2. **GitHub Actions token** (`secrets.GITHUB_TOKEN`): Represents the `github-actions[bot]` user.

The issue occurs in the "Auto-approve and trigger Goose" step where:
- The `gh pr review $PR_NUM --approve` command uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}` (correct)
- But the `gh pr edit $PR_NUM --add-label approved-for-build` command switches to `GH_TOKEN="$ISSUE_WRITE_TOKEN"` where `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`

This inconsistency means the approval appears to come from github-actions[bot], not the GitHub App, breaking the intended identity and permissions model.

## Requirements

### Functional Requirements

1. **Consistent App Token Usage**: The spec-auto-approve workflow must use the GitHub App token (`steps.app-token.outputs.token`) for both the PR approval AND the label operation.

2. **Identity Verification**: GitHub App identity should be clearly visible in PR review approvals (showing the app name/identity, not github-actions[bot]).

3. **Permissions**: The GitHub App must have the `pull-requests: write` permission (already granted in the workflow).

### Technical Requirements

1. Remove the token switching in the "Auto-approve and trigger Goose" step
2. Use `GH_TOKEN: ${{ steps.app-token.outputs.token }}` for all `gh` commands in that step
3. Remove the `ISSUE_WRITE_TOKEN` environment variable mapping to `secrets.GITHUB_TOKEN`

### Non-Functional Requirements

1. **Backward Compatibility**: The change must not break existing functionality (approval, labeling, goose-trigger)
2. **Audit Trail**: Approval identity should be traceable to the GitHub App for security auditing
3. **No Regression**: All validation checks and approval logic must remain unchanged

## Acceptance Criteria

### Primary Criteria

1. ✅ The PR approval in spec-auto-approve workflow comes from the GitHub App identity, not github-actions[bot]
2. ✅ The `approved-for-build` label is added using the same GitHub App token
3. ✅ The goose-build workflow is triggered successfully using the GitHub App token

### Verification Steps

1. Create a spec PR that passes all validation checks
2. Wait for spec-auto-approve workflow to run
3. Verify the PR review shows approval from the GitHub App (not github-actions[bot])
4. Verify the `approved-for-build` label was added by the GitHub App
5. Verify goose-build workflow was triggered

### Testing

- **Manual Test**: Create a test spec PR and verify approval identity
- **Workflow Test**: Ensure the workflow completes without errors
- **Integration Test**: Verify goose-build still triggers correctly after approval

## Out of Scope

This fix is intentionally limited to the spec-auto-approve workflow's authentication consistency. The following are NOT part of this change:

1. **goose-build.yml workflow**: The goose-build workflow's use of github-actions[bot] for git operations is intentional and correct (those commits should be attributed to github-actions[bot])
2. **Scheduled scan logic**: The scheduled scan section in spec-auto-approve.yml uses `github-script@v7` with separate token handling; this is not in scope
3. **Metrics collection**: Agent metrics upload functionality is unchanged
4. **GitHub App permissions**: No changes to required app permissions (`pull-requests: write`, `contents: write`, `issues: write`)
5. **Approval logic**: No changes to validation criteria or approval conditions

## Implementation Details

### File Changes

**File**: `.github/workflows/spec-auto-approve.yml`

**Section**: "Auto-approve and trigger Goose" step (approximately line 180-210)

**Current Code**:
```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    ISSUE_NUM: ${{ steps.issue.outputs.number }}
  run: |
    PR_NUM="${{ github.event.pull_request.number }}"

    echo "Auto-approving spec PR #$PR_NUM"

    # Approve the PR
    gh pr review $PR_NUM --approve --body "..."

    # Add the label for tracking
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build
```

**Updated Code**:
```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}
    ISSUE_NUM: ${{ steps.issue.outputs.number }}
  run: |
    PR_NUM="${{ github.event.pull_request.number }}"

    echo "Auto-approving spec PR #$PR_NUM"

    # Approve the PR
    gh pr review $PR_NUM --approve --body "..."

    # Add the label for tracking
    gh pr edit $PR_NUM --add-label approved-for-build

    # Trigger Goose
    gh workflow run goose-build.yml ...
```

**Key Changes**:
1. Removed `ISSUE_WRITE_TOKEN` environment variable
2. Removed `GH_TOKEN="$ISSUE_WRITE_TOKEN"` prefix from the `gh pr edit` command
3. All `gh` commands now consistently use `GH_TOKEN: ${{ steps.app-token.outputs.token }}` from the step's env

### Expected Behavior After Fix

When viewing an auto-approved spec PR:
- **Approver**: Should show the GitHub App identity (e.g., "app-name[bot]" or the custom app name)
- **Label applied by**: Should show the same GitHub App identity
- **Workflow triggered by**: Should show the GitHub App identity in the goose-build workflow dispatch

### Risk Assessment

**Risk Level**: Low

**Justification**:
- Change is isolated to authentication token usage
- No logic changes to validation or approval criteria
- GitHub App already has all required permissions
- Change simplifies the code (removes token switching)

**Rollback Plan**:
If issues arise, revert the single-line change to restore `GH_TOKEN="$ISSUE_WRITE_TOKEN"` for the label operation.
