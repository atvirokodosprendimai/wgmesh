# Issue #675 - Fix CI: spec-auto-approve approves as github-actions, not self (app token)

## Summary

The spec-auto-approve workflow currently approves pull requests as the `github-actions` bot instead of the user who triggered the workflow. This occurs because the approval step uses the GitHub App token (for authentication to the repository) instead of the `GITHUB_TOKEN` which preserves the actor identity. This specification addresses the token usage to ensure approvals appear as coming from the triggering user.

## Context

The `spec-auto-approve.yml` workflow validates and auto-approves spec PRs to trigger Goose implementation. The workflow generates a GitHub App token to bypass GitHub's actor-approval gate for `pull_request_target` events, but incorrectly uses this same token for the approval action.

Current behavior:
- The workflow uses `actions/create-github-app-token@v1` to generate an app token
- The `gh pr review` command approves PRs using the app token
- Reviews appear as coming from `github-actions[bot]` instead of the triggering user

The workflow already has `GITHUB_TOKEN` available via `secrets.GITHUB_TOKEN`, which preserves the triggering actor's identity. The labeling step correctly uses `ISSUE_WRITE_TOKEN` (aliased to `GITHUB_TOKEN`), but the review step does not.

## Requirements

The fix must ensure that:
1. PR approvals in the `validate` job appear as coming from the triggering user
2. The GitHub App token continues to be used for repository access (checkout, listing PRs)
3. The `GITHUB_TOKEN` is used for the approval action
4. Both `pull_request_target` and scheduled `scan` job paths maintain correct token usage
5. No breaking changes to workflow functionality or permissions

## Acceptance Criteria

1. **Approval uses correct token**: The `gh pr review` command uses `GITHUB_TOKEN` instead of the app token
2. **Appropriate permission handling**: The `GITHUB_TOKEN` has `pull-requests: write` permission (already configured)
3. **Both paths fixed**: Both the `validate` job (event-driven) and `scan` job (scheduled) use correct tokens for approvals
4. **Backward compatibility**: All existing workflow functionality remains intact
5. **Testing verification**: Manual verification confirms approvals appear as the triggering user

## Out of scope

- Changes to other workflows (e.g., `auto-merge.yml`, `approve-build.yml`)
- Modifications to the GitHub App token generation or usage for other steps
- Changes to workflow permissions or secrets configuration
- Modifications to the validation logic or checks
- Changes to the scheduled scan job's GitHub API usage (which correctly uses `issueGithub` for labeling)
