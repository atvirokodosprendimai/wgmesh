# Implementation Spec: Fix GitHub App Token Approval Identity

## Summary

Fix the spec-auto-approve workflow to approve PRs using the GitHub App's identity (e.g., `wgmesh-bot[bot]`) instead of the workflow runner's identity (`github-actions[bot]`).

## Context

The `spec-auto-approve.yml` workflow validates and auto-approves Copilot spec PRs. Currently, the workflow uses `gh pr review` with the app token, which causes the approval to appear as coming from `github-actions[bot]` rather than the GitHub App's own identity.

This occurs because:
1. The workflow generates an app token using `actions/create-github-app-token@v1`
2. The `validate` job uses this token via `gh pr review` (CLI command)
3. GitHub CLI inherits the workflow's actor identity, not the app's identity
4. In contrast, the `scan` job uses `github.rest.pulls.createReview` (REST API) which correctly uses the app's identity

The inconsistency means:
- Event-driven approvals (validate job) appear as `github-actions[bot]`
- Scheduled scan approvals (scan job) appear as the GitHub App's identity
- This creates confusion in review history and PR tracking

## Requirements

### Functional Requirements
1. The `validate` job in `spec-auto-approve.yml` must approve PRs using the GitHub App's identity
2. Both approval paths (event-driven and scheduled) must use the same approval mechanism
3. The approval must still trigger the downstream `goose-build.yml` workflow

### Technical Requirements
1. Replace `gh pr review` CLI command with GitHub REST API call in the validate job
2. Use `github.rest.pulls.createReview` with the app token to ensure app identity
3. Maintain identical approval body text between both jobs
4. Ensure the `approved-for-build` label is still applied correctly
5. Ensure the `goose-build.yml` workflow is still triggered with correct inputs

### Non-Functional Requirements
1. Minimal code changes - only modify the approval mechanism
2. Maintain backward compatibility with existing metrics collection
3. Preserve all validation logic and error handling
4. No changes to approval criteria or validation checks

## Acceptance Criteria

### AC1: Validate Job Uses App Identity
- The validate job approves PRs using `github.rest.pulls.createReview` (JavaScript action)
- Approval appears in GitHub UI as coming from the GitHub App (e.g., `wgmesh-bot[bot]`)
- Approval is NOT from `github-actions[bot]`

### AC2: Both Jobs Use Same Approval Method
- Both validate and scan jobs use `github.rest.pulls.createReview`
- Approval bodies are consistent between both paths
- Both paths trigger `goose-build.yml` identically

### AC3: All Functionality Preserved
- Spec validation checks remain unchanged
- `approved-for-build` label is applied correctly
- `goose-build.yml` workflow is triggered with correct `issue_number` and `spec_pr_number`
- Metrics collection continues to work
- Failure comments still post on validation failure

### AC4: Testing
- Manual test: Create a spec PR and verify approval author is the GitHub App
- Manual test: Verify `goose-build.yml` is triggered
- Manual test: Verify label is applied
- Manual test: Verify failure comments work when validation fails

## Out of scope

- Changes to validation logic or approval criteria
- Changes to the scheduled scan job (already works correctly)
- Changes to metrics collection
- Changes to `goose-build.yml` or downstream workflows
- Changes to the GitHub App configuration or permissions
- UI/UX changes to approval comments or labels
