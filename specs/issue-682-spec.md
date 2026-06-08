# Issue #682 - fix(ci): spec-auto-approve approves as github-actions, not self (app token)

## Summary

Fix the GitHub Actions workflow `spec-auto-approve.yml` to approve pull requests using the GitHub App's identity (as specified by the app token) instead of the `github-actions` bot identity. Currently, approvals are being attributed to `github-actions` rather than the app itself.

## Context

The `spec-auto-approve.yml` workflow validates spec PRs and auto-approves them to trigger Goose implementation. The workflow:

1. Generates a GitHub App token using `actions/create-github-app-token@v1`
2. Uses this token to approve PRs via the `gh pr review --approve` command
3. The approval is currently recorded as being from `github-actions` instead of the app

The workflow uses `pull_request_target` event with app token authentication, but the CLI command is not correctly using the app-provided token for the approval operation.

**Root Cause**: The `gh` CLI command uses the `GH_TOKEN` environment variable for authentication. In the current workflow, the token is being set, but the approval is still being attributed to `github-actions` rather than the GitHub App. This suggests either:
- The app token is not being passed correctly to the `gh` command
- There's a token substitution issue where the wrong token is being used

## Requirements

### Functional Requirements

1. **FR-1**: The `spec-auto-approve.yml` workflow must approve PRs using the GitHub App's identity
2. **FR-2**: Approvals must be recorded with the app's name/identity, not as `github-actions[bot]`
3. **FR-3**: The fix must apply to both the immediate validation path and the scheduled scan path

### Non-Functional Requirements

1. **NFR-1**: Must maintain existing validation logic
2. **NFR-2**: Must not break the scheduled scan fallback mechanism
3. **NFR-3**: Must preserve all agent metrics collection

## Acceptance Criteria

### AC-1: App Identity for Approvals
Given a spec PR that passes all validation checks
When the workflow runs the auto-approve step
Then the approval is attributed to the GitHub App (not `github-actions[bot]`)
And the review shows the app's identity in the GitHub UI

### AC-2: Both Approval Paths Work
Given the workflow runs via either:
- pull_request_target event (immediate path), OR
- schedule/workflow_dispatch (scan path)
When a valid spec PR is processed
Then the approval uses the app identity in both cases

### AC-3: Existing Logic Preserved
Given the workflow runs with the fix
When validation checks execute
Then all existing validation logic remains unchanged
And agent metrics are still collected correctly

### AC-4: Token Usage Verification
Given the auto-approve step executes
When inspecting the environment variables passed to `gh pr review`
Then `GH_TOKEN` is set to the app token output
And the token is valid for the repository

## Out of scope

- Changing the validation criteria or checks
- Modifying the scheduled scan logic beyond fixing the approval identity
- Changing the agent metrics collection
- Modifying the `goose-build.yml` workflow trigger
- Changing PR labeling logic
- Modifying the spec validation comments

## Implementation Notes

The issue title references issues #681 through #664, all of which appear to be related instances of the same problem (spec PRs being approved with `github-actions` identity). This suggests the problem has occurred across multiple spec PRs.

The fix should focus on ensuring the `gh pr review` command in both the immediate path (around line 177) and the scheduled scan path (around line 328) correctly use the GitHub App token for authentication.
