# Issue #679 - Fix CI: spec-auto-approve approves as github-actions, not app token identity

## Summary

The `spec-auto-approve.yml` workflow is inconsistently using authentication tokens, causing PR approvals to be submitted under the `github-actions[bot]` identity instead of the GitHub App token's identity. This breaks approval policies and confuses the approval attribution in the UI.

## Context

The `spec-auto-approve.yml` workflow has two paths for validating and approving spec PRs:

1. **Event-driven path** (`pull_request_target`): Triggers immediately when a spec PR is opened/edited
2. **Scheduled scan path** (`schedule`): Runs every 5 minutes to catch PRs blocked by GitHub's actor-approval gate

Both paths generate a GitHub App token using `actions/create-github-app-token@v1`, which should provide a consistent identity for approvals (e.g., `wgmesh-bot[bot]` or the app's configured name).

However, the workflow incorrectly mixes two different tokens:

- **App token** (`steps.app-token.outputs.token`): Generated from the GitHub App with proper identity
- **GitHub Actions token** (`secrets.GITHUB_TOKEN`): Always identifies as `github-actions[bot]`

In the event-driven path, the workflow uses the app token for the approval (`gh pr review`) but switches to `GITHUB_TOKEN` for adding the label. This creates an inconsistent actor identity. The scheduled scan path creates a second GitHub client (`issueGithub`) using `GITHUB_TOKEN` for label operations, further compounding the confusion.

**Current problematic code (event-driven path):**

```yaml
- name: Auto-approve and trigger Goose
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    # Approve with app token
    gh pr review $PR_NUM --approve --body "..."

    # Add label with github-actions token - WRONG
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build
```

This inconsistency causes:
- Approvals showing as from `github-actions[bot]` instead of the app identity
- Confusing UI attribution (mixed actors in the approval timeline)
- Potential policy violations if approval rules require the app identity
- Inconsistent actor identity between approval and label operations

## Requirements

The fix must ensure that:

1. **All operations use the app token**: Both PR approval and label operations must use the GitHub App token (`steps.app-token.outputs.token`)

2. **Remove token switching**: Eliminate the `ISSUE_WRITE_TOKEN` environment variable and all `GH_TOKEN="$ISSUE_WRITE_TOKEN"` overrides

3. **Fix both workflow paths**: Ensure consistency in both the event-driven path and the scheduled scan path

4. **Preserve all functionality**: The workflow must continue to:
   - Validate spec files
   - Post approval comments
   - Add `approved-for-build` label
   - Trigger `goose-build.yml` workflow

5. **Maintain permissions**: Keep existing permission scopes (pull-requests: write, contents: write, issues: write)

## Acceptance Criteria

- [ ] All PR operations (review/approval, label edit, comment) use the GitHub App token
- [ ] No references to `secrets.GITHUB_TOKEN` in approval/label operations
- [ ] Event-driven path: Approval and label both show the app identity (not `github-actions[bot]`)
- [ ] Scheduled scan path: Approval and label both show the app identity
- [ ] Workflow continues to pass all validation checks
- [ ] `goose-build.yml` trigger still works correctly
- [ ] No regressions in functionality (validation, metrics, error handling)

## Out of scope

- Changing the approval validation logic or checks
- Modifying the scheduled scan frequency or triggers
- Changing the `goose-build.yml` workflow or its inputs
- Adjusting repository permissions or app configurations
- Modifying agent metrics collection logic
- Changing the spec validation criteria (sections, classification, etc.)

## Affected Files

- `.github/workflows/spec-auto-approve.yml`:
  - Event-driven path: `validate` job, "Auto-approve and trigger Goose" step
  - Scheduled scan path: `scan` job, GitHub script step

## Implementation Notes

### Event-driven path fix

Replace the mixed token usage with consistent app token usage:

**Before:**
```yaml
- name: Auto-approve and trigger Goose
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    ISSUE_NUM: ${{ steps.issue.outputs.number }}
  run: |
    PR_NUM="${{ github.event.pull_request.number }}"
    gh pr review $PR_NUM --approve --body "..."
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build
    gh workflow run goose-build.yml ...
```

**After:**
```yaml
- name: Auto-approve and trigger Goose
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}
    ISSUE_NUM: ${{ steps.issue.outputs.number }}
  run: |
    PR_NUM="${{ github.event.pull_request.number }}"
    gh pr review $PR_NUM --approve --body "..."
    gh pr edit $PR_NUM --add-label approved-for-build
    gh workflow run goose-build.yml ...
```

### Scheduled scan path fix

Remove the second `issueGithub` client that uses `GITHUB_TOKEN`. Use the app token client for all operations.

**Before:**
```javascript
const issueGithub = new github.constructor({
  auth: process.env.ISSUE_WRITE_TOKEN,
});
// ...
await issueGithub.rest.issues.addLabels({
  owner, repo,
  issue_number: pr.number,
  labels: ['approved-for-build'],
});
```

**After:**
```javascript
// No secondary client needed
await github.rest.issues.addLabels({
  owner, repo,
  issue_number: pr.number,
  labels: ['approved-for-build'],
});
```

Remove the `env.ISSUE_WRITE_TOKEN` from the `with:` block and the step's `env:` section.

### Testing approach

1. Create a test spec PR that passes all validation checks
2. Verify the approval appears under the app identity (not `github-actions[bot]`)
3. Verify the label is added by the same identity
4. Confirm `goose-build.yml` is triggered successfully
5. Check the approval timeline shows consistent actor identity
