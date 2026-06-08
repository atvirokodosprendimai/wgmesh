# Issue #678 - Fix CI: Resolve Conflicting Spec-Auto-Approve Actor Identity Specs

## Summary

This specification resolves the conflicting approaches in issues #664, #675, #676, and #677 regarding the `spec-auto-approve.yml` workflow's approval actor identity. After analysis, the root cause is that GitHub App tokens generated via `actions/create-github-app-token@v1` are already correctly authenticating as the GitHub App itself, but visual inconsistency in the GitHub UI and audit logs may make approvals appear as `github-actions[bot]` under certain conditions. The fix ensures consistent use of GitHub App tokens throughout the workflow and clarifies the expected actor identity behavior.

## Context

The `spec-auto-approve.yml` workflow validates specification PRs and auto-approves them to trigger Goose implementation. Four previous specifications have addressed the same underlying issue with conflicting proposals:

- **#664**: Proposed using GitHub App token for all operations to ensure "wgmesh-ci[bot]" actor
- **#675**: Proposed using `GITHUB_TOKEN` for approvals to preserve triggering user identity
- **#676**: Proposed ensuring approvals appear as GitHub App self-approvals
- **#677**: Attempted to resolve conflicts but still relied on GitHub App token for approvals

Current state in `.github/workflows/spec-auto-approve.yml` (validate job, lines 165-189):

```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}  # GitHub App token
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}  # GITHUB_TOKEN (github-actions bot)
  run: |
    gh pr review $PR_NUM --approve --body "..."      # Uses GH_TOKEN (app token)
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit ...     # Uses ISSUE_WRITE_TOKEN (GITHUB_TOKEN)
    gh workflow run goose-build.yml ...              # Uses GH_TOKEN (app token)
```

**Root Cause Analysis:**

The workflow currently uses two different tokens:
1. GitHub App token (`steps.app-token.outputs.token`) for PR review and workflow dispatch
2. `GITHUB_TOKEN` (`secrets.GITHUB_TOKEN`) for label addition via `gh pr edit`

This creates inconsistency in the audit trail:
- Approval appears from GitHub App (e.g., "wgmesh-ci[bot]")
- Label addition appears from `github-actions[bot]`

**Why GitHub App Token is Correct:**

When `actions/create-github-app-token@v1` generates a token, it authenticates API requests as the GitHub App itself. This is the intended behavior:
- The GitHub App (identified by `APP_ID`) is the actor for all automated operations
- This provides clear audit trail showing the CI system approved the PR
- This is distinct from human approvals and other bot operations

**The Real Problem:**

The inconsistency comes from using `GITHUB_TOKEN` for label addition while using the GitHub App token for approval. Both should use the GitHub App token for consistency.

## Requirements

### Functional Requirements

**FR-1: Consistent Actor Identity**
All automated operations in spec-auto-approve.yml MUST use the GitHub App token, ensuring:
- PR review approval appears from GitHub App
- Label addition appears from GitHub App
- Workflow dispatch appears from GitHub App
- Audit trail shows consistent actor throughout

**FR-2: Remove Mixed Token Usage**
The workflow must NOT mix GitHub App token with `GITHUB_TOKEN` for automated operations:
- Remove `ISSUE_WRITE_TOKEN` environment variable
- Use GitHub App token for `gh pr review`
- Use GitHub App token for `gh pr edit` (label addition)
- Use GitHub App token for `gh workflow run` (already correct)

**FR-3: Scheduled Scan Consistency**
The scheduled `scan` job must also use GitHub App token consistently:
- Remove the separate `issueGithub` instance that uses `GITHUB_TOKEN`
- Use the GitHub App token for both review creation and labeling

### Technical Requirements

**TR-1: Token Usage in validate Job**
- Set `GH_TOKEN` to `steps.app-token.outputs.token` for all commands
- Remove `ISSUE_WRITE_TOKEN` environment variable entirely
- Remove inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` overrides

**TR-2: Token Usage in scan Job**
- Remove the `ISSUE_WRITE_TOKEN` environment variable from the `github-script` step
- Remove the separate `issueGithub` instance construction
- Use the primary `github` object (authenticated with GitHub App token) for all API calls

**TR-3: Permissions Verification**
Ensure the GitHub App has all required permissions:
- `pull-requests: write` (for approving PRs and adding labels)
- `contents: write` (for workflow dispatch)
- `issues: write` (for adding labels to PRs, which are issue objects)

### Non-Functional Requirements

**NFR-1: No Validation Changes**
- No changes to spec validation logic or checks
- No changes to approval comment body
- No changes to failure notification text

**NFR-2: Backward Compatibility**
- All existing workflow functionality remains intact
- Downstream workflows (goose-build.yml) continue to work
- No changes to when the workflow runs

**NFR-3: Idempotency**
- Re-running the workflow must not create duplicate approvals
- Re-running must not add duplicate labels

## Acceptance Criteria

### AC-1: Validate Job Uses GitHub App Token Exclusively
**Given**: A spec PR passes all validation checks in the `validate` job
**When**: The `Auto-approve and trigger Goose` step executes
**Then**: All commands use the GitHub App token from `steps.app-token.outputs.token`
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
- No `ISSUE_WRITE_TOKEN` environment variable in the github-script step
- No separate `issueGithub` instance construction
- All API calls use the primary `github` object

### AC-5: Goose Build Triggered Successfully
**Given**: A spec PR is auto-approved and labeled
**When**: The auto-approve step completes
**Then**: The `goose-build.yml` workflow is triggered with correct inputs
**Verification**: Check Actions tab for goose-build.yml run with matching `issue_number` and `spec_pr_number`

### AC-6: No Regression in Validation Logic
**Given**: A spec PR with validation failures (missing sections, code changes)
**When**: The spec-auto-approve.yml workflow runs
**Then**: The PR is NOT approved and a validation failure comment is posted
**Verification**: 
- PR has no approval
- PR has a comment starting with "## Spec Validation Failed ❌"
- No `approved-for-build` label is added

### AC-7: No Duplicate Approvals on Re-run
**Given**: A spec PR was already approved by the workflow
**When**: The workflow runs again (manual re-run or new commit)
**Then**: No duplicate approval is created and no duplicate label is added
**Verification**: PR timeline shows only one approval and one label addition event

### AC-8: Documentation and Comments Updated
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
9. **Related Issue Specs**: No modification to specs/issue-664-spec.md, specs/issue-675-spec.md, specs/issue-676-spec.md, or specs/issue-677-spec.md (leave them as-is for historical reference)

## Affected Files

- `.github/workflows/spec-auto-approve.yml` (both `validate` and `scan` jobs)

## Test Strategy

### Pre-Implementation Testing
1. **Baseline Verification**: Create a test spec PR and document current behavior:
   - Approval actor (should be GitHub App or github-actions[bot])
   - Label addition actor (should be github-actions[bot])
   - Note the inconsistency

2. **Token Usage Documentation**: Document current token usage in both validate and scan jobs

### Post-Implementation Testing

1. **Integration Test - Valid Spec PR**:
   - Create a new spec PR with valid structure
   - Verify approval appears from GitHub App
   - Verify label is added by GitHub App
   - Verify Goose build workflow triggers successfully
   - Check all three operations show same actor in timeline

2. **Regression Test - Invalid Spec PRs**:
   - Create spec PR with missing sections → verify no approval, validation comment posted
   - Create spec PR with code changes → verify no approval, validation comment posted
   - Create spec PR with non-actionable classification → verify no approval, validation comment posted

3. **Idempotency Test**:
   - Create a valid spec PR and let it be approved
   - Manually re-run the workflow
   - Verify no duplicate approval
   - Verify no duplicate label
   - Verify workflow completes without errors

4. **Scan Job Test**:
   - Create a valid spec PR but prevent event-driven approval (e.g., close PR before workflow runs)
   - Wait for scheduled scan or manually dispatch the scan job
   - Verify approval appears from GitHub App
   - Verify label is added by GitHub App
   - Verify no duplicate on subsequent scan runs

5. **GitHub UI Verification**:
   - Check the PR timeline in GitHub web UI
   - Visually confirm actor identity is consistent
   - Confirm both approval and label addition show same bot icon and name

6. **API Verification**:
   ```bash
   # Check approval actor
   gh pr view <pr_num> --json reviews --jq '.reviews[] | {author: .author.login, state: .state}'
   
   # Check label addition actor
   gh pr view <pr_num> --json timelineEvents --jq '.timelineEvents[] | select(.event == "labeled") | {actor: .actor.login, label: .label.name}'
   ```

## Estimated Complexity

**Low** - This is primarily a cleanup task to remove token inconsistency. The GitHub App token is already being used for most operations; the fix removes the unnecessary `GITHUB_TOKEN` usage for label addition.

The changes are:
1. Remove `ISSUE_WRITE_TOKEN` environment variable from validate job
2. Remove inline `GH_TOKEN="$ISSUE_WRITE_TOKEN"` override in validate job
3. Remove `ISSUE_WRITE_TOKEN` environment variable from scan job
4. Remove separate `issueGithub` instance in scan job

Estimated effort: 1-2 hours including testing and verification.

## Implementation Notes

### Why Not Use GITHUB_TOKEN for Approvals

Using `GITHUB_TOKEN` (which authenticates as `github-actions[bot]`) would create inconsistency with other automated workflows and would not provide the clear audit trail that the GitHub App provides. The GitHub App identity is:
- Configurable and controlled by repository maintainers
- Distinct from other GitHub Actions operations
- More auditable (explicit permissions granted during installation)

### GitHub App Token Behavior

When `actions/create-github-app-token@v1` generates a token:
- The token authenticates as the GitHub App itself
- API calls using this token show the GitHub App as the actor
- This is consistent across REST API, GraphQL API, and `gh` CLI commands
- The actor format is: `<app-name>[bot]` (e.g., "wgmesh-ci[bot]")

### Resolving the Conflicting Specs

This spec (#678) supersedes the conflicting approaches in:
- **#664**: Correctly identified GitHub App token should be used, but didn't address the `GITHUB_TOKEN` usage for labeling
- **#675**: Incorrectly suggested using `GITHUB_TOKEN` for approvals; this would create MORE inconsistency
- **#676**: Correctly identified the need for GitHub App self-approval, but didn't fully resolve token mixing
- **#677**: Attempted to resolve conflicts but still had mixed token usage

The correct solution is to use GitHub App token consistently throughout, not mix tokens.

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
```
