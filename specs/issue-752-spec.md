# Issue #752 - Build ROI calculator tool: 'What you'll save vs Tailscale' with PDF download gate

## Classification
feature

## Problem Analysis

The wgmesh project needs a competitive differentiation tool to help potential users understand the cost benefits of adopting wgmesh compared to Tailscale. Currently, there is no interactive way for users to compare pricing and calculate their potential savings.

The requirements specify:
- A web-based ROI calculator
- Comparison against Tailscale pricing model
- PDF download functionality (gated - likely email capture or similar)
- Public-facing tool (no authentication required for basic use)

Key considerations:
- Tailscale's pricing is publicly available (tiered based on node count and features)
- wgmesh is open-source and free (self-hosted), with potential managed service costs
- The calculator should handle different usage scenarios (number of nodes, team size, feature needs)
- PDF generation needs to be server-side or use a client-side library
- Email gating requires a backend service or third-party integration

## Proposed Approach

### Architecture

1. **Frontend Component** (New `cmd/roi/` or static web app)
   - Interactive calculator UI with input fields:
     - Number of nodes/devices
     - Number of users
     - Required features (SSO, MFA, logging, etc.)
     - Current/expected monthly costs (optional)
   - Real-time savings calculation display
   - Comparison table/visualization
   - Email capture form for PDF download gate

2. **Calculation Logic**
   - Tailscale pricing tiers (public data):
     - Free tier: 3 users, 100 devices, limited features
     - Premium: $6-15/user/month (depending on plan)
     - Enterprise: Custom pricing
   - wgmesh costs:
     - Self-hosted: $0 (infrastructure costs only)
     - Managed service (if applicable): TBD pricing tiers
   - Savings formula: `(Tailscale Annual Cost - wgmesh Annual Cost) / Tailscale Annual Cost * 100`

3. **PDF Generation**
   - Option A: Client-side library (jsPDF, Puppeteer) - no backend required
   - Option B: Server-side generation (Go + PDF library)
   - PDF content:
     - User inputs
     - Cost breakdown comparison
     - Savings summary
     - wgmesh branding and call-to-action

4. **Download Gate**
   - Email capture form validation
   - Optional: Third-party integration (Resend, SendGrid, or simple form service)
   - Store leads in database or email notification

### Technology Stack

- **Frontend**: Vanilla JavaScript + CSS (no framework dependency) OR React/Vue if component complexity warrants it
- **Backend (if needed)**: Go service using existing codebase patterns
- **PDF**: 
  - Client-side: jsPDF or similar
  - Server-side: `github.com/jung-kurt/gofpdf` or HTML-to-PDF with wkhtmltopdf
- **Hosting**: Static hosting (GitHub Pages, Cloudflare Pages) or integrated into existing web infrastructure

### Implementation Phases

1. **Phase 1**: Calculator logic and basic UI
2. **Phase 2**: PDF generation (client-side initially)
3. **Phase 3**: Email capture and download gate
4. **Phase 4**: Polish, mobile responsiveness, A/B testing hooks

## Acceptance Criteria

- [ ] Calculator accurately computes Tailscale pricing based on public tier information
- [ ] User can input node count, user count, and feature requirements
- [ ] Real-time savings percentage is displayed and updates with inputs
- [ ] PDF download generates a professional document with comparison details
- [ ] Email capture form validates email format before allowing download
- [ ] PDF download is gated behind email submission
- [ ] Calculator works on mobile devices (responsive design)
- [ ] No hard-coded secrets or API keys in code
- [ ] Tool is accessible via public URL (no authentication required for use)
- [ ] Branding is consistent with wgmesh project
- [ ] Calculator handles edge cases (very large node counts, zero users, etc.)

## Out of scope

- Integration with actual payment processing
- Real-time Tailscale API integration (use published pricing instead)
- Multi-language support initially
- Advanced analytics/tracking beyond basic lead capture
- Admin dashboard for viewing captured leads (can be added later)
- A/B testing framework (add hooks for future implementation)
- User authentication for basic calculator usage
- Comparison with other VPN services beyond Tailscale

