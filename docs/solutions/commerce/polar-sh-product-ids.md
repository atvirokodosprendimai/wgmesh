# Polar.sh Product IDs and Checkout Configuration

**Module:** commerce
**Tags:** polar.sh, billing, checkout, pricing
**Problem Type:** documentation
**Created:** 2026-06-10
**Related Issues:** #584

## Overview

This document tracks all Polar.sh product IDs used for wgmesh.dev sponsorship tiers and provides a runbook for updating checkout URLs.

## Current Product IDs

### Founding Member ($5/month)
- **Product ID:** `3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
- **Price:** $5.00 USD / month
- **Benefits:** Early access to new features, community role, special thanks in documentation
- **Checkout URL Pattern:**
  ```
  https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1
  ```
- **Files Using This Link:**
  - `wgmesh.dev/index.html` (pricing card CTA)
  - `public/index.html` (pricing card CTA)

### Edge Node ($20/month)
- **Product ID:** `1927e637-4cfd-4c94-8bee-c5518803bc89`
- **Price:** $20.00 USD / month
- **Benefits:** All Founding Member benefits, plus priority support and access to exclusive beta builds
- **Checkout URL Pattern:**
  ```
  https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89
  ```
- **Files Using This Link:**
  - `wgmesh.dev/index.html` (pricing card CTA, featured tier)
  - `public/index.html` (pricing card CTA, featured tier)

### Mesh Operator ($100/month)
- **Product ID:** `eb20683e-55ea-4354-9d8c-070e55a4eff5`
- **Price:** $100.00 USD / month
- **Benefits:** All Edge Node benefits, plus direct communication with core developers, dedicated support, roadmap influence
- **Checkout URL Pattern:**
  ```
  https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5
  ```
- **Files Using This Link:**
  - `wgmesh.dev/index.html` (pricing card CTA)
  - `public/index.html` (pricing card CTA)

## UTM Parameter Standardization

All checkout URLs include UTM parameters for basic attribution and analytics tracking:

```
utm_source=wgmesh.dev
utm_medium=landing-page
utm_campaign=pricing-tier-{tier-name}
utm_content=pricing-card
```

### Tier Names for UTM Campaign
- `founding-member` - $5 Founding Member tier
- `edge-node` - $20 Edge Node tier
- `mesh-operator` - $100 Mesh Operator tier

### Example Full URL with UTM Parameters
```
https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1&utm_source=wgmesh.dev&utm_medium=landing-page&utm_campaign=pricing-tier-founding-member&utm_content=pricing-card
```

## CTA Button Standards

All checkout links must follow these standards:

### HTML Attributes
- `class="cta-button"` - Consistent styling class
- `target="_blank"` - Opens in new tab
- `rel="noopener noreferrer"` - Security for external links
- `aria-label="{descriptive text}"` - Accessibility support

### Aria-Label Pattern
```
aria-label="{action text} - ${price} per month sponsorship tier on Polar.sh"
```

Examples:
- `aria-label="Become a Founding Member - $5 per month sponsorship tier on Polar.sh"`
- `aria-label="Sponsor as Edge Node - $20 per month sponsorship tier on Polar.sh"`
- `aria-label="Sponsor as Mesh Operator - $100 per month sponsorship tier on Polar.sh"`

## Runbook: Updating Checkout URLs

### Scenario 1: Polar.sh Changes Product IDs
**Trigger:** Polar.sh updates product IDs or reorganizes products

**Steps:**
1. Log in to Polar.sh dashboard and verify new product IDs
2. Update this document (`docs/solutions/commerce/polar-sh-product-ids.md`) with new product IDs
3. Update checkout URLs in all files:
   - `wgmesh.dev/index.html` (3 links)
   - `public/index.html` (3 links)
4. Preserve UTM parameters and aria-label attributes
5. Test each checkout link in a browser
6. Run `go build ./...` to ensure no syntax errors (if applicable)
7. Commit changes with message: "docs: update Polar.sh product IDs"

### Scenario 2: Adding New Pricing Tiers
**Trigger:** Business decision to add new sponsorship tier

**Steps:**
1. Create new product in Polar.sh dashboard
2. Record new product ID in this document
3. Add new pricing card to both landing pages:
   - Copy existing card structure
   - Update product ID, price, and description
   - Generate unique UTM campaign name (e.g., `pricing-tier-enterprise`)
   - Add appropriate aria-label
4. Test checkout flow end-to-end
5. Update any relevant pricing documentation

### Scenario 3: Changing Pricing
**Trigger:** Business decision to change tier pricing

**Steps:**
1. Update pricing in Polar.sh dashboard (product configuration)
2. Update price displays in landing pages:
   - `wgmesh.dev/index.html` - update `.price` elements
   - `public/index.html` - update `<h3>` tier titles
3. Update aria-label attributes with new pricing
4. Test checkout flow to confirm correct pricing displays
5. Update this document with new pricing

## Verification Checklist

When making changes to checkout links, verify:

- [ ] All product IDs match Polar.sh dashboard
- [ ] All checkout URLs include UTM parameters
- [ ] All CTAs have `target="_blank" rel="noopener noreferrer"`
- [ ] All CTAs have descriptive `aria-label` attributes
- [ ] All CTAs use `class="cta-button"` for consistent styling
- [ ] Mobile responsive behavior is maintained
- [ ] Checkout links open correctly in new tabs
- [ ] This documentation is updated with any changes

## Testing Procedures

### Manual Testing
1. Click each CTA button on both landing pages
2. Verify Polar.sh checkout page loads correctly
3. Verify correct product and pricing display in checkout
4. Test on multiple browsers (Chrome, Firefox, Safari, Edge)
5. Test on mobile devices (iOS Safari, Chrome Mobile)

### Automated Testing (Future)
Consider implementing automated tests for:
- Link validity (HTTP 200 response)
- UTM parameter presence
- HTML attribute compliance
- Accessibility standards (WCAG 2.1 Level AA)

## Related Documentation

- **Issue #584:** Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages
- **ROADMAP.md:** Item #1 - "Polar.sh billing wired up (#376)"
- **STRATEGY.md:** Commercial pipe (Bet A) section

## Notes

- **cloudroof.eu:** The cloudroof.eu domain is referenced in STRATEGY.md and ROADMAP.md as a separate commercial offering. This documentation applies specifically to wgmesh.dev landing pages. If cloudroof.eu landing page is created in the future, it should mirror these checkout URLs and standards.
- **Security:** Always use `rel="noopener noreferrer"` on external checkout links to prevent tabnabbing attacks.
- **Analytics:** UTM parameters enable basic attribution in Polar.sh analytics. For advanced conversion tracking, consider implementing Polar.sh webhooks or conversion pixels (future enhancement).

## History

| Date | Change | Author |
|------|--------|--------|
| 2026-06-10 | Initial documentation with product IDs and UTM standards | Issue #584 implementation |
