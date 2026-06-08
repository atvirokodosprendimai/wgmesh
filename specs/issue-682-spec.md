# Specification: Issue #682 - fix(ci): spec-auto-approve approves as github-actions, not self (app token)

## Summary

Fix the `spec-auto-approve.yml` GitHub Actions workflow to approve pull requests using the GitHub App token instead of the GitHub Actions token (`GITHUB_TOKEN`). Currently, approvals appear as coming from `github-actions[bot]` instead of the GitHub App's identity (e.g., `wgmesh-ci[bot]`), which breaks audit trail consistency and workflow actor attribution.

## Context

The `spec-auto-approve.yml` workflow validates spec PRs and auto-approves them to trigger Goose implementation. The workflow generates a GitHub App token using `actions/create-github-app-token@v1`, but incorrectly uses the GitHub Actions token (`GITHUB_TOKEN`) for approving PRs.

### Current Problem

In the `validate` job (event-driven path), the workflow:
1. ✅ Correctly generates an App token at `steps.app-token.outputs.token`
2. ❌ Uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}` for the `gh pr review` command
3. ❌ Uses `GH_TOKEN="$ISSUE_WRITE_TOKEN"` for the `gh pr edit --add-label` command, where `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`

In the `scan` job (scheduled path), the workflow:
1. ✅ Correctly generates an App token
2. ✅ Uses the App token for `github.rest.pulls.createReview()`
3. ❌ Uses a separate `issueGithub` client authenticated with `process.env.ISSUE_WRITE_TOKEN` (GitHub Actions token) for label addition

### Current Behavior

When the workflow approves a PR, the approval appears as:
- **Author**: `github-actions[bot]`
- **Source**: GitHub Actions system token (`GITHUB_TOKEN`)

### Expected Behavior

When the workflow approves a PR, the approval should appear as:
- **Author**: The GitHub App's configured identity (e.g., `wgmesh-ci[bot]`)
- **Source**: GitHub App token with explicit, scoped permissions

### Why This Matters

1. **Audit trail**: Approvals should clearly indicate they came from the automated GitHub App, not the generic Actions bot
2. **Consistency**: All spec PR approvals should use the same actor identity
3. **Permissions clarity**: GitHub App tokens have explicit, scoped permissions that are easier to audit than the broad `GITHUB_TOKEN`
4. **Debugging**: Easier to identify which automation approved a PR in the approval timeline

## Requirements

### Functional Requirements

#### FR1: Validate Job (Event-Driven Path)
Use the GitHub App token for ALL API interactions in the validate job:

1. **Approval operation** (line ~177):
   - Change from: `GH_TOKEN: ${{ steps.app-token.outputs.token }}`
   - Keep as-is: This is already correct
   - Verify: The `gh pr review` command should use the App token

2. **Label addition** (line ~191):
   - Change from: `GH_TOKEN="$ISSUE_WRITE_TOKEN"` where `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
   - Change to: `GH_TOKEN: ${{ steps.app-token.outputs.token }}`
   - Remove: The `ISSUE_WRITE_TOKEN` environment variable mapping
   - Reason: Label addition should also use the App token for consistency

3. **Goose trigger** (line ~195):
   - Keep: `gh workflow run` can use the App token (no permission requirement difference)

#### FR2: Scan Job (Scheduled Path)
Use the GitHub App token for ALL API interactions in the scan job:

1. **Approval operation** (line ~412):
   - Current: Uses `github.rest.pulls.createReview()` with the App token
   - Keep as-is: This is already correct

2. **Label addition** (line ~428):
   - Change from: `await issueGithub.rest.issues.addLabels()` where `issueGithub` uses `process.env.ISSUE_WRITE_TOKEN`
   - Change to: `await github.rest.issues.addLabels()` using the main `github` client (App token)
   - Remove: The `issueGithub` client instance
   - Reason: Use the same App token for all operations

#### FR3: Environment Variables
- Remove `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` from both jobs
- Remove `GH_TOKEN="$ISSUE_WRITE_TOKEN"` shell variable assignments
- Use `GH_TOKEN: ${{ steps.app-token.outputs.token }}` consistently

### Technical Requirements

1. Maintain existing validation logic (no changes to checks)
2. Preserve approval body text and format
3. Keep the scheduled scan and event-driven trigger behavior
4. Ensure agent metrics collection continues to work
5. Verify GitHub App has required permissions (`pull-requests: write`, `contents: write`, `issues: write`)

## Acceptance Criteria

### AC1: Event-Driven Path Uses App Token
**Given**: A spec PR is opened or edited

**When**: The PR passes all validation checks

**Then**:
- ✅ The `gh pr review` command uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}`
- ✅ The `gh pr edit --add-label` command uses `GH_TOKEN: ${{ steps.app-token.outputs.token }}`
- ✅ No reference to `secrets.GITHUB_TOKEN` or `ISSUE_WRITE_TOKEN` in the validate job

### AC2: Scheduled Scan Path Uses App Token
**Given**: The scheduled scan runs (every 5 minutes) or is manually triggered

**When**: A valid spec PR is found

**Then**:
- ✅ Both approval and label addition use the `github` client (App token)
- ✅ No separate `issueGithub` client instance exists
- ✅ No reference to `process.env.ISSUE_WRITE_TOKEN` in the scan job

### AC3: Approval Attribution is Correct
**Given**: A spec PR is approved (either path)

**When**: Checking the PR's review timeline

**Then**:
- ✅ The approval author is the GitHub App (e.g., `wgmesh-ci[bot]`)
- ✅ NOT `github-actions[bot]`
- ✅ The label is added by the same actor
- ✅ The approval body text is unchanged

### AC4: Goose Build Trigger Works
**Given**: A spec PR is approved

**When**: The approval step runs

**Then**:
- ✅ The `gh workflow run goose-build.yml` command succeeds
- ✅ The workflow is triggered with correct parameters
- ✅ Agent metrics are collected and uploaded

### AC5: No Breaking Changes
**Given**: Existing spec PR workflow

**When**: The changes are deployed

**Then**:
- ✅ Existing validation logic is unchanged
- ✅ Scheduled scan continues to work
- ✅ Event-driven triggers continue to work
- ✅ Agent metrics collection continues to work

## Out of Scope

The following are explicitly out of scope for this fix:

1. **Validation logic changes**: No changes to spec validation checks (sections, file types, classification)
2. **GitHub App configuration**: No changes to App ID, private key, or permissions setup
3. **Approval body text**: No changes to the approval comment format or content
4. **Other workflows**: No changes to `approve-build.yml` or other workflows
5. **Agent metrics**: No changes to metrics collection or upload logic
6. **Goose build workflow**: No changes to `goose-build.yml`

## Verification Steps

1. **Verify validate job changes**:
   ```bash
   # Check the validate job uses App token for all operations
   grep -A 20 "Auto-approve and trigger Goose" .github/workflows/spec-auto-approve.yml
   # Should show: GH_TOKEN: ${{ steps.app-token.outputs.token }}
   # Should NOT show: GH_TOKEN="$ISSUE_WRITE_TOKEN"
   ```

2. **Verify scan job changes**:
   ```bash
   # Check the scan job uses App token for all operations
   grep -A 50 "Scan and validate spec PRs" .github/workflows/spec-auto-approve.yml
   # Should use: github.rest.issues.addLabels
   # Should NOT use: issueGithub.rest.issues.addLabels
   ```

3. **Test event-driven path**:
   ```bash
   # Create a valid spec PR (e.g., via Copilot)
   # Check the approval author in the PR timeline
   # Should show: "wgmesh-ci[bot] approved this"
   # NOT: "github-actions[bot] approved this"
   ```

4. **Test scheduled scan path**:
   ```bash
   # Manually trigger the workflow_dispatch
   # Or wait for the 5-minute cron
   # Check approvals from the scan job
   # Should show: "wgmesh-ci[bot] approved this"
   # NOT: "github-actions[bot] approved this"
   ```
