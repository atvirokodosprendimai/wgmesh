# Issue #677 - Spec Auto-Approve Approves as github-actions Instead of Self (App Token)

## Summary

The `spec-auto-approve.yml` workflow approves pull requests using a GitHub App token, but the approval appears under the `github-actions` bot account instead of the GitHub App's identity. This causes confusion in PR review history and may affect automation behavior that depends on the approver's identity.

## Context

The `spec-auto-approve.yml` workflow performs automated validation and approval of spec PRs to trigger the Goose implementation pipeline. The workflow:

1. Generates a GitHub App token using `actions/create-github-app-token@v1`
2. Validates the spec PR meets all criteria
3. Approves the PR using `gh pr review --approve`
4. Adds the `approved-for-build` label
5. Triggers the `goose-build.yml` workflow

**Current problem**: The approval is attributed to `github-actions[bot]` instead of the GitHub App identity (e.g., `wgmesh-bot` or similar). This happens because:

- The `GITHUB_TOKEN` (passed via `secrets.GITHUB_TOKEN`) is used for the `gh pr edit --add-label` operation
- The App token is used for the approval operation
- However, the workflow may not be correctly associating the approval with the App identity

**Impact**:
- PR review history shows `github-actions[bot]` as the approver instead of the dedicated GitHub App
- Automation rules or branch protections that filter by approver identity may not work correctly
- Teams cannot distinguish between different automated approval sources
- Confusion in audit trails about which automation approved a PR

**Root cause analysis needed**:
- Verify which token is being used for the approval operation
- Check if `gh` CLI is resolving to `github-actions` identity
- Confirm GitHub App permissions and token scope

## Requirements

### Functional Requirements

1. **Correct Approver Identity**
   - Approvals must appear under the GitHub App's identity (e.g., `wgmesh-bot`)
   - NOT under `github-actions[bot]` or any other bot account
   - The approver must be the GitHub App specified by `APP_ID`

2. **Maintain Existing Functionality**
   - All validation checks must continue to work
   - Approval triggering of `goose-build.yml` must continue to work
   - Label addition must continue to work
   - Metrics collection must continue to work

3. **Token Usage Clarity**
   - App token must be used for PR approval
   - Separate `GITHUB_TOKEN` (or App token) must be used for label operations
   - Token sources must be explicitly documented

### Technical Requirements

1. **GitHub App Token Configuration**
   - Use `actions/create-github-app-token@v1` to generate App token
   - Ensure App has `pull_requests: write` permission
   - Ensure App has appropriate repo access

2. **Approval API Call**
   - Use App token for `gh pr review --approve` command
   - Ensure `gh` CLI uses the correct token via `GH_TOKEN` env var
   - Verify no token switching occurs during execution

3. **Label Addition**
   - Use appropriate token for label operations
   - May use `GITHUB_TOKEN` or App token (both have `issues: write`)

### Security Requirements

1. **Secrets Management**
   - `APP_ID` stored as repository variable (`vars.APP_ID`)
   - `APP_PRIVATE_KEY` stored as repository secret (`secrets.APP_PRIVATE_KEY`)
   - No changes to secret storage approach

2. **Permissions**
   - Workflow requires: `pull-requests: write`, `contents: write`, `issues: write`
   - No additional permissions needed

## Acceptance Criteria

### Primary Criteria

1. **Approver Identity Verification**
   - When `spec-auto-approve.yml` approves a PR, the approval appears under the GitHub App identity
   - The review comment shows the GitHub App's username, NOT `github-actions[bot]`
   - Test: Create a test spec PR, verify approver in PR review history

2. **Functionality Preserved**
   - All validation checks pass as before
   - PR approval succeeds
   - Label `approved-for-build` is added
   - `goose-build.yml` workflow is triggered
   - Test: Run full workflow on test spec PR

3. **Scheduled Scan Job**
   - The `scan` job (scheduled path) also approves under GitHub App identity
   - API-based approval uses App token correctly
   - Test: Trigger scheduled scan manually, verify approver identity

### Secondary Criteria

1. **Code Quality**
   - Token usage is clear and well-commented
   - Environment variable names are descriptive
   - No hardcoded credentials

2. **Documentation**
   - Workflow includes comments explaining token usage
   - README or docs updated if needed to explain approver identity

3. **Metrics**
   - Agent metrics collection continues to work
   - No changes to metrics schema or format

### Testing Scenarios

1. **Event-Driven Path (`validate` job)**
   - Open a spec PR
   - Verify workflow approves under GitHub App identity
   - Check PR review history for approver username

2. **Scheduled Scan Path (`scan` job)**
   - Create unapproved spec PR
   - Manually trigger workflow_dispatch
   - Verify approval appears under GitHub App identity

3. **Error Handling**
   - Test behavior if App token generation fails
   - Verify appropriate error messages

## Out of Scope

### Not Included in This Fix

1. **Workflow Logic Changes**
   - No changes to validation criteria
   - No changes to approval conditions
   - No changes to trigger mechanisms
   - No changes to metrics collection

2. **GitHub App Configuration**
   - Not creating a new GitHub App (assume one exists)
   - Not changing App permissions or access scope
   - Not modifying App ID or private key storage

3. **Other Workflows**
   - Not fixing `approve-build.yml` (separate workflow)
   - Not fixing `auto-merge.yml` (separate workflow)
   - Not fixing `goose-build.yml` (separate workflow)

4. **Non-Approval Operations**
   - Not changing how labels are added (as long as it works)
   - Not changing how workflows are triggered
   - Not changing comment formatting or content

5. **Branch Protection Rules**
   - Not modifying repository branch protection settings
   - Not changing required reviewers or automation bypass rules

### Future Considerations (Not in Scope)

1. Standardize token usage across all workflows (separate initiative)
2. Create unified approval library for spec and build approvals
3. Add audit logging for all automated approvals
4. Configure GitHub App with custom avatar or display name

## Implementation Notes

### Files to Modify

- `.github/workflows/spec-auto-approve.yml` - Fix token usage in `validate` job approval step and `scan` job approval script

### Potential Root Causes

1. **Token Mixing**: The approval step might be using `GITHUB_TOKEN` instead of App token
2. **gh CLI Behavior**: The `gh` CLI might default to `GITHUB_TOKEN` even when `GH_TOKEN` is set
3. **Permission Scope**: App token might not have correct scope for approvals
4. **API vs CLI**: GitHub REST API might handle App identity differently than `gh` CLI

### Suggested Investigation Steps

1. Check current token usage in approval step (lines with `gh pr review`)
2. Verify `GH_TOKEN` env var is set correctly
3. Test `gh` CLI behavior with explicit `--github-token` flag
4. Verify GitHub App has `pull_requests: write` permission
5. Check if switching from `gh` CLI to `actions/github-script@v7` resolves identity issue

### Reference Implementation Pattern

The `scan` job already uses `actions/github-script@v7` for approval. Consider if the `validate` job should use the same pattern for consistency, or ensure `gh` CLI correctly uses the App token.

## Deliverables

1. Updated `.github/workflows/spec-auto-approve.yml` with correct token usage
2. Test spec PR demonstrating approval under GitHub App identity
3. Brief documentation update explaining the fix (comment in workflow)
