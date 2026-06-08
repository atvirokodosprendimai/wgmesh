# Issue #668 - Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages

## Summary

Add prominent "Get Polar" checkout Call-to-Action (CTA) buttons to the wgmesh.dev and cloudroof.eu landing pages. The CTAs should link users directly to the Polar purchase/checkout flow to drive conversion for the managed ingress service offering.

## Context

**Product Offering**: Polar is a managed ingress service for wgmesh mesh networks, providing stable public endpoints without requiring users to configure their own infrastructure.

**Current State**: The landing pages at wgmesh.dev and cloudroof.eu exist but lack clear conversion paths to purchase Polar. Users may not be aware of the managed service option or how to acquire it.

**Business Need**: Improve conversion rate from landing page visitors to Polar customers by adding clear, prominent checkout CTAs. This is a marketing/UX enhancement, not a code-level feature change to the core wgmesh daemon.

**Related Issue**: #584 - Original tracking issue for Polar checkout CTAs

## Requirements

### 1. Landing Page Updates

#### wgmesh.dev
- Add a primary CTA button above the fold in the hero section: "Get Polar - Managed Ingress"
- Add a secondary CTA in the pricing/features section linking to checkout
- CTAs should link to the Polar checkout/purchase flow (URL to be specified in implementation)
- Button styling should be visually distinct and action-oriented (high contrast color, clear label)

#### cloudroof.eu
- Mirror the CTA placement from wgmesh.dev
- Ensure CTAs are equally prominent on mobile and desktop views
- Use consistent button styling with wgmesh.dev for brand cohesion

### 2. CTA Design Specifications

**Primary CTA (Hero Section)**:
- Label: "Get Polar" or "Get Polar - Managed Ingress"
- Placement: Top-right or center of hero section
- Style: High contrast (e.g., bright blue or green), rounded corners, hover effects
- Size: Minimum 44px height for accessibility (touch targets)

**Secondary CTA (Pricing/Features)**:
- Label: "Buy Now" or "Start with Polar"
- Placement: Near pricing information or feature highlights
- Style: Consistent with primary CTA but slightly less prominent if needed

### 3. Responsive Design

- CTAs must be fully functional on mobile, tablet, and desktop viewports
- Minimum touch target size: 44x44px
- Ensure sufficient contrast ratio (WCAG AA standard: 4.5:1 for normal text)

### 4. Link Destinations

- All CTAs must link to the Polar checkout flow (checkout URL)
- Consider UTM parameters for tracking: `?utm_source=landing&utm_campaign=polar_cta`
- Open in same tab (standard navigation) unless there's a specific reason for new tab

## Acceptance Criteria

### Functional Requirements
- [ ] wgmesh.dev homepage has a visible "Get Polar" CTA button in the hero section
- [ ] wgmesh.dev homepage has a secondary CTA linking to checkout
- [ ] cloudroof.eu homepage has equivalent CTA placement and styling
- [ ] All CTAs link to the correct Polar checkout URL
- [ ] CTAs are clickable and functional on mobile devices
- [ ] CTAs are clickable and functional on desktop browsers

### Visual/Design Requirements
- [ ] CTA buttons are visually distinct from other page elements
- [ ] Button text is clear and actionable
- [ ] Styling is consistent between wgmesh.dev and cloudroof.eu
- [ ] Buttons have appropriate hover/focus states for accessibility
- [ ] Color contrast meets WCAG AA standards (4.5:1 minimum)

### Technical Requirements
- [ ] No console errors on page load
- [ ] Links work correctly across major browsers (Chrome, Firefox, Safari, Edge)
- [ ] Responsive behavior tested at minimum breakpoints: 320px, 768px, 1024px, 1440px
- [ ] UTM parameters included for campaign tracking (if applicable)

### Documentation
- [ ] Landing page preview/screenshots documented in PR or issue
- [ ] Checkout URL confirmed and documented

## Out of Scope

- **Core wgmesh code changes**: This spec is limited to landing page HTML/CSS/JS updates only
- **Checkout flow implementation**: The actual Polar checkout/purchase system is separate
- **A/B testing**: Initial launch does not require multiple CTA variants or testing infrastructure
- **Analytics implementation**: Adding tracking beyond basic UTM parameters is out of scope
- **Content rewrites**: General landing page copy updates beyond CTA text
- **Pricing page updates**: This spec focuses on the homepage/landing page only
- **Documentation site updates**: Docs.wgmesh.dev or other documentation portals
- **Backend services**: No changes to wgmesh daemon, API, or infrastructure

## Implementation Notes

- **Checkout URL**: The specific destination URL for the Polar checkout flow should be confirmed with the product team before implementation
- **Brand Assets**: Use existing brand colors and styling from the landing pages
- **Testing**: Preview changes locally before deploying to production
- **Rollback**: Keep backup of original HTML/CSS for quick rollback if needed

## Files to Modify

- `wgmesh.dev/index.html` - Add CTAs to the landing page markup and styling
- `cloudroof.eu/index.html` - (if this file exists in the repository) Add equivalent CTAs

**Note**: If cloudroof.eu is hosted in a separate repository, coordinate with the appropriate team to ensure consistent implementation.
