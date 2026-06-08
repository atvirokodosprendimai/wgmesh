# Specification: Issue #675

## Classification

fix

## Problem Analysis

The `spec-auto-approve.yml` workflow is currently configured to approve pull requests using a GitHub App token (`steps.app-token.outputs.token`), which generates approvals on behalf of `github-actions[bot]`. The correct behavior is to approve as the GitHub App itself (e.g., `wgmesh-bot` or the app identity specified by `APP_ID`).

### Current Behavior
In `spec-auto-approve.yml`, the "Auto-approve and trigger Goose" step uses:
```yaml
env:
  GH_TOKEN: ${{ steps.app-token.outputs.token }}
```
This causes approvals to appear as coming from `github-actions[bot]`.

### Expected Behavior
Approvals should be attributed to the GitHub App identity (`wgmesh-bot`), not `github-actions[bot]`. This provides:
- Clear attribution that the approval came from the automated system
- Consistency with the documented design intent (see comments in `auto-merge.yml`)
- Distinct identity for audit purposes

### Affected Code Paths
1. **Event-driven path** (`validate` job): Immediate approval when a spec PR is opened/edited
2. **Scheduled scan path** (`scan` job): Approval when cron catches PRs blocked by actor-approval gate

Both paths currently use `github-script@v7` with the app token but need to verify the approval attribution.

## Proposed Approach

The fix ensures that GitHub REST API calls create reviews using the GitHub App token, which attributes the approval to the app identity rather than `github-actions[bot]`. 

Two implementation strategies:
1. **Via `gh` CLI**: Use `gh pr review` with `GH_TOKEN` set to the app token (currently used but may not respect app identity)
2. **Via `github-script@v7`**: Use `github.rest.pulls.createReview()` with `github-token: ${{ steps.app-token.outputs.token }}` (more reliable for app attribution)

The implementation will update both the event-driven and scheduled approval paths to use the `github-script` method, ensuring consistent app identity attribution.

## Implementation Tasks

### Task 1: Replace gh CLI approval with github-script (event-driven path)
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Convert the "Auto-approve and trigger Goose" step from bash/gh CLI to github-script action
- **Detail:** 
  1. Replace the bash `run:` block with `uses: actions/github-script@v7`
  2. Pass `github-token: ${{ steps.app-token.outputs.token }}`
  3. Convert approval logic to JavaScript:
     - Call `github.rest.pulls.createReview()` with `event: 'APPROVE'`
     - Add `approved-for-build` label using `issueGithub` (with `GITHUB_TOKEN` for issue write permission)
     - Trigger `goose-build.yml` via `github.rest.actions.createWorkflowDispatch()`
  4. Preserve the exact same approval body text

### Task 2: Update scheduled scan approval (if needed)
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Verify that the `scan` job's `github.rest.pulls.createReview()` call already uses the app token correctly
- **Detail:**
  1. The `scan` job already uses `github-script@v7` with `github-token: ${{ steps.app-token.outputs.token }}`
  2. Verify line ~320 uses `await github.rest.pulls.createReview()` (not gh CLI)
  3. If already correct, no changes needed. If using gh CLI, convert to github-script method

### Task 3: Add verification logging
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Add logging to confirm approval actor identity
- **Detail:**
  1. In both approval paths, add `core.info()` logging before `createReview()` call
  2. Log: `Approving PR #N as GitHub App (APP_ID: ${APP_ID})`
  3. This aids debugging and confirms the fix works

### Task 4: Update workflow documentation comments
- **File:** `.github/workflows/spec-auto-approve.yml` (modify)
- **What:** Clarify that approvals are made by the GitHub App, not github-actions[bot]
- **Detail:**
  1. Update the top-of-file comment to explicitly state: "Uses GitHub App (wgmesh-bot) for approvals, ensuring distinct identity from github-actions[bot]"
  2. Add inline comment at the approval step: "GitHub App token ensures approval attributed to app identity, not github-actions[bot]"

## Affected Files

```
.github/workflows/spec-auto-approve.yml  (modify: convert gh CLI to github-script for approval)
```

## Acceptance Criteria

- `go build ./...` passes (no syntax errors in modified YAML)
- Workflow runs successfully on test PR
- Approval appears in PR timeline as coming from `wgmesh-bot` (GitHub App identity), NOT `github-actions[bot]`
- Approval body text matches original format (preserved)
- `approved-for-build` label is added correctly
- `goose-build.yml` is triggered successfully with correct inputs
- Scheduled scan path also approves as GitHub App identity

## Estimated Complexity

low (1 file, ~40 lines of JavaScript conversion, external deps: none - uses existing actions)