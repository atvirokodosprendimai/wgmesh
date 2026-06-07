# Specification: Issue #652

## Classification

fix

## Problem Analysis

CI is reporting failure, blocking PR merges and reducing deployment confidence. After thorough investigation of the codebase and all workflow files, four concrete root causes are identified:

1. **`auto-merge.yml` references non-existent "CodeQL" workflow (line 13):** The `workflow_run` trigger at `.github/workflows/auto-merge.yml:12-14` watches for completions from `["Build and Push Docker Images", "CodeQL"]`. No `CodeQL` workflow exists anywhere in `.github/workflows/`. GitHub silently ignores non-matching workflow names, so the `workflow_run` trigger only fires for Docker builds. PRs that pass `status-check.yml` but don't trigger a Docker build (e.g., non-`main`-push scenarios) never get auto-merged via the event-driven path, forcing reliance on the 10-minute cron fallback and causing perceived CI stall.

2. **11 workflows still use expired `secrets.PUSH_TOKEN`:** Commits `e0b4905` (PR #659) and `79bb886` (PR #661) migrated four workflows to use app tokens (`actions/create-github-app-token`), but 11 workflows remain on the expired `PUSH_TOKEN` PAT. These workflows fail at runtime when they attempt API operations that require write access:
   - `.github/workflows/approve-build.yml` (3 references, lines 33/110/197)
   - `.github/workflows/auto-merge.yml` (1 reference, line 42 — used for `goose-review` dispatch via raw `fetch()`)
   - `.github/workflows/board-sync.yml` (1 reference, line 35)
   - `.github/workflows/bot-pr-review-merge.yml` (1 reference, line 55)
   - `.github/workflows/e2e-verify-close.yml` (2 references, lines 47/63)
   - `.github/workflows/goose-review.yml` (4 references, lines 45/121/297/359)
   - `.github/workflows/impl-merged-close.yml` (2 references, lines 75/91)
   - `.github/workflows/pulse.yml` (1 reference, line 24)
   - `.github/workflows/release.yml` (1 reference, line 66)
   - `.github/workflows/spec-merged-build.yml` (1 reference, line 26)
   - `.github/workflows/verify-comment-close.yml` (1 reference, line 36)

3. **`auto-merge.yml` uses raw `PUSH_TOKEN` for `fetch()` calls (lines 246, 267):** Even if the app token is generated, the goose-review dispatch logic at `.github/workflows/auto-merge.yml:234-270` uses `process.env.PUSH_TOKEN` for raw REST API calls to list workflow runs and dispatch the goose-review workflow. These calls fail silently when the PAT is expired, leaving unresolved Copilot review threads and blocking merge.

4. **`pr-review-merge.sh` references non-existent `spec-validation.yml`:** The guardrail at `company/scripts/pr-review-merge.sh:732-759` waits for a `spec-validation.yml` workflow that does not exist. When a spec PR enters the bot review flow, the script polls 6 times (30s each = 3 min) looking for the `approved-for-build` label from the non-existent validator, then escalates as "Spec validation did not complete within timeout." The label is actually applied by `spec-auto-approve.yml` via a different path, so this creates a race condition and potential false escalations.

## Proposed Approach

Fix the four root causes by: (1) removing the phantom "CodeQL" reference from `auto-merge.yml` and adding "Status Check" as the actual PR gate workflow, (2) migrating all 11 remaining workflows from `secrets.PUSH_TOKEN` to the app token pattern already established in PRs #659/#661, (3) replacing raw `PUSH_TOKEN` `fetch()` calls in `auto-merge.yml` with `github.rest.actions` via the app token, and (4) updating `pr-review-merge.sh` to reference `spec-auto-approve.yml` instead of the non-existent `spec-validation.yml`.

## Implementation Tasks

### Task 1: Fix auto-merge.yml workflow_run trigger
- **File:** `.github/workflows/auto-merge.yml` (modify)
- **What:** Replace the `workflow_run.workflows` list from `["Build and Push Docker Images", "CodeQL"]` to `["Build and Push Docker Images", "Status Check"]` on line 13.
- **Detail:** The `Status Check` workflow (defined in `.github/workflows/status-check.yml`) runs on every pull_request and is the actual gate that PRs must pass. Removing "CodeQL" (which doesn't exist) eliminates the phantom trigger. Adding "Status Check" ensures `auto-merge` fires promptly when a PR's status check completes, instead of waiting for the 10-minute cron fallback.

### Task 2: Migrate approve-build.yml to app token
- **File:** `.github/workflows/approve-build.yml` (modify)
- **What:** Add a `Generate app token` step (using `actions/create-github-app-token@v1` with `app-id: ${{ vars.APP_ID }}` and `private-key: ${{ secrets.APP_PRIVATE_KEY }}`) and replace all three `secrets.PUSH_TOKEN` references (lines 33, 110, 197) with `${{ steps.app-token.outputs.token }}`.
- **Detail:** Follow the exact same pattern used in `goose-build.yml` (line 50-55) and `goose-triage.yml` (lines 32-37). The step ID should be `app-token`. Replace `github-token: ${{ secrets.PUSH_TOKEN }}` with `github-token: ${{ steps.app-token.outputs.token }}` in all three `actions/github-script` steps.

### Task 3: Migrate auto-merge.yml PUSH_TOKEN to app token for fetch calls
- **File:** `.github/workflows/auto-merge.yml` (modify)
- **What:** Replace the raw `fetch()` calls at lines 246 and 267 that use `process.env.PUSH_TOKEN` with `github.rest.actions.listWorkflowRuns()` and `github.rest.actions.createWorkflowDispatch()` using the already-generated app token from `${{ steps.app-token.outputs.token }}`.
- **Detail:** The auto-merge workflow already generates an app token at its step `app-token` (lines 37-42). The `PUSH_TOKEN` env var on line 42 should be removed. The two `fetch()` blocks that list goose-review runs and dispatch goose-review should be rewritten using the `github` client available in `actions/github-script`, which already authenticates with the app token. Specifically:
  - Replace the `fetch` to `.../actions/workflows/goose-review.yml/runs?per_page=10` with `github.rest.actions.listWorkflowRuns({ owner, repo, workflow_id: 'goose-review.yml', per_page: 10 })`.
  - Replace the `fetch` POST to `.../actions/workflows/goose-review.yml/dispatches` with `github.rest.actions.createWorkflowDispatch({ owner, repo, workflow_id: 'goose-review.yml', ref: 'main', inputs: { pr_number: String(prNumber) } })`.
  - Remove the `PUSH_TOKEN` env var declaration at line 42.

### Task 4: Migrate board-sync.yml to app token
- **File:** `.github/workflows/board-sync.yml` (modify)
- **What:** Add a `Generate app token` step and replace `github-token: ${{ secrets.PUSH_TOKEN }}` (line 35) with `github-token: ${{ steps.app-token.outputs.token }}`.
- **Detail:** Same pattern as Task 2. Add `actions: write` to the `permissions` block if not already present (the workflow needs `issues: read` and `pull-requests: read` which are already declared; adding `contents: read` is fine).

### Task 5: Migrate bot-pr-review-merge.yml to app token
- **File:** `.github/workflows/bot-pr-review-merge.yml` (modify)
- **What:** Add a `Generate app token` step and replace `GH_TOKEN: ${{ secrets.PUSH_TOKEN }}` (line 55) with `GH_TOKEN: ${{ steps.app-token.outputs.token }}`.
- **Detail:** The script `company/scripts/pr-review-merge.sh` reads `GH_TOKEN` from the environment. The app token provides the same write permissions. The `permissions` block already has `contents: write`, `pull-requests: write`, `issues: write`, and `actions: read`.

### Task 6: Migrate e2e-verify-close.yml to app token
- **File:** `.github/workflows/e2e-verify-close.yml` (modify)
- **What:** Add a `Generate app token` step and replace both `secrets.PUSH_TOKEN` references (lines 47 and 63) with `${{ steps.app-token.outputs.token }}`.
- **Detail:** Line 47 is `GH_TOKEN: ${{ secrets.PUSH_TOKEN }}` in the label-creation step. Line 63 is `github-token: ${{ secrets.PUSH_TOKEN }}` in the `actions/github-script` step. Replace both with the app token output.

### Task 7: Migrate goose-review.yml to app token
- **File:** `.github/workflows/goose-review.yml` (modify)
- **What:** Add a `Generate app token` step and replace all four `secrets.PUSH_TOKEN` references with `${{ steps.app-token.outputs.token }}`.
- **Detail:** The four references are: (1) `github-token: ${{ secrets.PUSH_TOKEN }}` at line 45 in the PR metadata fetch step, (2) `token: ${{ secrets.PUSH_TOKEN }}` at line 121 in the checkout step, (3) `github-token: ${{ secrets.PUSH_TOKEN }}` at line 297 in the resolve threads step, (4) `github-token: ${{ secrets.PUSH_TOKEN }}` at line 359 in the comment step. Replace all with the app token output.

### Task 8: Migrate impl-merged-close.yml to app token
- **File:** `.github/workflows/impl-merged-close.yml` (modify)
- **What:** Add a `Generate app token` step and replace both `secrets.PUSH_TOKEN` references (lines 75 and 91) with `${{ steps.app-token.outputs.token }}`.
- **Detail:** Line 75 is `GH_TOKEN: ${{ secrets.PUSH_TOKEN }}` in the label creation step. Line 91 is `github-token: ${{ secrets.PUSH_TOKEN }}` in the `actions/github-script` step.

### Task 9: Migrate pulse.yml to app token
- **File:** `.github/workflows/pulse.yml` (modify)
- **What:** Add a `Generate app token` step and replace `token: ${{ secrets.PUSH_TOKEN }}` (line 24) with `token: ${{ steps.app-token.outputs.token }}`.
- **Detail:** The pulse workflow checks out the repo with `persist-credentials: true` and pushes to main. The app token must have `contents: write` permission, which is already declared in the workflow's `permissions` block.

### Task 10: Migrate release.yml to app token
- **File:** `.github/workflows/release.yml` (modify)
- **What:** Add a `Generate app token` step and replace `PUSH_TOKEN: ${{ secrets.PUSH_TOKEN }}` (line 66) with `PUSH_TOKEN: ${{ steps.app-token.outputs.token }}`.
- **Detail:** The `PUSH_TOKEN` env var is used by the goreleaser action. The app token provides the same write access.

### Task 11: Migrate spec-merged-build.yml to app token
- **File:** `.github/workflows/spec-merged-build.yml` (modify)
- **What:** Add a `Generate app token` step and replace `github-token: ${{ secrets.PUSH_TOKEN }}` (line 26) with `github-token: ${{ steps.app-token.outputs.token }}`.
- **Detail:** This workflow assigns the Copilot build agent and adds labels, which requires the token to have `issues: write` and `pull-requests: read`. These permissions are already declared.

### Task 12: Migrate verify-comment-close.yml to app token
- **File:** `.github/workflows/verify-comment-close.yml` (modify)
- **What:** Add a `Generate app token` step and replace `github-token: ${{ secrets.PUSH_TOKEN }}` (line 36) with `github-token: ${{ steps.app-token.outputs.token }}`.
- **Detail:** This workflow updates issues (close + label removal + comment), requiring `issues: write` which is already declared.

### Task 13: Fix pr-review-merge.sh spec-validation reference
- **File:** `company/scripts/pr-review-merge.sh` (modify)
- **What:** Replace the three references to `spec-validation.yml` with `spec-auto-approve.yml` in the spec PR guardrail block (lines 732-759).
- **Detail:** The comment on line 732 says "spec-validation.yml runs fast (~30s)". Replace `spec-validation` with `spec-auto-approve` in the comment (line 732) and the two log messages (lines 746, 759). The actual label check (`approved-for-build`) is already correct — `spec-auto-approve.yml` is the workflow that applies this label. Only the human-readable strings and comments reference the wrong workflow name.

### Task 14: Add GitHub Actions workflow tests for token migration
- **File:** `company/scripts/pr-review-merge_test.sh` (modify)
- **What:** Add a test case verifying that `pr-review-merge.sh` does not reference `spec-validation.yml` (regression guard).
- **Detail:** Add a grep assertion: `grep -q "spec-auto-approve" company/scripts/pr-review-merge.sh && ! grep -q "spec-validation" company/scripts/pr-review-merge.sh`. This prevents future drift back to the phantom workflow name.

## Affected Files

```
.github/workflows/auto-merge.yml       (modify: fix workflow_run trigger, replace PUSH_TOKEN fetch calls)
.github/workflows/approve-build.yml    (modify: migrate to app token)
.github/workflows/board-sync.yml       (modify: migrate to app token)
.github/workflows/bot-pr-review-merge.yml (modify: migrate to app token)
.github/workflows/e2e-verify-close.yml (modify: migrate to app token)
.github/workflows/goose-review.yml     (modify: migrate to app token)
.github/workflows/impl-merged-close.yml (modify: migrate to app token)
.github/workflows/pulse.yml            (modify: migrate to app token)
.github/workflows/release.yml          (modify: migrate to app token)
.github/workflows/spec-merged-build.yml (modify: migrate to app token)
.github/workflows/verify-comment-close.yml (modify: migrate to app token)
company/scripts/pr-review-merge.sh     (modify: fix spec-validation reference)
company/scripts/pr-review-merge_test.sh (modify: add regression guard)
```

## Acceptance Criteria

- `go build ./...` passes
- `go test ./...` passes (all existing tests including `company/scripts/pr-review-merge_test.sh`)
- `go vet ./...` passes
- No workflow file references `secrets.PUSH_TOKEN` (verifiable: `grep -r 'secrets.PUSH_TOKEN' .github/workflows/` returns empty)
- No workflow file references "CodeQL" (verifiable: `grep -r 'CodeQL' .github/workflows/` returns empty)
- `auto-merge.yml` `workflow_run` trigger lists only workflows that exist (verifiable: the names match actual workflow `name:` fields)
- `pr-review-merge.sh` does not reference `spec-validation.yml` (verifiable: `grep 'spec-validation' company/scripts/pr-review-merge.sh` returns empty)
- Every migrated workflow follows the same app-token pattern: `Generate app token` step → `steps.app-token.outputs.token` (verifiable: consistent with `goose-build.yml`, `goose-triage.yml`, `spec-auto-approve.yml`)

## Estimated Complexity

medium (13 files modified, all following an established pattern; no algorithmic changes, purely mechanical token migration + one trigger fix + one string fix)
