# Issue #694: Fix spec-auto-approve approves as github-actions[bot] instead of GitHub App identity

## Summary

The spec-auto-approve workflow currently approves spec PRs using the GitHub Actions bot identity (`github-actions[bot]`) instead of the GitHub App identity (`wgmesh-bot`). This causes a problem where the workflow appears to be approving PRs on its own behalf rather than as a distinct application identity, which contradicts the design intent and may cause confusion in audit logs and approval tracking.

The issue occurs because the workflow uses `secrets.GITHUB_TOKEN` for the `gh pr review` command, which always executes as `github-actions[bot]`, even though a GitHub App token is generated earlier in the workflow and used for other operations.

## Context

### Current Architecture

The spec-auto-approve workflow (`.github/workflows/spec-auto-approve.yml`) has two operational paths:

1. **Event-driven path**: Triggered by `pull_request_target` events when spec PRs are opened/edited
2. **Scheduled scan path**: Runs every 5 minutes to catch spec PRs blocked by GitHub's actor-approval gate

Both paths:
- Generate a GitHub App token using `actions/create-github-app-token@v1` with APP_ID and APP_PRIVATE_KEY
- Validate spec files against required sections
- Approve PRs using `gh pr review` command
- Add the `approved-for-build` label
- Trigger the goose-build workflow

### The Bug

In the event-driven path (validate job), the approval step uses:

```yaml
env:
  GH_TOKEN: ${{ steps.app-token.outputs.token }}  # GitHub App token
  ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}  # GitHub Actions token
run: |
  gh pr review $PR_NUM --approve --body "..."
  GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build
```

The `gh pr review` command inherits `GH_TOKEN` from the environment, which is set to the GitHub App token. However, the `gh` CLI tool may be falling back to `secrets.GITHUB_TOKEN` internally, or there may be an environment variable ordering issue causing the wrong token to be used.

The scheduled scan path correctly uses the GitHub App token via the `actions/github-script@v7` action:

```javascript
await github.rest.pulls.createReview({
  owner, repo,
  pull_number: pr.number,
  event: 'APPROVE',
  body: '...',
});
```

This suggests the issue is specific to the `gh` CLI usage in the event-driven path.

### Related Systems

The auto-merge workflow (`.github/workflows/auto-merge.yml`) follows a similar pattern but uses the GitHub App token consistently for all operations, including PR approval. It successfully approves as `wgmesh-bot`.

## Requirements

### Functional Requirements

1. **Correct Approval Identity**: The spec-auto-approve workflow must approve spec PRs using the GitHub App identity (`wgmesh-bot`), not the GitHub Actions bot identity.

2. **Consistent Token Usage**: All GitHub API operations (PR approval, label addition, workflow dispatch) within the spec-auto-approve workflow must use the GitHub App token.

3. **Backward Compatibility**: The fix must not break existing functionality, including:
   - Spec validation logic
   - Label operations
   - Goose workflow triggering
   - Agent metrics collection

### Non-Functional Requirements

1. **Minimal Changes**: The fix should require the smallest possible code change to reduce risk.

2. **Audit Trail**: After the fix, all approvals in the PR review history should show `wgmesh-bot` as the approver.

3. **Testing**: The fix must be testable through manual verification of the approval identity.

## Acceptance Criteria

1. **Event-driven path**: When a spec PR is opened/edited and passes validation, the approval comment shows `wgmesh-bot` as the author.

2. **Scheduled scan path**: When the scheduled scan validates and approves a spec PR, the approval comment shows `wgmesh-bot` as the author.

3. **No regressions**: All existing functionality remains intact:
   - Spec validation logic unchanged
   - Label operations work correctly
   - Goose workflow triggers successfully
   - Agent metrics upload successfully

4. **Verification**: A test spec PR can be created and the approval identity verified in the PR review timeline.

## Out of Scope

1. **GitHub App Configuration**: Changes to the GitHub App installation, permissions, or credentials are out of scope.

2. **Workflow Logic Changes**: The spec validation logic and approval criteria are not in scope.

3. **Other Workflows**: Changes to other workflows (e.g., auto-merge.yml) are out of scope unless they share the same issue.

4. **GitHub Actions Bot Identity**: Eliminating all uses of `github-actions[bot]` in the repository is out of scope; only the spec-auto-approve approval identity is in scope.

5. **Token Generation Mechanism**: Changing how the GitHub App token is generated is out of scope; the issue is with token usage, not generation.
