# Issue #680 - Implementation Spec

## Summary

This specification consolidates and resolves the conflicting approaches from issues #664, #675, #676, #677, and #678 regarding the `spec-auto-approve.yml` workflow's approval actor identity. After comprehensive analysis, the workflow should use GitHub App tokens consistently for all operations (approval, labeling, workflow dispatch) to ensure all automated actions appear as coming from the GitHub App (e.g., "wgmesh-ci[bot]"). The inconsistency stems from mixing GitHub App token usage with `GITHUB_TOKEN` usage within the same workflow.

## Context

The `spec-auto-approve.yml` workflow validates specification PRs and auto-approves them to trigger Goose implementation. Multiple specifications have addressed the same underlying issue with conflicting proposals:

- **#664 (2025-12-20)**: Proposed using GitHub App token for all operations to ensure "wgmesh-ci[bot]" actor identity
- **#675 (2025-12-21)**: Proposed using `GITHUB_TOKEN` for approvals to preserve triggering user identity
- **#676 (2025-12-22)**: Proposed ensuring approvals appear as GitHub App self-approvals
- **#677 (2025-12-23)**: Attempted to resolve conflicts but still had mixed token usage
- **#678 (2025-12-24)**: Recognized the root cause as token mixing but maintained conflicting approaches

### Current State Analysis

The workflow currently uses two different tokens in the `validate` job:

```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}           # GitHub App token
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}           # GITHUB_TOKEN (github-actions bot)
  run: |
    gh pr review $PR_NUM --approve --body "..."              # Uses GH_TOKEN (app token)
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit ...             # Uses ISSUE_WRITE_TOKEN (GITHUB_TOKEN)
    gh workflow run goose-build.yml ...                      # Uses GH_TOKEN (app token)
```

The `scan` job also has token inconsistency:
- Uses `github.rest.pulls.createReview` with GitHub App token
- Uses separate `issueGithub` instance (authenticated with `GITHUB_TOKEN`) for labeling

### Root Cause

The inconsistency in approval actor identity stems from:
1. **Mixed token usage**: Using GitHub App token for some operations and `GITHUB_TOKEN` for others
2. **Inconsistent audit trail**: Approvals and labels appear from different actors
3. **API vs CLI behavior**: `gh` CLI commands and REST API calls may handle token attribution differently

### Why GitHub App Token is Correct

GitHub App tokens generated via `actions/create-github-app-token@v1` provide:
- **Consistent actor identity**: All operations authenticate as the GitHub App itself
- **Clear audit trail**: Shows "wgmesh-ci[bot]" (or configured app name) consistently
- **Explicit permissions**: App permissions are granted during installation and easily audited
- **Separation of concerns**: Distinct from human approvals and other GitHub Actions operations

## Requirements

### Functional Requirements

**FR-1: Unified Actor Identity**
All automated operations in `spec-auto-approve.yml` MUST use the GitHub App token, ensuring:
- PR review approvals appear from GitHub App
- Label additions appear from GitHub App  
- Workflow dispatches appear from GitHub App
- All operations in both `validate` and `scan` jobs use the same actor

**FR-2: Eliminate Mixed Token Usage**
The workflow must NOT use `GITHUB_TOKEN` for any approval or labeling operations:
- Remove all references to `ISSUE_WRITE_TOKEN` environment variable
- Remove all inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` overrides
- Use GitHub App token for all `gh` CLI commands and REST API calls

**FR-3: Consistent Behavior Across Paths**
Both approval paths must use identical token strategy:
- Event-driven path (`validate` job): GitHub App token for all operations
- Scheduled scan path (`scan` job): GitHub App token for all operations

### Technical Requirements

**TR-1: Validate Job Token Usage**
In the `validate` job (`Auto-approve and trigger Goose` step):
- Set `GH_TOKEN` environment variable to `steps.app-token.outputs.token`
- Remove `ISSUE_WRITE_TOKEN` environment variable entirely
- Remove inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` override for `gh pr edit`
- Ensure all `gh` commands use the default `GH_TOKEN` (GitHub App token)

**TR-2: Scan Job Token Usage**
In the `scan` job (github-script step):
- Remove `ISSUE_WRITE_TOKEN` from environment variables
- Remove separate `issueGithub` instance construction
- Use the primary `github` object (authenticated with GitHub App token) for all API calls
- Ensure both review creation and labeling use the same `github` object

**TR-3: Permissions Verification**
Ensure the GitHub App has all required permissions:
- `pull-requests: write` (for approving PRs and adding labels)
- `contents: write` (for workflow dispatch)
- `issues: write` (for adding labels to PRs, which are issue objects)

### Non-Functional Requirements

**NFR-1: No Validation Logic Changes**
- No changes to spec validation checks (sections, file types, code changes, classification)
- No changes to approval comment body text
- No changes to failure notification text

**NFR-2: Backward Compatibility**
- All existing workflow functionality remains intact
- Downstream workflows (goose-build.yml) continue to work correctly
- No changes to workflow triggers or schedule

**NFR-3: Idempotency**
- Re-running the workflow must not create duplicate approvals
- Re-running must not add duplicate labels
- Workflow must handle already-approved PRs gracefully

## Acceptance Criteria

### AC-1: Validate Job Uses GitHub App Token Exclusively
**Given**: A spec PR passes all validation checks in the `validate` job
**When**: The `Auto-approve and trigger Goose` step executes
**Then**: All operations use the GitHub App token from `steps.app-token.outputs.token`
**Verification**:
- No `ISSUE_WRITE_TOKEN` environment variable in the step
- No inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` overrides
- All `gh` commands use default `GH_TOKEN` environment variable

### AC-2: Approval Actor is GitHub App
**Given**: A spec PR is auto-approved by the workflow
**When**: Viewing the PR review timeline in GitHub UI or via API
**Then**: The approval appears from the GitHub App (e.g., "wgmesh-ci[bot]")
**Verification**: Run `gh pr view <pr_num> --json reviews --jq '.reviews[] | {author: .author.login, state: .state}'` and confirm author is the GitHub App (format: `<app-name>[bot]`)

### AC-3: Label Addition Actor Matches Approval Actor
**Given**: A spec PR passes validation and is auto-approved
**When**: The `gh pr edit --add-label approved-for-build` command executes
**Then**: The label addition appears from the same GitHub App actor that approved the PR
**Verification**: Check PR timeline events; both approval and label addition show same author

### AC-4: Scan Job Uses GitHub App Token Exclusively
**Given**: The scheduled `scan` job runs and finds an unapproved spec PR
**When**: The job creates a review and adds a label via GitHub API
**Then**: Both operations use the GitHub App token via the `github` object
**Verification**:
- No `ISSUE_WRITE_TOKEN` environment variable in github-script step
- No separate `issueGithub` instance construction
- All API calls use the primary `github` object

### AC-5: Both Paths Show Consistent Actor
**Given**: Two spec PRs are approved via different paths
**When**: PR #1 is approved by event-driven `validate` job and PR #2 is approved by scheduled `scan` job
**Then**: Both approvals appear from the same GitHub App actor
**Verification**: Compare review authors for both PRs; both show the GitHub App

### AC-6: Goose Build Triggered Successfully
**Given**: A spec PR is auto-approved and labeled
**When**: The auto-approve step completes
**Then**: The `goose-build.yml` workflow is triggered with correct inputs
**Verification**: Check Actions tab for goose-build.yml run with matching `issue_number` and `spec_pr_number`

### AC-7: No Regression in Validation Logic
**Given**: A spec PR with validation failures (missing sections, code changes)
**When**: The spec-auto-approve.yml workflow runs
**Then**: The PR is NOT approved and a validation failure comment is posted
**Verification**:
- PR has no approval
- PR has a comment starting with "## Spec Validation Failed ❌"
- No `approved-for-build` label is added

### AC-8: No Duplicate Approvals on Re-run
**Given**: A spec PR was already approved by the workflow
**When**: The workflow runs again (manual re-run or new commit)
**Then**: No duplicate approval is created and no duplicate label is added
**Verification**: PR timeline shows only one approval and one label addition event

### AC-9: Documentation and Comments Updated
**Given**: The fix is implemented
**When**: Reading the workflow YAML
**Then**: Comments accurately reflect the GitHub App token usage and consistent actor identity
**Verification**: Workflow comments mention "GitHub App token" and clarify that all operations use the same actor

## Out of Scope

The following are explicitly out of scope for this fix:

1. **GitHub App Configuration**: No changes to App ID, private key, or permissions in repository/org settings
2. **Validation Logic**: No changes to spec validation checks (sections, file types, code changes, classification)
3. **Workflow Triggers**: No changes to when the workflow runs (pull_request_target, schedule, workflow_dispatch)
4. **Approval Comment Content**: No changes to the body text of auto-approval reviews
5. **Downstream Workflows**: No modifications to goose-build.yml, approve-build.yml, auto-merge.yml, etc.
6. **Scheduled Scan Frequency**: No changes to cron schedule (currently every 5 minutes)
7. **Error Handling**: No changes to failure notifications, error messages, or validation output format
8. **Metrics Collection**: No changes to agent metrics upload or format
9. **Historical Specs**: No modification to specs/issue-664-spec.md, specs/issue-675-spec.md, specs/issue-676-spec.md, specs/issue-677-spec.md, or specs/issue-678-spec.md (leave them as-is for historical reference)
10. **GitHub App Creation**: No changes to how the GitHub App is created or installed

## Affected Files

- `.github/workflows/spec-auto-approve.yml` (both `validate` and `scan` jobs)

## Test Strategy

### Pre-Implementation Testing

1. **Baseline Verification**:
   - Create a test spec PR and document current behavior
   - Record approval actor (expected: GitHub App or github-actions[bot])
   - Record label addition actor (expected: github-actions[bot] if using ISSUE_WRITE_TOKEN)
   - Document the inconsistency

2. **Token Usage Audit**:
   - Document all current token usage in both validate and scan jobs
   - List all environment variables that reference tokens
   - Identify all mixed token usage patterns

### Post-Implementation Testing

1. **Integration Test - Valid Spec PR (Event-Driven Path)**:
   - Create a new spec PR with valid structure
   - Verify approval appears from GitHub App (not github-actions[bot])
   - Verify label is added by GitHub App (same actor as approval)
   - Verify Goose build workflow triggers successfully
   - Check all three operations show same actor in timeline

2. **Integration Test - Valid Spec PR (Scheduled Scan Path)**:
   - Create a valid spec PR but prevent event-driven approval (e.g., disable validate job)
   - Wait for scheduled scan or manually dispatch the scan job
   - Verify approval appears from GitHub App
   - Verify label is added by GitHub App (same actor as approval)
   - Verify no duplicate on subsequent scan runs

3. **Regression Test - Invalid Spec PRs**:
   - Create spec PR with missing sections → verify no approval, validation comment posted
   - Create spec PR with code changes → verify no approval, validation comment posted
   - Create spec PR with non-actionable classification → verify no approval, validation comment posted

4. **Idempotency Test**:
   - Create a valid spec PR and let it be approved
   - Manually re-run the workflow
   - Verify no duplicate approval
   - Verify no duplicate label
   - Verify workflow completes without errors

5. **Cross-Path Consistency Test**:
   - Approve PR #1 via event-driven path (validate job)
   - Approve PR #2 via scheduled scan path (scan job)
   - Compare approval actors via GitHub API
   - Verify both show the same GitHub App actor

6. **GitHub UI Verification**:
   - Check the PR timeline in GitHub web UI
   - Visually confirm actor identity is consistent
   - Confirm both approval and label addition show same bot icon and name

7. **API Verification**:
   ```bash
   # Check approval actor
   gh pr view <pr_num> --json reviews --jq '.reviews[] | {author: .author.login, state: .state}'
   
   # Check label addition actor
   gh pr view <pr_num> --json timelineEvents --jq '.timelineEvents[] | select(.event == "labeled") | {actor: .actor.login, label: .label.name}'
   
   # Check workflow runs
   gh run list --workflow=spec-auto-approve.yml --json status,conclusion,headBranch,displayTitle
   ```

## Estimated Complexity

**Low** - This is primarily a cleanup task to remove token inconsistency. The GitHub App token is already being used for most operations; the fix removes the unnecessary `GITHUB_TOKEN` usage for label addition and review creation in the scan job.

The changes are:
1. Remove `ISSUE_WRITE_TOKEN` environment variable from validate job (1 location)
2. Remove inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` override in validate job (1 location)
3. Remove `ISSUE_WRITE_TOKEN` environment variable from scan job (1 location)
4. Remove separate `issueGithub` instance in scan job (1 location)

Estimated effort: 1-2 hours including testing and verification.

## Implementation Notes

### Why Not Use GITHUB_TOKEN for Approvals

Using `GITHUB_TOKEN` (which authenticates as `github-actions[bot]`) would create inconsistency with other automated workflows and would not provide the clear audit trail that the GitHub App provides. The GitHub App identity is:
- Configurable and controlled by repository maintainers
- Distinct from other GitHub Actions operations
- More auditable (explicit permissions granted during installation)
- Consistent across all automated operations

### GitHub App Token Behavior

When `actions/create-github-app-token@v1` generates a token:
- The token authenticates as the GitHub App itself
- API calls using this token show the GitHub App as the actor
- This is consistent across REST API, GraphQL API, and `gh` CLI commands
- The actor format is: `<app-name>[bot]` (e.g., "wgmesh-ci[bot]")

### Resolving the Conflicting Specs

This spec (#680) supersedes and consolidates the conflicting approaches in:
- **#664**: Correctly identified GitHub App token should be used, but didn't fully address token mixing
- **#675**: Incorrectly suggested using `GITHUB_TOKEN` for approvals; this would create MORE inconsistency
- **#676**: Correctly identified the need for GitHub App self-approval, but didn't fully resolve token mixing
- **#677**: Attempted to resolve conflicts but still had mixed token usage
- **#678**: Recognized token mixing as root cause but maintained conflicting approaches

The correct solution is to use GitHub App token consistently throughout, eliminating all token mixing.

### Testing Commands

Useful commands for testing:
```bash
# View PR reviews with actor info
gh pr view <pr_num> --json reviews --jq '.reviews[] | {author: .author.login, state: .state, body: .body}'

# View PR timeline events (including label additions)
gh pr view <pr_num> --json timelineEvents --jq '.timelineEvents[] | select(.event == "labeled" or .event == "reviewed") | {event: .event, actor: .actor.login}'

# Check workflow run status
gh run list --workflow=spec-auto-approve.yml --json status,conclusion,headBranch,displayTitle

# Re-trigger workflow manually
gh workflow run spec-auto-approve.yml -f issue_number=<num> -f spec_pr_number=<pr_num>

# Check GitHub App installation
gh api /repos/atvirokodosprendimai/wgmesh/installations
```

### Security Considerations

- GitHub App tokens are short-lived (typically 1 hour expiration)
- GitHub App tokens have explicit, scoped permissions
- GitHub App tokens are more auditable than `GITHUB_TOKEN`
- GitHub App tokens provide better separation of concerns than `GITHUB_TOKEN`

### Rollback Plan

If issues arise after implementation:
1. Revert the workflow YAML changes
2. Restore the previous mixed token usage
3. Document the issues encountered
4. Re-evaluate the approach

The changes are minimal and easily reversible.
