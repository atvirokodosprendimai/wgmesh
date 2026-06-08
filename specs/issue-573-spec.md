# Specification: Issue #573

## Classification
fix

## Deliverables
code

## Problem Analysis

`.github/workflows/goose-triage.yml` currently fires only on `issues: types: [labeled]` with a
guard `if: github.event.label.name == 'needs-triage'` (lines 10, 21-22 of the file).

When an issue is **reopened** (`closed → open`), GitHub emits an `issues` event with
`action: reopened` but does **not** re-emit a `labeled` event for labels that are already on the
issue. As a result, the triage workflow is never triggered on reopen.

This is a **cold-start gate gap**: the issue has `needs-triage` but sits idle because the
workflow never runs to generate a specification document.

Additionally, an issue that went through a previous triage round will carry a stale
`copilot-triaging` label from that round. If the workflow is triggered on reopen without cleaning
up that stale label first, the next triage round starts in an inconsistent label state.

The trigger configuration, job guard, and label-cleanup logic need three targeted fixes.

## Proposed Approach

Add `reopened` to the workflow trigger types, update the job-level guard to accept both
`labeled` (with `needs-triage`) and `reopened` events using a boolean expression that safely
handles the missing `github.event.label` field on reopen, and add label cleanup at the start of
the script step to remove any stale `copilot-triaging` label before the new triage round begins.

## Implementation Tasks

### Task 1: Extend `on:` trigger to include `reopened`

- **File:** `.github/workflows/goose-triage.yml` (modify)
- **What:** Add `reopened` to the `on.issues.types` list so the workflow is queued by GitHub
  whenever an issue transitions from closed to open.
- **Detail:** Locate the `on:` block at lines 9-11:
  ```yaml
  'on':
    issues:
      types: [labeled]
  ```
  Replace it with:
  ```yaml
  'on':
    issues:
      types: [labeled, reopened]
  ```
  No other changes to the `on:` block.

### Task 2: Update the job-level `if:` guard

- **File:** `.github/workflows/goose-triage.yml` (modify)
- **What:** Expand the `triage` job's `if:` condition to pass on either a `needs-triage` label
  event or a `reopened` event. The current single-expression guard (`github.event.label.name`)
  would throw on `reopened` events because `github.event.label` is undefined when there is no
  label event.
- **Detail:** Locate the `triage` job declaration around line 21-24:
  ```yaml
  jobs:
    triage:
      if: github.event.label.name == 'needs-triage'
      runs-on: ubuntu-latest
      timeout-minutes: 40
  ```
  Replace the `if:` line with the following multi-line expression. Use the `>-` folded-stripped
  scalar so GitHub Actions receives a single-line expression with no trailing newline:
  ```yaml
  jobs:
    triage:
      if: >-
        (github.event.action == 'labeled' && github.event.label.name == 'needs-triage')
        || github.event.action == 'reopened'
      runs-on: ubuntu-latest
      timeout-minutes: 40
  ```

### Task 3: Remove stale `copilot-triaging` label at the top of the script step

- **File:** `.github/workflows/goose-triage.yml` (modify)
- **What:** At the very beginning of the `Extract issue context` step's `script:` block,
  insert a guard that removes the stale `copilot-triaging` label when the event is `reopened`.
  This ensures each new triage round starts from a clean label state.
- **Detail:** The `script:` block currently starts by defining `issueNumber` and `issue`
  (lines 54-55 in the existing file). Insert the following block **immediately after** those two
  constant declarations and **before** the `safeTitle` constant:
  ```javascript
  // On reopen: remove stale copilot-triaging from the previous triage round so
  // the new round starts from a clean label state.
  if (context.payload.action === 'reopened') {
    try {
      await github.rest.issues.removeLabel({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issueNumber,
        name: 'copilot-triaging'
      });
      console.log('Removed stale copilot-triaging label (reopen path)');
    } catch (e) {
      console.log('copilot-triaging not present or already removed:', e.message);
    }
  }
  ```
  The existing `try/catch` that removes `needs-triage` (lines 216-222 in the existing file) already
  handles the case where `needs-triage` is not present — no change needed there.

## Affected Files

```
.github/workflows/goose-triage.yml   (modify: 3 targeted edits)
```

## Acceptance Criteria

- `goose-triage.yml` passes YAML lint (`yamllint .github/workflows/goose-triage.yml`).
- Manually re-opening a previously closed issue that carries both `needs-triage` and
  `copilot-triaging` (stale) causes the workflow to queue, strip `copilot-triaging`,
  generate a new spec, open a new spec PR, and swap to `copilot-triaging` — identical to the
  `labeled: needs-triage` path.
- Manually re-opening a previously closed issue that carries `needs-triage` but **not**
  `copilot-triaging` causes the workflow to queue and complete without error on the `removeLabel`
  call (the try/catch absorbs the 404).
- Manually re-opening a previously closed issue that does **not** carry `needs-triage` does NOT
  trigger the workflow (guard filters it out).
- The `labeled: needs-triage` path is unaffected: applying `needs-triage` to an open issue still
  fires the workflow as before.

## Estimated Complexity
low (1 file, 3 targeted edits, <30 lines changed)
