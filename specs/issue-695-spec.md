# Issue #695: fix(ci): spec-auto-approve approves as github-actions, not self (app token)

## Summary

Fix the `spec-auto-approve.yml` workflow to use the GitHub App token for PR approval operations instead of the default `GITHUB_TOKEN` (which acts as `github-actions[bot]`). Currently, when the workflow approves a spec PR, the approval appears to come from `github-actions[bot]` instead of the GitHub App identity, which can cause confusion and may not properly represent the intended approver identity.

## Context

The `spec-auto-approve.yml` workflow validates and auto-approves spec PRs that pass all checks. It uses two different tokens:

1. **GitHub App token** (`steps.app-token.outputs.token`): Generated via `actions/create-github-app-token@v1` using `APP_ID` and `APP_PRIVATE_KEY`
2. **Default GITHUB_TOKEN** (`secrets.GITHUB_TOKEN`): Automatically provided by GitHub Actions, acts as `github-actions[bot]`

In the "Auto-approve and trigger Goose" step, the workflow uses:
- `GH_TOKEN` (set to App token) for `gh pr review` approval
- `ISSUE_WRITE_TOKEN` (set to `secrets.GITHUB_TOKEN`) for label operations and workflow dispatch

The problem occurs because the `gh` CLI tool respects the `GH_TOKEN` environment variable, but there may be a configuration issue where the approval is actually being performed by `github-actions[bot]` instead of the GitHub App identity. This could be due to:

1. Token variable confusion in the `gh pr review` command
2. `gh` CLI defaulting to `GITHUB_TOKEN` when `GH_TOKEN` is not properly scoped
3. Race condition or token shadowing in the environment

Expected behavior: Approval should appear from the GitHub App (e.g., `wgmesh-bot[bot]` or the app's configured name).
Actual behavior: Approval appears from `github-actions[bot]`.

This issue is distinct from but related to the auto-merge workflow (`auto-merge.yml`), which correctly uses the App token for approvals.

## Requirements

### 1. Token Usage Audit and Fix

Verify and fix token usage in the `spec-auto-approve.yml` workflow:

**In the "Auto-approve and trigger Goose" step:**
- Ensure `gh pr review` uses the GitHub App token (via `GH_TOKEN` environment variable)
- Ensure all GitHub API operations that should represent the App identity use the App token
- Ensure `secrets.GITHUB_TOKEN` is only used for operations that intentionally should appear as `github-actions[bot]` (if any)

**Current code location:** `.github/workflows/spec-auto-approve.yml`, lines ~180-210

**Required fix:**
```yaml
- name: Auto-approve and trigger Goose
  if: steps.validate.outputs.valid == 'true'
  env:
    GH_TOKEN: ${{ steps.app-token.outputs.token }}  # App token for approval
    ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}  # Bot token for labels/dispatch
  run: |
    PR_NUM="${{ github.event.pull_request.number }}"

    echo "Auto-approving spec PR #$PR_NUM"

    # Approve the PR using GH_TOKEN (App token)
    gh pr review $PR_NUM --approve --body "## Auto-Approved ✅

    This spec PR passed all validation checks:

    - ✅ Spec file exists at \`specs/issue-${ISSUE_NUM}-spec.md\`
    - ✅ Required sections present (Classification, Problem Analysis, Proposed Approach)
    - ✅ No code changes detected
    - ✅ Classification is actionable

    Goose will now implement the code based on this specification."

    # Add the label using ISSUE_WRITE_TOKEN (github-actions bot)
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build

    # Trigger Goose using ISSUE_WRITE_TOKEN (or App token, depending on permission needs)
    GH_TOKEN="$ISSUE_WRITE_TOKEN" gh workflow run goose-build.yml \
      -f issue_number="${ISSUE_NUM}" \
      -f spec_pr_number="${PR_NUM}"

    echo "Goose implementation triggered"
```

### 2. Scheduled Scan Job Fix

The scheduled scan job (lines ~290+) also performs approvals. Ensure it uses the App token correctly:

**Current code location:** `.github/workflows/spec-auto-approve.yml`, `scan` job

**Required fix:** In the `actions/github-script@v7` step, the `github-token` input is already set to the App token. Verify that all `github.rest.pulls.createReview()` calls use this token (they should via the `github` object).

### 3. Verification Method

Add logging to confirm which identity is being used for approval:

**Add to "Auto-approve and trigger Goose" step:**
```bash
# Log which token/identity is being used
echo "App token endpoint: $(gh auth status)"
echo "GitHub App ID: ${{ vars.APP_ID }}"
```

## Acceptance Criteria

1. **Token configuration verified**: The workflow explicitly uses the GitHub App token for all approval operations
2. **Approval identity correct**: When a spec PR is auto-approved, the approval appears from the GitHub App (not `github-actions[bot]`)
3. **Both paths fixed**: Both the event-driven `validate` job and the scheduled `scan` job use the correct token
4. **Backward compatibility maintained**: Label operations and workflow dispatch still work correctly
5. **Logging added**: Sufficient logging exists to verify token usage in future debugging

### Testing Scenarios

1. **Event-driven approval**: Create a spec PR, verify approval appears from GitHub App
2. **Scheduled scan approval**: Create a spec PR that bypasses event trigger, verify approval appears from GitHub App
3. **Token verification**: Check workflow logs for explicit token identity confirmation
4. **Integration test**: Verify `approved-for-build` label is still added and Goose is still triggered

## Out of scope

1. **GitHub App configuration**: Changing the App ID, private key, or permissions
2. **Auto-merge workflow**: The `auto-merge.yml` workflow (already uses App token correctly)
3. **Validation logic**: Changing the spec validation checks (classification, sections, etc.)
4. **Goose build workflow**: Modifying how `goose-build.yml` is triggered
5. **Other workflows**: Fixing token usage in other workflow files unless they exhibit the same issue
