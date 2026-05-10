# Specification: Issue #603

## Classification
documentation

## Problem Analysis

Issue #603 requires a public milestone update that reflects real commercial traction:

- 5 active subscribers
- 4 EUR MRR established
- Multiple paid orders (100 EUR each) since March
- Stage 5 Revenue criteria met

Current public-facing assets are stale for this milestone:

1. `README.md` does not explicitly state that wgmesh now has paying customers.
2. `docs/index.html` still shows Revenue as blocked (`0 — needs billing`) and MSC progress at zero.
3. Funnel-stage documentation in `eidos/spec - first-customer - roadmap to first paying customer.md` still defines Stage 5 as a future state instead of achieved.
4. There is no repository-tracked announcement copy for GitHub/social channels, and no concrete publish checklist.

## Proposed Approach

Ship a documentation-only milestone package in four parts:

1. Update public product copy (`README.md`) with a concise “paying customers” milestone section.
2. Update funnel and traction visuals/text in `docs/index.html` to mark Revenue stage as achieved and show current metrics.
3. Update the first-customer funnel spec to record Stage 5 as achieved with the exact evidence points.
4. Add a dedicated announcement document with ready-to-post copy for GitHub + social and a publish checklist, then post the announcement publicly from that copy.

## Implementation Tasks

### Task 1: Update README milestone messaging

**File:** `README.md`

Add a new section directly after the intro paragraph (before `## Motivation`) with this heading and content:

```markdown
## Milestone — Stage 5 Revenue Achieved

wgmesh now has first paying customers.

- **5 active subscribers**
- **€4 MRR established**
- **Multiple paid €100 orders since March**
- **Customer-factory Stage 5 (Revenue) criteria met**
```

Keep wording factual and concise; do not add unverified claims beyond the four metrics above.

### Task 2: Update landing/dashboard funnel status to show Revenue achieved

**File:** `docs/index.html`

Update the static traction/funnel section (Customer Factory + MSC summary) as follows:

1. In MSC progress block (`<div class="msc-bar" ...>` near the “Minimum Success Criteria — 3-Year Target: $100K ARR” heading):
   - Change `.msc-current` from `0` to `5`.
   - In the sibling `<span style="color:var(--muted);font-size:0.875rem">...`, replace text `of 1 customer (90-day target)` with `5 active subscribers · Stage 5 Revenue achieved`.

2. In Customer Factory funnel:
   - Change Revenue stage container class from `funnel-stage blocked` to `funnel-stage ready`.
   - Change Revenue metric text from `Paid subscriber` to `Paying subscribers`.
   - Change Revenue status text from `0 — needs billing` to `5 active · €4 MRR · paid orders since March` and color to green.

3. In the “single biggest constraint” sentence under the funnel:
   - Replace the current “First paying customer ... Need billing integration + first customer...” sentence with:
     - Stage 5 achieved language
     - Explicit mention of 5 active subscribers, €4 MRR, and multiple €100 paid orders since March
     - Next focus: retention and referrals (not first purchase).

4. In horizon cards:
   - Mark `1 paying customer from personal network` as completed (strikethrough + checkmark style consistent with neighboring completed items).
   - Mark `4 paying customers ($20/mo)` as completed and annotate actual result (`5 active subscribers`).

Do not change unrelated dashboard logic, scripts, map, or pipeline tables.

### Task 3: Update funnel stage documentation to record Stage 5 as achieved

**File:** `eidos/spec - first-customer - roadmap to first paying customer.md`

In the `### The funnel` subsection, update the Stage 5 entry from future-tense to achieved state:

- Keep the stage name `Stage 5: Revenue`.
- Replace the existing exit criterion text with an achieved-status statement that includes all four issue metrics:
  - 5 active subscribers
  - €4 MRR
  - multiple paid €100 orders since March
  - Stage 5 criteria met
- Add one explicit sentence stating the next operational constraint is retention (30-day activity) and referral growth.

Do not rewrite other stages (0–4) except for minimal context needed for consistency.

### Task 4: Add milestone announcement copy for GitHub + social

**New file:** `docs/announcements/2026-05-stage-5-revenue.md`

Create a publish-ready announcement document with these exact sections:

1. `# wgmesh milestone: Stage 5 Revenue achieved`
2. `## Verified metrics`
   - List the four issue metrics exactly.
3. `## GitHub announcement copy`
   - 1 long-form post suitable for GitHub (4–8 short paragraphs) including:
     - what was achieved
     - why it matters
     - thanks to users/customers
     - CTA to README/quickstart/checkout links
4. `## Social post copy (short)`
   - 1 X/Twitter-sized version
   - 1 LinkedIn-sized version
5. `## Publish checklist`
   - [ ] Post GitHub announcement (discussion or repo update)
   - [ ] Post social copy publicly
   - [ ] Link back to `README.md` and `docs/index.html` milestone updates

Keep all numbers exactly aligned with the issue statement; do not introduce additional financial claims.

### Task 5: Celebrate publicly and capture proof

After Tasks 1–4 are merged:

1. Publish the prepared announcement text publicly (GitHub + at least one social channel).
2. Add a follow-up comment on issue #603 summarizing:
   - where it was posted
   - confirmation that README/landing/funnel docs were updated
   - link to `docs/announcements/2026-05-stage-5-revenue.md`.

This task is required to satisfy “Celebrate the achievement publicly.”

## Affected Files

- **Modified:** `README.md`
- **Modified:** `docs/index.html`
- **Modified:** `eidos/spec - first-customer - roadmap to first paying customer.md`
- **New:** `docs/announcements/2026-05-stage-5-revenue.md`

## Test Strategy

1. Content verification with ripgrep:
   - `rg "Stage 5 Revenue achieved|5 active subscribers|€4 MRR|€100" README.md docs/index.html "eidos/spec - first-customer - roadmap to first paying customer.md" docs/announcements/2026-05-stage-5-revenue.md`
2. Manual rendering check:
   - Open `README.md` in GitHub preview.
   - Open `docs/index.html` in browser and verify MSC + funnel Revenue stage text/classes.
3. Consistency check:
   - Ensure all four metric claims are identical across all updated files.
4. Public-post proof:
   - Confirm issue #603 comment includes links to the public announcement post(s) and the announcement doc.

## Estimated Complexity
low
