# Issue #584: Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages

## Summary

Add Polar.sh checkout call-to-action (CTA) buttons to the wgmesh.dev and cloudroof.eu landing pages to enable sponsored tier signups. This involves updating existing HTML landing pages with prominent checkout buttons that link to predefined Polar.sh product URLs.

## Context

wgmesh is a decentralized WireGuard mesh networking product with two operational modes (centralized and decentralized). The project has established Polar.sh sponsorship tiers at three price points ($5, $20, $100/month) to support ongoing development.

Currently, the wgmesh.dev landing page exists at `/opt/wgmesh-checkout/wgmesh.dev/index.html` and already contains Polar checkout CTAs. However, the public-facing landing page at `/opt/wgmesh-checkout/public/index.html` may lack these CTAs, and a cloudroof.eu landing page needs to be created or updated with similar checkout functionality.

Polar.sh is a sponsorship platform that enables creators to accept recurring payments from supporters. The checkout URLs follow the pattern: `https://polar.sh/checkout?productId={PRODUCT_ID}`

## Requirements

### 1. wgmesh.dev Landing Page Updates

**Location**: `/opt/wgmesh-checkout/wgmesh.dev/index.html`

- Verify existing Polar checkout CTAs are present and functional
- Ensure all three pricing tiers have proper checkout links:
  - Founding Member ($5/month): `https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
  - Edge Node ($20/month): `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
  - Mesh Operator ($100/month): `https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5`

### 2. Public Landing Page Updates

**Location**: `/opt/wgmesh-checkout/public/index.html`

- Add a pricing/sponsorship section with Polar checkout CTAs
- Use consistent styling with existing design
- Include all three pricing tiers with appropriate descriptions
- Ensure responsive design for mobile devices

### 3. cloudroof.eu Landing Page

**Location**: Create or update `/opt/wgmesh-checkout/cloudroof.eu/index.html`

- Create landing page for cloudroof.eu domain (managed ingress use case)
- Add Polar checkout CTAs targeting the same sponsorship tiers
- Tailor messaging for the edge/CDN use case (Lighthouse CDN, managed ingress)
- Maintain design consistency with wgmesh.dev

### 4. CTA Button Specifications

- Use `.cta-button` CSS class for styling
- Button text: "Become a Founding Member", "Sponsor as Edge Node", "Sponsor as Mesh Operator"
- Include `target="_blank"` and `rel="noopener noreferrer"` attributes
- Apply hover effects (color change, slight elevation)
- Ensure proper spacing and visual hierarchy

## Acceptance Criteria

1. **wgmesh.dev landing page** has three working Polar checkout CTAs
2. **Public landing page** includes a pricing section with Polar checkout CTAs
3. **cloudroof.eu landing page** is created/updated with Polar checkout CTAs
4. All checkout links point to correct Polar.sh product IDs
5. All CTAs have proper styling and hover effects
6. Pages are responsive and functional on mobile devices
7. No broken links or missing attributes on checkout buttons
8. Design consistency across both landing pages

## Out of scope

- Backend integration with Polar.sh API
- User authentication or account management
- Payment processing (handled by Polar.sh)
- Analytics or conversion tracking implementation
- A/B testing framework
- Multi-language support
- Content management system for landing page updates
- Automated deployment pipeline updates
- DNS configuration for cloudroof.eu domain
- SSL certificate management

