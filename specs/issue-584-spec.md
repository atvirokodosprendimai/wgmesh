# Specification: Issue #584

## Classification
feature

## Deliverables
code + documentation

## Summary

Add Polar.sh checkout call-to-action (CTA) buttons and links to both wgmesh.dev and cloudroof.eu landing pages to enable paid subscription signup for the managed ingress service (Bet A commercial pipe). This is the primary conversion path for users to become paying customers.

## Context

### Current State
- wgmesh.dev and cloudroof.eu landing pages exist but lack clear monetization paths
- No integration with Polar.sh billing system on the marketing sites
- Users cannot easily discover how to sign up for paid managed ingress service
- STRATEGY.md defines "Paying customers — count of active Polar.sh subscriptions" as a key metric

### Business Context
From STRATEGY.md and brainstorm docs:
- **Bet A commercial pipe**: Polar.sh billing, landing repositioning at cloudroof.eu
- **Primary users**: LLM agent builders running stacks like Hermes/openclaw — devs not devops
- **Billing model**: Flat monthly tier via Polar.sh (EU MoR)
- **Target flow**: User visits landing page → clicks CTA → Polar.sh checkout → subscription → `wgmesh service add` enabled

### Technical Context
- Lighthouse server (pkg/lighthouse/) already has `cr_*` API-key auth model
- Polar.sh webhook integration planned for subscription state gating
- Domain strategy: wgmesh.dev → cloudroof.eu for commercial positioning
- Landing pages are static/public-facing (not part of Go codebase)

## Requirements

### Functional Requirements

#### 1. wgmesh.dev Landing Page CTAs
- Add prominent "Get Started" or "Subscribe" button in hero section
- Add secondary "Pricing" or "Plans" link in navigation
- Link to Polar.sh checkout URL for flat monthly tier SKU
- Ensure mobile-responsive button placement
- Add "Powered by Polar.sh" footer or badge for transparency

#### 2. cloudroof.eu Landing Page CTAs
- Mirror wgmesh.dev CTA placement for consistency
- Add "Subscribe" button in hero section
- Link to same Polar.sh checkout URL
- Include pricing summary (flat monthly tier cost)
- Add "Billing via Polar.sh" indicator near checkout button

#### 3. Polar.sh Checkout Integration
- Define checkout URL structure (e.g., `https://polar.sh/checkout/<product_id>`)
- Document UTM parameter tracking for CTA attribution (e.g., `utm_source=landing`)
- Ensure checkout flow redirects back to cloudroof.eu on completion
- Handle subscription state in post-checkout redirect (e.g., `?subscription=active`)

#### 4. Navigation Updates
- Add "Pricing" nav item linking to pricing section or Polar.sh product page
- Ensure "Documentation" link remains prominent (separate from monetization)
- Consider "Login" link for existing subscribers to manage via Polar.sh

#### 5. Documentation Updates
- Create or update docs/landing-pages.md with CTA placement guidance
- Document Polar.sh checkout URL and UTM parameters
- Document post-checkout redirect flow for future Lighthouse webhook integration
- Update CONTRIBUTING.md or docs/ if landing pages have their own repo

### Non-Functional Requirements

#### Performance
- CTA buttons must load without external dependencies (no Polar.sh SDK on landing page)
- Page load time impact must be < 100ms (inline SVG icons, minimal CSS)
- Checkout link must open in new tab (`target="_blank"`) to avoid losing landing page context

#### Accessibility
- CTA buttons must meet WCAG AA contrast requirements
- Include proper `aria-label` for screen readers (e.g., "Subscribe to wgmesh managed ingress on Polar.sh")
- Ensure keyboard navigation works for all CTA elements

#### Brand Consistency
- Use wgmesh/cloudroof color scheme (no Polar.sh branding on buttons themselves)
- Maintain existing landing page design language
- CTA copy should match product tone (technical, direct, dev-focused)

#### Analytics
- Implement click tracking for CTA buttons (e.g., PostHog, Plausible)
- Track conversion funnel: landing page visit → CTA click → checkout init → subscription
- Document tracking events for future analysis

## Acceptance Criteria

### Code Changes
- [ ] wgmesh.dev landing page has hero CTA button linking to Polar.sh checkout
- [ ] cloudroof.eu landing page has hero CTA button linking to Polar.sh checkout
- [ ] Both pages have "Pricing" nav item or secondary CTA
- [ ] CTA buttons use consistent styling across both domains
- [ ] Checkout links include UTM parameters for attribution tracking
- [ ] Post-checkout redirect handling documented (even if Lighthouse webhook not yet implemented)
- [ ] Mobile-responsive CTA placement verified on 375px, 768px, 1024px breakpoints
- [ ] Accessibility audit passed (keyboard nav, screen reader, contrast)

### Documentation
- [ ] docs/landing-pages.md created or updated with CTA placement guide
- [ ] Polar.sh checkout URL and product ID documented in docs/
- [ ] UTM parameter schema documented
- [ ] Post-checkout redirect flow documented for Lighthouse integration reference

### Testing
- [ ] Manual click-through test: CTA → Polar.sh checkout → subscription initiation
- [ ] Mobile test: CTA visible and clickable on iOS/Android browsers
- [ ] Analytics test: CTA click events fired correctly
- [ ] Redirect test: Post-checkout return to cloudroof.eu works as documented

### Deployment
- [ ] No changes to wgmesh Go codebase (landing pages are separate)
- [ ] DNS records for wgmesh.dev and cloudroof.eu unchanged
- [ ] SSL certificates valid for both domains (checkout links are external)

## Out of Scope

### Explicitly Out of Scope
- **Lighthouse webhook integration**: This spec only adds CTAs; subscription state gating (gating `POST /v1/sites` on org subscription) is deferred to future specs (e.g., issue-550, issue-551)
- **Polar.sh product page creation**: Assumes Polar.sh product/tier already exists; if not, separate "Create Polar.sh product" task needed
- **Dynamic pricing display**: CTA links to checkout; pricing page with dynamic SKU fetching is separate work
- **Subscription state UI**: No "My Subscription" section on landing pages; that's Polar.sh dashboard territory
- **CLI `wgmesh signup` command**: CLI signup flow is separate from landing page CTAs
- **Key-challenge signup**: v1.1 feature (per brainstorm docs); this spec uses existing `cr_*` model or direct Polar.sh checkout
- **Bandwidth metering**: p95 billing is v1.1; this spec is for flat monthly tier only

### Deferred to Future Specs
- Lighthouse webhook receiver for `subscription.created` / `subscription.updated` events
- Gating `wgmesh service add` on active subscription state
- CLI `--account <cr_*>` flag integration with subscription status
- Multi-tier pricing display on landing pages
- "Login with Polar.sh" functionality for existing subscribers

## Implementation Notes

### Landing Page Technology
This spec assumes landing pages are static HTML/CSS/JS (likely Hugo, Jekyll, or similar). If landing pages use a different stack (e.g., Next.js, Webflow), adjust implementation accordingly but keep acceptance criteria identical.

### Polar.sh Integration
- Polar.sh checkout URLs follow pattern: `https://polar.sh/checkout/<organization>/<product_id>`
- UTM parameters recommended: `utm_source=landing&utm_medium=cta&utm_campaign=bet_a_launch`
- Post-checkout redirect: Configure return URL in Polar.sh product settings or use `success_url` parameter

### Coordination with Other Work
- Aligns with STRATEGY.md Bet A commercial pipe milestone
- Prerequisite for Lighthouse webhook integration (future spec)
- Supports "Paying customers" metric tracking via checkout analytics

### Risk Mitigation
- If Polar.sh product does not yet exist, create placeholder checkout URL pointing to "Coming soon" page or Polar.sh profile
- If landing page repo is separate from wgmesh Go repo, ensure PR process includes docs/landing-pages.md update
- If cloudroof.eu domain not yet live, implement wgmesh.dev CTAs first and mirror to cloudroof.eu when domain active
