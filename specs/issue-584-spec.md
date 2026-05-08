# Specification: Issue #584

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

This issue is a revenue-path gap, not a product or infrastructure gap. The three Polar checkout
products already exist and are already wired in `docs/index.html`:

- Founding Member — `$5/mo` — `https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
- Edge Node — `$20/mo` — `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
- Mesh Operator — `$100/mo` — `https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5`

The customer-facing surfaces named in the issue do not expose those checkouts:

1. **`wgmesh.dev`** is sourced from the separate repository `atvirokodosprendimai/wgmeshdev`. The
   live landing page already has the sections `#features`, `#how-it-works`, `#install`, `#modes`,
   and `#story`, but no pricing/sponsor section and no Polar checkout CTA.
2. **`cloudroof.eu`** currently serves the anycast deck from
   `evolution/wgmesh-cdn-slides.html`. The deck ends with a summary slide and has no dedicated sales
   CTA and no Polar footer link.
3. The acceptance criteria require direct conversion paths on both pages, plus manual verification
   that the checkout UI loads for every CTA without completing a purchase.

This is explicitly landing-page work only. Do not create new Polar products, do not invent new
pricing, and do not add metered bandwidth commerce in this issue.

## Proposed Approach

Reuse the already-published sponsor tier structure from `docs/index.html` as the canonical source
for product names, prices, benefit bullets, checkout URLs, CTA labels, and the `Polar.sh` org link.

Implementation should be split into two concrete edits:

1. **`wgmesh.dev` landing page (external repo `atvirokodosprendimai/wgmeshdev`)**
   - Find the existing root landing-page source file by searching for the known section anchors
     `#features`, `#how-it-works`, `#install`, `#modes`, and `#story`.
   - Add a new above-the-fold section with `id="pricing"` (preferred) or `id="sponsor"` directly
     after the hero block and before the existing `#features` section.
   - Render the three existing tiers with direct Polar checkout buttons.
   - Add a footer link with the exact visible text `Payment via Polar.sh`.

2. **`cloudroof.eu` slide deck (`evolution/wgmesh-cdn-slides.html` in this repo)**
   - Add one new final slide after the existing summary slide.
   - Make that final slide a single-purpose sales CTA for the `$20` Edge Node checkout only.
   - Add a persistent page footer link with the exact visible text `Payment via Polar.sh`.
   - Update slide navigation/counting so the new final slide is reachable via scroll, nav dots, and
     keyboard navigation.

Do not redesign the entire pages. Keep the existing visual systems and make the smallest possible
landing-page-only additions that satisfy the acceptance criteria.

## Implementation Tasks

### Task 1: Use the existing Polar URLs and CTA copy as the single source of truth

Before editing anything, copy the exact values already present in `docs/index.html`:

- **Founding Member**
  - Price: `$5/mo`
  - Checkout URL:
    `https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
  - Button label: `Become a founding member →`
- **Edge Node**
  - Price: `$20/mo`
  - Checkout URL:
    `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
  - Button label: `Reserve your edge node →`
- **Mesh Operator**
  - Price: `$100/mo`
  - Checkout URL:
    `https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5`
  - Button label: `Become a mesh operator →`
- **Footer / payment link**
  - Visible text: `Payment via Polar.sh`
  - Link target: `https://polar.sh/atvirokodosprendimai`

Do not change product IDs, prices, or checkout destinations in this issue.

### Task 2: Add an above-the-fold pricing section to the `wgmesh.dev` landing page

**Repository:** `atvirokodosprendimai/wgmeshdev`  
**Target file:** the single root landing-page source file that already renders the existing section
anchors `#features`, `#how-it-works`, `#install`, `#modes`, and `#story`.

Make exactly these content changes in that landing-page file:

1. Insert a new section with `id="pricing"` immediately **after the hero section** and **before**
   the section whose root anchor is `#features`. The section must therefore appear above the fold on
   desktop and before the main feature walkthrough.
2. Give the section a heading that clearly signals sponsorship / checkout, for example:
   - `Pricing`
   - `Sponsor wgmesh`
   Either is acceptable, but keep the section id as `pricing` if possible.
3. Add a short one-sentence intro below the heading that says these are the live founding-customer
   / sponsor tiers for supporting wgmesh.
4. Render **three tier cards** in this exact order:
   - Founding Member
   - Edge Node
   - Mesh Operator
5. Each card must contain:
   - Tier name
   - Price (`$5/mo`, `$20/mo`, `$100/mo`)
   - 3–5 concise benefit bullets copied from the existing sponsor card in `docs/index.html`
   - A direct checkout button pointing to the exact Polar checkout URL from Task 1
6. Preserve the page’s existing responsive layout system. On mobile, the tier cards may stack; on
   desktop, they should appear as a row or responsive grid.
7. Every Polar checkout button must open in a new tab/window and include `rel="noopener noreferrer"`.
8. Do **not** remove or rename any existing anchors (`#features`, `#how-it-works`, `#install`,
   `#modes`, `#story`) because they are already linked from the live page.
9. In the existing page footer, add a new link with the exact visible text `Payment via Polar.sh`
   pointing to `https://polar.sh/atvirokodosprendimai`, also opening in a new tab with
   `rel="noopener noreferrer"`.

If the landing page already has a nav/header link list, add a `Pricing` in-page link pointing to
`#pricing`. Do not create a separate pricing page or a modal checkout flow.

### Task 3: Add a final-slide Edge Node CTA to `evolution/wgmesh-cdn-slides.html`

**Repository:** this repository  
**File:** `evolution/wgmesh-cdn-slides.html`

Modify the slide deck as follows:

1. Leave all existing slides intact. Add **one new final slide** after the current last slide
   (`id="slide-7"` in the current file).
2. Give the new slide a new sequential id (for example `id="slide-8"`), and update all slide-deck
   bookkeeping to match:
   - add one nav dot
   - ensure `slides.length` reports the new slide
   - ensure the visible total changes from `08` to `09`
   - ensure keyboard navigation can reach the new slide
3. Keep the existing deck design language:
   - reuse the existing `.slide`, `.content`, `.tag`, `.card`, `.highlight-box`, and typography
     patterns where possible
   - add only the minimal new CSS needed for the CTA button/footer if no matching class exists
4. The new final slide must be a focused sales CTA for the **Edge Node** tier only:
   - headline referencing Edge Node sponsorship / reservation
   - price shown as `$20/mo`
   - 2–4 supporting bullets copied or condensed from the existing Edge Node card in
     `docs/index.html` (for example beta access queue, private channel, README logo)
   - one primary CTA button linking directly to
     `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
   - button text should reuse `Reserve your edge node →`
5. The CTA button must open in a new tab/window and include `rel="noopener noreferrer"`.
6. Add a **page footer link** for the slide deck with the exact visible text `Payment via Polar.sh`
   pointing to `https://polar.sh/atvirokodosprendimai`.
   - Prefer a small persistent footer anchored near the bottom-right of the viewport so it is
     visible on every slide without obscuring the content.
   - If that conflicts with the existing fixed slide counter / nav, place the footer at the bottom
     of the new final slide instead — but keep it styled as a footer element, not body copy.

Do not add the `$5` or `$100` checkout buttons to the slide deck. The acceptance criterion for
`cloudroof.eu` is only the `$20` Edge Node final-slide CTA plus the Polar footer link.

### Task 4: Manually verify every checkout CTA loads the Polar checkout UI

Do not add automated tests for this issue. Perform manual verification only.

#### 4.1 `wgmesh.dev` verification

After implementing Task 2 in `atvirokodosprendimai/wgmeshdev`:

1. Open the landing page locally or via preview deployment.
2. Confirm the new `#pricing` section appears before `#features`.
3. Click all three tier buttons:
   - Founding Member
   - Edge Node
   - Mesh Operator
4. For each click, verify the destination loads Polar’s checkout UI (product title, checkout shell,
   and hosted Polar page chrome). Do **not** complete payment.
5. Confirm the page footer contains the `Payment via Polar.sh` link and that it opens the Polar org page.

#### 4.2 `cloudroof.eu` verification

After implementing Task 3 in `evolution/wgmesh-cdn-slides.html`:

1. Open the HTML file locally in a browser.
2. Navigate to the new last slide using:
   - mouse/trackpad scroll
   - nav dot click
   - keyboard arrow navigation
3. Confirm the visible total slide count shows `09`.
4. Click `Reserve your edge node →` and verify the Polar checkout UI loads for the `$20` product.
5. Confirm the slide page exposes the `Payment via Polar.sh` footer link and that it opens the
   Polar org page.

Capture screenshots of:

1. the new `wgmesh.dev` pricing section
2. the new `cloudroof.eu` final CTA slide

Include those screenshots in the implementation PR description or review comment so reviewers can
see the landing-page changes without pulling the branch locally.

## Affected Files

- **New (this repo):** `specs/issue-584-spec.md`
- **Modify during implementation (external repo):** the root landing-page source file in
  `atvirokodosprendimai/wgmeshdev` that renders `#features`, `#how-it-works`, `#install`,
  `#modes`, and `#story`
- **Modify during implementation (this repo):** `evolution/wgmesh-cdn-slides.html`

## Test Strategy

Manual validation only:

1. Verify the `wgmesh.dev` landing page contains a new above-the-fold `#pricing` section with three
   direct Polar checkout CTAs.
2. Verify `wgmesh.dev` preserves the pre-existing anchor sections (`#features`, `#how-it-works`,
   `#install`, `#modes`, `#story`).
3. Verify the slide deck has a new last slide dedicated to the `$20` Edge Node offer.
4. Verify both pages expose a visible `Payment via Polar.sh` link.
5. Verify each checkout CTA opens the Polar-hosted checkout UI, without completing a purchase:
   - 3 CTAs on `wgmesh.dev`
   - 1 CTA on `cloudroof.eu`

## Estimated Complexity

medium
