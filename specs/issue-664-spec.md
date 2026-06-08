# Issue #664: Fix CI - Spec Auto-Approve Approves as github-actions, Not Self (App Token)

## Summary

The `spec-auto-approve.yml` workflow currently approves spec PRs using `GITHUB_TOKEN` (the `github-actions` bot), but this creates inconsistent approval attribution. The workflow should approve using the generated GitHub App token from `actions/create-github-app-token@v1`, which provides consistent actor identity under the GitHub App's name (e.g., "wgmesh-ci[bot]") rather than the generic `github-actions` bot.

## Context

GitHub Actions provides two types of tokens:

1. **`GITHUB_TOKEN`**: Automatic token provided to every workflow run, authenticating as the `github-actions` bot
2. **GitHub App token**: Generated via `actions/create-github-app-token@v1`, authenticating as the configured GitHub App (e.g., "wgmesh-ci[bot]")

Current behavior in `.github/workflows/spec-auto-approve.yml`:
- The `validate` job (event-driven) correctly generates an App token in the `app-token` step
- The approval step uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}` for the `gh pr review` command
- However, the label addition uses `GH_TOKEN="$ISSUE_WRITE_TOKEN"` where `ISSUE_WRITE_TOKEN` is set to `${{ secrets.GITHUB_TOKEN }}`
- This means the approval may be attributed to `github-actions` instead of the GitHub App

Why this matters:
- Consistency: All spec PR approvals should come from the same actor (the GitHub App)
- Permissions: GitHub App tokens have explicit, scoped permissions that are easier to audit
- Clarity: In PR review timelines, it's clearer to see "wgmesh-ci[bot]" approved rather than "github-actions"

## Requirements

### Functional Requirements

1. **Event-driven path (`validate` job)**: Use the GitHub App token for ALL API interactions
   - `gh pr review` must use `steps.app-token.outputs.token`
   - `gh pr edit` (label addition) must use `steps.app-token.outputs.token`
   - `gh workflow run` (Goose trigger) can use either token (no permission requirement difference)

2. **Scheduled scan path (`scan` job)**: Verify consistent actor usage
   - The scan already uses `steps.app-token.outputs.token` for `github.rest.pulls.createReview`
   - The scan uses a separate `issueGithub` instance with `process.env.ISSUE_WRITE_TOKEN` for label addition
   - This should be unified to use the App token for consistency

3. **Permissions validation**: Ensure the GitHub App has required permissions
   - `pull-requests: write` (for approving PRs)
   - `contents: write` (for workflow dispatch, if needed)
   - `issues: write` (for adding labels)

### Non-Functional Requirements

1. **Backward compatibility**: The change must not break existing spec PR approval flow
2. **No behavior change**: Only the actor identity changes; validation logic remains identical
3. **Idempotency**: Re-running the workflow should not cause duplicate approvals or labels

## Acceptance Criteria

### AC1: Event-driven path uses App token for approval
- [ ] `gh pr review` command uses `GH_TOKEN` set to `${{ steps.app-token.outputs.token }}`
- [ ] No reference to `secrets.GITHUB_TOKEN` or `ISSUE_WRITE_TOKEN` in approval step
- [ ] Approval appears in PR timeline from GitHub App actor (e.g., "wgmesh-ci[bot]")

### AC2: Event-driven path uses App token for label addition
- [ ] `gh pr edit` command uses `GH_TOKEN` set to `${{ steps.app-token.outputs.token }}`
- [ ] Label addition performed by same actor as approval

### AC3: Scheduled scan path uses consistent token
- [ ] Both `github.rest.pulls.createReview` and label addition use the same token
- [ ] Prefer GitHub App token for all operations in the scan job

### AC4: Verification via test PR
- [ ] Create a test spec PR
- [ ] Verify approval actor is GitHub App, not `github-actions`
- [ ] Verify label is added by same actor
- [ ] Verify Goose build is triggered successfully

### AC5: Documentation updated
- [ ] Workflow comments updated to reflect App token usage
- [ ] No misleading references to `GITHUB_TOKEN` for approval operations

## Out of Scope

The following are explicitly out of scope for this fix:

1. **Validation logic changes**: No changes to spec validation checks (sections, file types, etc.)
2. **GitHub App configuration**: No changes to App ID, private key, or permissions setup
3. **Scheduled scan frequency**: No changes to cron schedule or scan logic
4. **Agent metrics**: No changes to metrics collection or upload
5. **Error handling**: No changes to failure notification or error messaging
6. **Goose build workflow**: No changes to `goose-build.yml` or its trigger mechanism

## Affected Files

- `.github/workflows/spec-auto-approve.yml` (both `validate` and `scan` jobs)

## Test Strategy

1. **Unit verification**: Manual inspection of workflow YAML to ensure correct token usage
2. **Integration test**: Create a test spec PR and observe approval actor in GitHub UI
3. **Regression test**: Verify existing approved spec PRs show consistent actor history
4. **Permission test**: Ensure GitHub App token has all required scopes

## Estimated Complexity

**Low** - This is a straightforward variable substitution change with no logic modifications.

Estimated effort: 1-2 hours including testing.
