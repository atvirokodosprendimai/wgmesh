# Specification: Issue #573

## Classification
bug

## Problem Analysis

The triage workflow (`goose-triage.yml`) triggers only on issue label events via the `issues: [labeled]` trigger. When a closed issue is reopened, GitHub does not fire a label event even if the issue retains the `needs-triage` label. This creates a cold-start gap: reopened issues stall at the `needs-triage` label without triggering specification generation, preventing the Goose implementation pipeline from activating.

**Current trigger (line 9-10):**
```yaml
'on':
  issues:
    types: [labeled]
```

**GitHub behavior:** reopening a closed issue fires `reopened` event type, not `labeled`. The workflow ignores this event, leaving the issue in a triage state with no automated path forward.

**Impact:** Manual intervention is required to either remove/re-add the `needs-triage` label (inefficient) or manually draft the specification (bypasses automation). This breaks the self-service triage-to-implementation pipeline.

## Proposed Approach

Add `reopened` to the workflow trigger types so that `goose-triage.yml` executes when an issue with the `needs-triage` label is reopened. The existing `if: github.event.label.name == 'needs-triage'` guard on line 47 prevents spurious execution for issues without the label, so no additional gating is required.

## Implementation Tasks

### Task 1: Add reopened trigger type
- **File:** `.github/workflows/goose-triage.yml` (modify)
- **What:** Add `reopened` to the `types` array on line 10
- **Detail:** Change `types: [labeled]` to `types: [labeled, reopened]`. The conditional on line 47 already checks for the `needs-triage` label, which covers both events. For reopened events, we must read the label from `github.event.issue.labels` instead of `github.event.label.name`.

### Task 2: Handle reopened event label extraction
- **File:** `.github/workflows/goose-triage.yml` (modify)
- **Detail:** The workflow uses `github.event.label.name` in step "Validate prerequisites" (line 47) and step "Extract issue context" (embedded script). For `reopened` events, `github.event.label` is undefined. Add logic to detect the event type and extract the label name from `github.event.issue.labels` array when the event is `reopened`. Specifically:

1. In the "Validate prerequisites" step (currently line 47), the `if` condition must handle both cases:
   - For `labeled`: check `github.event.label.name == 'needs-triage'`
   - For `reopened`: check if any label in `github.event.issue.labels` has name `'needs-triage'`

2. In the "Extract issue context" step (around line 77), the script constructs issue context but does not explicitly validate the label. Ensure the validation logic handles both event types.

## Affected Files

```
.github/workflows/goose-triage.yml  (modify: add reopened trigger type and update label extraction logic)
```

## Acceptance Criteria

- Workflow YAML syntax is valid (`yamllint` passes)
- Workflow triggers on `issues: [labeled]` events (existing behavior preserved)
- Workflow triggers on `issues: [reopened]` events when `needs-triage` label is present
- Workflow does not trigger on `issues: [reopened]` events when `needs-triage` label is absent
- No changes to spec generation logic or branch naming

## Out of scope

- Modifying label removal/swapping logic (lines 247-264 remain unchanged)
- Changes to spec auto-approve workflow (`spec-auto-approve.yml`)
- Changes to Goose build workflow (`goose-build.yml`)
- Modifying the sanitise gate or empty output guard
