# Specification: Issue #609

## Classification
feature

## Deliverables
documentation

## Problem Analysis

The current funnel definition in `/home/runner/work/wgmesh/wgmesh/eidos/spec - first-customer - roadmap to first paying customer.md` treats Stage 5 (Revenue) as “first invoice paid” with only a broad retention note. This is too loose now that the company already has 5 active subscribers and €0.04 MRR.

Without explicit Stage 5 exit thresholds and a defined Stage 6 target state, the loop cannot consistently decide whether to keep optimizing early monetization or switch to growth/retention execution. The system prompt also lacks explicit post-first-revenue guidance, which risks ambiguous or low-leverage action selection.

## Proposed Approach

Define a concrete Stage 5 exit gate and introduce a new Stage 6 (Growth) with measurable objectives tied to retention, usage quality, and expansion revenue. Update both the funnel roadmap doc and the company loop system prompt so the LLM uses the same thresholds when assessing stage progression.

## Implementation Tasks

### Task 1: Update funnel stage definitions in `eidos/spec - first-customer - roadmap to first paying customer.md`

Edit `/home/runner/work/wgmesh/wgmesh/eidos/spec - first-customer - roadmap to first paying customer.md` in the `### The funnel` section.

1. Keep Stage 0–4 definitions unchanged.
2. Replace the current Stage 5 block with the exact block below.
3. Add the new Stage 6 block immediately after Stage 5.

Use this exact markdown block:

```markdown
- **Stage 5: Revenue** — early recurring revenue exists but is still fragile
  - Entry when: first successful paid subscription exists
  - Exit when all conditions hold for a trailing 30-day window:
    - `>= 5` paying subscribers are still active after 30+ days from first payment
    - MRR is `>= €0.04` for 30 consecutive days (no full drop to zero)
    - 30-day gross customer retention is `>= 80%`
    - Usage baseline exists for all paying subscribers (at least one `wgmesh` activity signal per subscriber in the last 14 days)
- **Stage 6: Growth** — recurring revenue is real; focus shifts to compounding growth efficiency
  - Objective: scale paying usage without collapsing retention quality
  - Success milestones (evaluate monthly):
    - Net subscriber growth `>= 20%` month-over-month
    - Net MRR growth `>= 25%` month-over-month
    - 30-day gross customer retention `>= 85%`
    - At least `1` expansion revenue event per month (upgrade, add-on, or seat increase)
  - Primary risks to monitor: churn spikes, inactive paid subscribers, growth driven only by one-off low-value payments
```

### Task 2: Update stage-exit verification bullets in the same roadmap file

In `/home/runner/work/wgmesh/wgmesh/eidos/spec - first-customer - roadmap to first paying customer.md`, update the `## Verification` list so post-revenue checks align with the new stage model.

1. Keep existing verification bullets unrelated to Stage 5/6 as-is.
2. Replace the current bullet `First payment received (Stage 5 exit)` with these two bullets:

```markdown
- Stage 5 exit evidence recorded: `>=5` active 30+ day subscribers, `>=€0.04` MRR sustained for 30 days, retention and usage thresholds met
- Stage 6 monthly scorecard recorded: subscriber growth, MRR growth, retention, and expansion revenue trend
```

### Task 3: Create a loop system prompt file with explicit Stage 5/6 logic

Create `/home/runner/work/wgmesh/wgmesh/company/system-prompt.md` (new file).

Add the following required sections in this exact order:

1. `# Company Loop System Prompt`
2. `## Funnel Stage Definitions`
3. `## Stage Transition Rules`
4. `## Required Metrics for Stage 5 and Stage 6`
5. `## Output Requirements`

Populate it with concrete instructions that mirror Task 1 thresholds exactly (same numeric gates). Include all of the following mandatory rules:

- The model must not declare Stage 5 exit unless **all** Stage 5 conditions are satisfied in the same trailing 30-day window.
- If Stage 5 conditions pass, the model must classify as Stage 6 and prioritize growth/retention actions over “first payment” actions.
- Stage 6 assessments must always report: subscriber MoM growth, MRR MoM growth, 30-day retention, expansion revenue count, and top churn risk.
- If any Stage 6 milestone is missed for 2 consecutive months, the model must flag `needs-human` and propose a retention recovery plan.

For `## Output Requirements`, require the model to emit a compact JSON object containing at minimum:

- `stage_current`
- `stage_confidence`
- `stage_5_exit_checks` (object with pass/fail per criterion)
- `stage_6_growth_checks` (object with pass/fail per milestone)
- `top_actions` (array, max 3)
- `needs_human` (boolean)

### Task 4: Keep eidos template stage list in sync

Edit `/home/runner/work/wgmesh/wgmesh/eidos/spec - ai pipeline template - autonomous product loop for ai-native startups.md`.

Find the bullet that currently lists funnel stages as:

- `Foundation → Dogfood → Presence → Reachable → Pipeline → Revenue`

Replace it with:

- `Foundation → Dogfood → Presence → Reachable → Pipeline → Revenue → Growth`

Do not change any other bullets in that section.

## Affected Files

- `/home/runner/work/wgmesh/wgmesh/specs/issue-609-spec.md` (new)
- `/home/runner/work/wgmesh/wgmesh/eidos/spec - first-customer - roadmap to first paying customer.md`
- `/home/runner/work/wgmesh/wgmesh/company/system-prompt.md` (new)
- `/home/runner/work/wgmesh/wgmesh/eidos/spec - ai pipeline template - autonomous product loop for ai-native startups.md`

## Test Strategy

This is a documentation/spec change.

Validation steps for the implementation PR:

1. Confirm both eidos docs contain the updated Stage 5/6 wording exactly as specified.
2. Confirm `company/system-prompt.md` exists with all required headings and numeric gates.
3. Confirm Stage 5 numeric thresholds are identical across the roadmap doc and system prompt.
4. Confirm Stage 6 output requirements include all mandated JSON keys.
5. Confirm no non-documentation code files are modified.

## Estimated Complexity
low
