# Specification: Issue #587

## Classification
feature

## Deliverables
documentation

## Problem Analysis

The repository currently exposes sponsor pricing in `docs/index.html` with three public tiers:

- Founding Member: **$5/mo**
- Edge Node: **$20/mo**
- Mesh Operator: **$100/mo**

The issue reports a monetization baseline of **€4 MRR from 5 active subscribers** with multiple **€1 payments**. This indicates a mismatch between public tier positioning and actual payment behavior (micro-commitments dominating instead of full-tier conversions).

To make pricing decisions executable, we need one concrete analysis artifact that:

1. Documents the current tier model and real customer distribution.
2. Quantifies observable payment behavior from recent orders.
3. Recommends a revised pricing structure with migration steps.
4. Defines specific A/B experiments with success/failure thresholds.

## Proposed Approach

Create a single, implementation-ready pricing analysis document in `docs/plans/` that uses:

- existing tier definitions from `docs/index.html`
- issue-provided baseline metrics (`€4 MRR`, `5 active subscribers`, `multiple €1 payments`)

The document must include computed metrics, concrete pricing recommendations, and an A/B test matrix that can be executed without additional discovery work.

## Implementation Tasks

### Task 1: Create `docs/plans/2026-05-08-001-pricing-optimization-analysis.md`

Create a new markdown document at:

- `docs/plans/2026-05-08-001-pricing-optimization-analysis.md`

Use exactly these top-level sections (in this order):

1. `# Pricing Optimization Analysis (Issue #587)`
2. `## 1. Current Pricing Model`
3. `## 2. Customer Distribution Snapshot`
4. `## 3. Payment Pattern Analysis`
5. `## 4. Price Sensitivity Indicators`
6. `## 5. Recommended Pricing Structure`
7. `## 6. A/B Test Plan`
8. `## 7. 30/60/90-Day Execution Plan`

### Task 2: Fill `Current Pricing Model` with source-of-truth tier table

In section `## 1. Current Pricing Model`, include a markdown table with these exact rows:

| Tier | Listed Price | Checkout Product ID | Source |
|---|---:|---|---|
| Founding Member | $5/mo | 3f5d75de-936b-49d8-a21b-4b79d9fd22c1 | docs/index.html |
| Edge Node | $20/mo | 1927e637-4cfd-4c94-8bee-c5518803bc89 | docs/index.html |
| Mesh Operator | $100/mo | eb20683e-55ea-4354-9d8c-070e55a4eff5 | docs/index.html |

Immediately below the table, add a short note that public listing is in USD while reported MRR in the issue is EUR, so normalization is needed before conversion analysis.

### Task 3: Fill `Customer Distribution Snapshot` with explicit baseline calculations

In section `## 2. Customer Distribution Snapshot`, include these exact baseline values and formulas:

- Active subscribers: `5`
- Current MRR: `€4`
- ARPA (MRR / active subscribers): `€0.80`

Add one interpretation bullet stating that `€0.80` ARPA is materially below the lowest listed tier price, signaling either micro-tier dominance, discounting, or misaligned checkout path.

### Task 4: Fill `Payment Pattern Analysis` using issue-provided order signal

In section `## 3. Payment Pattern Analysis`, include the following required findings as bullets:

- Recent orders include multiple `€1` payments.
- Micro-payments appear to be the dominant observed transaction amount.
- Current value capture is likely concentrated in low-commitment contributions rather than recurring full-tier subscriptions.

Then add a compact `Observed Pattern → Implication` table with at least 3 rows that maps:

1. `Multiple €1 payments` → `High willingness to try, low willingness to commit`
2. `€4 MRR across 5 subscribers` → `Weak recurring monetization depth`
3. `Public tiers start at $5` → `Packaging/price-anchor mismatch`

### Task 5: Add explicit price-sensitivity indicators

In section `## 4. Price Sensitivity Indicators`, include exactly four indicators with status labels (`High`, `Medium`, `Low`):

1. Entry-price friction (status: High)
2. Commitment aversion (status: High)
3. Upsell readiness (status: Medium)
4. Enterprise willingness-to-pay signal (status: Low)

For each indicator, add one sentence explaining why the status is justified by the baseline.

### Task 6: Provide concrete optimized pricing proposal

In section `## 5. Recommended Pricing Structure`, define a 3-tier proposal in EUR with these exact candidate prices:

- Supporter: `€1/mo`
- Builder: `€6/mo`
- Operator: `€24/mo`

For each tier, include:

- target user profile
- included benefits (2–4 bullets)
- conversion role in funnel (entry / core / expansion)

Add a migration note requiring old checkout links to remain valid during rollout and recommending a 14-day dual-display period (old vs new pricing copy).

### Task 7: Create executable A/B test plan

In section `## 6. A/B Test Plan`, define exactly 2 experiments in a table with columns:

- `Experiment`
- `Hypothesis`
- `Variant A`
- `Variant B`
- `Primary Metric`
- `Guardrail`
- `Decision Rule`

Use these experiment definitions:

1. **Price Ladder Test**
   - Variant A: current tier copy/prices
   - Variant B: `€1 / €6 / €24` ladder
   - Primary metric: checkout conversion rate
   - Guardrail: MRR per new payer does not decrease by more than 20%
   - Decision rule: adopt B if conversion increases by at least 30% and guardrail passes

2. **Annual Anchor Test**
   - Variant A: monthly-only pricing
   - Variant B: monthly + annual option with 2 months free equivalent
   - Primary metric: share of annual commitments
   - Guardrail: refund/cancellation rate within 14 days
   - Decision rule: keep annual option if annual-share is >= 20% with no guardrail breach

### Task 8: Add 30/60/90-day execution plan

In section `## 7. 30/60/90-Day Execution Plan`, include:

- **Day 0–30**: instrument tracking fields (tier selected, payment amount, first-payment date, renewal date)
- **Day 31–60**: run both A/B experiments and publish weekly summary
- **Day 61–90**: finalize winning pricing, remove losing variant, publish post-test MRR delta report

End the section with a KPI target line:

- `Target: increase MRR from €4 baseline to at least €15 while maintaining >=5 active subscribers.`

## Affected Files

- `docs/plans/2026-05-08-001-pricing-optimization-analysis.md` (new)

## Test Strategy

Since this is a documentation-only implementation:

1. Verify markdown headings match this spec exactly.
2. Verify all required tables/values are present.
3. Verify all numeric formulas in the document are internally consistent (`€4 / 5 = €0.80`).

## Estimated Complexity
low
