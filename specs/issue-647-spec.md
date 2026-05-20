# Specification: Issue #647

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

Issue #647 reports a data contradiction:

- Revenue signals show **5 recent paid orders**.
- All three seeded cloudroof/wgmesh checkout products show **0 active subscribers**.
- A specific product ID (`8e8e1c33-cd06-4652-9032-6cb3b49ec6b4`) appears to be driving revenue but is not mapped in repository reporting.

Current repository reporting cannot explain this discrepancy:

1. `scripts/pulse.sh` only queries Polar **subscriptions** (`GET /v1/subscriptions`) and computes aggregate active count + MRR; it does not attribute by product ID.
2. `scripts/pulse.sh` does not query Polar **orders**, so one-time/alternate-product payments are invisible.
3. Seed product IDs are only embedded in landing pages (`docs/index.html`, `public/index.html`, `wgmesh.dev/index.html`) and are not used as a canonical attribution list in reporting logic.
4. Pulse outputs in `docs/pulse-reports/*.md` therefore can show `0 active subscriptions` even when paid orders exist on a different product.

Without product-level attribution, the team cannot answer whether revenue is coming from wgmesh/cloudroof offers or unrelated products in the same Polar organization.

## Proposed Approach

Add deterministic Polar revenue attribution to pulse generation and ship a one-page investigation report for Issue #647.

The implementation must:

1. Treat the three checkout IDs published in repo as the **seed product set**.
2. Pull both Polar subscriptions and Polar paid orders.
3. Produce per-product attribution (orders, paid revenue, active subscriptions) for the same reporting window.
4. Resolve and document what `8e8e1c33-cd06-4652-9032-6cb3b49ec6b4` is, and whether it is wgmesh/cloudroof-related.
5. Persist findings in a committed markdown artifact, and include PMF learnings + instrumentation backlog so future pulse reports do not regress.

## Implementation Tasks

### Task 1: Add canonical seed product registry and attribution helpers in `scripts/pulse.sh`

Modify `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh`.

Add these constants near existing Polar config:

- `POLAR_SEED_PRODUCT_IDS` containing exactly:
  - `3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
  - `1927e637-4cfd-4c94-8bee-c5518803bc89`
  - `eb20683e-55ea-4354-9d8c-070e55a4eff5`
- `POLAR_INVESTIGATED_PRODUCT_ID="8e8e1c33-cd06-4652-9032-6cb3b49ec6b4"`

Add helper logic to classify each observed product into one of:

- `seed`
- `non-seed`
- `unknown` (missing product ID)

Use exact rule order:

1. If product ID is empty/null → `unknown`.
2. If product ID is in `POLAR_SEED_PRODUCT_IDS` → `seed`.
3. Otherwise → `non-seed`.

### Task 2: Extend Polar data collection to include paid orders and product metadata

In `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh`, extend `query_polar`.

Required API calls (same auth header pattern already used):

1. Existing: `GET /v1/subscriptions?limit=100` (keep)
2. New: `GET /v1/orders?limit=100` with pagination handling parallel to subscriptions
3. New: Product lookup for each unique product ID seen in subscriptions or orders:
   - `GET /v1/products/{id}`

For orders, count only successfully paid/completed orders. Treat an order as paid when status is one of:

- `paid`
- `succeeded`
- `completed`

(If none of these statuses exist in a response payload, fall back to current amount > 0 and not refunded/cancelled heuristics, and document the fallback in output.)

### Task 3: Compute and expose per-product attribution metrics

In `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh`, compute for each product ID in the reporting window:

- `paid_orders_count`
- `paid_orders_revenue_cents`
- `active_subscriptions_count`
- `active_subscriptions_mrr_cents`
- `seed_class` (`seed`/`non-seed`/`unknown`)

Also compute rollups:

- `seed_paid_orders_count`
- `seed_active_subscriptions_count`
- `non_seed_paid_orders_count`
- `non_seed_active_subscriptions_count`

Ensure existing top-line metrics are preserved and backward-compatible.

### Task 4: Add a dedicated “Revenue attribution” section to pulse markdown output

Modify the report rendering block in `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh`.

Add a new section after `## Usage` and before `## System performance`:

- Heading: `## Revenue attribution (Polar)`
- Include a markdown table with columns:
  - `Product ID`
  - `Product Name`
  - `Seed Class`
  - `Paid Orders (window)`
  - `Paid Revenue (window)`
  - `Active Subs`
  - `Active MRR`
  - `wgmesh/cloudroof relation`

For `wgmesh/cloudroof relation`, use deterministic rule:

- `yes` if product ID is seed, or product name/description contains `wgmesh` or `cloudroof` (case-insensitive)
- `no` otherwise
- `unknown` if product metadata unavailable

Add one summary bullet under the table that explicitly states whether `8e8e1c33-cd06-4652-9032-6cb3b49ec6b4` is `seed` or `non-seed` and whether it is `yes/no/unknown` related.

### Task 5: Create investigation artifact with explicit findings and PMF learnings

Create `/home/runner/work/wgmesh/wgmesh/docs/plans/2026-05-20-001-revenue-attribution-seed-vs-non-seed.md`.

Use this exact section order:

1. `# Revenue Attribution Investigation (Issue #647)`
2. `## 1. Scope`
3. `## 2. Seed Product Baseline`
4. `## 3. Observed Revenue Product Mapping`
5. `## 4. wgmesh/cloudroof Relationship Assessment`
6. `## 5. Product-Market-Fit Learnings`
7. `## 6. Revenue Tracking Improvements`
8. `## 7. Execution Checklist`

Required content constraints:

- In section 2, include the three seed product IDs and their source file references.
- In section 3, include product ID `8e8e1c33-cd06-4652-9032-6cb3b49ec6b4` and observed order/subscriber metrics.
- In section 4, include explicit verdict: `related`, `unrelated`, or `inconclusive`, with evidence.
- In section 5, include at least 3 concrete PMF learnings explaining the seed-vs-non-seed discrepancy.
- In section 6, include at least 5 concrete tracking improvements (owner + metric + data source + cadence).

### Task 6: Add guardrails so future pulses always attribute revenue by product class

In `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh` and generated pulse output:

- Add followup item when paid orders exist but seed active subscriptions are zero.
- Add followup item when revenue is dominated by non-seed products.
- Keep existing followup behavior intact.

### Task 7: Update docs reference for strategy metric interpretation

Update `/home/runner/work/wgmesh/wgmesh/STRATEGY.md` metric wording for “Paying customers” to clarify that:

- subscription count alone is insufficient,
- product-attributed paid orders + active subscriptions are both required,
- seed vs non-seed split is a mandatory reporting dimension.

Keep the rest of STRATEGY sections unchanged.

## Affected Files

- `/home/runner/work/wgmesh/wgmesh/specs/issue-647-spec.md` (new, this spec)
- `/home/runner/work/wgmesh/wgmesh/scripts/pulse.sh` (modify)
- `/home/runner/work/wgmesh/wgmesh/docs/plans/2026-05-20-001-revenue-attribution-seed-vs-non-seed.md` (new)
- `/home/runner/work/wgmesh/wgmesh/STRATEGY.md` (modify)
- `/home/runner/work/wgmesh/wgmesh/docs/pulse-reports/*.md` (regenerated by pulse workflow)

## Test Strategy

Implementation PR validation steps:

1. Run `bash -n /home/runner/work/wgmesh/wgmesh/scripts/pulse.sh`.
2. Run pulse locally without token and confirm graceful fallback still produces a report.
3. Run pulse with valid Polar token and confirm:
   - product table is present,
   - `8e8e1c33-cd06-4652-9032-6cb3b49ec6b4` appears in attribution output when present in Polar data,
   - seed/non-seed rollups are shown.
4. Verify all existing pulse sections remain present and unchanged in order except for the added attribution section.
5. Verify investigation doc includes required verdict + PMF learnings + tracking backlog.

## Estimated Complexity
medium
