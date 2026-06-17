# Specification: Issue #733

## Classification
feature

## Problem Analysis

Current project documentation (`README.md`, `docs/quickstart.md`, and related marketing content) is primarily developer-centric and implementation-focused. While this serves the existing technical audience, it misses the SEO opportunity for DevOps engineers and infrastructure practitioners searching for "self-hosted VPN" and related alternatives to commercial solutions like Tailscale, ZeroTier, and Netmaker.

Key observations:
- The README focuses on architecture and implementation details rather than use cases
- No dedicated landing page exists for self-hosted VPN search intent
- DevOps decision-makers typically search for specific problems: "VPN for server fleet", "self-hosted mesh VPN", "WireGuard management tool"
- Competitors have strong SEO positioning for these search terms
- wgmesh has compelling differentiators (no SaaS, decentralized discovery, SSH deployment) but they're not prominently featured in search-optimized content

The opportunity is to create a landing page that:
1. Targets high-intent search phrases around self-hosted VPN solutions
2. Positions wgmesh as the DevOps-focused alternative to managed services
3. Highlights technical depth without requiring immediate code comprehension
4. Provides clear migration/evaluation paths from competitor tools

## Proposed Approach

### Phase 1: Content Structure

Create `public/vpn-alternative.md` (or `public/self-hosted-vpn.md`) as a dedicated landing page with:

1. **Hero Section** (above-the-fold value proposition)
   - Headline: "Self-Hosted VPN for DevOps Teams"
   - Subheadline highlighting key differentiators vs. SaaS alternatives
   - Quick start command (one-liner install)
   - Badges: WireGuard-based, No SaaS, Decentralized, Open Source

2. **Problem/Solution Fit**
   - Common pain points addressed: vendor lock-in, pricing opacity, data sovereignty, compliance
   - wgmesh value proposition for each pain point
   - Comparison table: wgmesh vs. Tailscale vs. ZeroTier vs. Netmaker vs. raw WireGuard

3. **Use Case Sections** (mirroring `docs/use-cases/` but marketing-oriented)
   - Remote team access (development workstations)
   - Fleet management (server infrastructure)
   - Multi-cloud networking (VPC-to-VPC)
   - Site-to-site connectivity (branch offices)
   - Each use case with: problem → wgmesh solution → example architecture diagram → CLI commands

4. **Technical Depth Sections**
   - "How it works" overview (decentralized vs. centralized modes)
   - Security model (encryption key derivation, no central server)
   - Deployment options (SSH fleet management vs. daemon-only)
   - Integration examples (CI/CD pipelines, GitOps)

5. **Migration/Evaluation Paths**
   - "Migrating from Tailscale" section
   - "Migrating from ZeroTier" section
   - "Evaluating wgmesh" checklist (links to `docs/pilot-evaluation-guide.md`)
   - Quick comparison: when to use wgmesh vs. competitors

6. **Call-to-Action Sections**
   - "Get Started" (link to `docs/quickstart.md`)
   - "Deploy Your First Mesh" (interactive example)
   - "Production Checklist" (link to `docs/evaluation-checklist.md`)
   - Community links (GitHub, support documentation)

### Phase 2: SEO Optimization

1. **Keyword Targeting**
   - Primary: "self-hosted VPN", "WireGuard VPN", "DevOps VPN"
   - Secondary: "Tailscale alternative", "ZeroTier alternative", "mesh VPN", "VPN for servers"
   - Long-tail: "VPN for Kubernetes clusters", "site-to-site VPN self-hosted"

2. **Meta Data**
   - Title tag: "Self-Hosted VPN for DevOps | wgmesh - WireGuard Mesh Network"
   - Meta description: "Decentralized WireGuard mesh VPN for DevOps teams. No SaaS, no vendor lock-in. Deploy self-hosted VPN for servers, remote teams, and multi-cloud networks."
   - Open Graph tags for social sharing

3. **Structured Data** (JSON-LD)
   - SoftwareApplication schema
   - How-to guides for common tasks
   - FAQPage schema for common questions

### Phase 3: Visual Assets

1. **Architecture Diagrams**
   - High-level mesh topology diagram
   - Comparison diagrams: centralized vs. decentralized deployment
   - Use case architecture diagrams (one per major use case)

2. **Screenshots/Terminal Captures**
   - CLI workflow demonstrations (init, join, status)
   - Config file examples
   - Health/metrics output examples

3. **Comparison Graphics**
   - Feature comparison table visual
   - Security model diagram
   - Discovery layer animation (static SVG)

### Phase 4: Integration Points

1. **Cross-References**
   - Link to `docs/quickstart.md` for detailed setup
   - Link to `docs/use-cases/` for deeper dives
   - Link to `docs/evaluation-checklist.md` for production evaluation
   - Link to GitHub repository for source code

2. **Navigation Updates**
   - Update `docs/index.html` to include prominent link to VPN landing page
   - Update README.md "Getting Started" section to reference the landing page
   - Add link to website navigation (if exists)

3. **CLI Integration**
   - Consider adding `wgmesh docs vpn` command that opens/prints the landing page
   - Add "Learn more" links in relevant `--help` output

## Acceptance Criteria

### Must-Have Requirements
1. Landing page file exists at `public/vpn-alternative.md` with all sections populated
2. Content includes keyword phrases: "self-hosted VPN", "Tailscale alternative", "WireGuard mesh"
3. Hero section includes quick-start command that works (tested)
4. Comparison table includes at least 4 competitor tools with accurate feature data
5. All 5 use cases from `docs/use-cases/` are represented in marketing-oriented format
6. At least 3 architecture diagrams included (as mermaid, D2, or SVG)
7. Migration section exists for at least 2 major competitors
8. Meta tags (title, description, OG tags) included as HTML frontmatter or separate file
9. All cross-reference links are valid (tested)
10. README.md updated with link to landing page

### Should-Have Requirements
1. Structured data JSON-LD included
2. Terminal capture screenshots for CLI workflows
3. "Get Started" call-to-action in each use case section
4. Mobile-responsive rendering (if HTML) or clean formatting (if Markdown)
5. Performance metrics (page load time under 2s if HTML)

### Could-Have Requirements
1. Interactive mesh topology visualization
2. FAQ section with 10+ common questions
3. Testimonials/case study section (placeholder for future content)
4. Video demo embed or link
5. Pricing calculator (SaaS savings comparison)

### Verification Steps
1. `grep -i "self-hosted VPN" public/vpn-alternative.md` returns matches
2. All `[link]` references resolve to valid files or URLs
3. Mermaid/D2 diagrams render correctly (if applicable)
4. HTML rendering (if used) passes accessibility checks
5. Page preview in Open Graph debugger shows correct images/descriptions

## Out of Scope

1. **Backend Changes**: No changes to wgmesh codebase, CLI, or daemon functionality
2. **New Features**: No new wgmesh features are created as part of this work
3. **Video Production**: Video content creation is out of scope (placeholder links acceptable)
4. **Paid Advertising**: SEO content only; no ad campaign setup or management
5. **Website Redesign**: Full website redesign is out of scope; this is a single landing page
6. **Translation**: Non-English versions are out of scope
7. **Competitive Analysis Updates**: Ongoing monitoring of competitor changes is out of scope
8. **Analytics Integration**: Analytics setup is not required (can be added as follow-up)
9. **Content Maintenance**: This spec covers creation only; ongoing updates are separate work

## Implementation Notes

### File Structure Recommendation
```
public/
  vpn-alternative.md          # Main landing page
  vpn-alternative.html        # Rendered HTML (optional, if site generator used)
  diagrams/
    mesh-topology.d2          # Mermaid/D2 diagram source
    mesh-topology.svg         # Rendered diagram
    comparison-table.svg      # Feature comparison visual
    use-case-remote.svg       # Remote team architecture
    use-case-fleet.svg        # Fleet management architecture
    use-case-multicloud.svg   # Multi-cloud architecture
    use-case-site2site.svg    # Site-to-site architecture
  screenshots/
    cli-init.png              # Terminal screenshot: wgmesh init
    cli-join.png              # Terminal screenshot: wgmesh join
    cli-status.png            # Terminal screenshot: wgmesh status
```

### Content Guidelines
- Tone: Professional, technical but accessible, DevOps-focused
- Avoid: Over-promising, unsupported claims, direct competitor disparagement
- Include: Concrete examples, real command outputs, specific version references
- Accuracy: All technical claims must be verified against current codebase behavior
- Honesty: Acknowledge where wgmesh may not be the best fit (helps build trust)

### SEO Best Practices
- Frontload keywords in headings (H1, H2)
- Use descriptive internal link text (not "click here")
- Include alt text for all images
- Keep URL structure simple (`/vpn-alternative` or `/self-hosted-vpn`)
- Add canonical tags if multiple URL patterns exist
- Include breadcrumbs if site template supports them

### Timeline Considerations
This work is primarily content creation and can be completed in parallel with feature development. No blocking dependencies on wgmesh codebase changes.

## Related Documentation

- `docs/quickstart.md` - Technical quickstart guide
- `docs/use-cases/` - Detailed use case documentation
- `docs/evaluation-checklist.md` - Production evaluation criteria
- `docs/FAQ.md` - Common technical questions
- `README.md` - Main project README (to be updated)
