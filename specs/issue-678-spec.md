# Specification: Issue #678

## Summary

Fix the spec auto-approve workflow (`.github/workflows/spec-auto-approve.yml`) to correctly approve pull requests as the GitHub App actor instead of as `github-actions`. The workflow currently uses the app token for API calls but the approval is being attributed to the wrong actor.

## Context

The spec auto-approve workflow was introduced in issue #664 to automatically validate and approve specification PRs that meet certain criteria. The workflow uses `actions/create-github-app-token@v1` to generate an app token, which should allow approvals to be attributed to the GitHub App rather than the generic `github-actions` bot.

### Current Behavior

When the workflow approves a PR, the approval appears to come from `github-actions` rather than the GitHub App identity. This causes:

1. **Confusing PR history**: Developers see approvals from the generic GitHub Actions bot instead of a recognizable application identity
2. **Audit trail issues**: The approval actor doesn't reflect the actual automated system that performed the approval
3. **Trust problems**: Teams may not trust approvals from `github-actions` as much as app-specific approvals

### Relevant Code

The workflow file at `.github/workflows/spec-auto-approve.yml` contains two jobs:

1. **`validate` job**: Event-driven path that runs on PR open/edit
2. **`scan` job**: Scheduled scan that catches blocked PRs

Both jobs use the same pattern:
```yaml
- name: Generate app token
  id: app-token
  uses: actions/create-github-app-token@v1
  with:
    app-id: ${{ vars.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}
```

Then later use the token:
```yaml
env:
  GH_TOKEN: ${{ steps.app-token.outputs.token }}
run: |
  gh pr review $PR_NUM --approve --body "..."
```

### Root Cause Analysis

The issue likely stems from one of these problems:

1. **Token usage inconsistency**: The `GH_TOKEN` environment variable may not be consistently passed to all `gh` CLI invocations
2. **Separate authentication contexts**: The workflow uses `GH_TOKEN` (app token) for some operations and `GITHUB_TOKEN` (default) for others, causing mixed actor attribution
3. **API vs CLI authentication**: The `gh` CLI may be falling back to default authentication when the app token is not properly passed
4. **Scheduled job API client**: The `scan` job creates a separate `github` constructor with `GITHUB_TOKEN` instead of using the app token

## Requirements

### Functional Requirements

1. **App token consistency**: All approval and PR modification operations must use the GitHub App token
2. **Correct actor attribution**: PR approvals must appear as coming from the GitHub App, not `github-actions`
3. **No breaking changes**: The workflow must continue to function correctly (validation, approval, labeling, Goose triggering)

### Technical Requirements

1. **Event-driven path (validate job)**:
   - Ensure `gh pr review` uses app token via `GH_TOKEN`
   - Ensure `gh pr edit` (labeling) uses app token via `GH_TOKEN`
   - Ensure `gh workflow run` (Goose trigger) uses app token via `GH_TOKEN`

2. **Scheduled path (scan job)**:
   - The `github-script` action must use the app token for both `github` and `issueGithub` clients
   - Currently: `github` uses app token, `issueGithub` uses `GITHUB_TOKEN`
   - Fix: Both should use the app token

3. **Verification method**:
   - Add debug logging to show which actor is being used
   - Test with a real PR to verify approval actor attribution

## Acceptance Criteria

1. **Event-driven approval**: When a spec PR is approved via the `validate` job, the approval appears in the PR timeline as coming from the GitHub App (e.g., "wgmesh-bot[bot]" or the configured app name)

2. **Scheduled approval**: When a spec PR is approved via the `scan` job, the approval appears as coming from the GitHub App

3. **All operations use app token**: All `gh` CLI commands and GitHub API calls that modify PRs (review, label, comment) must use the app token

4. **Workflow continues to work**: 
   - Validation checks still pass correctly
   - Approvals are still posted
   - Labels are still added
   - Goose build workflow is still triggered

5. **No regressions**: Existing functionality (validation failure comments, metrics collection) must continue to work

## Out of Scope

1. **GitHub App configuration**: Setting up the GitHub App, APP_ID, or APP_PRIVATE_KEY is outside the scope of this fix
2. **Workflow logic changes**: The validation logic (what makes a spec valid) must not be modified
3. **Goose build workflow**: Changes to `.github/workflows/goose-build.yml` are not in scope
4. **Metric collection**: The agent metrics collection logic does not need to change
5. **Other workflows**: Fixing similar issues in other workflow files is not in scope (should be separate issues if they exist)
