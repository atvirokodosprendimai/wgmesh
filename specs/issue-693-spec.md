# Implementation Spec: Issue #693

## Summary

Fix the spec-auto-approve workflow to approve PRs using the GitHub App token instead of the default `GITHUB_TOKEN`. The current implementation approves PRs as `github-actions[bot]` instead of the GitHub App identity, causing inconsistent actor attribution and potential permission issues.

## Context

The `.github/workflows/spec-auto-approve.yml` workflow automatically approves spec PRs that pass validation checks. Currently, the approval step (`gh pr review`) uses the default `GH_TOKEN` environment variable which defaults to `GITHUB_TOKEN` (the `github-actions` bot), while the label-editing step explicitly overrides `GH_TOKEN` with `ISSUE_WRITE_TOKEN` (also `GITHUB_TOKEN`).

However, the workflow generates a GitHub App token using `actions/create-github-app-token@v1` and stores it in `steps.app-token.outputs.token`. The workflow should use this App token for both the approval and labeling operations to ensure consistent actor identity and proper permissions.

The scheduled scan job correctly uses the GitHub App token via the `github-script` action's `github-token` input, but the event-driven `validate` job has this inconsistency.

## Requirements

1. Use the GitHub App token (`${{ steps.app-token.outputs.token }}`) for both PR approval and labeling operations
2. Remove the redundant `ISSUE_WRITE_TOKEN` environment variable since the App token has the necessary permissions
3. Ensure the App token has `pull-requests: write` permission (already configured in workflow permissions)
4. Maintain the same approval comment text and workflow behavior
5. Ensure the fix applies to both the event-driven `validate` job

## Acceptance Criteria

- [ ] The `Auto-approve and trigger Goose` step uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}` for the `gh pr review` command
- [ ] The label-editing command uses the same App token (either via env or explicit `GH_TOKEN=` override)
- [ ] `ISSUE_WRITE_TOKEN` environment variable is removed from the job
- [ ] Workflow permissions include `pull-requests: write` (already present)
- [ ] Test run shows PR approval by the GitHub App (e.g., `wgmesh-bot[bot]` or the configured App name) instead of `github-actions[bot]`
- [ ] No changes to the scheduled scan job (already correct)
- [ ] Approval comment and label functionality remain unchanged

## Out of scope

- Changes to the scheduled scan job (already uses GitHub App token correctly)
- Changes to the validation logic or checks
- Changes to the approval comment text
- Changes to the `goose-build.yml` workflow triggering logic
- Changes to workflow permissions beyond documenting existing `pull-requests: write`
