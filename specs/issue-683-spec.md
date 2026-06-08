# Issue #683: fix(ci): spec-auto-approve approves as github-actions, not self (app token)

## Summary
This spec resolves a CI authentication issue where the `spec-auto-approve` GitHub Action is approving Pull Requests using the GitHub app token identity (`github-actions`) instead of its own bot identity. This causes confusion in commit attribution and approval tracking.

## Context

### Problem
When a specification PR is created with the `approved-for-build` label, the `spec-auto-approve` workflow approves the PR to trigger the automated implementation agent. However, the approval is being recorded as coming from the `github-actions[bot]` user (associated with the GitHub Actions app token) rather than from the workflow's own identity.

### Current Behavior
- Workflow runs: `.github/workflows/spec-auto-approve.yml`
- Approval appears as: `github-actions[bot]`
- Expected: Approval should appear as the workflow's designated bot identity

### Technical Background
GitHub Actions runs using two types of tokens:
1. **GITHUB_TOKEN**: Automatically provided, associated with `github-actions[bot]`
2. **App Token**: Using GitHub App authentication with a private key, appears as the App's identity

The current implementation is using `GITHUB_TOKEN` for the approval API call, which causes the approval to be attributed to the generic GitHub Actions bot.

## Requirements

### Functional Requirements
1. The `spec-auto-approve` workflow must authenticate using a GitHub App token
2. Approval on PRs with `approved-for-build` label must be attributed to the configured app identity
3. Workflow must maintain idempotency (multiple runs should not cause errors)
4. Must only approve PRs that have the required label and pass required status checks

### Technical Requirements
1. Generate or use existing GitHub App credentials:
   - App ID
   - Private key (PEM format)
2. Store private key as GitHub Actions Secret (`SPEC_APP_PRIVATE_KEY`)
3. Store App ID as GitHub Actions Secret/Variable (`SPEC_APP_ID`)
4. Update workflow to:
   - Generate JWT from App credentials
   - Exchange JWT for installation access token
   - Use installation token for GitHub API calls
5. Update workflow to use installation token for approval API call

### Configuration Requirements
1. GitHub App must have repository permissions:
   - `pull_requests: write` (to approve PRs)
   - `contents: read` (basic access)
2. App must be installed on the `atvirokodosprendimai/wgmesh` repository

## Acceptance Criteria

### Criterion 1: GitHub App Configuration
- [ ] GitHub App exists with appropriate permissions
- [ ] App ID stored as secret/variable
- [ ] Private key stored as secret
- [ ] App installed on target repository

### Criterion 2: Workflow Updates
- [ ] Workflow updated to generate JWT from App credentials
- [ ] Workflow exchanges JWT for installation access token
- [ ] Workflow uses installation token for GitHub API authentication
- [ ] No remaining use of `GITHUB_TOKEN` for approval operations

### Criterion 3: Functional Testing
- [ ] Create a spec PR with `approved-for-build` label
- [ ] Workflow runs successfully
- [ ] Approval appears as App identity (not `github-actions[bot]`)
- [ ] PR is approved and ready for implementation agent

### Criterion 4: Idempotency and Error Handling
- [ ] Running workflow multiple times on same PR does not error
- [ ] Workflow handles missing secrets gracefully
- [ ] Workflow logs authentication method used

### Criterion 5: Documentation
- [ ] Update `.github/AGENTS.md` with App authentication details
- [ ] Document required secrets in repository setup guide
- [ ] Update CI/CD section in main project docs

## Out of Scope

- Modifying the implementation agent's behavior
- Changing approval criteria or label requirements
- Modifying other GitHub Actions workflows
- Changing the automated implementation trigger logic
- Implementing additional bot features (e.g., comments, labels)
- Modifying GitHub App permissions beyond what's necessary for approvals
- Setting up GitHub App for other repositories or organizations
- Implementing approval revocation or undo functionality
