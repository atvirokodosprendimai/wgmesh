# Specification: Issue #643

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

The repository has internal dogfooding data (`docs/dogfooding/README.md` and
`docs/dogfooding/stability-log.md`) and comparative benchmark-style numbers in
use-case docs, but there is no single 1-page, externally readable case study
that:

1. Names the internal topology/workloads clearly.
2. Shows concrete before/after numbers in one place.
3. Quantifies cost/time/complexity deltas.
4. Separates measured facts from assumptions and source links.
5. Provides a short landing-page-ready testimonial block.

Without this, external prospects cannot quickly validate that wgmesh has
credible, defensible internal proof-of-value.

## Proposed Approach

Create one canonical, one-page case study document under `docs/dogfooding/`
that is built from already-documented repository metrics and explicitly cites
each claim source. Then add a compact “testimonial/proof” section to
`docs/index.html` that links to the full case study and displays only sourced
numbers.

## Implementation Tasks

### Task 1: Create the canonical 1-page case study document

**File:** `docs/dogfooding/internal-proof-of-value-case-study.md` (new)

Create a single-page Markdown document with these exact top-level sections (in
this order):

1. `# Internal Case Study: wgmesh Dogfood Proof of Value`
2. `## Scope and Evidence Window`
3. `## Internal Deployment Profile`
4. `## Measured Outcomes`
5. `## Before vs After`
6. `## Cost and Complexity Impact`
7. `## Claim Defensibility`
8. `## Landing-Page Testimonial Snippet`

Populate with concrete values sourced from existing repository docs:

- **Internal usage profile**: node list, environments, workloads from
  `docs/dogfooding/README.md`.
- **Reliability metrics**: attempts/success/failures/success rate and outage
  note from `docs/dogfooding/stability-log.md`.
- **Setup-time comparison (OpenVPN/manual baseline)**: from
  `docs/use-cases/hybrid-site-to-site.md` and
  `docs/use-cases/remote-dev-team.md`.
- **Cost baseline**: cloud VPN gateway monthly range from
  `docs/use-cases/multi-cloud.md`.
- **Tailscale comparison number**: use the setup/pricing values currently
  documented in `specs/issue-551-spec.md` and mark them as “repository internal
  benchmark reference”. If `specs/issue-551-spec.md` is missing or the values
  are absent, omit Tailscale numeric rows and add a `Data gap` note instead of
  inventing replacement values.

Required tables:

1. **Deployment Profile Table** with columns:
   `Node | Role/Workload | Network Type | Runtime Window`.
2. **Measured Outcomes Table** with columns:
   `Metric | Value | Source`.
3. **Before vs After Table** with columns:
   `Dimension | Before (OpenVPN/Tailscale/manual) | After (wgmesh) | Delta`.
4. **Claim Defensibility Matrix** with columns:
   `Claim | Number | Evidence Source | Classification (Measured/Estimated)`.

Formatting/quality rules:

- Keep content to ~1 printed page equivalent (target: 450–900 prose words,
  excluding table cells).
- Do **not** introduce unsourced numbers.
- Every numeric claim must include a source file path in-table.
- Explicitly label each non-measured value as **Estimated**.
- Include one short testimonial quote block that contains only numbers already
  present in the tables.

### Task 2: Add a landing-page testimonial/proof section linking the case study

**File:** `docs/index.html`

Add a new static card section titled `Internal Dogfood Proof` in the business /
traction area (before “Sponsor Benefits”). The section must include:

1. A 2–3 sentence summary.
2. Three numeric proof badges/cards:
   - connection success rate,
   - observed uptime window,
   - setup-time delta (before vs after).
3. A link to
   `./dogfooding/internal-proof-of-value-case-study.md` with anchor text:
   `Read full internal case study`.

Rules:

- Reuse existing CSS token palette/typography conventions in `docs/index.html`.
- Keep section static (no new JS fetch logic).
- Only show numbers that appear in the case-study file.

### Task 3: Add discoverability link from README documentation list

**File:** `README.md`

In the documentation/troubleshooting links area (where
`docs/dogfooding/README.md` is already linked), add one additional bullet:

- `Internal Proof-of-Value Case Study` →
  `docs/dogfooding/internal-proof-of-value-case-study.md`

### Task 4: Manual validation checklist (documentation-only)

No code tests required. Validate by inspection:

1. New case-study document renders correctly (tables, quote block, links).
2. All case-study numeric claims are traceable to repository sources.
3. `docs/index.html` displays the new proof section with no malformed HTML.
4. Link from `docs/index.html` to the case study resolves.
5. Link from `README.md` to the case study resolves.
6. The case study contains explicit before/after numeric comparisons and a
   claim-defensibility matrix (no marketing-only statements).

## Affected Files

- **New:** `docs/dogfooding/internal-proof-of-value-case-study.md`
- **Modified:** `docs/index.html`
- **Modified:** `README.md`

## Test Strategy

Documentation-only change set. Perform manual validation listed in Task 4.

## Estimated Complexity
low
