# Issue #699 - fix(ci): spec-auto-approve approves as github-actions, not self (app token)

## Summary

The `spec-auto-approve.yml` workflow is currently using the GitHub Actions token (`GITHUB_TOKEN`) for certain operations, which causes PR approvals and reviews to appear as coming from the `github-actions` bot instead of the GitHub App's identity. This reduces clarity in audit logs and makes it harder to track which automation performed the approval action. Additionally, the workflow creates reviews using two different authentication contexts (app token vs. `GITHUB_TOKEN`), leading to inconsistent actor attribution.

## Context

The `spec-auto-approve.yml` workflow validates spec PRs and auto-approves them to trigger the Goose implementation process. It uses:

1. **GitHub App token** (from `actions/create-github-app-token@v1`): Used for most operations including PR checkout, validation, and approval via `gh pr review`
2. **GitHub Actions token** (`GITHUB_TOKEN`): Used for:
   - Adding the `approved-for-build` label via `gh pr edit`
   - Triggering the `goose-build.yml` workflow via `gh workflow run`

The workflow also has a scheduled scan job that approves PRs using the GitHub REST API (`github.rest.pulls.createReview`) with the app token, but then uses a separate `issueGithub` instance (authenticated with `GITHUB_TOKEN`) for adding labels and triggering workflows.

This dual authentication approach causes:
- Inconsistent actor attribution in PR review history
- Some actions appearing as `github-actions[bot]` while others appear as the app name
- Potential permission issues since `GITHUB_TOKEN` has different scopes than the app token
- Confusion in audit logs about which automation system performed actions

## Requirements

### Functional Requirements

1. **FR1**: All PR approval operations (`gh pr review --approve` and `github.rest.pulls.createReview`) must use the GitHub App token authentication
2. **FR2**: All label addition operations (`gh pr edit --add-label`) must use the GitHub App token authentication
3. **FR3**: All workflow trigger operations (`gh workflow run` and `github.rest.actions.createWorkflowDispatch`) must use the GitHub App token authentication
4. **FR4**: Remove the separate `ISSUE_WRITE_TOKEN` environment variable and the dual `issueGithub` instance
5. **FR5**: Ensure consistent actor attribution across all workflow operations

### Non-Functional Requirements

1. **NFR1**: The workflow must not break existing functionality
2. **NFR2**: The workflow must maintain all current validation checks
3. **NFR3**: The workflow must maintain proper error handling
4. **NFR4**: Changes must not introduce new security vulnerabilities

## Acceptance Criteria

1. **AC1**: The `validate` job uses only the GitHub App token for all operations (PR review, label addition, workflow trigger)
2. **AC2**: The `scan` job uses only the GitHub App token for all operations (no `issueGithub` instance)
3. **AC3**: All references to `ISSUE_WRITE_TOKEN` are removed from the workflow
4. **AC4**: All `GITHUB_TOKEN` usages are removed except where explicitly required by GitHub Actions constraints
5. **AC5**: Testing confirms that PR approvals appear with the GitHub App's identity, not `github-actions[bot]`
6. **AC6**: The scheduled scan successfully approves PRs with consistent actor attribution
7. **AC7**: All agent metrics collection continues to work correctly
8. **AC8**: Both event-driven (`pull_request_target`) and scheduled (`cron`) paths work correctly

## Out of scope

1. Modifying the `goose-build.yml` workflow triggered by this spec
2. Changing the GitHub App permissions or configuration
3. Modifying other workflows that may use similar patterns
4. Changing the validation logic or checks performed by the workflow
5. Modifying the agent metrics collection structure
6. Changes to the Copilot triage workflow or related automation
