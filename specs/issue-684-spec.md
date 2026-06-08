# Issue #684 - Fix CI: spec-auto-approve approves as github-actions, not self (app token)

## Summary

The spec-auto-approve.yml workflow is currently approving Pull Requests using the built-in `GITHUB_TOKEN` (which appears as the `github-actions` user), rather than using the generated GitHub App token. This causes confusion in approval attribution and potentially violates repository governance policies that require approvals from authenticated bots.

## Context

The workflow generates a GitHub App token using `actions/create-github-app-token@v1` with proper APP_ID and APP_PRIVATE_KEY credentials. However, when performing the actual PR approval operations, the workflow inconsistently uses:

1. **Event-driven path** (validate job): Uses `gh pr review` with `GH_TOKEN` set to `${{ steps.app-token.outputs.token }}` (correct), but when adding the `approved-for-build` label, it uses `ISSUE_WRITE_TOKEN` (which is `${{ secrets.GITHUB_TOKEN }}`) instead of the app token.

2. **Scheduled scan path** (scan job): Uses the GitHub REST API `createReview()` method authenticated with `${{ steps.app-token.outputs.token }}` (correct), but when adding labels via `issueGithub.rest.issues.addLabels()`, it uses `ISSUE_WRITE_TOKEN` (which is `${{ secrets.GITHUB_TOKEN }}`) instead of the app token.

The `GITHUB_TOKEN` is a repository-scoped token that always appears as the `github-actions` user, while the GitHub App token appears as the configured GitHub App (e.g., `wgmesh-bot` or similar). Using the app token provides:
- Clearer attribution (specific bot vs generic github-actions)
- Better governance compliance
- Consistent identity across all operations

## Requirements

### Functional Requirements

1. **Consistent authentication**: All operations in the spec-auto-approve workflow must use the GitHub App token for both PR reviews and label operations
2. **Event-driven path fixes**: The validate job must use the app token for:
   - PR approval (`gh pr review`)
   - Label addition (`gh pr edit --add-label`)
   - PR comments (validation failures)
3. **Scheduled scan path fixes**: The scan job must use the app token for:
   - PR approval via REST API
   - Label addition via REST API
4. **Token naming**: Rename `ISSUE_WRITE_TOKEN` references to use the app token to prevent future confusion

### Technical Requirements

1. Remove `${{ secrets.GITHUB_TOKEN }}` usage from approval and label operations
2. Ensure all GitHub API calls use `${{ steps.app-token.outputs.token }}`
3. Maintain proper separation between the two GitHub client instances in the scan job
4. Preserve existing error handling and retry logic

## Acceptance Criteria

1. [ ] Event-driven validate job uses app token for all GitHub operations (review, label, comment)
2. [ ] Scheduled scan job uses app token for all GitHub operations (review, label)
3. [ ] No references to `secrets.GITHUB_TOKEN` remain in approval/label operations
4. [ ] Workflow runs successfully with proper app token authentication
5. [ ] PR reviews appear as the GitHub App user (not github-actions)
6. [ ] Label additions appear as the GitHub App user (not github-actions)
7. [ ] Existing metrics collection and artifact upload remain unchanged
8. [ ] All validation checks continue to work as before

## Out of Scope

- Changes to the validation logic or check criteria
- Changes to the GitHub App token generation process
- Changes to agent metrics collection
- Changes to the goose-build.yml workflow trigger logic
- Changes to other workflows in the repository
- Changes to the required sections in spec files
