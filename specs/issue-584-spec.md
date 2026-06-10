# Issue #584 Specification: Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages

## Classification
feature

## Problem Analysis

**Current State:**
The wgmesh.dev landing page (`wgmesh.dev/index.html`) includes Polar.sh checkout links embedded within the pricing section. Each pricing tier card contains a CTA button with a direct Polar.sh checkout URL. However, there is no visibility into:

1. Whether these CTAs are actually converting (no click tracking, no analytics)
2. Whether cloudroof.eu has a corresponding landing page with similar CTAs
3. Whether the CTAs are optimally positioned, styled, or copy-optimized

**Business Context (from ROADMAP.md):**
- **Horizon 1, Item #1:** "Polar.sh billing wired up (#376)" flagged as conversion blocker per first-customer brainstorm
- **Roadmap rationale:** "GitHub Sponsors has too many steps" — Polar.sh is intended to reduce friction
- **Strategic alignment:** The landing page repositioning (Item #3) calls for making the site "legible to ICP above the fold" — CTAs are part of that funnel

**Pain Points:**
1. **No conversion visibility:** Without tracking, we cannot optimize CTA placement or copy
2. **Inconsistent implementation:** cloudroof.eu CTAs may be missing or misaligned with wgmesh.dev
3. **Conversion blockers:** If checkout flows are broken, we're leaking potential $5/mo+ customers
4. **No A/B testing foundation:** Current implementation doesn't support iterative optimization

**What We're NOT Solving:**
- Landing page content strategy (copywriting, positioning, imagery)
- Polar.sh account setup or product configuration
- Backend conversion analytics infrastructure (that's a separate concern)

## Proposed Approach

### Phase 1: Audit & Standardize (Immediate)

**1.1 Inventory Existing CTAs**
- Audit all CTAs on wgmesh.dev/index.html
- Document current Polar.sh product IDs and checkout URLs
- Check if cloudroof.eu landing page exists and what CTAs are present

**1.2 Verify Checkout Links**
- Test each Polar.sh checkout URL for validity
- Confirm product IDs match the intended tiers:
  - Founding Member: `productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
  - Edge Node: `productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
  - Mesh Operator: `productId=eb20683e-55ea-4354-9d8c-070e55a4eff5`

**1.3 Standardize CTA Attributes**
- Add consistent `rel="noopener noreferrer"` to all external checkout links
- Ensure all CTAs use the same CSS class (`.cta-button`)
- Add descriptive `aria-label` attributes for accessibility

### Phase 2: cloudroof.eu Implementation (If Missing)

**2.1 Create/Update cloudroof.eu Landing Page**
- If cloudroof.eu landing page doesn't exist, create a minimal page
- If it exists, audit it for missing CTAs
- Align CTAs with wgmesh.dev pricing tiers and checkout URLs

**2.2 Consistent Design Language**
- Match button styles, colors, and hover states to wgmesh.dev
- Ensure responsive behavior matches (mobile, tablet, desktop)

### Phase 3: CTA Optimization (Foundation)

**3.1 Click Tracking Foundation**
- Add UTM parameters to Polar.sh checkout URLs:
  - `utm_source=wgmesh.dev` (or `cloudroof.eu`)
  - `utm_medium=landing-page`
  - `utm_campaign=pricing-tier-{tier-name}`
  - `utm_content={cta-location}`
- This enables basic analytics without implementing full tracking infrastructure

**3.2 CTA Copy Review**
- Review button text for clarity and action-orientation
- Ensure "Become a Founding Member", "Sponsor as Edge Node", etc. are compelling
- Consider adding urgency or benefit language if appropriate

**3.3 Placement Review**
- Confirm CTAs are in optimal locations (currently: within pricing cards)
- Consider if additional CTAs are needed elsewhere (hero section, footer, etc.)
- Document placement decisions for future optimization

### Phase 4: Validation & Documentation

**4.1 Cross-Browser/Device Testing**
- Test CTAs on major browsers (Chrome, Firefox, Safari, Edge)
- Test on mobile devices (iOS Safari, Chrome Mobile)
- Verify checkout links open in new tabs correctly

**4.2 Documentation**
- Document all Polar.sh product IDs and checkout URLs in project docs
- Create a runbook for updating checkout links if Polar.sh changes them
- Note any analytics tracking setup for future reference

## Acceptance Criteria

### Must Have (P0)
- [ ] All Polar.sh checkout URLs on wgmesh.dev are verified and functional
- [ ] All checkout URLs include `target="_blank" rel="noopener noreferrer"`
- [ ] cloudroof.eu landing page exists with matching Polar.sh CTAs (or clear documentation of why not)
- [ ] All checkout URLs have UTM parameters for basic attribution: `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`
- [ ] CTAs use consistent styling (same CSS class, colors, hover states)
- [ ] CTAs have accessible `aria-label` attributes

### Should Have (P1)
- [ ] Document inventory of all Polar.sh product IDs in use
- [ ] Document process for updating checkout URLs in future
- [ ] CTA copy reviewed and standardized across both domains
- [ ] Mobile-responsive behavior verified for all CTA placements

### Nice to Have (P2)
- [ ] Additional CTA placements beyond pricing section (if warranted)
- [ ] A/B testing framework documented for future copy/color experiments
- [ ] Analytics integration plan documented (even if not implemented)

## Out of Scope

- **Landing page redesign:** Visual design, layout changes, content strategy
- **Polar.sh configuration:** Account setup, product tier pricing changes, webhook setup
- **Analytics infrastructure:** Implementation of tracking pixels, conversion events, or dashboard
- **Marketing optimization:** A/B testing platforms, heatmaps, user recording tools
- **SEO optimization:** Meta tags, structured data, or search-focused content changes
- **Backend changes:** No wgmesh daemon or CLI modifications required
- **New payment flows:** Integration with alternative payment processors beyond Polar.sh

---

## Implementation Notes

**Files Expected to Modify:**
- `wgmesh.dev/index.html` — CTA standardization, UTM parameter addition, accessibility
- `public/index.html` (if this is the prod landing page) — mirror wgmesh.dev changes
- `docs/` or `specs/` — documentation of product IDs and runbooks

**Estimated Complexity:** Low — primarily frontend markup changes with documentation. No backend logic or infrastructure changes required.

**Dependencies:** 
- Polar.sh account access to verify product IDs (if not already documented)
- DNS/routing confirmation for cloudroof.eu (if creating new page)

**Success Metrics (Post-Implementation):**
- All checkout URLs return HTTP 200 and load Polar.sh checkout flow
- UTM parameters visible in Polar.sh analytics (if accessible)
- No console errors related to CTA links on either domain
- Mobile rendering matches desktop intent (buttons clickable, visible)
