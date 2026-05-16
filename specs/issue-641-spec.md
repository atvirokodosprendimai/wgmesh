# Specification: Issue #641

## Classification
feature

## Deliverables
documentation

## Problem Analysis

The repository currently has funnel context in two places:

- `docs/index.html` shows a visual "Customer Factory" (Acquisition → Activation → Revenue → Retention → Referral), but it is static and not tied to channel-level economics.
- `docs/pulse-reports/*.md` and `scripts/pulse.sh` expose a few top-level business signals (e.g., paying customers, MRR, GitHub stars), but they do not produce:
  - stage-by-stage conversion rates,
  - channel attribution,
  - customer acquisition cost (CAC) by source,
  - lifetime value (LTV) by source,
  - funnel drop-off analysis with optimization recommendations.

Issue #641 requires a concrete revenue funnel dashboard published at `company/revenue-funnel-metrics.md` that makes acquisition optimization decisions executable.

## Proposed Approach

Create one source-of-truth markdown dashboard at `company/revenue-funnel-metrics.md` with:

1. Explicit funnel stage definitions and stage-entry/exit events.
2. A channel attribution model with formulas for conversion, CAC, attributed revenue, and LTV.
3. A current-state baseline snapshot using latest available repository signals.
4. Drop-off analysis by stage transition.
5. Prioritized optimization recommendations and 30/60/90-day growth projection scenarios.

The document must be implementation-ready and deterministic: all required sections, tables, formulas, and recommendation rules are explicitly defined below.

## Implementation Tasks

### Task 1: Create `company/revenue-funnel-metrics.md` with exact section structure

Create a new file with these top-level headings in this exact order:

1. `# Revenue Funnel Metrics Dashboard (Issue #641)`
2. `## 1. Scope and Update Rules`
3. `## 2. Funnel Stage Definitions`
4. `## 3. Channel Attribution Model`
5. `## 4. Current Baseline Snapshot`
6. `## 5. Conversion Tracking by Stage`
7. `## 6. Channel Performance (CAC, Revenue, LTV)`
8. `## 7. Funnel Drop-off Analysis`
9. `## 8. Acquisition Optimization Recommendations`
10. `## 9. Growth Projection Models (30/60/90 Days)`
11. `## 10. Data Gaps and Instrumentation Backlog`

### Task 2: Define funnel stages and conversion math

In `## 2. Funnel Stage Definitions`, add one markdown table with these exact stage rows and column names:

Columns:
- `Stage`
- `Definition`
- `Entry Event`
- `Exit Event`
- `Primary KPI`

Rows (exact `Stage` names, in order):
1. `Acquisition`
2. `Lead`
3. `Activation`
4. `Revenue`
5. `Retention (30d)`

Immediately below the table, add these exact formulas as bullet points:

- `Lead Conversion Rate = Leads / Acquired Visitors`
- `Activation Rate = Activated Leads / Leads`
- `Revenue Conversion Rate = Paying Customers / Activated Leads`
- `30-Day Retention Rate = Retained Paying Customers (30d) / Paying Customers`
- `End-to-End Funnel Conversion = Paying Customers / Acquired Visitors`

### Task 3: Add attribution rules and channel taxonomy

In `## 3. Channel Attribution Model`, include:

1. A table `Channel Taxonomy` with exact channel rows:
   - `GitHub Organic (stars/issues/README traffic)`
   - `Direct Outreach (founder-led)`
   - `Content/SEO (docs, blog, tutorials)`
   - `Referral/Word-of-mouth`
   - `Paid/Experimental`

2. A bullet list with these exact attribution rules:
   - `Use first-touch channel for acquisition attribution.`
   - `If first-touch is unknown, classify as "Unattributed" and exclude from channel ranking but include in total funnel counts.`
   - `Revenue attribution uses first-touch at customer level for this dashboard version (single-touch model).`
   - `Store confidence flag per channel row: High / Medium / Low based on source completeness.`

### Task 4: Populate baseline snapshot from currently available repo signals

In `## 4. Current Baseline Snapshot`, add a two-column table (`Metric`, `Current Value`) with these required metrics:

- `Active paying customers`
- `MRR`
- `GitHub stars added (last 7d)`
- `External issues opened (last 7d)`
- `Weekly active meshes`
- `Time-to-mesh (p50)`

Use the latest pulse report in-repo (`docs/pulse-reports/2026-05-16_08-03.md`) as baseline source and include a `Source` line directly below the table:

- `Source: docs/pulse-reports/2026-05-16_08-03.md`

When a metric is missing in source data, write `no data (instrumentation pending)` exactly (do not invent values).

### Task 5: Add conversion tracking and drop-off tables

In `## 5. Conversion Tracking by Stage`, create a table with columns:

- `Transition`
- `Numerator`
- `Denominator`
- `Conversion %`
- `Status`

Required transition rows:
- `Acquisition → Lead`
- `Lead → Activation`
- `Activation → Revenue`
- `Revenue → Retention (30d)`

In `## 7. Funnel Drop-off Analysis`, create a table with columns:

- `Transition`
- `Drop-off Formula`
- `Current Drop-off`
- `Primary Suspected Cause`
- `Next Experiment`

Use exact formula format in each row:
- `Drop-off = 1 - Conversion%`

### Task 6: Add channel economics (CAC, revenue attribution, LTV)

In `## 6. Channel Performance (CAC, Revenue, LTV)`, create one table with columns:

- `Channel`
- `Leads`
- `Activated Leads`
- `Paying Customers`
- `Estimated Channel Cost (€)`
- `CAC (€)`
- `Attributed MRR (€)`
- `Estimated LTV (€)`
- `LTV:CAC`
- `Confidence`

Below the table, add these exact formulas:

- `CAC = Channel Cost / Paying Customers`
- `Attributed MRR = Sum(MRR from paying customers attributed to channel)`
- `Estimated LTV = ARPA / Monthly Churn Rate`
- `LTV:CAC = Estimated LTV / CAC`

If denominator is zero, output `n/a` (not infinity or blank).

### Task 7: Add optimization recommendations with deterministic ranking rule

In `## 8. Acquisition Optimization Recommendations`, include exactly 5 recommendations in a numbered list.

Each recommendation must include:
- `Target transition` (one funnel transition),
- `Target channel`,
- `Expected KPI movement`,
- `Implementation effort` (`Low`, `Medium`, or `High`),
- `Priority score`.

Use this exact priority formula for each recommendation:

- `Priority score = (Expected revenue impact 1-5 × Confidence 1-5) / Effort 1-5`

Sort recommendations by descending `Priority score` and include the numeric score.

### Task 8: Add three explicit growth projection scenarios

In `## 9. Growth Projection Models (30/60/90 Days)`, add a table with columns:

- `Scenario`
- `Assumed Lead Growth`
- `Assumed Activation Rate`
- `Assumed Revenue Conversion`
- `Assumed 30d Retention`
- `Projected Paying Customers (90d)`
- `Projected MRR (90d)`

Required scenario rows:
- `Conservative`
- `Base`
- `Aggressive`

Below the table, add one short paragraph stating which scenario is the planning baseline and why.

### Task 9: Explicitly capture data gaps and instrumentation backlog

In `## 10. Data Gaps and Instrumentation Backlog`, add a checklist with at least 6 items, including these exact backlog items:

- `[ ] Track first-touch channel on every new lead`
- `[ ] Track stage transition timestamps for each lead/customer`
- `[ ] Record channel-level spend in a machine-readable source`
- `[ ] Capture retention cohorts by acquisition channel`
- `[ ] Connect checkout/subscription events to attributed channel ID`
- `[ ] Add dashboard refresh cadence (weekly) and owner`

## Affected Files

- `company/revenue-funnel-metrics.md` (new)

## Test Strategy

Documentation-only implementation verification:

1. Confirm the new file exists at the exact path above.
2. Confirm all required section headings and tables exist exactly as specified.
3. Confirm all required formulas are present verbatim.
4. Confirm baseline metrics are sourced from `docs/pulse-reports/2026-05-16_08-03.md` with `no data (instrumentation pending)` where applicable.
5. Confirm recommendations are sorted by descending numeric priority score.

## Estimated Complexity
low
