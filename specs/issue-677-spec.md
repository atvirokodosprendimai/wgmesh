# Issue #677 - Implementation Spec

## Summary

Resolve conflicting specifications (issues #664, #675, #676) regarding the `spec-auto-approve.yml` workflow's approval actor identity. The workflow currently approves spec PRs using the GitHub App token (`steps.app-token.outputs.token`), which appears as `github-actions[bot]` in the PR timeline. The fix should ensure approvals appear as coming from the GitHub App itself (e.g., "wgmesh-ci[bot]"), providing consistent actor identity across all workflow operations.

## Context

The `spec-auto-approve.yml` workflow validates and auto-approves specification PRs to trigger Goose implementation. The workflow has been refined through three previous specs (#664, #675, #676), but their approaches conflict:

- **#664**: Proposed using GitHub App token for all operations (approval, labeling, workflow dispatch)
- **#675**: Proposed using `GITHUB_TOKEN` for approvals to preserve triggering user identity
- **#676**: Proposed ensuring approvals appear as GitHub App self-approvals

Current state (lines 165-189 in `.github/workflows/spec-auto-approve.yml`):
```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}  # ← GitHub App token
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}  # ← GITHUB_TOKEN (github-actions bot)
  run: |
    gh pr review $PR_NUM --approve --body "..."      # Uses app token → github-actions[bot]
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit ...     # Uses GITHUB_TOKEN → github-actions[bot]
```

The core issue: When a GitHub App token generated via `actions/create-github-app-token@v1` is used with `gh pr review`, the approval appears as coming from `github-actions[bot]` instead of the GitHub App itself.

The GitHub App is configured with:
- `APP_ID` environment variable (via `vars.APP_ID`)
- `APP_PRIVATE_KEY` secret (via `secrets.APP_PRIVATE_KEY`)
- Permissions: `pull-requests: write`, `contents: write`, `issues: write`

## Requirements

### Functional Requirements

**FR-1: Consistent Actor Identity**
All spec PR auto-approvals MUST appear as coming from the GitHub App (e.g., "wgmesh-ci[bot]"), not from `github-actions[bot]`. This provides:
- Clear audit trail showing which automated system approved the PR
- Consistency with other automated operations (labeling, workflow dispatch)
- Distinction from human approvals and other bot operations

**FR-2: Approval Flow**
The auto-approve step must:
1. Use the GitHub App token for the `gh pr review` command
2. Apply the `approved-for-build` label using the same token
3. Trigger the `goose-build.yml` workflow via `gh workflow run`

**FR-3: Scheduled Scan Path**
The scheduled `scan` job (cron: every 5 minutes) must also use the GitHub App token consistently for:
- Creating reviews via `github.rest.pulls.createReview`
- Adding labels via the separate `issueGithub` instance

### Technical Requirements

**TR-1: Token Usage**
In the `validate` job:
- Replace `GH_TOKEN: ${{ steps.app-token.outputs.token }}` with a single environment variable approach
- Remove `ISSUE_WRITE_TOKEN` environment variable (use app token for all operations)
- Ensure `gh pr review` uses the GitHub App token

**TR-2: Scan Job Token Consistency**
In the `scan` job:
- Unify token usage to use GitHub App token for both review creation and labeling
- Remove the separate `issueGithub` instance that uses `GITHUB_TOKEN`

**TR-3: Verification**
After implementation, verify that:
- New spec PR approvals show the GitHub App as the actor (checkable via `gh pr view --json reviews`)
- The `approved-for-build` label is applied by the same actor
- The `goose-build.yml` workflow is triggered successfully

### Non-Functional Requirements

**NFR-1: Backward Compatibility**
- No changes to validation logic or checks
- No changes to approval comment body
- No changes to workflow triggers or schedule

**NFR-2: Idempotency**
- Re-running the workflow must not create duplicate approvals
- Re-running must not add duplicate labels

**NFR-3: No Breaking Changes**
- All existing functionality remains intact
- Downstream workflows (goose-build.yml) continue to work

## Acceptance Criteria

### AC-1: Validate Job Uses GitHub App Token for Approval
**Given**: A spec PR passes all validation checks in the `validate` job
**When**: The `Auto-approve and trigger Goose` step executes
**Then**: The `gh pr review` command uses the GitHub App token from `steps.app-token.outputs.token`
**Verification**: Inspect workflow YAML; no reference to `secrets.GITHUB_TOKEN` in approval step env

### AC-2: Label Addition Uses GitHub App Token
**Given**: A spec PR passes validation
**When**: The auto-approve step runs `gh pr edit --add-label approved-for-build`
**Then**: The command uses the GitHub App token (not `ISSUE_WRITE_TOKEN`)
**Verification**: Check that `GH_TOKEN` environment variable is set to app token for label command

### AC-3: Approval Actor is GitHub App
**Given**: A spec PR is auto-approved by the workflow
**When**: Viewing the PR review timeline in GitHub UI or via API
**Then**: The approval appears from the GitHub App (e.g., "wgmesh-ci[bot]"), not "github-actions[bot]"
**Verification**: Run `gh pr view <pr_num> --json reviews --jq '.reviews[] | {author: .author.login, state: .state}'` and confirm author is the GitHub App

### AC-4: Scan Job Uses Consistent Token
**Given**: The scheduled `scan` job runs and finds an unapproved spec PR
**When**: The job creates a review and adds a label
**Then**: Both operations use the GitHub App token
**Verification**: No separate `issueGithub` instance using `process.env.ISSUE_WRITE_TOKEN`

### AC-5: Goose Build Triggered Successfully
**Given**: A spec PR is auto-approved and labeled
**When**: The auto-approve step completes
**Then**: The `goose-build.yml` workflow is triggered with correct inputs
**Verification**: Check Actions tab for goose-build.yml run with matching issue_number and spec_pr_number

### AC-6: No Regression in Validation Logic
**Given**: A spec PR with validation failures (missing sections, code changes)
**When**: The spec-auto-approve.yml workflow runs
**Then**: The PR is NOT approved and a validation failure comment is posted
**Verification**: PR has no approval and has a comment starting with "## Spec Validation Failed ❌"

### AC-7: No Duplicate Approvals on Re-run
**Given**: A spec PR was already approved by the workflow
**When**: The workflow runs again (manual re-run or new commit)
**Then**: No duplicate approval is created
**Verification**: PR timeline shows only one approval from the GitHub App

### AC-8: Documentation Updated
**Given**: The fix is implemented
**When**: Reading the workflow YAML
**Then**: Comments accurately reflect the token usage and approval actor
**Verification**: Workflow comments mention "GitHub App token" and "approves as the GitHub App"

## Out of Scope

The following are explicitly out of scope for this fix:

1. **Validation Logic**: No changes to spec validation checks (sections, file types, code changes)
2. **GitHub App Configuration**: No changes to App ID, private key, or permissions in repository settings
3. **Workflow Triggers**: No changes to when the workflow runs (pull_request_target, schedule, workflow_dispatch)
4. **Approval Comment Content**: No changes to the body text of auto-approval reviews
5. **Downstream Workflows**: No modifications to goose-build.yml, approve-build.yml, auto-merge.yml, etc.
6. **Scheduled Scan Frequency**: No changes to cron schedule (currently every 5 minutes)
7. **Error Handling**: No changes to failure notifications or error messages
8. **Metrics Collection**: No changes to agent metrics upload
9. **PR Comment on Failure**: No changes to validation failure comment posting logic

## Affected Files

- `.github/workflows/spec-auto-approve.yml` (jobs: `validate` and `scan`)

## Test Strategy

### Pre-Implementation Testing
1. **Baseline Verification**: Create a test spec PR and confirm current behavior (approval shows `github-actions[bot]`)
2. **Token Inspection**: Document current token usage in both validate and scan jobs

### Post-Implementation Testing
1. **Integration Test**: Create a new spec PR with valid structure and verify:
   - Approval appears from GitHub App (not github-actions[bot])
   - Label is added by GitHub App
   - Goose build workflow triggers successfully

2. **Regression Test**: Create invalid spec PRs (missing sections, code changes) and verify:
   - No approval is created
   - Validation failure comment is posted
   - No label is added

3. **Idempotency Test**: Re-run the workflow on an already-approved PR and verify:
   - No duplicate approval
   - No duplicate label
   - No error in workflow logs

4. **Scan Job Test**: Wait for scheduled scan or manually dispatch and verify:
   - Unapproved spec PRs are approved by GitHub App
   - Labels are added by GitHub App
   - No duplicate approvals on subsequent scans

5. **GitHub UI Verification**: Check the PR timeline in GitHub web UI to confirm actor identity visually

## Estimated Complexity

**Low to Medium** - The fix involves straightforward token usage changes but requires careful attention to:
- Ensuring consistent token usage across both validate and scan jobs
- Verifying the GitHub App has all required permissions
- Testing both approval paths (event-driven and scheduled scan)

Estimated effort: 2-4 hours including testing and verification.

## Notes

### GitHub Token Types Reference

1. **`GITHUB_TOKEN`**: Automatic token, authenticates as `github-actions[bot]`
   - Available as `secrets.GITHUB_TOKEN`
   - Permissions set in workflow `permissions` block
   - Does NOT preserve triggering user identity in pull_request_target events

2. **GitHub App Token**: Generated via `actions/create-github-app-token@v1`
   - Authenticates as the GitHub App (e.g., "wgmesh-ci[bot]")
   - Permissions explicitly granted during App installation
   - Consistent actor identity across all operations

### Why GitHub App Token for Approval

Using the GitHub App token for approvals provides:
- **Consistency**: All automated operations (approval, labeling, workflow dispatch) from the same actor
- **Auditability**: Clear record that the CI system (not a human) approved the spec
- **Distinction**: Separates automated approvals from human reviews and other bot activities
- **Stability**: GitHub App identity is stable and controlled by the repository maintainers

### Related Issues

This spec resolves the conflicts in:
- #664: Spec auto-approve approves as github-actions, not self (app token)
- #675: Fix CI: spec-auto-approve approves as github-actions, not self (app token)
- #676: Fix CI: spec-auto-approve approves as github-actions, not self (app token)

All three issues address the same underlying problem with different proposed solutions.
