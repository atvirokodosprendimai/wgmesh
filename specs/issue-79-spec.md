# Specification: Issue #79

## Classification
fix

## Problem Analysis

Issue #79 was originally closed by PR #80, which changed `board-sync.yml` and
`copilot-undraft.yml` from `pull_request` to `pull_request_target`. That change
is already present on `main`, but the failure mode still exists today.

Current repository state:

- `/home/runner/work/wgmesh/wgmesh/.github/workflows/board-sync.yml` already
  uses `pull_request_target` for PR events.
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/copilot-undraft.yml`
  already uses `pull_request_target` and also has a scheduled fallback.
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/spec-auto-approve.yml`
  already documents that GitHub blocks `pull_request_target` runs from the
  Copilot bot and therefore includes a scheduled scan fallback.
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/bot-pr-review-merge.yml`
  is still triggered directly by `pull_request`.
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/docker-build.yml` is still
  `pull_request`-triggered and **must remain gated**, because it checks out and
  builds untrusted PR code.

Recent evidence from repository Actions runs shows that the safe workflows are
still being created with `conclusion: action_required` and zero jobs on
Copilot-authored branches, even when they use `pull_request_target`:

- Run `25550761250` — `Spec Auto-Approve` on
  `copilot/fix-github-actions-workflows`, event `pull_request_target`,
  conclusion `action_required`, jobs `0`
- Run `25550728847` — `Sync Board Status` on
  `copilot/fix-github-actions-workflows`, event `pull_request_target`,
  conclusion `action_required`
- Run `25550728877` — `Auto-undraft Copilot PRs` on
  `copilot/fix-github-actions-workflows`, event `pull_request_target`,
  conclusion `action_required`
- Run `25550729240` — `Bot PR Review and Merge` on
  `copilot/fix-github-actions-workflows`, event `pull_request`,
  conclusion `action_required`
- Run `25550729267` — `Build and Push Docker Images` on
  `copilot/fix-github-actions-workflows`, event `pull_request`,
  conclusion `action_required`

Therefore the actual bug is not “these two workflows still use
`pull_request`”. The real bug is: **safe automation still depends on PR-event
workflows that GitHub suppresses for Copilot-authored PRs, so the automation
never starts.**

The fix must remove that dependency for safe workflows, while preserving the
existing security boundary for workflows that execute PR code.

## Proposed Approach

Do **not** try to solve this by loosening repository-wide Actions approval
settings. Keep the current security posture for untrusted PR code.

Instead:

1. Keep `docker-build.yml` as-is and explicitly treat its `action_required`
   state on Copilot PRs as expected behavior.
2. For workflows that only read PR metadata or mutate GitHub state from the
   trusted base branch (`board-sync.yml`, `copilot-undraft.yml`,
   `spec-auto-approve.yml`, `bot-pr-review-merge.yml`), stop relying on direct
   PR event triggers for correctness.
3. Replace those PR-triggered entry points with scheduled and/or manual scans
   that enumerate eligible PRs through the GitHub API and perform the same
   action from a trusted context.
4. Add monitoring so future regressions are visible when any supposedly-safe
   workflow starts generating `action_required` runs on `copilot/*` branches
   again.

The implementation should prefer the smallest viable workflow-only change set.
Do not add new external dependencies.

## Implementation Tasks

### Task 1: Convert `board-sync.yml` from PR-event-driven to scan-driven PR sync

Modify `/home/runner/work/wgmesh/wgmesh/.github/workflows/board-sync.yml`.

Required changes:

1. Keep the existing `issues` trigger exactly as-is.
2. Remove the PR trigger (`pull_request_target`) entirely so Copilot PR creation
   no longer produces a useless `action_required` run for this workflow.
3. Add `schedule` and `workflow_dispatch` triggers.
   - Use a short polling interval (5 minutes is acceptable and matches the cadence
     already used by other Copilot recovery workflows in this repo).
4. Preserve the existing label-to-column mapping and the current “opened/reopened
   with no matching label” defaults.
5. Add a new scheduled/manual code path that:
   - lists PRs targeting `main`
   - includes both open PRs and recently-updated closed PRs so merged/closed PRs
     still move to `Done`
   - recomputes the target board column from PR state + labels exactly the same way
     the current event-driven code does
   - adds the PR to the project board if it is missing
   - updates the project status field to the computed column
6. Do **not** use `actions/checkout` of the PR head.
7. Do **not** change the project ID, field ID, or existing column option IDs.

Acceptance criteria for this task:

- Opening or reopening a Copilot PR no longer creates a `board-sync.yml`
  `action_required` run.
- Within one polling interval, the PR appears on the board in the same column it
  would previously have received from the direct PR event.
- Closing/merging the PR moves the board item to `Done` via the scheduled sweep.

### Task 2: Make `copilot-undraft.yml` schedule-driven only and expand the trusted author list

Modify `/home/runner/work/wgmesh/wgmesh/.github/workflows/copilot-undraft.yml`.

Required changes:

1. Remove the `pull_request_target` trigger and remove the event-driven `undraft`
   job that depends on that trigger.
2. Keep the existing `schedule` and `workflow_dispatch` entry points.
3. Keep the existing scheduled scan logic, but update the trusted-author filter
   everywhere it appears so it matches all supported agent identities:
   - `copilot-swe-agent[bot]`
   - `Copilot`
   - `pupabobas[bot]`
4. Keep the workflow limited to draft PRs only.
5. Do not add any checkout step.

Acceptance criteria for this task:

- New Copilot-authored draft PRs are marked ready for review by the scheduled
  scan without requiring a maintainer to click “Approve and run”.
- `copilot-undraft.yml` no longer produces `action_required` runs on
  `copilot/*` branches.

### Task 3: Make `spec-auto-approve.yml` rely on the existing scan path instead of PR events

Modify `/home/runner/work/wgmesh/wgmesh/.github/workflows/spec-auto-approve.yml`.

Required changes:

1. Remove the `pull_request_target` trigger.
2. Remove the event-driven `validate` job that currently depends on
   `github.event.pull_request`.
3. Keep the existing `schedule` and `workflow_dispatch` scan path as the single
   authoritative implementation.
4. Do **not** remove the actual validation rules. The scheduled/manual path must
   still enforce all current checks:
   - spec file exists at `specs/issue-N-spec.md`
   - PR contains no non-spec changes except permitted workflow-file edits
   - required sections are present
   - classification is actionable
5. Leave the existing approval + label + `goose-build.yml` dispatch behavior in
   place after a successful scan validation.
6. If the scheduled scan currently auto-approves human-authored spec PRs, do not
   broaden that behavior further; keep the existing semantics unless a change is
   strictly required to make Copilot PRs work.

Acceptance criteria for this task:

- A Copilot-created spec PR can still be validated and auto-approved by the
  scheduled scan.
- `spec-auto-approve.yml` no longer creates `action_required` runs on
  `copilot/*` branches.

### Task 4: Replace the direct PR trigger in `bot-pr-review-merge.yml` with a trusted-PR scan

Modify `/home/runner/work/wgmesh/wgmesh/.github/workflows/bot-pr-review-merge.yml`.

Required changes:

1. Remove the `pull_request` trigger entirely.
2. Add `schedule` and `workflow_dispatch` triggers.
   - Use a polling interval similar to the existing autonomous workflows in this
     repo (5–10 minutes is acceptable).
3. Replace the current single-PR event logic with a scan that enumerates open PRs
   targeting `main` and selects the same set of candidates the workflow handles
   today:
   - PR author is `copilot-swe-agent[bot]`
   - or PR author is `Copilot`
   - or PR author is `pupabobas[bot]`
   - or PR author is `goose[bot]`
   - or PR author is `nycterent`
   - or PR title contains `heal:`
   - or PR title contains `loop:`
   - or PR title contains `impl:`
4. For each selected PR, invoke the existing
   `/home/runner/work/wgmesh/wgmesh/company/scripts/pr-review-merge.sh` logic
   instead of duplicating its guardrails in YAML.
5. If the script or its tests need a small update so the approved-author logic
   recognizes `Copilot` and `pupabobas[bot]`, make the smallest possible change
   in:
   - `/home/runner/work/wgmesh/wgmesh/company/scripts/pr-review-merge.sh`
   - `/home/runner/work/wgmesh/wgmesh/company/scripts/test-pr-review-merge.sh`
6. Do not change the script’s security guardrails, protected-path handling, or
   escalation behavior unless required for the new trigger model.

Acceptance criteria for this task:

- Eligible bot-authored PRs are still reviewed/merged automatically.
- `bot-pr-review-merge.yml` no longer creates `action_required` runs on
  `copilot/*` branches.
- If script behavior changes, the existing standalone shell regression test still
  passes.

### Task 5: Add recurrence monitoring for unexpected `action_required` runs on `copilot/*` branches

Add one new workflow file:

- `/home/runner/work/wgmesh/wgmesh/.github/workflows/copilot-action-required-monitor.yml`

Required behavior:

1. Trigger on `schedule` (daily is sufficient) and `workflow_dispatch`.
2. Query recent Actions runs for this repository and find runs where:
   - `conclusion == action_required`
   - `head_branch` starts with `copilot/`
3. Treat the following workflow names as **unexpected** and fail the monitor if
   any matching runs are found:
   - `Sync Board Status`
   - `Auto-undraft Copilot PRs`
   - `Spec Auto-Approve`
   - `Bot PR Review and Merge`
4. Explicitly ignore `Build and Push Docker Images`, because it executes PR code
   and its gated behavior is expected.
5. The failure output must print the offending workflow names, run IDs, and
   branch names so a maintainer can investigate quickly.

This workflow is the regression alarm requested in the issue comments. It does
not need to auto-fix anything; visible failure is enough.

### Task 6: Keep `docker-build.yml` intentionally unchanged

Do **not** change
`/home/runner/work/wgmesh/wgmesh/.github/workflows/docker-build.yml` trigger
behavior.

The spec must be implemented so that only the safe metadata-only workflows stop
producing `action_required` runs. The Docker build workflow should remain
`pull_request`-triggered and should continue to require manual approval on
Copilot PRs, because it checks out and builds untrusted code.

## Affected Files

Expected implementation files:

- `/home/runner/work/wgmesh/wgmesh/.github/workflows/board-sync.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/copilot-undraft.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/spec-auto-approve.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/bot-pr-review-merge.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/copilot-action-required-monitor.yml` (new)

Conditionally affected only if needed by Task 4:

- `/home/runner/work/wgmesh/wgmesh/company/scripts/pr-review-merge.sh`
- `/home/runner/work/wgmesh/wgmesh/company/scripts/test-pr-review-merge.sh`

Files that should remain unchanged for this issue:

- `/home/runner/work/wgmesh/wgmesh/.github/workflows/docker-build.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/close-resolved-issues.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/impl-merged-close.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/spec-merged-build.yml`
- `/home/runner/work/wgmesh/wgmesh/.github/workflows/goose-build.yml`

## Test Strategy

This issue is primarily workflow/configuration work, so verification must focus
on workflow behavior plus any existing script regression tests.

Required validation for the implementation PR:

1. Run repository formatting/tests that already exist and are relevant to any
   touched non-YAML files.
   - If `company/scripts/pr-review-merge.sh` changes, run:
     `bash company/scripts/test-pr-review-merge.sh`
2. Manually inspect the modified workflow YAML files to confirm that the safe
   workflows no longer have PR-event triggers (`pull_request` or
   `pull_request_target`) and now use scheduled/manual scans instead.
3. Verify that `docker-build.yml` still uses `pull_request`.
4. After the implementation branch is pushed, inspect GitHub Actions runs for the
   PR and confirm:
   - no new `action_required` runs are created for `Sync Board Status`,
     `Auto-undraft Copilot PRs`, `Spec Auto-Approve`, or
     `Bot PR Review and Merge`
   - `Build and Push Docker Images` may still be `action_required`
5. Trigger the new monitor workflow with `workflow_dispatch` (or wait for the
   first scheduled run) and confirm it succeeds when only the expected Docker
   workflow is gated.

## Estimated Complexity

medium
