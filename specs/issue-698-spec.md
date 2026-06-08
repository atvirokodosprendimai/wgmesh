# Issue #698 - CI Auto-Approve Approves as Wrong GitHub Identity

## Summary

Fix the GitHub Actions auto-approve workflow (triggered by `approved-for-build` label on spec PRs) to approve PRs using the GitHub App installation token identity rather than the `github-actions` bot identity. This ensures approvals are attributed to the actual app making the approval request.

## Context

The current CI auto-approve functionality triggers when a spec PR receives the `approved-for-build` label. However, the approval is being performed as the `github-actions` bot instead of using the GitHub App installation token's identity. This causes:

- Incorrect attribution of PR approvals in the GitHub UI
- Potential audit trail confusion about which system/approved the PR
- Misleading commit history and PR activity feeds

The workflow uses a GitHub App installation token that should be used for both the approval action and any subsequent build automation. The token has the necessary permissions, but the approval API call is not using it correctly.

## Requirements

### Functional Requirements

1. **Identity Correction**: The auto-approve workflow must use the GitHub App installation token identity when approving PRs, not the `github-actions` bot identity.

2. **Token Usage**: Ensure the GitHub App JWT/app installation token is properly passed to and used by the approval API call.

3. **Permission Validation**: The workflow must verify that the token has `pull_request:write` scope before attempting approval.

4. **Backward Compatibility**: The fix must not break existing spec PR approval workflows or build triggers.

### Non-Functional Requirements

1. **Error Handling**: Proper error messages when token is invalid or lacks permissions.

2. **Idempotency**: Multiple triggers on the same PR should not cause errors or duplicate approvals.

3. **Security**: The GitHub App private key must remain secure (GitHub Actions secrets).

4. **Testing**: The fix must be testable without affecting production PRs.

## Acceptance Criteria

1. **Correct Approval Identity**: When a spec PR is labeled `approved-for-build`, the approval appears in the PR timeline as coming from the GitHub App (e.g., `wgmesh-bot[bot]` or similar app identity), not `github-actions[bot]`.

2. **Successful PR Transition**: The approval successfully transitions the PR from "requires approval" to "approved" state, enabling merge automation.

3. **No Duplicate Approvals**: Re-running the workflow or re-labeling does not create duplicate approval entries or errors.

4. **Workflow Logs**: GitHub Actions workflow logs clearly show the app identity being used for the approval API call.

5. **Token Scope Verification**: Workflow validates `pull_request:write` permission before proceeding.

6. **Test PR Validation**: A test PR demonstrates the fix working correctly (can be created in a test repository or using a test branch).

## Out of scope

- Changes to the GitHub App permissions or scopes beyond what is necessary for PR approval
- Modifications to other GitHub Actions workflows that do not involve PR approval
- Changes to the `approved-for-build` label triggering mechanism
- Alterations to the build automation that runs after approval
- GitHub App UI/UX changes or branding
- Historical approval data migration (existing approvals remain attributed as-is)
- Token generation or authentication logic changes to the GitHub App itself
