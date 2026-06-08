# Specification: Issue #681

## Classification

fix

## Problem Analysis

The `.github/workflows/spec-auto-approve.yml` workflow has inconsistent token usage that causes spec PR approvals to be attributed to the `github-actions` bot instead of the GitHub App actor (e.g., "wgmesh-ci[bot]"). This breaks approval attribution consistency and makes audit trails unclear.

Current issues in `.github/workflows/spec-auto-approve.yml`:

1. **Line 169**: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - creates a variable pointing to the generic `GITHUB_TOKEN`
2. **Line 189**: `GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build` - uses `GITHUB_TOKEN` for label addition instead of the GitHub App token
3. **Line 264**: `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - uses `GITHUB_TOKEN` for failure comments
4. **Line 299**: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - creates another `GITHUB_TOKEN` reference in the scheduled scan job
5. **Line 304**: `auth: process.env.ISSUE_WRITE_TOKEN` - uses `GITHUB_TOKEN`-derived auth for label operations in the scan

While the approval itself (line 180) correctly uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}`, the label addition and other operations use `GITHUB_TOKEN`, creating mixed actor attribution.

## Proposed Approach

Replace all `GITHUB_TOKEN` and `ISSUE_WRITE_TOKEN` references with the GitHub App token (`${{ steps.app-token.outputs.token }}`) for consistency. This ensures all operations (approval, labeling, comments, workflow dispatch) are performed by the same GitHub App actor.

The change is minimal: variable substitutions only, no logic changes.

## Implementation Tasks

### Task 1: Update event-driven job token usage
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Replace `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` with a single App token variable, and update all references
- **Detail:**
  - Remove line 169: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
  - Replace line 189 environment variable from `GH_TOKEN="$ISSUE_WRITE_TOKEN"` to `GH_TOKEN: ${{ steps.app-token.outputs.token }}` (using YAML syntax)
  - Replace line 264 `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` with `GH_TOKEN: ${{ steps.app-token.outputs.token }}`
  - Ensure all steps in the `validate` job use `steps.app-token.outputs.token` for API operations

### Task 2: Update scheduled scan job token usage
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Remove `ISSUE_WRITE_TOKEN` and use App token for all operations
- **Detail:**
  - Remove line 299: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
  - Remove the separate `issueGithub` instance construction (line 302-304)
  - Use the existing `github` context (already authenticated with App token) for all operations including label addition
  - Change `await issueGithub.rest.issues.addLabels` to `await github.rest.issues.addLabels`

### Task 3: Update workflow comments
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Update inline comments to reflect App token usage
- **Detail:**
  - Review all comments mentioning `GITHUB_TOKEN` or token usage
  - Ensure comments accurately describe that all operations use the GitHub App token

### Task 4: Verify permissions
- **File:** `.github/workflows/spec-auto-approve.yml` (verify only)
- **What:** Confirm GitHub App token has required permissions
- **Detail:**
  - Verify `permissions:` block includes `pull-requests: write`, `contents: write`, `issues: write`
  - These are already present and sufficient; no changes needed

## Affected Files

```
.github/workflows/spec-auto-approve.yml  (modify: token usage in validate and scan jobs)
```

## Acceptance Criteria

- `gh pr review` uses `steps.app-token.outputs.token` (unchanged, already correct)
- `gh pr edit` for label addition uses `steps.app-token.outputs.token` (changed from `GITHUB_TOKEN`)
- `gh pr comment` for failure notifications uses `steps.app-token.outputs.token` (changed from `GITHUB_TOKEN`)
- Scheduled scan job uses single GitHub token for all operations (removed `issueGithub` instance)
- No references to `secrets.GITHUB_TOKEN` remain in the file
- No references to `ISSUE_WRITE_TOKEN` remain in the file
- Create a test spec PR and verify approval, label, and comment all appear from GitHub App actor (e.g., "wgmesh-ci[bot]")
- Verify Goose build workflow is triggered successfully after approval

## Estimated Complexity

low (1-2 files, <100 lines)
