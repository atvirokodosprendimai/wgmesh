# Specification: Issue #691

## Classification
fix

## Problem Analysis

The `goose-triage.yml` workflow currently only triggers on the `labeled` event type. When a closed issue is reopened, it loses the `needs-triage` label (the label state is preserved from before closure, but the reopening event does not trigger the workflow). This creates a **cold-start gate gap**: the issue is open but not triaged, and requires manual re-application of the `needs-triage` label to trigger the automated triage workflow.

**Current state in `.github/workflows/goose-triage.yml`:**
```yaml
'on':
  issues:
    types: [labeled]
```

**Expected behavior:** When an issue is reopened, the workflow should automatically evaluate whether it needs triage and run the spec generation process without requiring manual label re-application.

**Impact:** This gap means that:
1. Previously closed issues that are reopened sit in limbo without automated triage
2. Operators must manually notice and re-label reopened issues
3. The automated workflow is not "cold-start" capable — it requires human intervention to fire on reopened issues

## Proposed Approach

Add the `reopened` event type to the workflow trigger and update the conditional logic to handle both label-based and reopen-based triggers. When triggered by a reopen event, the workflow should check if the issue already has a spec PR, and if not, proceed with spec generation and apply the standard triage flow (spec PR creation, label swap to `copilot-triaging`). This ensures the workflow can fire both on explicit labeling (`needs-triage`) and on issue reopen, eliminating the manual re-labeling step.

## Implementation Tasks

### Task 1: Update workflow trigger to include reopened events
- **File:** `.github/workflows/goose-triage.yml` (modify)
- **What:** Add `reopened` to the `issues.types` trigger array
- **Detail:** Change line 7 from `types: [labeled]` to `types: [labeled, reopened]`. This allows the workflow to fire on both label addition and issue reopening events. No other changes to the trigger block are required.

### Task 2: Update job conditional to handle reopen trigger
- **File:** `.github/workflows/goose-triage.yml` (modify)
- **Detail:** The current `if: github.event.label.name == 'needs-triage'` guard on line 24 only evaluates label events. Add an `||` clause to also allow the job to proceed when `github.event.action == 'reopened'`. This ensures the job runs on reopen events regardless of label state, while still gating on the correct label for labeled events. The new condition should be: `if: github.event.label.name == 'needs-triage' || github.event.action == 'reopened'`

### Task 3: Add spec existence check for reopened issues
- **File:** `.github/workflows/goose-triage.yml` (modify, after "Extract issue context" step)
- **What:** Add a new step that checks if a spec PR already exists for this issue to prevent duplicate specs on re-reopen
- **Detail:** Use `actions/github-script@v7` to query the GitHub API for pull requests with head branch matching `spec/issue-${ISSUE_NUM}` format. Store the result in an output variable. If a matching open PR exists, skip the spec generation and post a comment explaining that a spec already exists. This prevents duplicate work on multiple reopens of the same issue.

### Task 4: Add conditional spec generation guard
- **File:** `.github/workflows/goose-triage.yml` (modify, before "Run Goose with recipe" step)
- **What:** Add an `if` guard on the "Run Goose with recipe" step that checks the spec existence check output
- **Detail:** The step should only run if no existing spec PR was found. If a spec exists, the workflow should exit cleanly after posting a comment. Use the output from the spec existence check (Task 3) to gate the Goose execution. This ensures we don't regenerate specs for issues that already have one.

### Task 5: Update label swap logic to handle reopen events
- **File:** `.github/workflows/goose-triage.yml` (modify, "Swap issue label" step)
- **Detail:** The current label removal assumes `needs-triage` is present. When triggered by reopen, the label may not exist. Wrap the `removeLabel` call in a try-catch (already present) but ensure no error is logged for missing label in reopen case. Ensure the `copilot-triaging` label is always added regardless of trigger type, since this indicates triage is in progress. The existing try-catch should handle this, but verify it doesn't log spurious errors for reopens without the label.

## Affected Files
```
.github/workflows/goose-triage.yml  (modify)
```

## Acceptance Criteria

- `goose-triage.yml` trigger includes `reopened` event type
- Workflow job conditional allows both `labeled` and `reopened` triggers
- Reopening a closed issue without `needs-triage` label triggers the workflow
- Reopening a closed issue that already has a spec PR does not create a duplicate spec
- Reopening a closed issue without a spec PR creates a new spec PR
- Workflow applies `copilot-triaging` label on successful spec creation from reopen
- Workflow posts appropriate comment when spec already exists
- No errors logged for missing `needs-triage` label on reopen-triggered runs
- Manual label application (`needs-triage`) still works as before (regression test)

## Estimated Complexity
low (1 file, ~20 lines modified/added, conditional logic only)
