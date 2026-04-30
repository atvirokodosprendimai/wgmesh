# Specification: Issue #551

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

cloudroof.eu is served from `docs/index.html` via GitHub Pages (`.github/workflows/pages.yml`
uploads the entire `docs/` directory). The site already exposes three sponsor tiers (Founding
Member $5/mo, Edge Node $20/mo, Mesh Operator $100/mo) in an internal dashboard view, but there
is no standalone commercial pricing page at `cloudroof.eu/pricing`.

The acceptance criteria require:

1. Clear pricing tiers (pilot/trial, production, enterprise)
2. A comparison table against alternatives (traditional self-hosted VPN, Tailscale, Netmaker /
   Netbird)
3. Pilot program details — what it is, who qualifies, how to apply
4. A contact / CTA section pointing prospects to start a sales conversation
5. A link from `docs/index.html` (the main landing page) to `/pricing`

The deliverable is one new file `docs/pricing.html` plus a small navigation addition to
`docs/index.html`.

## Implementation Tasks

### Task 1: Create `docs/pricing.html`

Create the file `docs/pricing.html` with exactly the following content.  The page reuses the
same CSS custom-properties and visual language as `docs/index.html` (dark theme, card layout,
tier/CTA components).

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pricing — cloudroof.eu</title>
<style>
:root {
  --bg: #0d1117; --surface: #161b22; --border: #30363d;
  --text: #e6edf3; --muted: #8b949e; --accent: #58a6ff;
  --green: #3fb950; --red: #f85149; --yellow: #d29922; --purple: #bc8cff;
  --orange: #f0883e;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.5; padding: 1.5rem; max-width: 1100px;
  margin: 0 auto; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

/* ── Nav ── */
.nav { display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2.5rem; flex-wrap: wrap; }
.nav-brand { font-size: 1rem; font-weight: 700; color: var(--text); }
.nav a { font-size: 0.875rem; color: var(--muted); }
.nav a:hover { color: var(--text); }
.nav a.active { color: var(--accent); font-weight: 600; }

/* ── Page header ── */
.page-header { text-align: center; margin-bottom: 3rem; }
.page-header h1 { font-size: 2rem; font-weight: 700; margin-bottom: 0.75rem; }
.page-header p { color: var(--muted); font-size: 1rem; max-width: 640px; margin: 0 auto; }
.pilot-badge { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 999px;
  background: rgba(63,185,80,0.15); color: var(--green); font-size: 0.75rem; font-weight: 600;
  margin-bottom: 0.75rem; }

/* ── Tier grid ── */
.tier-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1.25rem; margin-bottom: 3rem; }
.tier { background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 1.5rem; display: flex; flex-direction: column; }
.tier h3 { font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem; }
.tier .tier-label { font-size: 0.6875rem; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.05em; margin-bottom: 0.75rem; color: var(--muted); }
.tier .tier-price { font-size: 2rem; font-weight: 700; color: var(--accent);
  margin-bottom: 0.25rem; font-variant-numeric: tabular-nums; }
.tier .tier-price-note { font-size: 0.75rem; color: var(--muted); margin-bottom: 1rem; }
.tier ul { list-style: none; font-size: 0.875rem; flex: 1; margin-bottom: 1.25rem; }
.tier li { padding: 0.375rem 0; padding-left: 1.375rem; position: relative; color: var(--text); }
.tier li::before { content: '\2713'; position: absolute; left: 0; color: var(--green);
  font-weight: 700; }
.tier li.muted { color: var(--muted); }
.tier li.muted::before { color: var(--muted); content: '\2013'; }
.tier.featured { border-color: var(--accent); }
.tier.featured h3 { color: var(--accent); }
.tier.enterprise { border-color: var(--purple); }
.tier.enterprise h3 { color: var(--purple); }
.tier-cta { display: block; width: 100%; padding: 0.625rem 1rem;
  background: rgba(88,166,255,0.12); border: 1px solid var(--accent); border-radius: 6px;
  color: var(--accent); font-size: 0.875rem; font-weight: 600; text-align: center;
  transition: background 0.15s; text-decoration: none; margin-top: auto; }
.tier-cta:hover { background: rgba(88,166,255,0.22); text-decoration: none; }
.tier-cta:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px;
  background: rgba(88,166,255,0.22); text-decoration: none; }
.tier.featured .tier-cta { }
.tier.enterprise .tier-cta { background: rgba(188,140,255,0.12); border-color: var(--purple);
  color: var(--purple); }
.tier.enterprise .tier-cta:hover { background: rgba(188,140,255,0.22); }
.tier.enterprise .tier-cta:focus-visible { outline: 2px solid var(--purple); outline-offset: 2px;
  background: rgba(188,140,255,0.22); text-decoration: none; }

/* ── Section headings ── */
.section { margin-bottom: 3rem; }
.section h2 { font-size: 1.25rem; font-weight: 700; margin-bottom: 0.5rem;
  padding-bottom: 0.5rem; border-bottom: 1px solid var(--border); }
.section .section-sub { color: var(--muted); font-size: 0.875rem; margin-bottom: 1.5rem; }

/* ── Comparison table ── */
.cmp-table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
.cmp-table th { text-align: left; padding: 0.625rem 0.875rem; border-bottom: 2px solid var(--border);
  font-weight: 600; color: var(--muted); font-size: 0.75rem; text-transform: uppercase;
  letter-spacing: 0.04em; }
.cmp-table td { padding: 0.625rem 0.875rem; border-bottom: 1px solid var(--border);
  vertical-align: middle; }
.cmp-table tr:last-child td { border-bottom: none; }
.cmp-table .feature-col { font-weight: 500; color: var(--text); }
.cmp-yes { color: var(--green); font-weight: 600; }
.cmp-no { color: var(--red); }
.cmp-partial { color: var(--yellow); }
.cmp-highlight { background: rgba(88,166,255,0.04); }
.cmp-highlight td:nth-child(2) { color: var(--accent); font-weight: 600; }

/* ── Pilot section ── */
.pilot-card { background: linear-gradient(135deg, rgba(63,185,80,0.05), rgba(88,166,255,0.05));
  border: 1px solid var(--border); border-radius: 8px; padding: 1.5rem; }
.pilot-card h3 { font-size: 1rem; font-weight: 700; color: var(--green); margin-bottom: 0.75rem; }
.pilot-steps { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem; margin: 1rem 0; }
.pilot-step { background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
  padding: 1rem; }
.step-num { display: inline-block; width: 24px; height: 24px; border-radius: 50%;
  background: var(--green); color: #0d1117; font-size: 0.75rem; font-weight: 700;
  line-height: 24px; text-align: center; margin-bottom: 0.5rem; }
.pilot-step h4 { font-size: 0.875rem; font-weight: 600; margin-bottom: 0.375rem; }
.pilot-step p { font-size: 0.8125rem; color: var(--muted); }
.qualify-list { list-style: none; font-size: 0.875rem; margin-top: 0.75rem; }
.qualify-list li { padding: 0.25rem 0 0.25rem 1.375rem; position: relative; }
.qualify-list li::before { content: '\25B6'; position: absolute; left: 0; font-size: 0.5625rem;
  color: var(--green); top: 0.5rem; }

/* ── Contact / CTA ── */
.contact-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 1.5rem; display: flex; gap: 2rem; flex-wrap: wrap; align-items: flex-start; }
.contact-body { flex: 1; min-width: 260px; }
.contact-body h3 { font-size: 1rem; font-weight: 700; margin-bottom: 0.5rem; }
.contact-body p { font-size: 0.875rem; color: var(--muted); margin-bottom: 1rem; }
.contact-actions { display: flex; flex-direction: column; gap: 0.625rem; min-width: 200px; }
.btn-primary { display: block; padding: 0.75rem 1.25rem; background: var(--accent); color: #0d1117;
  border-radius: 6px; font-weight: 700; font-size: 0.875rem; text-align: center;
  text-decoration: none; transition: opacity 0.15s; }
.btn-primary:hover { opacity: 0.85; text-decoration: none; }
.btn-primary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; opacity: 0.85;
  text-decoration: none; }
.btn-secondary { display: block; padding: 0.75rem 1.25rem; background: transparent;
  border: 1px solid var(--border); color: var(--text); border-radius: 6px; font-weight: 600;
  font-size: 0.875rem; text-align: center; text-decoration: none; transition: border-color 0.15s; }
.btn-secondary:hover { border-color: var(--accent); text-decoration: none; }
.btn-secondary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px;
  border-color: var(--accent); text-decoration: none; }

footer { text-align: center; color: var(--muted); font-size: 0.75rem; padding: 2.5rem 0 1rem; }
</style>
</head>
<body>

<!-- ── Navigation ── -->
<nav class="nav">
  <span class="nav-brand">cloudroof.eu</span>
  <a href="/">Dashboard</a>
  <a href="/pricing" class="active">Pricing</a>
  <a href="https://github.com/atvirokodosprendimai/wgmesh" target="_blank" rel="noopener noreferrer">GitHub</a>
</nav>

<!-- ── Page header ── -->
<div class="page-header">
  <div class="pilot-badge">Pilot Program Open</div>
  <h1>Simple, Transparent Pricing</h1>
  <p>wgmesh powers the cloudroof.eu CDN mesh. Use it yourself — self-hosted WireGuard mesh
  networking with zero infrastructure lock-in.</p>
</div>

<!-- ── Pricing tiers ── -->
<div class="tier-grid">

  <!-- Tier 1: Pilot / Trial -->
  <div class="tier">
    <h3>Pilot</h3>
    <div class="tier-label">Free trial · 90 days</div>
    <div class="tier-price">$0</div>
    <div class="tier-price-note">No credit card required</div>
    <ul>
      <li>Up to 10 mesh nodes</li>
      <li>All 4 discovery layers (DHT, LAN, registry, gossip)</li>
      <li>NAT traversal &amp; relay</li>
      <li>Route advertising (subnet exposure)</li>
      <li>Community support (GitHub Issues)</li>
      <li>Full wgmesh binary — no feature restrictions</li>
      <li class="muted">SLA not included</li>
      <li class="muted">Email support not included</li>
    </ul>
    <a class="tier-cta" href="https://github.com/atvirokodosprendimai/wgmesh/releases/latest"
       target="_blank" rel="noopener noreferrer">Download &amp; start free →</a>
  </div>

  <!-- Tier 2: Production (featured) -->
  <div class="tier featured">
    <h3>Production</h3>
    <div class="tier-label">Commercial · per month</div>
    <div class="tier-price">$20</div>
    <div class="tier-price-note">per operator / month · billed monthly</div>
    <ul>
      <li>Unlimited mesh nodes</li>
      <li>Everything in Pilot</li>
      <li>cloudroof.eu edge node — GeoDNS + failover</li>
      <li>Priority issue triage (&lt;48 h response)</li>
      <li>Private Matrix / Slack channel</li>
      <li>Quarterly architecture review call</li>
      <li>Logo on project README</li>
      <li class="muted">Custom SLA not included (see Enterprise)</li>
    </ul>
    <a class="tier-cta"
       href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Production+plan+inquiry&labels=sales"
       target="_blank" rel="noopener noreferrer">Start production conversation →</a>
  </div>

  <!-- Tier 3: Enterprise -->
  <div class="tier enterprise">
    <h3>Enterprise</h3>
    <div class="tier-label">Custom · contact us</div>
    <div class="tier-price">Custom</div>
    <div class="tier-price-note">volume &amp; SLA negotiated</div>
    <ul>
      <li>Everything in Production</li>
      <li>Dedicated edge nodes in regions you choose</li>
      <li>Custom SLA (99.9 % uptime commitment)</li>
      <li>Dedicated Slack / on-call support</li>
      <li>Custom feature development</li>
      <li>On-premises / air-gapped deployment option</li>
      <li>Invoice billing</li>
    </ul>
    <a class="tier-cta"
       href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Enterprise+plan+inquiry&labels=sales"
       target="_blank" rel="noopener noreferrer">Contact for enterprise pricing →</a>
  </div>

</div>

<!-- ── Comparison table ── -->
<div class="section">
  <h2>How wgmesh Compares</h2>
  <div class="section-sub">Honest comparison against common alternatives for private overlay networking.</div>
  <div style="overflow-x:auto">
    <table class="cmp-table">
      <thead>
        <tr>
          <th class="feature-col">Feature</th>
          <th>wgmesh</th>
          <th>Self-hosted WireGuard</th>
          <th>Tailscale</th>
          <th>Netbird / Netmaker</th>
        </tr>
      </thead>
      <tbody>
        <tr class="cmp-highlight">
          <td class="feature-col">Zero-config peer discovery</td>
          <td class="cmp-yes">✓ 4-layer (DHT, LAN, registry, gossip)</td>
          <td class="cmp-no">✗ manual key exchange</td>
          <td class="cmp-yes">✓ via coordination server</td>
          <td class="cmp-yes">✓ via management server</td>
        </tr>
        <tr>
          <td class="feature-col">No central coordination server required</td>
          <td class="cmp-yes">✓ fully serverless</td>
          <td class="cmp-yes">✓ (config is manual)</td>
          <td class="cmp-no">✗ requires Tailscale SaaS or self-hosted headscale</td>
          <td class="cmp-no">✗ requires management server</td>
        </tr>
        <tr class="cmp-highlight">
          <td class="feature-col">NAT traversal (UDP hole-punching)</td>
          <td class="cmp-yes">✓ built-in + relay fallback</td>
          <td class="cmp-partial">⚠ requires public endpoint or VPN server</td>
          <td class="cmp-yes">✓</td>
          <td class="cmp-yes">✓</td>
        </tr>
        <tr>
          <td class="feature-col">Open source &amp; self-hostable</td>
          <td class="cmp-yes">✓ Apache 2.0 / MIT</td>
          <td class="cmp-yes">✓ WireGuard is GPLv2</td>
          <td class="cmp-partial">⚠ client open, control server proprietary</td>
          <td class="cmp-yes">✓ open source</td>
        </tr>
        <tr class="cmp-highlight">
          <td class="feature-col">Subnet route advertising</td>
          <td class="cmp-yes">✓ <code>--advertise-routes</code></td>
          <td class="cmp-yes">✓ manual AllowedIPs</td>
          <td class="cmp-yes">✓ subnet router</td>
          <td class="cmp-yes">✓</td>
        </tr>
        <tr>
          <td class="feature-col">CDN / edge proxy integration</td>
          <td class="cmp-yes">✓ cloudroof.eu (GeoDNS, failover)</td>
          <td class="cmp-no">✗ DIY only</td>
          <td class="cmp-no">✗ not a CDN</td>
          <td class="cmp-no">✗ not a CDN</td>
        </tr>
        <tr class="cmp-highlight">
          <td class="feature-col">Privacy — no telemetry / no account required</td>
          <td class="cmp-yes">✓ no account needed</td>
          <td class="cmp-yes">✓</td>
          <td class="cmp-no">✗ account required, telemetry to SaaS</td>
          <td class="cmp-partial">⚠ account required</td>
        </tr>
        <tr>
          <td class="feature-col">Setup time (two nodes)</td>
          <td class="cmp-yes">~2 min (one command per node)</td>
          <td class="cmp-no">30–60 min (key gen, config, firewall)</td>
          <td class="cmp-yes">~5 min</td>
          <td class="cmp-partial">~15 min (server setup first)</td>
        </tr>
        <tr class="cmp-highlight">
          <td class="feature-col">Pricing (small team, 10 nodes)</td>
          <td class="cmp-yes">Free (Pilot tier)</td>
          <td class="cmp-yes">Free (infra cost only)</td>
          <td class="cmp-partial">$18/mo (3 users, Tailscale free plan) or $72/mo (premium)</td>
          <td class="cmp-partial">$29/mo (Netbird cloud) or self-host</td>
        </tr>
      </tbody>
    </table>
  </div>
  <p style="font-size:0.75rem;color:var(--muted);margin-top:0.75rem">
    Pricing for alternatives as of April 2026. ✓ = fully supported &nbsp; ⚠ = partial / caveats &nbsp; ✗ = not supported.
  </p>
</div>

<!-- ── Pilot program ── -->
<div class="section">
  <h2>Pilot Program</h2>
  <div class="section-sub">
    Get a free, supported 90-day pilot. We work with you to validate wgmesh in your environment
    before you commit to a Production plan.
  </div>
  <div class="pilot-card">
    <h3>What's included in the Pilot</h3>
    <ul class="qualify-list">
      <li>Full wgmesh binary with no node limit during pilot</li>
      <li>Direct async support via GitHub Issues (tagged <code>pilot</code>)</li>
      <li>Architecture call at start and end of pilot (30 min each)</li>
      <li>Help setting up monitoring (Prometheus / Coroot compatible metrics endpoint)</li>
      <li>Migration guide from your current VPN / overlay solution</li>
    </ul>

    <h3 style="margin-top:1.25rem">Who qualifies</h3>
    <ul class="qualify-list">
      <li>Organisation with 2–200 Linux / macOS nodes that need a private overlay network</li>
      <li>Willing to provide a 30-minute debrief call after the pilot</li>
      <li>Have a real workload (not just evaluation for its own sake)</li>
      <li>Can deploy on Linux kernel ≥ 5.6 or macOS with wireguard-go</li>
    </ul>

    <h3 style="margin-top:1.25rem">How to apply — three steps</h3>
    <div class="pilot-steps">
      <div class="pilot-step">
        <div class="step-num">1</div>
        <h4>Open a GitHub Issue</h4>
        <p>Title: <em>"Pilot application: &lt;your org&gt;"</em>. Label: <code>pilot</code>.
        Include: node count, OS, current networking solution, and your goal for the pilot.</p>
      </div>
      <div class="pilot-step">
        <div class="step-num">2</div>
        <h4>30-min kick-off call</h4>
        <p>We review your architecture, confirm fit, and set success criteria together.
        Scheduled within 5 business days of your application.</p>
      </div>
      <div class="pilot-step">
        <div class="step-num">3</div>
        <h4>Deploy &amp; iterate</h4>
        <p>Follow the quickstart guide. Raise issues as they arise — we triage pilot issues
        within 24 h. At 90 days: debrief call and optional upgrade to Production.</p>
      </div>
    </div>

    <div style="margin-top:1.25rem">
      <a class="btn-primary" style="display:inline-block;width:auto;padding:0.75rem 1.5rem"
         href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Pilot+application&labels=pilot"
         target="_blank" rel="noopener noreferrer">Apply for pilot →</a>
    </div>
  </div>
</div>

<!-- ── Contact / CTA ── -->
<div class="section">
  <h2>Talk to Us</h2>
  <div class="section-sub">
    Not sure which tier fits? Have a non-standard deployment? We're happy to discuss.
  </div>
  <div class="contact-card">
    <div class="contact-body">
      <h3>Start a conversation</h3>
      <p>
        The fastest path is a GitHub Issue — we read everything. For a more structured discussion
        about Production or Enterprise deployments, open an issue titled
        <em>"Sales inquiry: &lt;your topic&gt;"</em> with the <code>sales</code> label and we will
        respond within 2 business days to schedule a call.
      </p>
      <p style="font-size:0.8125rem">
        Billing integration is currently being set up. In the meantime, Production and Enterprise
        plans are available via direct arrangement — reach out and we will handle invoicing
        manually until the self-serve checkout is live.
      </p>
    </div>
    <div class="contact-actions">
      <a class="btn-primary"
         href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Sales+inquiry&labels=sales"
         target="_blank" rel="noopener noreferrer">Open a sales issue →</a>
      <a class="btn-secondary"
         href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Pilot+application&labels=pilot"
         target="_blank" rel="noopener noreferrer">Apply for pilot →</a>
      <a class="btn-secondary"
         href="https://polar.sh/atvirokodosprendimai"
         target="_blank" rel="noopener noreferrer">Sponsor via Polar.sh →</a>
    </div>
  </div>
</div>

<footer>
  cloudroof.eu &mdash; WireGuard mesh CDN &middot;
  <a href="/">Dashboard</a> &middot;
  <a href="https://github.com/atvirokodosprendimai/wgmesh">GitHub</a> &middot;
  <a href="https://github.com/atvirokodosprendimai/wgmesh/issues/new?title=Pricing+question&labels=sales">Questions?</a>
</footer>

</body>
</html>
```

---

### Task 2: Add a "Pricing" link to `docs/index.html`

In `docs/index.html`, find the `<footer>` element. It currently reads:

```html
<footer>
  wgmesh Agent Pipeline Dashboard &middot;
  <a href="https://github.com/atvirokodosprendimai/wgmesh">Repository</a> &middot;
  <a href="https://cloudroof.eu">cloudroof.eu</a> &middot;
   <span id="footer-mode">Data via chimney proxy (authenticated, 5,000 req/hr)</span>
</footer>
```

Replace that `<footer>` block with:

```html
<footer>
  wgmesh Agent Pipeline Dashboard &middot;
  <a href="https://github.com/atvirokodosprendimai/wgmesh">Repository</a> &middot;
  <a href="https://cloudroof.eu">cloudroof.eu</a> &middot;
  <a href="/pricing">Pricing</a> &middot;
   <span id="footer-mode">Data via chimney proxy (authenticated, 5,000 req/hr)</span>
</footer>
```

Also, in `docs/index.html`, find the Sponsor Benefits section heading. It currently reads:

```html
<div class="sponsor-card" style="margin-bottom:1.5rem">
  <h2>Sponsor Benefits</h2>
```

Replace that with:

```html
<div class="sponsor-card" style="margin-bottom:1.5rem">
  <h2>Sponsor Benefits &mdash; <a href="/pricing" style="font-size:0.75rem;font-weight:400;color:var(--accent)">See full pricing page →</a></h2>
```

---

## Affected Files

- **New:** `docs/pricing.html` — standalone pricing page served at `cloudroof.eu/pricing`
- **Modified:** `docs/index.html` — add Pricing link in `<footer>` and a cross-link from the
  Sponsor Benefits heading

No Go source files, no `go.mod`, no CI/CD workflows are changed.

## Test Strategy

No automated tests are required. Verify manually:

1. Open `docs/pricing.html` in a browser (or GitHub Pages preview). Confirm:
   - Three tiers render (Pilot, Production, Enterprise) with correct prices and features.
   - Comparison table renders without horizontal scroll on a 1280 px viewport.
   - All four CTA buttons/links are clickable and point to GitHub Issues with the correct
     pre-filled titles and labels.
   - The pilot programme section shows the three steps (numbered circles, step cards).
   - The contact card shows two action buttons and a Polar.sh link.
   - The page is readable in a mobile viewport (375 px wide) — tier cards stack vertically.

2. Open `docs/index.html` in a browser. Confirm:
   - Footer contains a "Pricing" link pointing to `/pricing`.
   - The Sponsor Benefits heading includes the "See full pricing page →" cross-link.

3. Confirm all relative links (`/pricing`, `/`) in `pricing.html` will resolve correctly when
   served from the root of `cloudroof.eu` via GitHub Pages.

4. Confirm no existing content in `docs/index.html` was accidentally removed by checking that
   the existing sponsor tier cards (Founding Member, Edge Node, Mesh Operator) and their Polar.sh
   checkout links remain unchanged.

## Estimated Complexity
low

**Reasoning:** Pure static HTML/CSS documentation. One new file (~260 lines) and two small edits
to `docs/index.html` (footer link + one heading change). No JavaScript, no external dependencies,
no Go code, no CI changes. Estimated effort: 45–60 minutes.
