# Implementation Spec: Issue #696

## Summary

Fix the `spec-auto-approve.yml` workflow to use the GitHub App token consistently for all operations (approval, labeling, workflow dispatch, comments) instead of mixing GitHub App token with `GITHUB_TOKEN`. The current implementation causes approvals to be attributed to `github-actions[bot]` instead of the GitHub App actor, creating inconsistent actor attribution and confusing audit trails.

## Context

The `.github/workflows/spec-auto-approve.yml` workflow validates and auto-approves Copilot spec PRs that pass all checks. It currently has inconsistent token usage across two code paths:

### Event-driven Job (validate)
- **Line 168**: `GH_TOKEN: ${{ steps.app-token.outputs.token }}` - Correctly uses GitHub App token for checkout
- **Line 169**: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - Creates variable pointing to generic GITHUB_TOKEN
- **Line 180**: `gh pr review` uses `GH_TOKEN` (inherited from env, App token) - Correct for approval
- **Line 189**: `GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit` - Uses GITHUB_TOKEN for label addition - INCORRECT
- **Line 192**: `gh workflow run` uses `GH_TOKEN` (inherited from env, App token) - Correct for dispatch
- **Line 264**: `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - Uses GITHUB_TOKEN for validation failure comments - INCORRECT

### Scheduled Scan Job (scan)
- **Line 290-291**: Generates GitHub App token correctly
- **Line 299**: `ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}` - Creates variable pointing to generic GITHUB_TOKEN
- **Line 301**: `github-token: ${{ steps.app-token.outputs.token }}` - Correctly uses GitHub App token for primary github client
- **Line 302-304**: Creates separate `issueGithub` instance using `GITHUB_TOKEN` for label operations - INCORRECT
- **Line 437**: `await issueGithub.rest.issues.addLabels()` - Uses GITHUB_TOKEN client for labels - INCORRECT

### Impact

This mixed token usage means:
- The PR review approval appears from the GitHub App actor (e.g., "wgmesh-ci[bot]")
- The label addition appears from `github-actions[bot]`
- Validation failure comments appear from `github-actions[bot]`
- The scheduled scan uses two different actors for different operations

This causes:
1. **Confusing PR history**: Developers see mixed actors (GitHub App vs github-actions) for what should be a single automated operation
2. **Audit trail issues**: The approval timeline shows multiple actors, making it unclear which system performed the approval
3. **Downstream automation assumptions**: Other scripts may expect consistent bot attribution
4. **Trust problems**: Teams may question why approval and labeling come from different actors

## Requirements

### Functional Requirements

1. **All operations use GitHub App token**: Both PR approval and labeling operations must use the GitHub App token (`steps.app-token.outputs.token`), not `GITHUB_TOKEN`

2. **Remove token switching**: Eliminate the `ISSUE_WRITE_TOKEN` environment variable and all `GH_TOKEN="$ISSUE_WRITE_TOKEN"` overrides

3. **Fix both workflow paths**: Ensure consistency in both the event-driven path and the scheduled scan path

4. **Preserve all functionality**: The workflow must continue to:
   - Validate spec files
   - Post approval comments
   - Add `approved-for-build` label
   - Trigger `goose-build.yml` workflow
   - Collect agent metrics

5. **Maintain permissions**: Keep existing permission scopes (pull-requests: write, contents: write, issues: write)

### Technical Requirements

1. **Event-driven path (validate job)**:
   - Ensure `gh pr review` uses App token via `GH_TOKEN` (already correct)
   - Ensure `gh pr edit --add-label` uses App token via `GH_TOKEN` (needs fix)
   - Ensure `gh workflow run` uses App token via `GH_TOKEN` (already correct)
   - Ensure `gh pr comment` for failures uses App token via `GH_TOKEN` (needs fix)

2. **Scheduled path (scan job)**:
   - The `github-script` action must use the App token for both `github` and all operations
   - Remove the separate `issueGithub` instance that uses `GITHUB_TOKEN`
   - Use the existing `github` client (already authenticated with App token) for all operations including labeling

3. **Verification method**:
   - Add debug logging to confirm App token usage
   - Test with a real PR to verify approval actor attribution

## Acceptance Criteria

### Event-driven Job (validate)
- [ ] All `gh` CLI commands use `GH_TOKEN: ${{ steps.app-token.outputs.token }}` (via environment variable)
- [ ] No references to `secrets.GITHUB_TOKEN` remain in the validate job
- [ ] No references to `ISSUE_WRITE_TOKEN` remain in the validate job
- [ ] The `gh pr edit --add-label approved-for-build` command uses GitHub App token
- [ ] The `gh pr comment` command for validation failures uses GitHub App token

### Scheduled Scan Job (scan)
- [ ] All GitHub REST API calls use the GitHub App token-authenticated `github` context
- [ ] No separate `issueGithub` instance is created
- [ ] All operations (approval, labeling, workflow dispatch) use the same `github` client
- [ ] No references to `secrets.GITHUB_TOKEN` remain in the scan job
- [ ] No references to `ISSUE_WRITE_TOKEN` remain in the scan job

### Verification
- [ ] Create a test spec PR and verify all operations (approval, label, comments) appear from the GitHub App actor (e.g., "wgmesh-ci[bot]")
- [ ] Verify the `approved-for-build` label is added successfully
- [ ] Verify the Goose build workflow is triggered successfully after approval
- [ ] Confirm no changes to validation logic or approval criteria
- [ ] Verify workflow logs show consistent token usage

## Out of Scope

1. **GitHub App configuration**: Changing the App ID, private key, or permissions
2. **Workflow logic changes**: The validation criteria or approval conditions
3. **Other workflows**: Changes to `auto-merge.yml`, `approve-build.yml`, or `goose-build.yml`
4. **Token generation mechanism**: Changing how the GitHub App token is generated
5. **Scheduled scan timing**: Modifying the cron schedule or trigger conditions
6. **Agent metrics**: Changes to metrics collection or artifact upload logic
7. **Spec validation rules**: Changes to required sections or classification checks
8. **GitHub App permissions**: Beyond documenting existing required permissions

## Affected Files

```
.github/workflows/spec-auto-approve.yml  (modify: token usage in validate and scan jobs)
```

## Implementation Notes

### Event-driven path fix

**File:** `.github/workflows/spec-auto-approve.yml` (validate job)

**Remove line 169:**
```yaml
ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Replace line 189:**
```bash
GH_TOKEN="$ISSUE_WRITE_TOKEN" gh pr edit $PR_NUM --add-label approved-for-build
```

**With:**
```bash
gh pr edit $PR_NUM --add-label approved-for-build
```

**Replace line 264:**
```yaml
env:
  GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**With:**
```yaml
env:
  GH_TOKEN: ${{ steps.app-token.outputs.token }}
```

### Scheduled scan path fix

**File:** `.github/workflows/spec-auto-approve.yml` (scan job)

**Remove lines 299-304:**
```yaml
env:
  ISSUE_WRITE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
with:
  github-token: ${{ steps.app-token.outputs.token }}
  script: |
    const issueGithub = new github.constructor({
      auth: process.env.ISSUE_WRITE_TOKEN,
    });
```

**With:**
```yaml
with:
  github-token: ${{ steps.app-token.outputs.token }}
  script: |
```

**Replace line 437:**
```javascript
await issueGithub.rest.issues.addLabels({
```

**With:**
```javascript
await github.rest.issues.addLabels({
```

### Testing approach

1. Create a test spec PR that passes all validation checks
2. Verify the approval appears under the app identity (not `github-actions[bot]`)
3. Verify the label is added by the same identity
4. Verify validation failure comments (if any) appear from the app identity
5. Confirm `goose-build.yml` is triggered successfully
6. Check the approval timeline shows consistent actor identity

### Risk Assessment

**Risk Level**: Low

**Justification**:
- Change is isolated to authentication token usage
- No logic changes to validation or approval criteria
- GitHub App already has all required permissions
- Change simplifies the code (removes token switching)
- Both paths already use App token for most operations

**Rollback Plan**:
If issues arise, restore the original token switching by reverting the removed lines and variable references.
