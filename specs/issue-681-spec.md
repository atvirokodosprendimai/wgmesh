# Specification: Issue #681

## Summary

Fix the spec-auto-approve.yml workflow to approve pull requests using the GitHub App token instead of the GitHub Actions token. Currently, approvals appear as coming from "github-actions[bot]" instead of the GitHub App's identity, which breaks approval workflows and audit trails.

## Context

The `spec-auto-approve.yml` workflow validates spec PRs and auto-approves them to trigger Goose implementation. It generates a GitHub App token but incorrectly uses the GitHub Actions token (`GITHUB_TOKEN`) for approving PRs in two places:

1. **validate job** (line ~177): Uses `GH_TOKEN="$ISSUE_WRITE_TOKEN"` where `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
2. **scan job** (line ~412): Uses `issueGithub.rest.pulls.createReview()` where `issueGithub` is authenticated with `process.env.ISSUE_WRITE_TOKEN` (the GitHub Actions token)

The workflow already generates a proper GitHub App token at `steps.app-token.outputs.token` but doesn't use it for approvals.

### Current Behavior

When the workflow approves a PR, the approval appears as:
- **Author**: `github-actions[bot]`
- **Source**: GitHub Actions system token

### Expected Behavior

When the workflow approves a PR, the approval should appear as:
- **Author**: The GitHub App's configured identity (e.g., "wgmesh-bot[bot]")
- **Source**: GitHub App token with proper permissions

### Why This Matters

1. **Audit trail**: Approvals should clearly indicate they came from the automated App, not the generic Actions bot
2. **Approval consistency**: Using the App token ensures approvals from all workflow jobs use the same identity
3. **Trust**: PR authors and reviewers can distinguish between automated App approvals and manual Actions-triggered events
4. **Debugging**: Easier to identify which automation approved a PR in the approval timeline

## Requirements

### Functional Requirements

1. **Validate job approval** (line ~177):
   - Use `GH_TOKEN: ${{ steps.app-token.outputs.token }}` instead of `GH_TOKEN="$ISSUE_WRITE_TOKEN"`
   - Remove the shell variable substitution approach
   - Pass the app token directly to the `gh pr review` command

2. **Scan job approval** (line ~412):
   - Use `github.rest.pulls.createReview()` instead of `issueGithub.rest.pulls.createReview()`
   - Remove the separate `issueGithub` client instance
   - Approve using the same `github` client that's already authenticated with the app token

3. **Labeling** (line ~191 in validate job):
   - Keep using `GH_TOKEN="$ISSUE_WRITE_TOKEN"` for the `gh pr edit --add-label` command
   - This is correct because labeling uses the Actions token to avoid permission conflicts
   - Only the approval operation needs to change

### Technical Requirements

1. Maintain existing permission structure (App token for approvals, Actions token for labeling)
2. Preserve all validation logic and check outputs
3. Keep the approval body text and format unchanged
4. Maintain the scheduled scan and event-driven trigger behavior
5. Ensure agent metrics collection continues to work

## Acceptance Criteria

### Given: A spec PR is opened or the scheduled scan runs

### When: The PR passes all validation checks

### Then:
1. ✅ The PR is approved
2. ✅ The approval author is the GitHub App (e.g., "wgmesh-bot[bot]"), NOT "github-actions[bot]"
3. ✅ The approval appears in the PR's review timeline with correct authorship
4. ✅ The `approved-for-build` label is added (still using Actions token)
5. ✅ The goose-build.yml workflow is triggered with correct parameters
6. ✅ Agent metrics are collected and uploaded

### Verification Steps

1. **Test validate job (event-driven path)**:
   ```bash
   # Create a valid spec PR (e.g., via Copilot)
   # Verify the workflow triggers on pull_request_target
   # Check the approval author in the PR timeline
   # Should show: "wgmesh-bot[bot] approved this"
   # NOT: "github-actions[bot] approved this"
   ```

2. **Test scan job (scheduled path)**:
   ```bash
   # Manually trigger the workflow_dispatch
   # Or wait for the 5-minute cron
   # Check approvals from the scan job
   # Should show: "wgmesh-bot[bot] approved this"
   # NOT: "github-actions[bot] approved this"
   ```

3. **Verify approval text unchanged**:
   ```bash
   # Check the approval comment body
   # Should contain "Auto-Approved ✅" with the same checklist format
   ```

4. **Verify labeling still works**:
   ```bash
   # Check that the "approved-for-build" label is added
   # This should use the Actions token (github-actions[bot])
   ```

### Test Scenarios

| Scenario | Trigger | Current Approval Actor | Expected Approval Actor |
|----------|---------|------------------------|------------------------|
| Valid spec PR opened | pull_request_target | github-actions[bot] | wgmesh-bot[bot] |
| Spec PR edited | pull_request_target (edited) | github-actions[bot] | wgmesh-bot[bot] |
| Scheduled scan catches PR | schedule (cron) | github-actions[bot] | wgmesh-bot[bot] |
| Manual workflow dispatch | workflow_dispatch | github-actions[bot] | wgmesh-bot[bot] |

## Out of Scope

- Changes to validation logic (keep all existing checks)
- Changes to the approval body text or format
- Changes to the scheduled scan frequency or trigger conditions
- Changes to agent metrics collection
- Changes to the goose-build.yml workflow trigger
- Changes to GitHub App permissions or scope
- Modifications to other workflows that use GITHUB_TOKEN
- Changes to the Actions token usage for labeling operations
