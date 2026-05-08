# Specification: Issue #573

## Classification
fix

## Deliverables
code

## Problem Analysis

`.github/workflows/copilot-triage.yml` currently fires only on `issues: types: [labeled]` with a
guard `if: github.event.label.name == 'needs-triage'` (lines 8–17 of the file).

When an issue is **reopened** (`closed → open`), GitHub emits an `issues` event with
`action: reopened` but does **not** re-emit a `labeled` event for labels that are already on the
issue. As a result, the triage workflow is never triggered on reopen.

Additionally, an issue that went through a previous triage round will carry a stale
`copilot-triaging` label from that round. If the workflow is triggered on reopen without cleaning
up that stale label first, the next triage round starts in an inconsistent label state.

Two lines in the workflow need changing, and the inline `github-script` block needs a small guard
added at its start to remove the stale `copilot-triaging` label on the `reopened` path.

## Implementation Tasks

### Task 1: Extend `on:` trigger to include `reopened`

- **File:** `.github/workflows/copilot-triage.yml` (modify)
- **What:** Add `reopened` to the `on.issues.types` list so the workflow is queued by GitHub
  whenever an issue transitions from closed to open.
- **Detail:** Locate the `on:` block at lines 7–9:
  ```yaml
  on:
    issues:
      types: [labeled]
  ```
  Replace it with:
  ```yaml
  on:
    issues:
      types: [labeled, reopened]
  ```
  No other changes to the `on:` block.

### Task 2: Update the job-level `if:` guard

- **File:** `.github/workflows/copilot-triage.yml` (modify)
- **What:** Expand the `triage` job's `if:` condition to pass on either a `needs-triage` label
  event or a `reopened` event. The current single-expression guard (`github.event.label.name`)
  would throw on `reopened` events because `github.event.label` is undefined when there is no
  label event.
- **Detail:** Locate the `triage` job declaration around line 16–17:
  ```yaml
  jobs:
    triage:
      if: github.event.label.name == 'needs-triage'
  ```
  Replace the `if:` line with the following multi-line expression. Use the `>-` folded-stripped
  scalar so GitHub Actions receives a single-line expression with no trailing newline:
  ```yaml
  jobs:
    triage:
      if: >-
        (github.event.action == 'labeled' && github.event.label.name == 'needs-triage')
        || github.event.action == 'reopened'
  ```

### Task 3: Remove stale `copilot-triaging` label at the top of the script step

- **File:** `.github/workflows/copilot-triage.yml` (modify)
- **What:** At the very beginning of the `Assign Copilot coding agent to write specification`
  step's `script:` block, insert a guard that removes the stale `copilot-triaging` label when
  the event is `reopened`. This ensures each new triage round starts from a clean label state.
- **Detail:** The `script:` block currently starts by defining `issueNumber` and `issue`
  (lines 25–26 in the existing file). Insert the following block **immediately after** those two
  constant declarations and **before** the `specInstructions` constant:
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
  The existing `try/catch` that removes `needs-triage` (further down in the script) already
  handles the case where `needs-triage` is not present — no change needed there.

## Affected Files

```
.github/workflows/copilot-triage.yml   (modify: 3 targeted edits)
```

## Acceptance Criteria

- `copilot-triage.yml` passes YAML lint (`yamllint .github/workflows/copilot-triage.yml`).
- Manually re-opening a previously closed issue that carries `copilot-triaging` (stale) causes
  the workflow to queue, strip `copilot-triaging`, assign `copilot-swe-agent[bot]`, add
  `copilot-triaging`, and post the triage comment — identical to the `labeled: needs-triage` path.
- Manually re-opening a previously closed issue that does **not** carry `copilot-triaging` causes
  the workflow to queue and complete without error on the `removeLabel` call (the try/catch
  absorbs the 404).
- The `labeled: needs-triage` path is unaffected: applying `needs-triage` to an open issue still
  fires the workflow as before.

## Estimated Complexity
low
