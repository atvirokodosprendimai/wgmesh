# Specification: Issue #496

## Classification
feature

## Deliverables
documentation (HTML/CSS/JS — no Go code changes)

## Problem Analysis

`docs/index.html` currently serves the internal Agent Pipeline Dashboard — a developer-facing view of
CI/CD pipeline state, DORA metrics, and Goose build runs. Because GitHub Pages serves this file at
the root of the project site (e.g. `https://atvirokodosprendimai.github.io/wgmesh/`), any visitor
arriving from a search result, README badge, or direct link sees the pipeline dashboard instead of a
product pitch.

The requirement is to provide a public product landing page at the GitHub Pages root that clearly
communicates what wgmesh is, why someone should use it, and how to get started in 30 seconds.

The pipeline dashboard must NOT be deleted — it is used by the team for internal visibility. It must
be preserved at a different path (`docs/pipeline.html`).

## Implementation Tasks

### Task 1 — Rename the pipeline dashboard

Rename `docs/index.html` → `docs/pipeline.html`. No content changes; only the filename changes.

### Task 2 — Create `docs/index.html` (the product landing page)

Create the file `docs/index.html` with exactly the following content. Do not truncate, omit, or
alter any part of it.

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wgmesh — Decentralized WireGuard Mesh Networking</title>
<meta name="description" content="Build encrypted mesh networks in minutes. Share a secret, run wgmesh join on each node, and let DHT discovery wire everything together — NAT traversal included.">
<style>
:root {
  --bg: #0d1117; --surface: #161b22; --border: #30363d;
  --text: #e6edf3; --muted: #8b949e; --accent: #58a6ff;
  --green: #3fb950; --purple: #bc8cff; --orange: #f0883e;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.6; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }

/* Nav */
nav { display: flex; align-items: center; justify-content: space-between; padding: 1rem 2rem;
  border-bottom: 1px solid var(--border); flex-wrap: wrap; gap: 0.75rem; }
.nav-brand { font-size: 1.125rem; font-weight: 700; color: var(--text); letter-spacing: -0.02em; }
.nav-links { display: flex; align-items: center; gap: 1.25rem; font-size: 0.875rem; }
.nav-links a { color: var(--muted); }
.nav-links a:hover { color: var(--text); text-decoration: none; }
.btn { display: inline-flex; align-items: center; gap: 0.375rem; padding: 0.5rem 1rem;
  border-radius: 6px; font-size: 0.875rem; font-weight: 500; cursor: pointer; }
.btn-primary { background: var(--accent); color: #0d1117; }
.btn-primary:hover { background: #79c0ff; text-decoration: none; }
.btn-outline { border: 1px solid var(--border); color: var(--text); }
.btn-outline:hover { border-color: var(--accent); color: var(--accent); text-decoration: none; }

/* Hero */
.hero { max-width: 860px; margin: 0 auto; padding: 5rem 2rem 4rem; text-align: center; }
.hero-badge { display: inline-flex; align-items: center; gap: 0.375rem; padding: 0.25rem 0.75rem;
  border: 1px solid var(--border); border-radius: 999px; font-size: 0.75rem; color: var(--muted);
  margin-bottom: 1.5rem; }
.hero-badge .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--green); }
.hero h1 { font-size: clamp(2rem, 5vw, 3.25rem); font-weight: 800; letter-spacing: -0.04em;
  line-height: 1.15; margin-bottom: 1.25rem; }
.hero h1 span { color: var(--accent); }
.hero p { font-size: 1.125rem; color: var(--muted); max-width: 560px; margin: 0 auto 2rem; }
.hero-actions { display: flex; align-items: center; justify-content: center; gap: 0.75rem;
  flex-wrap: wrap; margin-bottom: 3rem; }
.hero-actions .btn { font-size: 1rem; padding: 0.625rem 1.375rem; }
.stars-badge { display: inline-flex; align-items: center; }

/* Terminal demo */
.terminal { background: #010409; border: 1px solid var(--border); border-radius: 8px;
  max-width: 640px; margin: 0 auto; text-align: left; overflow: hidden; }
.terminal-bar { background: var(--surface); padding: 0.5rem 0.875rem; display: flex;
  align-items: center; gap: 0.375rem; border-bottom: 1px solid var(--border); }
.terminal-dot { width: 10px; height: 10px; border-radius: 50%; }
.terminal-dot.red { background: #f85149; }
.terminal-dot.yellow { background: #d29922; }
.terminal-dot.green { background: var(--green); }
.terminal-body { padding: 1rem 1.25rem; font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 0.8125rem; line-height: 1.75; }
.t-cmd { color: var(--green); }
.t-comment { color: var(--muted); }
.t-out { color: var(--text); opacity: 0.75; }
.t-prompt { color: var(--muted); margin-right: 0.375rem; }

/* Features grid */
.features { max-width: 1060px; margin: 0 auto; padding: 5rem 2rem; }
.features-header { text-align: center; margin-bottom: 3rem; }
.features-header h2 { font-size: 1.875rem; font-weight: 700; letter-spacing: -0.03em; margin-bottom: 0.75rem; }
.features-header p { color: var(--muted); font-size: 1rem; max-width: 480px; margin: 0 auto; }
.features-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem; }
.feature-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
  padding: 1.5rem; }
.feature-icon { font-size: 1.5rem; margin-bottom: 0.875rem; }
.feature-card h3 { font-size: 1rem; font-weight: 600; margin-bottom: 0.5rem; }
.feature-card p { font-size: 0.875rem; color: var(--muted); }

/* How it works */
.howitworks { background: var(--surface); border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border); }
.howitworks-inner { max-width: 860px; margin: 0 auto; padding: 5rem 2rem; }
.howitworks-inner h2 { font-size: 1.875rem; font-weight: 700; letter-spacing: -0.03em;
  text-align: center; margin-bottom: 3rem; }
.steps { display: flex; flex-direction: column; gap: 2rem; }
.step { display: flex; gap: 1.5rem; align-items: flex-start; }
.step-num { flex-shrink: 0; width: 2rem; height: 2rem; border-radius: 50%;
  background: rgba(88,166,255,0.15); color: var(--accent); display: flex; align-items: center;
  justify-content: center; font-weight: 700; font-size: 0.875rem; margin-top: 0.125rem; }
.step-body h3 { font-size: 1rem; font-weight: 600; margin-bottom: 0.375rem; }
.step-body p { font-size: 0.875rem; color: var(--muted); }
code { font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  background: rgba(88,166,255,0.08); color: var(--accent); padding: 0.1rem 0.375rem;
  border-radius: 4px; font-size: 0.8125em; }

/* Modes */
.modes { max-width: 1060px; margin: 0 auto; padding: 5rem 2rem; }
.modes h2 { font-size: 1.875rem; font-weight: 700; letter-spacing: -0.03em;
  text-align: center; margin-bottom: 3rem; }
.modes-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 1.25rem; }
.mode-card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 1.75rem; }
.mode-card h3 { font-size: 1.125rem; font-weight: 600; margin-bottom: 0.5rem; display: flex;
  align-items: center; gap: 0.5rem; }
.mode-tag { font-size: 0.6875rem; font-weight: 600; padding: 0.125rem 0.5rem; border-radius: 999px;
  background: rgba(88,166,255,0.15); color: var(--accent); }
.mode-tag.green { background: rgba(63,185,80,0.15); color: var(--green); }
.mode-card > p { font-size: 0.875rem; color: var(--muted); margin-bottom: 1.25rem; }
.mode-features { list-style: none; display: flex; flex-direction: column; gap: 0.5rem; }
.mode-features li { font-size: 0.875rem; display: flex; align-items: baseline; gap: 0.5rem; }
.mode-features li::before { content: '✓'; color: var(--green); flex-shrink: 0; font-size: 0.75rem; }

/* Install strip */
.install { max-width: 860px; margin: 0 auto; padding: 5rem 2rem; text-align: center; }
.install h2 { font-size: 1.875rem; font-weight: 700; letter-spacing: -0.03em; margin-bottom: 0.75rem; }
.install > p { color: var(--muted); margin-bottom: 2rem; }
.install-options { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.75rem;
  margin-bottom: 1.5rem; }
.install-opt { background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
  padding: 0.75rem 1.25rem; font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 0.8125rem; color: var(--text); cursor: pointer; transition: border-color 0.15s; }
.install-opt:hover { border-color: var(--accent); }
.install-note { font-size: 0.8125rem; color: var(--muted); }
.install-note a { color: var(--accent); }

/* Footer */
footer { border-top: 1px solid var(--border); padding: 2rem; text-align: center;
  font-size: 0.8125rem; color: var(--muted); }
footer a { color: var(--muted); }
footer a:hover { color: var(--text); text-decoration: none; }
.footer-links { display: flex; justify-content: center; gap: 1.5rem; flex-wrap: wrap; }

/* Responsive */
@media (max-width: 600px) {
  nav { padding: 0.75rem 1rem; }
  .hero { padding: 3rem 1rem 2.5rem; }
  .features, .modes, .install { padding: 3rem 1rem; }
  .howitworks-inner { padding: 3rem 1rem; }
  .modes-grid { grid-template-columns: 1fr; }
}
</style>
</head>
<body>

<!-- Navigation -->
<nav>
  <span class="nav-brand">wgmesh</span>
  <div class="nav-links">
    <a href="quickstart.html">Quickstart</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh/blob/main/docs/FAQ.md">FAQ</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh" target="_blank" rel="noopener">GitHub</a>
    <a href="quickstart.html" class="btn btn-primary">Get started →</a>
  </div>
</nav>

<!-- Hero -->
<section class="hero">
  <div class="hero-badge">
    <span class="dot"></span>
    Decentralized · Zero-config · Open source
  </div>
  <h1>Build a <span>WireGuard mesh</span><br>in minutes, not hours</h1>
  <p>Share a secret. Run <code>wgmesh join</code> on each node. DHT discovery, NAT traversal, and encrypted routing — all automatic.</p>
  <div class="hero-actions">
    <a href="quickstart.html" class="btn btn-primary">Get started →</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh" class="btn btn-outline" target="_blank" rel="noopener">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
      GitHub
    </a>
    <span class="stars-badge">
      <a href="https://github.com/atvirokodosprendimai/wgmesh/stargazers" target="_blank" rel="noopener">
        <img src="https://img.shields.io/github/stars/atvirokodosprendimai/wgmesh?style=social" alt="GitHub Stars" loading="lazy">
      </a>
    </span>
  </div>

  <!-- Terminal demo -->
  <div class="terminal">
    <div class="terminal-bar">
      <span class="terminal-dot red"></span>
      <span class="terminal-dot yellow"></span>
      <span class="terminal-dot green"></span>
    </div>
    <div class="terminal-body">
      <div><span class="t-comment"># Step 1 — generate a shared secret (run once)</span></div>
      <div><span class="t-prompt">$</span><span class="t-cmd"> wgmesh init --secret</span></div>
      <div class="t-out">wgmesh://v1/dGhpcyBpcyBhIHRlc3QgZm9yIHFzZWN1cmU</div>
      <div>&nbsp;</div>
      <div><span class="t-comment"># Step 2 — join from every node (same secret)</span></div>
      <div><span class="t-prompt">$</span><span class="t-cmd"> sudo wgmesh join --secret "wgmesh://v1/..."</span></div>
      <div class="t-out">INFO  peer discovered via DHT  pubkey=AbCdEf… endpoint=203.0.113.10:51820</div>
      <div class="t-out">INFO  peer discovered via LAN   pubkey=XyZaBc… endpoint=192.168.1.42:51820</div>
      <div class="t-out">INFO  mesh converged            peers=2</div>
    </div>
  </div>
</section>

<!-- Features -->
<section class="features">
  <div class="features-header">
    <h2>Everything you need for a production mesh</h2>
    <p>No coordination server. No key distribution ceremony. No interface restarts.</p>
  </div>
  <div class="features-grid">
    <div class="feature-card">
      <div class="feature-icon">🌐</div>
      <h3>DHT peer discovery</h3>
      <p>Nodes find each other over the BitTorrent Mainline DHT — no central bootstrap server required. Works across the public internet from first boot.</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔒</div>
      <h3>End-to-end encrypted</h3>
      <p>All traffic is encrypted by WireGuard. The shared secret is never transmitted — it derives all keys locally via HKDF-SHA256.</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔀</div>
      <h3>NAT traversal built-in</h3>
      <p>UDP hole-punching, STUN-based endpoint detection, and automatic relay fallback keep every node connected even behind strict NAT.</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">⚡</div>
      <h3>Zero-restart updates</h3>
      <p>Live config diffing with <code>wg set</code> means adding or removing peers never disrupts existing tunnels.</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">📡</div>
      <h3>LAN &amp; gossip discovery</h3>
      <p>LAN multicast finds local peers in &lt; 1 second. In-mesh gossip propagates topology changes transitively across the overlay.</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🗂️</div>
      <h3>Dual operating modes</h3>
      <p>Fully decentralized secret-based mode for dynamic fleets, or SSH-based centralized mode for operator-controlled deployments.</p>
    </div>
  </div>
</section>

<!-- How it works -->
<section class="howitworks">
  <div class="howitworks-inner">
    <h2>How it works</h2>
    <div class="steps">
      <div class="step">
        <div class="step-num">1</div>
        <div class="step-body">
          <h3>Generate a shared secret</h3>
          <p>Run <code>wgmesh init --secret</code> once on any machine. The output is a 256-bit random secret encoded as a <code>wgmesh://v1/…</code> URI. Store it in your secrets manager.</p>
        </div>
      </div>
      <div class="step">
        <div class="step-num">2</div>
        <div class="step-body">
          <h3>Join from every node</h3>
          <p>Run <code>sudo wgmesh join --secret "wgmesh://v1/…"</code> on every machine. No other flags required. Each node derives a deterministic WireGuard keypair and mesh IP from the secret.</p>
        </div>
      </div>
      <div class="step">
        <div class="step-num">3</div>
        <div class="step-body">
          <h3>Discovery runs automatically</h3>
          <p>The daemon announces itself via DHT, LAN multicast, and a GitHub Issues registry simultaneously. Peers are discovered typically within 5–30 seconds. NAT traversal is attempted automatically.</p>
        </div>
      </div>
      <div class="step">
        <div class="step-num">4</div>
        <div class="step-body">
          <h3>Mesh stays up-to-date</h3>
          <p>A 5-second reconcile loop applies live WireGuard diffs as peers come and go. Run <code>wgmesh peers list</code> to inspect the current mesh state from any node.</p>
        </div>
      </div>
    </div>
  </div>
</section>

<!-- Modes -->
<section class="modes">
  <h2>Two modes, one binary</h2>
  <div class="modes-grid">
    <div class="mode-card">
      <h3>Decentralized mode <span class="mode-tag green">Recommended</span></h3>
      <p>Nodes self-discover and self-configure using only a shared secret. No server to run or trust.</p>
      <ul class="mode-features">
        <li>DHT + LAN + gossip discovery</li>
        <li>Deterministic mesh IPs from secret</li>
        <li>NAT hole-punching &amp; relay fallback</li>
        <li>Systemd service install in one command</li>
        <li>Route advertisement &amp; propagation</li>
      </ul>
    </div>
    <div class="mode-card">
      <h3>Centralized mode <span class="mode-tag">SSH fleet</span></h3>
      <p>Operator controls topology via a <code>mesh-state.json</code> file. Changes pushed over SSH with no interface restarts.</p>
      <ul class="mode-features">
        <li>Single-file topology definition</li>
        <li>Diff-based live updates via <code>wg set</code></li>
        <li>AES-256-GCM encrypted state file</li>
        <li>SSH key or password auth</li>
        <li>Routable network and vault support</li>
      </ul>
    </div>
  </div>
</section>

<!-- Install -->
<section class="install">
  <h2>Install in seconds</h2>
  <p>Available for Linux (amd64, arm64, armv7), macOS, and Docker.</p>
  <div class="install-options">
    <code class="install-opt">brew install atvirokodosprendimai/tap/wgmesh</code>
    <code class="install-opt">docker pull ghcr.io/atvirokodosprendimai/wgmesh:latest</code>
  </div>
  <p class="install-note">
    Pre-built binaries, <code>.deb</code>, and <code>.rpm</code> packages on the
    <a href="https://github.com/atvirokodosprendimai/wgmesh/releases">releases page</a>.
    Full walkthrough in the <a href="quickstart.html">Quickstart guide</a>.
  </p>
</section>

<!-- Footer -->
<footer>
  <div class="footer-links">
    <a href="quickstart.html">Quickstart</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh/blob/main/docs/FAQ.md">FAQ</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh/blob/main/docs/centralized-mode.md">Centralized mode</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh/blob/main/docs/troubleshooting.md">Troubleshooting</a>
    <a href="https://github.com/atvirokodosprendimai/wgmesh" target="_blank" rel="noopener">GitHub</a>
    <a href="pipeline.html">Pipeline dashboard</a>
  </div>
  <p style="margin-top:1rem">
    © 2026 wgmesh contributors · Open source under MIT License
  </p>
</footer>

</body>
</html>
```

### Task 3 — Create `docs/quickstart.html`

The landing page links to `quickstart.html` (a GitHub Pages URL). Create `docs/quickstart.html` as a
minimal HTML wrapper that renders the Markdown quickstart content in the same dark-theme style.

Create the file `docs/quickstart.html` with exactly the following content:

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>wgmesh — Quickstart Guide</title>
<style>
:root {
  --bg: #0d1117; --surface: #161b22; --border: #30363d;
  --text: #e6edf3; --muted: #8b949e; --accent: #58a6ff;
  --green: #3fb950;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.6; }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
nav { display: flex; align-items: center; gap: 1rem; padding: 1rem 2rem;
  border-bottom: 1px solid var(--border); }
.nav-brand { font-size: 1.125rem; font-weight: 700; color: var(--text); }
.nav-brand a { color: inherit; }
.content { max-width: 760px; margin: 0 auto; padding: 3rem 2rem 5rem; }
.content h1 { font-size: 2rem; font-weight: 700; letter-spacing: -0.03em; margin-bottom: 1rem; }
.content h2 { font-size: 1.375rem; font-weight: 600; margin: 2.5rem 0 0.75rem;
  padding-bottom: 0.375rem; border-bottom: 1px solid var(--border); }
.content h3 { font-size: 1.0625rem; font-weight: 600; margin: 1.75rem 0 0.5rem; }
.content p { margin-bottom: 1rem; color: var(--text); }
.content ul, .content ol { margin: 0.5rem 0 1rem 1.5rem; }
.content li { margin-bottom: 0.375rem; }
.content pre { background: #010409; border: 1px solid var(--border); border-radius: 6px;
  padding: 1rem; overflow-x: auto; margin: 1rem 0; }
.content pre code { background: none; color: var(--text); padding: 0; border-radius: 0;
  font-size: 0.8125rem; }
code { font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  background: rgba(88,166,255,0.08); color: var(--accent); padding: 0.1rem 0.375rem;
  border-radius: 4px; font-size: 0.8125em; }
.content blockquote { border-left: 3px solid var(--border); padding: 0.5rem 0 0.5rem 1rem;
  color: var(--muted); margin: 1rem 0; }
table { width: 100%; border-collapse: collapse; margin: 1rem 0; font-size: 0.875rem; }
th { text-align: left; color: var(--muted); font-weight: 500;
  padding: 0.5rem 0.75rem; border-bottom: 1px solid var(--border); }
td { padding: 0.5rem 0.75rem; border-bottom: 1px solid var(--border); vertical-align: top; }
hr { border: none; border-top: 1px solid var(--border); margin: 2.5rem 0; }
@media (max-width: 600px) {
  .content { padding: 2rem 1rem 4rem; }
  nav { padding: 0.75rem 1rem; }
}
</style>
</head>
<body>
<nav>
  <span class="nav-brand"><a href="index.html">wgmesh</a></span>
  <a href="index.html" style="color:var(--muted);font-size:0.875rem">← Back to home</a>
</nav>
<div class="content" id="content">
  <p style="color:var(--muted)">Loading quickstart guide…</p>
</div>
<script>
// Fetch the Markdown quickstart and render it as simple HTML.
// This avoids bundling a Markdown parser — GitHub Pages serves static files only.
// We fetch the raw markdown from GitHub and convert it with a tiny inline renderer.
(async function() {
  const el = document.getElementById('content');
  try {
    const res = await fetch('https://raw.githubusercontent.com/atvirokodosprendimai/wgmesh/main/docs/quickstart.md');
    if (!res.ok) throw new Error('fetch failed');
    const md = await res.text();
    el.innerHTML = renderMarkdown(md);
  } catch(e) {
    el.innerHTML = '<p>Could not load quickstart guide. <a href="https://github.com/atvirokodosprendimai/wgmesh/blob/main/docs/quickstart.md">View on GitHub</a>.</p>';
  }

  function esc(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  function inlineFormat(s) {
    // code spans
    s = s.replace(/`([^`]+)`/g, (_,c) => `<code>${esc(c)}</code>`);
    // bold
    s = s.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    // links
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
    // strikethrough
    s = s.replace(/~~(.+?)~~/g, '<s>$1</s>');
    return s;
  }

  function renderMarkdown(md) {
    const lines = md.split('\n');
    let html = '';
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      // fenced code block
      if (line.startsWith('```')) {
        let code = '';
        i++;
        while (i < lines.length && !lines[i].startsWith('```')) {
          code += esc(lines[i]) + '\n';
          i++;
        }
        html += `<pre><code>${code}</code></pre>`;
        i++;
        continue;
      }
      // headings
      const hm = line.match(/^(#{1,4})\s+(.*)/);
      if (hm) {
        const level = hm[1].length;
        const id = hm[2].toLowerCase().replace(/[^\w]+/g,'-');
        html += `<h${level} id="${id}">${inlineFormat(esc(hm[2]))}</h${level}>`;
        i++; continue;
      }
      // hr
      if (/^---+$/.test(line.trim())) { html += '<hr>'; i++; continue; }
      // blockquote
      if (line.startsWith('> ')) {
        html += `<blockquote><p>${inlineFormat(esc(line.slice(2)))}</p></blockquote>`;
        i++; continue;
      }
      // table
      if (line.includes('|') && lines[i+1] && /^\|?[\s\-|]+\|?$/.test(lines[i+1])) {
        const headers = line.split('|').filter(c => c.trim() !== '').map(c => `<th>${inlineFormat(esc(c.trim()))}</th>`).join('');
        i += 2; // skip separator
        let rows = '';
        while (i < lines.length && lines[i].includes('|')) {
          const cells = lines[i].split('|').filter(c => c.trim() !== '').map(c => `<td>${inlineFormat(esc(c.trim()))}</td>`).join('');
          rows += `<tr>${cells}</tr>`;
          i++;
        }
        html += `<table><thead><tr>${headers}</tr></thead><tbody>${rows}</tbody></table>`;
        continue;
      }
      // unordered list
      if (/^\s*[-*]\s/.test(line)) {
        html += '<ul>';
        while (i < lines.length && /^\s*[-*]\s/.test(lines[i])) {
          html += `<li>${inlineFormat(esc(lines[i].replace(/^\s*[-*]\s/,'')))}</li>`;
          i++;
        }
        html += '</ul>';
        continue;
      }
      // ordered list
      if (/^\s*\d+\.\s/.test(line)) {
        html += '<ol>';
        while (i < lines.length && /^\s*\d+\.\s/.test(lines[i])) {
          html += `<li>${inlineFormat(esc(lines[i].replace(/^\s*\d+\.\s/,'')))}</li>`;
          i++;
        }
        html += '</ol>';
        continue;
      }
      // paragraph
      if (line.trim() !== '') {
        html += `<p>${inlineFormat(esc(line))}</p>`;
      }
      i++;
    }
    return html;
  }
})();
</script>
</body>
</html>
```

## Affected Files

| File | Action |
|------|--------|
| `docs/index.html` | **Renamed** to `docs/pipeline.html` (no content change) |
| `docs/index.html` | **Created** — new product landing page (self-contained HTML/CSS; no external runtime dependencies other than the GitHub stars badge image and quickstart Markdown fetch) |
| `docs/quickstart.html` | **Created** — lightweight HTML wrapper that fetches and renders `docs/quickstart.md` from the main branch |

No Go source files, `go.mod`, or `go.sum` are changed. No new npm/pip/other package manager dependencies.

## Test Strategy

Manual verification only (no existing HTML test infrastructure):

1. Open `docs/index.html` in a browser locally — confirm dark theme renders, hero section shows, terminal demo block is visible, feature cards and mode cards are fully visible on both desktop and mobile (≤ 600 px viewport).
2. Check all in-page links:
   - `quickstart.html` → `docs/quickstart.html`
   - `pipeline.html` → `docs/pipeline.html` (renamed dashboard)
   - GitHub links open in new tab
3. Open `docs/quickstart.html` in a browser — confirm the Markdown is fetched and rendered correctly (headings, code blocks, tables, lists all visible).
4. Open `docs/pipeline.html` — confirm it is identical to the original `docs/index.html` content (Agent Pipeline Dashboard loads and fetches data normally).
5. Confirm the GitHub stars badge image loads (`img.shields.io`).
6. Confirm the page passes a basic accessibility check: every `<img>` has an `alt` attribute, heading levels are sequential.

## Estimated Complexity
low

**Reasoning:** Pure static HTML/CSS/JS. No Go code changes, no build pipeline changes, no dependency updates. Two new files created and one file renamed. Estimated effort: 60–90 minutes.
