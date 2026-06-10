# Specification: Issue #714

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

The issue reports 7 PRs stalled with `copilot-triaging` labels for weeks, indicating a systematic blockage in the spec generation pipeline. After analyzing the codebase, the root cause is architectural, not transient:

1. **No retry or fallback when `goose-triage.yml` fails.** The workflow (`.github/workflows/goose-triage.yml`) fires once on the `needs-triage` label event, runs Goose, and if it fails (API error, timeout, sanitise gate), the issue is left in `copilot-triaging` with no retry mechanism. There is no scheduled scan analogous to the one `spec-auto-approve.yml` provides for spec PRs (line 119: `cron: '*/5 * * * *'`).

2. **No staleness detection for issues stuck in `copilot-triaging`.** The existing `e2e-stalled-watcher.js` only monitors `awaiting-verification` issues. Nothing monitors issues carrying `copilot-triaging` for too long. The `board-sync.yml` maps `copilot-triaging` to the "Spec in Progress" column (line 51), but never alerts on dwell time.

3. **No pipeline-health dashboard surface.** The `agent-metrics-report.yml` produces weekly DORA metrics from uploaded artifacts, but has no concept of pipeline stage latency per issue. The `pulse.sh` script queries external signals (Polar, GitHub stars) but never queries internal pipeline health.

4. **The `spec-auto-approve.yml` scheduled scan (line 119) already demonstrates the pattern** — it catches spec PRs that `pull_request_target` missed. The same pattern should apply to the triage stage.

The fix requires three new components:

- A **spec-triage-watcher** workflow (cron + workflow_dispatch) that detects issues with `copilot-triaging` label for longer than a configurable SLA and either re-triggers `goose-triage.yml` or alerts.
- A **pipeline health** section in the pulse report showing triage latency and stall counts.
- A new **pipeline-health label** (`spec-stalled`) for surfacing stuck issues.

## Proposed Approach

Add a scheduled triage watcher workflow (analogous to `e2e-stalled-watcher.yml`) that detects issues carrying `copilot-triaging` beyond a configurable SLA (default 6 hours), labels them `spec-stalled`, and re-dispatches `goose-triage.yml` for automatic recovery. Extend `scripts/pulse.sh` with a new "Pipeline Health" section querying triage-stage latency and stall counts via `gh issue list`. Create a testable JavaScript handler module following the `e2e-stalled-watcher.js` pattern.

## Implementation Tasks

### Task 1: Create label `spec-stalled` in `.github/labels.yml`

- **File:** `.github/labels.yml` (modify)
- **What:** Add a new label entry for `spec-stalled` after the existing resolution labels block.
- **Detail:** Append the following entry to the YAML array in `.github/labels.yml`, after the `needs-info` label block (around line 37):
  ```yaml
  - name: spec-stalled
    color: "B60205"
    description: "Issue stuck in copilot-triaging beyond SLA — spec generation did not complete"
  ```

### Task 2: Create testable handler module `scripts/workflows/spec-triage-watcher.js`

- **File:** `scripts/workflows/spec-triage-watcher.js` (create)
- **What:** Export an async `handler({github, context, core, nowMs})` function that detects issues stuck in `copilot-triaging` beyond an SLA budget, labels them `spec-stalled`, and returns the stalled issue numbers.
- **Detail:** Follow the exact pattern from `scripts/workflows/e2e-stalled-watcher.js`. The module must:
  1. Export a `handler` function and helper functions for testing (`shouldFlag`, `labelNamesOf`, `STALL_BUDGET_MS`, `TERMINAL_LABELS`).
  2. Define `STALL_BUDGET_MS = 6 * 60 * 60 * 1000` (6 hours).
  3. Define `TERMINAL_LABELS = new Set(['spec-stalled', 'wont-do', 'needs-info'])` — if an issue already carries any of these, skip it.
  4. `shouldFlag({labels, updatedAt, now, budgetMs})` returns `true` when: `labels` includes `copilot-triaging`, `now - updatedMs > budgetMs`, and no terminal label is present.
  5. `handler` paginates open issues with label `copilot-triaging` via `github.rest.issues.listForRepo`. For each issue where `shouldFlag` returns true, call `github.rest.issues.addLabels` with `['spec-stalled']` and log via `core.info`.
  6. Append a step summary to `GITHUB_STEP_SUMMARY` (if set) showing count and issue numbers.
  7. Return `{ stalledCount, stalledNumbers }`.
  8. Filter out PRs (issues where `pull_request` key is truthy).

### Task 3: Create unit tests `scripts/workflows/spec-triage-watcher.test.js`

- **File:** `scripts/workflows/spec-triage-watcher.test.js` (create)
- **What:** Write unit tests for `spec-triage-watcher.js` following the pattern in `scripts/workflows/e2e-stalled-watcher.test.js`.
- **Detail:** Use Node.js `node:assert` (no test framework). Test `shouldFlag` with these cases:
  - Issue with `copilot-triaging`, updated 7h ago → `true`
  - Issue with `copilot-triaging`, updated 3h ago → `false` (within SLA)
  - Issue with `copilot-triaging` + `spec-stalled`, updated 7h ago → `false` (terminal label)
  - Issue with `copilot-triaging` + `wont-do`, updated 7h ago → `false` (terminal label)
  - Issue without `copilot-triaging`, updated 7h ago → `false`
  - Issue with missing `updatedAt` → `false`
  Test `handler` with a mock `github` object that returns a controlled issue list. Verify it calls `addLabels` only for qualifying issues and returns correct counts. Run tests via `node scripts/workflows/spec-triage-watcher.test.js`.

### Task 4: Create workflow `.github/workflows/spec-triage-watcher.yml`

- **File:** `.github/workflows/spec-triage-watcher.yml` (create)
- **What:** Create a scheduled workflow that runs the watcher handler every 30 minutes and optionally re-triggers failed triage runs.
- **Detail:** Follow the exact structure of `.github/workflows/e2e-stalled-watcher.yml`. The workflow must:
  1. Trigger on `schedule: cron: '*/30 * * * *'` and `workflow_dispatch`.
  2. Permissions: `contents: write`, `pull-requests: write`, `issues: write`, `actions: write`.
  3. Steps:
     a. Generate app token via `actions/create-github-app-token@v1` (same as other workflows).
     b. Checkout repository.
     c. Ensure `spec-stalled` label exists via `gh label create spec-stalled --description "Issue stuck in copilot-triaging beyond SLA" --color B60205 --force`.
     d. Run the handler via `actions/github-script@v8`:
        ```javascript
        const handler = require('./scripts/workflows/spec-triage-watcher.js');
        const result = await handler({github, context, core});
        ```
     e. Re-trigger stalled issues: if `result.stalledCount > 0`, for each stalled issue number, remove `copilot-triaging` label, add `needs-triage` label (which re-triggers `goose-triage.yml`), and comment on the issue: `"Spec generation stalled for over 6h. Re-triggering triage automatically."`
        Use `github.rest.issues.removeLabel` to remove `copilot-triaging`, `github.rest.issues.addLabels` to add `needs-triage`, and `github.rest.issues.createComment` to post the comment.
     f. Log re-triggered count to step summary.
  4. Use `GITHUB_TOKEN` (from secrets) for issue operations and `app-token` outputs for checkout.

### Task 5: Add pipeline health section to `scripts/pulse.sh`

- **File:** `scripts/pulse.sh` (modify)
- **What:** Add a "Pipeline Health" section to the pulse report output that shows triage-stage stall counts and latency.
- **Detail:** Add a new function `query_pipeline_health()` after the existing `query_github_external_issues()` function (around line 200 in `scripts/pulse.sh`). The function must:
  1. Call `gh_ready` to check `gh` CLI availability.
  2. Query open issues with label `copilot-triaging` via `gh issue list -R "$GH_REPO" --state open --label copilot-triaging --limit 100 --json number,updatedAt --jq '.'`. Count them as `TRIAGING_OPEN`.
  3. Query open issues with label `spec-stalled` via `gh issue list -R "$GH_REPO" --state open --label spec-stalled --limit 100 --json number --jq 'length'`. Store as `SPEC_STALLED_COUNT`.
  4. Set `PIPELINE_HEALTH_RENDER` to a human-readable string like `"${TRIAGING_OPEN} issues in triage, ${SPEC_STALLED_COUNT} spec-stalled"`.
  5. If `gh` is not available, set `PIPELINE_HEALTH_RENDER="no data (gh CLI unavailable)"` and append to `QUERY_FAILURES`.
  6. Call `query_pipeline_health` after `query_github_external_issues` in the main body (around line 260).
  7. Add to the report output: in the `## Headlines` section, add a line `- Pipeline: ${PIPELINE_HEALTH_RENDER}.`. In the `## Followups` section, add `- If SPEC_STALLED_COUNT > 0, review spec-stalled issues for manual triage.` using the existing `render_followups` pattern.

### Task 6: Update `docs/pipeline-flow.d2` with spec-triage-watcher node

- **File:** `docs/pipeline-flow.d2` (modify)
- **What:** Add a `spec-triage-watcher` node to the pipeline flow diagram showing the feedback loop from stalled issues back to `goose-triage`.
- **Detail:** In `docs/pipeline-flow.d2`, add after the existing `e2e-stalled-watcher` definition (if present) or after the `goose-triage` node:
  ```
  spec-triage-watcher: {
    shape: rectangle
    label: "spec-triage-watcher (cron */30m)"
  }
  spec-triage-watcher -> goose-triage: "re-trigger needs-triage"
  copilot-triaging -> spec-triage-watcher: "detects stall >6h"
  spec-triage-watcher -> spec-stalled: "labels spec-stalled"
  ```

## Affected Files

```
.github/labels.yml                                         (modify: add spec-stalled label)
.github/workflows/spec-triage-watcher.yml                  (new)
scripts/workflows/spec-triage-watcher.js                   (new)
scripts/workflows/spec-triage-watcher.test.js              (new)
scripts/pulse.sh                                           (modify: add pipeline health query)
docs/pipeline-flow.d2                                      (modify: add watcher node)
```

## Acceptance Criteria

- `node scripts/workflows/spec-triage-watcher.test.js` passes all test cases.
- `spec-triage-watcher.yml` is valid YAML (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/spec-triage-watcher.yml'))"` exits 0).
- The `spec-stalled` label definition exists in `.github/labels.yml`.
- `scripts/pulse.sh` includes a `query_pipeline_health` function and emits a "Pipeline:" headline line.
- The watcher workflow triggers on schedule (`*/30 * * * *`) and `workflow_dispatch`.
- Stalled issues (>6h with `copilot-triaging` and no spec PR) are labeled `spec-stalled` and re-triggered with `needs-triage`.
- `go build ./...` passes (no Go code changes, but verify no breakage).
- `make lint` passes (verify with `gofmt -l .` showing no Go changes).

## Estimated Complexity
medium
