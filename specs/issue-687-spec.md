# Implementation Spec: Issue #687

## Summary

Fix the scheduled scan job in `.github/workflows/spec-auto-approve.yml` to approve PRs using the GitHub App token (`issueGithub`) instead of the default GitHub client (`github`). The current implementation approves PRs as `github-actions[bot]` instead of the GitHub App identity, causing inconsistent actor attribution between the event-driven and scheduled approval paths.

## Context

The `.github/workflows/spec-auto-approve.yml` workflow has two code paths for auto-approving spec PRs:

1. **Event-driven path** (`validate` job): Runs immediately when a spec PR is opened/edited, uses `gh pr review` CLI with the GitHub App token
2. **Scheduled scan path** (`scan` job): Runs every 5 minutes to catch PRs blocked by the actor-approval gate, uses GitHub JavaScript API via `actions/github-script@v7`

The workflow generates a GitHub App token using `actions/create-github-app-token@v1` and stores it in `steps.app-token.outputs.token`. Both paths receive this token, but they use it differently:

- Event-driven path: Explicitly sets `GH_TOKEN: ${{ steps.app-token.outputs.token }}` for `gh pr review`
- Scheduled scan path: Passes the App token to `actions/github-script@v7` via `github-token` input, creating a primary `github` client

The scheduled scan job also creates a secondary `issueGithub` client using `GITHUB_TOKEN` (the `github-actions` bot) for labeling:

```javascript
const issueGithub = new github.constructor({
  auth: process.env.ISSUE_WRITE_TOKEN,
});
```

Currently, the scheduled scan uses the `github` client (App token) for creating the review approval, but uses the `issueGithub` client (github-actions bot) for adding labels. This creates an inconsistency where the approval appears to come from different actors depending on which code path is taken.

## Requirements

1. Use the GitHub App token (`github` client) for both PR approval and labeling operations in the scheduled scan job
2. Remove the redundant `issueGithub` client and `ISSUE_WRITE_TOKEN` environment variable
3. Ensure both approval paths (event-driven and scheduled) use the same GitHub App identity
4. Maintain the same approval comment text and workflow behavior
5. Ensure the fix applies only to the scheduled scan job (`scan` job)

## Acceptance Criteria

- [ ] The `scan` job uses `github.rest.issues.addLabels()` instead of `issueGithub.rest.issues.addLabels()`
- [ ] The `ISSUE_WRITE_TOKEN` environment variable is removed from the `scan` job
- [ ] The `issueGithub` client instantiation is removed from the JavaScript script
- [ ] Workflow permissions include `issues: write` (already present, required for labeling)
- [ ] Test run shows PR approval by the GitHub App (e.g., `wgmesh-bot[bot]` or the configured App name) instead of `github-actions[bot]`
- [ ] No changes to the event-driven `validate` job (already correct)
- [ ] Approval comment, label functionality, and Goose triggering remain unchanged

## Out of scope

- Changes to the event-driven `validate` job (already uses GitHub App token correctly via `gh pr review`)
- Changes to the validation logic or checks
- Changes to the approval comment text
- Changes to the `goose-build.yml` workflow triggering logic
- Changes to workflow permissions beyond documenting existing `issues: write` and `pull-requests: write`
