# Issue #745 - Create cloudroof vs Tailscale comparison page for high-intent evaluator traffic

## Classification
feature

## Problem Analysis

High-intent evaluators (prospects actively comparing WireGuard mesh solutions) need clear, objective differentiation between cloudroof and Tailscale. Currently, there is no dedicated comparison page addressing their key decision criteria:

- **Security model**: Centralized broker vs decentralized mesh
- **Architecture**: Proprietary control plane vs open-source self-hostable
- **Privacy**: Data collection policies and relay behavior
- **Cost**: Subscription pricing vs self-hosted TCO
- **Use case fit**: Edge/IoT scenarios vs enterprise remote access
- **Compliance**: Open-source auditability vs closed-source vendor

These evaluators typically arrive via search queries like "cloudroof vs Tailscale", "Tailscale alternatives", or "WireGuard mesh comparison". Without a dedicated comparison page, they must navigate multiple documentation pages to form conclusions, increasing friction and bounce rates.

## Proposed Approach

Create a new marketing page at `/docs/comparison/tailscale.html` (or equivalent Hugo content path) that presents a structured, objective comparison. The page will:

### Content Structure

1. **Executive Summary** (above-fold)
   - One-sentence positioning: cloudroof is a decentralized WireGuard mesh for edge/IoT; Tailscale is a centralized NAT traversal service for remote access
   - Quick comparison table (3-5 key differentiators)

2. **Architecture Comparison**
   - cloudroof: Decentralized, DHT-based discovery, peer-to-peer routing, no central broker
   - Tailscale: Centralized coordination server, DERP relays, control plane dependency
   - Diagram: Simple architecture comparison (optional, if visual assets available)

3. **Security & Privacy**
   - Trust model comparison
   - Data collection differences
   - Relay behavior (Dandelion++ vs DERP)
   - Auditability: open-source codebase vs proprietary

4. **Deployment & Operations**
   - Self-hosted vs managed service
   - Infrastructure requirements
   - Configuration complexity
   - Update/upgrade model

5. **Cost Comparison**
   - cloudroof: Self-hosted TCO (infrastructure only)
   - Tailscale: Subscription tiers, per-user/per-device pricing
   - Breakeven analysis scenarios (optional)

6. **Use Case Guidance**
   - When to choose cloudroof: Edge deployments, IoT fleets, air-gapped networks, compliance requirements
   - When to choose Tailscale: Quick remote access setup, non-technical users, small teams, mobile-first

7. **Feature Matrix**
   - Structured comparison table: Discovery methods, relay options, authentication, access control, monitoring, integrations

### Technical Implementation

- **Framework**: Use existing Hugo setup (check `docs/` structure for patterns)
- **URL path**: `/comparison/tailscale/` or `/docs/comparison/tailscale.html`
- **Navigation**: Add to main docs nav under "Compare" or "Resources" section
- **SEO optimization**: 
  - Title tag: "cloudroof vs Tailscale | WireGuard Mesh Comparison"
  - Meta description: Objective comparison of cloudroof and Tailscale for WireGuard networking
  - Target keywords: "Tailscale alternative", "WireGuard mesh", "decentralized VPN"
- **Internal linking**: Reference related docs (Architecture, Security, Deployment guides)
- **External linking**: Link to Tailscale docs where appropriate for fairness

### Content Guidelines

- Maintain neutral, factual tone; avoid promotional language
- Use Tailscale's documented capabilities (cite public sources)
- Acknowledge Tailscale's strengths where appropriate (e.g., ease of use, ecosystem)
- Focus on architectural differences rather than feature lists
- Update date visible; commit to quarterly reviews

## Acceptance Criteria

1. Page exists at published URL with clean slug (/comparison/tailscale/)
2. All 7 sections from Content Structure are present
3. Executive summary visible above-fold without scrolling
4. Comparison table renders correctly with responsive layout
5. Page is linked from main documentation navigation
6. SEO metadata present (title, description, keywords)
7. Internal links to relevant cloudroof documentation pages
8. External links to Tailscale documentation for factual claims
9. Mobile-responsive layout tested
10. Content reviewed for accuracy and neutrality
11. Last-updated date visible on page

## Out of Scope

- Interactive comparison tools or calculators
- Video content or animations
- Paid search/SEM campaigns (SEO optimization only)
- Comparison with other WireGuard solutions (Nebula, Firezone, etc.) - separate future work
- Translations (English only initially)
- User comments or discussion forums
- Real-time pricing updates
- Technical deep-dive sections beyond architectural overview
