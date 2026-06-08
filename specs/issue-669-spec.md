# Issue #669 - spec: Issue #668 - spec: Issue #584 - Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages

## Summary

Implement hero section Call-to-Action (CTA) buttons on wgmesh.dev and cloudroof.eu landing pages that link directly to Polar.sh checkout for the managed ingress service. The CTAs should appear above the fold in the header/hero section, providing a clear conversion path from landing page visitor to paying customer.

## Context

**Current State**: The wgmesh.dev landing page (wgmesh.dev/index.html) already has:
- Existing CTA button styles (`.cta-button` class with blue #007bff background)
- Polar.sh checkout integration in the pricing section with 3 existing checkout links
- "Payment via Polar.sh" footer link
- Navigation structure with pricing anchor

**Gap**: No hero-level CTA above the fold. Users must scroll to the pricing section to find checkout options, reducing conversion rate.

**Business Context**: From STRATEGY.md:
- **Key metric**: "Paying customers — count of active Polar.sh subscriptions"
- **Track**: "Commercial pipe (Bet A)" includes "landing repositioning at cloudroof.eu"
- **Milestone**: 2026-Q2 target for first paying customer
- **Target users**: LLM agent builders (devs not devops) who need fast mesh networking

**Related Issues**: 
- #584: Original tracking issue for Polar checkout CTAs (fully specified)
- #668: Immediate predecessor spec for adding CTAs to landing pages

**Product**: Polar is the managed ingress service offering, providing stable public endpoints for wgmesh meshes without requiring users to configure their own infrastructure.

## Requirements

### 1. Hero Section CTA - wgmesh.dev

Add a primary CTA button to the header/hero section (above the fold):

**Placement**:
- Location: Inside the `<header>` element, below the navigation links
- Position: Centered or below the main headline/description
- Must be visible without scrolling (above the fold)

**Button Specifications**:
- Label: "Get Polar - Managed Ingress" or "Get Started with Polar"
- Class: Use existing `.cta-button` class for consistent styling
- Link: Polar.sh checkout URL (use one of the existing product IDs from pricing section)
- Attributes: `target="_blank" rel="noopener noreferrer"` (matches existing pattern)
- UTM parameters: Include `?utm_source=landing&utm_medium=hero_cta&utm_campaign=polar_checkout`

**Styling**:
- Use existing `.cta-button` styles: background-color #007bff, white text, rounded corners
- Ensure hover effects work (existing transform and shadow transitions)
- Minimum height: 44px for accessibility (touch targets)
- High contrast for visibility

### 2. Hero Section CTA - cloudroof.eu

**Note**: This spec assumes cloudroof.eu either:
- Mirrors wgmesh.dev structure (implement same CTA in same location)
- Is hosted separately (coordinate with appropriate team/repository)

If cloudroof.eu uses the same codebase or structure:
- Mirror the wgmesh.dev CTA implementation exactly
- Use consistent button styling and placement
- Link to same Polar.sh checkout URL
- Include identical UTM parameters

If cloudroof.eu is a separate repository/domain:
- Document the CTA requirements in docs/landing-pages.md
- Provide HTML/CSS snippet for cloudroof.eu team to implement
- Ensure visual consistency with wgmesh.dev

### 3. Navigation Enhancement (Optional but Recommended)

Add a "Pricing" or "Get Polar" link to the main navigation:
- Location: In the `<nav>` section alongside existing links
- Label: "Pricing" (anchor to #pricing) or "Get Polar" (direct checkout link)
- If using "Get Polar": link directly to checkout with UTM `?utm_source=nav&utm_campaign=polar_checkout`

### 4. Mobile Responsiveness

Ensure CTAs work correctly on mobile devices:
- CTA button must not overflow on small screens (max-width: 100% or responsive sizing)
- Minimum touch target: 44x44px (WCAG AA standard)
- Test on breakpoints: 375px, 768px, 1024px, 1440px
- Ensure header layout doesn't break with added CTA

### 5. Accessibility

- CTA button must have accessible text (screen reader friendly)
- Ensure color contrast meets WCAG AA (4.5:1 for normal text)
- Include focus states for keyboard navigation
- Test with keyboard: Tab key should reach CTA, Enter/Space should activate

### 6. Link Destinations and Tracking

**Polar.sh Checkout URL**:
- Use existing checkout URL pattern from pricing section
- Recommended: Use "Founding Member" product ID: `3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
- Full URL: `https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`

**UTM Parameters**:
- Hero CTA: `?utm_source=landing&utm_medium=hero_cta&utm_campaign=polar_checkout`
- Nav CTA (if added): `?utm_source=nav&utm_medium=nav_link&utm_campaign=polar_checkout`
- Pricing CTAs: Keep existing UTM structure or add for consistency

**Link Behavior**:
- Open in new tab: `target="_blank" rel="noopener noreferrer"` (matches existing pattern)
- Ensure `rel="noopener noreferrer"` for security

## Acceptance Criteria

### Functional Requirements
- [ ] wgmesh.dev header/hero section contains a CTA button above the fold
- [ ] Hero CTA links to Polar.sh checkout with correct product ID
- [ ] Hero CTA includes UTM parameters for tracking
- [ ] Hero CTA opens in new tab (`target="_blank"`)
- [ ] Hero CTA uses existing `.cta-button` class for consistent styling
- [ ] Hero CTA is visible on page load without scrolling
- [ ] (Optional) Navigation includes "Pricing" or "Get Polar" link
- [ ] (If cloudroof.eu in same repo) cloudroof.eu has equivalent CTA implementation

### Visual/Design Requirements
- [ ] CTA button is visually distinct and prominent in hero section
- [ ] Button text is clear and actionable ("Get Polar" or similar)
- [ ] Button styling matches existing `.cta-button` design (blue #007bff)
- [ ] Button has hover/focus states (existing CSS transitions)
- [ ] Color contrast meets WCAG AA standards (4.5:1 minimum)
- [ ] Button placement doesn't break header layout

### Technical Requirements
- [ ] No console errors on page load
- [ ] Links work correctly across major browsers (Chrome, Firefox, Safari, Edge)
- [ ] Responsive behavior tested on mobile (375px) and desktop (1440px+)
- [ ] Touch targets meet minimum 44x44px on mobile
- [ ] CTA is keyboard accessible (Tab navigation, Enter/Space activation)
- [ ] UTM parameters are correctly formatted and trackable

### Documentation
- [ ] If cloudroof.eu is separate, document CTA requirements in docs/landing-pages.md
- [ ] Document which Polar.sh product ID is used for hero CTA
- [ ] Document UTM parameter schema for analytics
- [ ] (If implemented) Include navigation link in documentation

### Testing
- [ ] Manual click-through test: Hero CTA → Polar.sh checkout
- [ ] Mobile test: CTA visible and clickable on iOS/Android browsers
- [ ] Responsive test: Verify layout at 375px, 768px, 1024px, 1440px
- [ ] Accessibility test: Keyboard navigation and screen reader check
- [ ] Cross-browser test: Chrome, Firefox, Safari, Edge

## Out of Scope

### Explicitly Out of Scope
- **Polar.sh product/checkout flow implementation**: The Polar.sh checkout system is external; this spec only adds links to it
- **Lighthouse webhook integration**: Subscription state gating and backend integration are separate (see issue-550, issue-551)
- **Pricing section redesign**: The existing pricing section and its CTAs remain unchanged
- **New Polar.sh product creation**: Assumes existing product IDs are valid; creating new SKUs is separate work
- **Analytics implementation**: Adding tracking beyond UTM parameters (e.g., PostHog, Plausible) is out of scope
- **A/B testing infrastructure**: Multiple CTA variants or testing frameworks are not included
- **Content rewrites**: General landing page copy updates beyond the CTA button itself
- **Footer updates**: The existing "Payment via Polar.sh" footer link remains unchanged
- **Documentation site updates**: Docs.wgmesh.dev or other documentation portals are out of scope
- **Backend services**: No changes to wgmesh daemon, API, Lighthouse, or infrastructure

### cloudroof.eu Considerations
- If cloudroof.eu is hosted in a separate repository or managed by a different team:
  - This spec covers wgmesh.dev implementation only
  - Coordination with cloudroof.eu team is a separate task
  - Provide documentation/snippets but don't implement directly
- If cloudroof.eu is in the same repository and mirrors wgmesh.dev structure:
  - Include cloudroof.eu implementation in this spec
  - Ensure identical CTA placement and styling

## Implementation Notes

### File to Modify
- **Primary**: `wgmesh.dev/index.html` - Add hero CTA to header section
- **Optional**: `cloudroof.eu/index.html` - If it exists in this repository

### Code Placement
Insert CTA in the `<header>` section, after the `<nav>` element:

```html
<header>
    <h1>wgmesh.dev - Revolutionizing Your Network</h1>
    <p>Your gateway to secure, decentralized, and high-performance networking.</p>
    <nav>
        <!-- existing nav links -->
    </nav>
    <!-- ADD HERO CTA HERE -->
    <a href="https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1&utm_source=landing&utm_medium=hero_cta&utm_campaign=polar_checkout" 
       class="cta-button" 
       target="_blank" 
       rel="noopener noreferrer"
       style="display: inline-block; margin-top: 1.5em;">
        Get Polar - Managed Ingress
    </a>
</header>
```

### Existing Product IDs (from pricing section)
- Founding Member: `3f5d75de-936b-49d8-a21b-4b79d9fd22c1` (recommended for hero CTA)
- Edge Node Sponsor: `1927e637-4cfd-4c94-8bee-c5518803bc89`
- Mesh Operator Sponsor: `eb20683e-55ea-4354-9d8c-070e55a4eff5`

### CSS Considerations
- Use existing `.cta-button` class (already defined with proper styling)
- Add `style="display: inline-block; margin-top: 1.5em;"` for proper spacing
- Ensure no conflicts with existing header layout

### Testing Checklist
1. Local preview: Open `wgmesh.dev/index.html` in browser
2. Verify CTA is visible without scrolling
3. Click CTA and verify it opens Polar.sh checkout in new tab
4. Test on mobile device or browser dev tools (responsive mode)
5. Test keyboard navigation (Tab to CTA, Enter to activate)
6. Verify UTM parameters appear in URL

### Rollback Plan
- Keep backup of original `wgmesh.dev/index.html`
- If issues arise, remove the hero CTA and restore original header
- No database or backend changes to roll back

### Coordination
- If cloudroof.eu is separate, notify appropriate team of CTA requirements
- Provide HTML snippet and styling guidelines for consistency
- Document in docs/landing-pages.md if needed
