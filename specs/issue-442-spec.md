# Specification: Issue #442

## Classification
fix

## Deliverables
code

## Problem Analysis

Issue #442 was created on 2026-03-13 by the company observation loop, which detected that `company/costs.json` had all cost fields set to `null` and could not calculate monthly burn or runway.

A human-operator PR (#421, merged 2026-03-14) partially addressed this by filling in cost estimates: €242/month burn across five categories, yielding ~9.9 months runway on €2400 capital. This matched the March 14–15 loop assessments which reported "cost tracking finally configured."

However, on 2026-03-15 commit `e6d4a6b` ("feat: --account flag, company loop migration") removed `company/costs.json`, `company/loop-state.json`, `company/system-prompt.md`, and the `company-loop.yml` workflow as part of migrating the observation loop to the `ai-pipeline-template` repository. As a result:

- `company/costs.json` no longer exists in the repo
- `company/loop-state.json` no longer exists in the repo
- Issue #442 remains open with its acceptance criteria unmet

The values established by PR #421 are preserved in git history (commit `4f51818`). This spec restores these files with their calculated values so any future observation loop or tooling reading `company/` has accurate data.

### Acceptance Criteria (from issue)
- Monthly burn calculated from actual spend ← `costs.json runway.monthly_burn`
- Runway months_remaining populated ← `costs.json runway.months_remaining`
- Cost categories populated with estimates ← `costs.json categories`
- Survival mode threshold (3 months) monitored ← `loop-state.json runway` field

## Proposed Approach

Restore two JSON files under `company/` with the values established by PR #421, updated to reflect the current date and loop run count. Add a `runway` sub-object to `loop-state.json` so any reader of the loop state can see survival-mode status without also reading `costs.json`.

No workflow changes are needed — the company loop is now external. These files serve as static configuration that any future observation loop reads on startup.

## Implementation Tasks

### Task 1: Restore `company/costs.json`

Create the file `company/costs.json` with the following **exact** content:

```json
{
  "last_updated": "2026-03-19",
  "currency": "EUR",
  "runway": {
    "available_capital": 2400,
    "monthly_burn": 240,
    "months_remaining": 10.0,
    "survival_mode": false,
    "note": "Compute is USD-denominated (~20 USD/mo). All others EUR. Runway values maintained manually; the loop reads them."
  },
  "categories": {
    "compute": { "provider": "Hetzner", "monthly_estimate": 18, "note": "~20 USD/mo converted to EUR — VPS for chimney + cloudroof" },
    "dns": { "provider": "bundled", "monthly_estimate": 0 },
    "domains": { "provider": null, "monthly_estimate": 2, "note": "beerpub.dev, cloudroof.eu — annual amortised" },
    "llm": { "provider": "OpenRouter/Anthropic", "monthly_estimate": 200, "note": "daily loop + goose pipeline + dev" },
    "ci": { "provider": "GitHub Actions", "monthly_estimate": 20 }
  },
  "principle": "Can this be zero? > Can this be cheap? > Is this necessary?"
}
```

Field explanations:
- `runway.available_capital`: €2400 total capital on hand
- `runway.monthly_burn`: sum of all category `monthly_estimate` values: 18 + 0 + 2 + 200 + 20 = 240
- `runway.months_remaining`: `available_capital / monthly_burn` = 2400 / 240 = 10.0
- `runway.survival_mode`: `false` because 10.0 months > 3-month threshold
- `categories`: five categories with provider and monthly_estimate; null provider means cost is amortised/shared

### Task 2: Restore `company/loop-state.json`

Create the file `company/loop-state.json` with the following **exact** content:

```json
{
  "funnel_stage": 0,
  "stage_name": "Foundation",
  "stage_entered": "2026-02-28",
  "last_run": "2026-03-15T00:00:00Z",
  "run_count": 18,
  "runway": {
    "months_remaining": 10.0,
    "survival_mode": false,
    "survival_mode_threshold_months": 3
  },
  "history": []
}
```

Field explanations:
- `funnel_stage`: 0 = Foundation (from last committed loop-state before migration)
- `run_count`: 18 (last assessment was run 18 on 2026-03-15)
- `runway.months_remaining`: mirrors `costs.json runway.months_remaining`
- `runway.survival_mode`: `true` when `months_remaining < survival_mode_threshold_months`
- `runway.survival_mode_threshold_months`: 3 — this is the hard-coded threshold from the system spec; survival mode triggers when runway drops below this value

### Task 3: Validate JSON is well-formed

After creating both files, verify they parse correctly:

```bash
jq . company/costs.json
jq . company/loop-state.json
```

Both commands must exit 0 with formatted JSON output. No other code changes are needed.

## Affected Files

- `company/costs.json` — create (was deleted in e6d4a6b)
- `company/loop-state.json` — create (was deleted in e6d4a6b)

No Go source files, workflow files, or test files are modified.

## Test Strategy

1. Run `jq . company/costs.json` — must exit 0 and output valid JSON
2. Run `jq . company/loop-state.json` — must exit 0 and output valid JSON
3. Verify `costs.json` runway calculation: `monthly_burn` equals the sum of all category `monthly_estimate` values: `18 + 0 + 2 + 200 + 20 = 240`
4. Verify `months_remaining`: `available_capital / monthly_burn = 2400 / 240 = 10.0`
5. Verify `survival_mode` is `false` because `months_remaining (10.0) > survival_mode_threshold_months (3)`
6. Verify `loop-state.json runway.months_remaining` matches `costs.json runway.months_remaining`

## Estimated Complexity
low
