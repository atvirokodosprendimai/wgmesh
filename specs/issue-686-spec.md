# Specification: Issue #686

## Summary
Fix the spec auto-approval workflow to approve PRs as the GitHub App itself (not as `github-actions` user) when using GitHub App authentication tokens.

## Context

The `spec-auto-approve.yml` workflow auto-approves spec PRs that pass validation checks. Currently, there are TWO approval paths in this workflow:

### Current Implementation Details

**Path 1: Event-driven validation (lines 168-195)**
- Uses GitHub App token (`steps.app-token.outputs.token`)
- Generated via `actions/create-github-app-token@v1` with `APP_ID` and `APP_PRIVATE_KEY`
- Uses `gh pr review` CLI command to approve
- **Problem**: The `gh` CLI shows the approval as coming from the `github-actions` user, not the App itself

**Path 2: Scheduled scan validation (lines 283-437)**
- Uses GitHub App token (`steps.app-token.outputs.token`) 
- Generated via `actions/create-github-app-token@v1` with `APP_ID` and `APP_PRIVATE_KEY`
- Uses `github.rest.pulls.createReview()` API method
- **Correct**: This properly shows the App as the approver

### Root Cause Analysis

The issue stems from different token usage patterns:

1. **Event-driven path** uses the GitHub App token but then uses the `gh` CLI for approval. The `gh` CLI tool, when invoked with an App token, displays the approval as coming from the `github-actions` user because it's being executed in a GitHub Actions context.

2. **Scheduled scan path** uses the same GitHub App token but invokes the GitHub REST API directly via `github.rest.pulls.createReview()`, which correctly attributes the approval to the App.

### Why This Matters

When a PR is approved by `github-actions` instead of the App:
- It's unclear which automation/approved the PR
- It creates inconsistency in the approval audit trail
- The scheduled scan path (which works correctly) creates a different user attribution
- It undermines the purpose of using GitHub App authentication (which provides clear identity)

### File Analysis

**`.github/workflows/spec-auto-approve.yml`**:

Lines 168-195 (event-driven approval):
```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}  # App token
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    gh pr review $PR_NUM --approve --body "..."  # Shows as github-actions
```

Lines 412-427 (scheduled scan approval):
```javascript
await github.rest.pulls.createReview({  // Shows as App
  owner, repo,
  pull_number: pr.number,
  event: 'APPROVE',
  body: [...]
});
```

The scheduled scan path already works correctly and should be used as the reference pattern.

## Requirements

### Functional Requirements

1. **Consistent App Identity**: The event-driven approval path must show the GitHub App as the approver, matching the behavior of the scheduled scan path.

2. **Minimal Change**: Modify only the approval mechanism in the event-driven path to use GitHub REST API instead of `gh` CLI, preserving all other functionality (labeling, comments, workflow triggering).

3. **Preserve All Functionality**: The fix must maintain:
   - Validation checks (all existing checks must remain)
   - Approval with detailed body message
   - Label addition (`approved-for-build`)
   - Goose build workflow triggering
   - Agent metrics collection
   - Error handling and failure comments

### Technical Requirements

1. **API Method**: Replace `gh pr review` CLI command with `github.rest.pulls.createReview()` API call.

2. **Token Usage**: Continue using GitHub App token (`steps.app-token.outputs.token`) for authentication.

3. **Workflow Structure**: Keep the existing step structure - only change the approval mechanism within the "Auto-approve and trigger Goose" step.

4. **JavaScript vs Bash**: The event-driven path currently uses bash/shell scripts. The fix needs to either:
   - Convert the approval step to use `actions/github-script@v7` (preferred, matches scheduled scan)
   - Or find a way to make `gh pr review` show the App as the actor (not recommended)

## Acceptance Criteria

1. **Event-driven approval shows App as actor**: When a spec PR is approved via the event-driven path, the review must show the GitHub App as the approver (not `github-actions`).

2. **Scheduled scan unchanged**: The scheduled scan path must continue to work exactly as it does now.

3. **All validation preserved**: All existing validation checks must remain functional:
   - Spec file exists at correct path
   - No code changes detected
   - Required sections present
   - Classification is actionable

4. **Approval message preserved**: The approval body message must contain the same information (checklist with ✅ marks).

5. **Labeling works**: PR must still be labeled with `approved-for-build`.

6. **Goose triggering works**: The `goose-build.yml` workflow must still be triggered with correct parameters.

7. **Error handling preserved**: Validation failures must still post comments explaining why approval failed.

8. **Metrics preserved**: Agent metrics collection must continue to work for both success and failure cases.

## Out of Scope

1. **Scheduled scan path**: The scheduled scan path (lines 283-437) already works correctly and should not be modified.

2. **App authentication setup**: GitHub App configuration (`APP_ID`, `APP_PRIVATE_KEY`) is not in scope.

3. **Validation logic**: The validation checks themselves are not changing - only the approval mechanism.

4. **Other workflows**: Changes are limited to `spec-auto-approve.yml` only.

5. **Label definitions**: No changes to `.github/labels.yml` or label creation logic.

6. **Goose build workflow**: No changes to `goose-build.yml` or how it receives parameters.

7. **PR review handler workflow**: No changes to `approve-build.yml` or manual approval flows.

8. **Token permissions**: No changes to workflow permissions (`pull-requests: write`, etc.).
