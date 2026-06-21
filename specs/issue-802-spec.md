# Specification: Issue #802

## Classification
feature

## Problem Analysis

The cloudroof.eu homepage (served from `public/index.html` in this repo) does not currently show
a working product demonstration to first-time visitors. The above-the-fold viewport is consumed by
the `<header>` block and the first section, which is `#pricing` (`public/index.html:333`). A new
visitor has to scroll past sponsorship tiers to encounter any evidence that the mesh actually works,
and even then the only "instant" signal is the static `#install` one-liner at
`public/index.html:381-386`.

Issue #802 asks for a **60-second mesh demo** animated GIF or looping video to be embedded on the
cloudroof.eu homepage **above the fold**, so visitors immediately see a mesh being created and
traffic flowing before they are asked to install or sponsor.

Relevant existing state:
- `public/index.html` is the cloudroof.eu landing page (distinct from the developer-facing
  `wgmesh.dev/index.html`). Its `<body>` starts at `public/index.html:326` with a `<header>`,
  followed by `<div class="container">` containing ordered `<section>` blocks.
- The first visual section after the header is `#pricing`. The page has no hero media today.
- A self-hosted OpenPanel analytics script is already present (`public/index.html`, `window.op(...)`),
  so demo-view engagement can be tracked through existing event instrumentation rather than a new
  analytics dependency.
- The product already ships a documented 60-second quickstart (`public/index.html:381`,
  `curl -fsSL https://install.wgmesh.dev | sh`), which is the canonical "demo" flow the asset should
  represent.

Concerns worth flagging (does not block classification):
- "Above the fold" varies by viewport. The asset and surrounding copy must fit a 1080p desktop
  viewport (≈ 960px content width, ≤ 720px tall hero band) without forcing the pricing section out
  of easy reach, and must degrade cleanly on mobile (≤ 414px) where vertical space is the constraint.
- A GIF is simpler to embed and auto-plays everywhere, but a 60-second GIF at viewable resolution is
  typically several MB. A looping `<video>` (MP4 + WebM) is materially smaller and is the
  recommended primary asset, with the GIF kept as a fallback. The spec supports both so the
  implementer can choose based on final asset size.

## Proposed Approach

### Task 1: Produce the demo asset

Record a real 60-second terminal capture of the canonical mesh-up flow. The recording must show, in
order:

1. `curl -fsSL https://install.wgmesh.dev | sh` (or repo install command) — ~5s
2. `wgmesh init` / join creating a mesh — ~10s
3. A second node joining the same mesh — ~10s
4. `wgmesh status` showing two peers with a direct/relayed connection — ~10s
5. `ping` / `curl` between the two mesh IPs proving traffic flows — ~15s
6. Final `wgmesh status` confirming the live tunnel — ~10s

Constraints on the asset:
- Total duration: 55–60 seconds, set to loop seamlessly (no hard cut).
- Resolution: 1280×720 (16:9) source; encode down to ≤ 1920px wide max display.
- Files to produce under `public/` (no third-party hosting; keeps the page self-contained and
  privacy-respecting):
  - `public/demo/wgmesh-60s-demo.mp4` (H.264, faststart, target ≤ 4 MB)
  - `public/demo/wgmesh-60s-demo.webm` (VP9, target ≤ 3 MB)
  - `public/demo/wgmesh-60s-demo.gif` (fallback / for OG/social preview, target ≤ 6 MB)
  - `public/demo/wgmesh-60s-demo-poster.jpg` (first-frame still, used as `poster=`, target ≤ 80 KB)
- No audio track (autoplay policies and silent browsing).
- No terminal content that includes secrets, real private keys, real peer public keys, customer
  identifiers, IPs of production nodes, or revenue figures. Use throwaway `wgmesh0` demo meshes and
  RFC1918/`100.64.0.0/10` example addresses only.

### Task 2: Add the hero demo section above the fold in `public/index.html`

Insert a new `<section id="demo">` as the **first** child of `<div class="container">`
(immediately after `<div class="container">` opens, before the existing `#pricing` section at
`public/index.html:333`). This guarantees the demo is the first content a visitor sees inside the
content column and remains above the fold on desktop.

Section markup (exact structure to implement):

```html
<section id="demo" class="hero-demo" aria-labelledby="demo-heading">
  <div class="hero-demo__copy">
    <h2 id="demo-heading">A live mesh in 60 seconds</h2>
    <p>Two nodes. One command each. Encrypted WireGuard tunnel up and passing traffic — no
       port forwarding, no control plane to stand up.</p>
    <a class="hero-demo__cta" href="#install">Get the one-liner</a>
  </div>
  <div class="hero-demo__media">
    <video
      id="mesh-demo"
      class="hero-demo__video"
      autoplay muted loop playsinline
      preload="metadata"
      poster="/demo/wgmesh-60s-demo-poster.jpg"
      aria-label="60-second wgmesh mesh demo"
      width="1280" height="720">
      <source src="/demo/wgmesh-60s-demo.webm" type="video/webm">
      <source src="/demo/wgmesh-60s-demo.mp4" type="video/mp4">
      <img src="/demo/wgmesh-60s-demo.gif"
           alt="Animated 60-second wgmesh mesh demo"
           width="1280" height="720"
           loading="eager" decoding="async">
    </video>
  </div>
</section>
```

Implementation notes for the markup:
- `autoplay muted loop playsinline` is required for reliable autoplay on iOS Safari and Chrome.
- `preload="metadata"` plus a `poster` keeps initial paint cheap; the video stream only loads once
  the element is in view (Task 3 adds `IntersectionObserver` gating to defer decoding further on
  slow connections).
- The `<img>` inside `<video>` is the GIF fallback for browsers with video disabled; it doubles as a
  no-JS / very-old-browser path.
- Anchor the "Get the one-liner" CTA to the existing `#install` section; do not invent a new page.

### Task 3: Add CSS for the hero demo band

Add a scoped `.hero-demo` style block to the existing `<style>` in `public/index.html` (do not add an
external stylesheet). Requirements:

- Desktop (default): two-column grid — copy on the left (≈ 38%), media on the right (≈ 62%). Cap the
  section height so pricing remains reachable without scrolling on a 1080p desktop viewport: target
  hero band height ≤ 480px, video displayed at `aspect-ratio: 16 / 9; max-width: 100%; height:
  auto;` and `border-radius: 8px; box-shadow: 0 4px 20px rgba(0,0,0,0.08);`.
- Use the page's existing palette: blue accent `#0056b3`, button green `#28a745`, background tint
  `#f0f7ff` / dashed border `#a0d1ff` already used by `.quickstart-section` so the hero feels native.
- CTA button `.hero-demo__cta` reuses `.cta-button` styling (green, hover lift).
- Mobile (`@media (max-width: 768px)`): collapse to a single column, copy first then media. Reduce
  heading size. Keep the video `width: 100%`.
- Reduced motion: add
  `@media (prefers-reduced-motion: reduce) { .hero-demo__video { display: none; } .hero-demo__media img { display: block; } }`
  so reduced-motion users see the static poster/GIF still frame instead of looping motion. Make sure
  the fallback `<img>` is shown in that case.

### Task 4: Defer autoplay for offscreen / slow connections (progressive enhancement)

Add a small inline `<script>` at the end of `<body>` (next to the existing OpenPanel script) that:

1. Selects `#mesh-demo`.
2. Uses `IntersectionObserver` to call `video.play()` only when the hero enters the viewport, and
   `video.pause()` when it leaves, reducing CPU/battery use on background tabs.
3. If `navigator.connection?.saveData === true` or `navigator.connection?.effectiveType` is
   `2g`/`slow-2g`, sets `video.preload = 'none'`, does **not** call `play()`, and leaves the poster
   image visible. This protects metered connections from a multi-MB autoplay.
4. Wraps every call in a `try/catch` and checks `video.play` is a function, so a failure never
  blocks page render.

### Task 5: Analytics engagement event

Extend the existing OpenPanel instrumentation (do not add a new analytics SDK). When the demo
becomes visible via the `IntersectionObserver` from Task 4, fire once per page load:

```js
window.op && window.op('track', 'demo_viewed', { asset: 'wgmesh-60s-demo', placement: 'hero' });
```

No PII is collected — the event carries only the asset name and placement.

### Task 6: Update OpenGraph / social preview (optional but recommended)

Add to the `<head>` of `public/index.html`:

```html
<meta property="og:image" content="https://cloudroof.eu/demo/wgmesh-60s-demo-poster.jpg">
<meta property="og:image:alt" content="60-second wgmesh mesh demo">
<meta property="og:video" content="https://cloudroof.eu/demo/wgmesh-60s-demo.mp4">
<meta property="og:video:type" content="video/mp4">
<meta name="twitter:card" content="player">
```

Use the absolute `https://cloudroof.eu/...` URLs since OG tags must be absolute for crawlers.

### Task 7: Verification checklist

- `public/index.html` renders with `#demo` as the first section inside `.container`, before `#pricing`.
- On a 1920×1080 desktop viewport with default Chrome, the demo video is fully visible without
  scrolling, and at least the heading of `#pricing` is reachable within one screen-height.
- On a 414×896 mobile viewport, copy and a 16:9 video stack vertically and nothing overflows
  horizontally.
- With `prefers-reduced-motion: reduce`, the looping video is replaced by the static still.
- Total above-the-fold transfer (HTML + poster + first video segment) is reasonable: poster ≤ 80 KB;
  video segments are streamed, not fully downloaded on load.
- No console errors; the page still passes a basic HTML validator run.
- `git status` shows only `public/index.html` modified and `public/demo/*` added — no unrelated files
  touched.

## Acceptance Criteria

1. A new `<section id="demo">` exists in `public/index.html` as the first child of
   `<div class="container">`, above the existing `#pricing` section.
2. The section embeds a 55–60-second looping demo as a `<video autoplay muted loop playsinline>`
   with WebM + MP4 sources and a GIF `<img>` fallback, plus a `poster` still.
3. Demo assets exist at `public/demo/wgmesh-60s-demo.{webm,mp4,gif}` and
   `public/demo/wgmesh-60s-demo-poster.jpg`, none of which contain secrets, real keys, customer
   data, or revenue figures.
4. On a 1080p desktop viewport the demo is fully visible above the fold without scrolling.
5. On a ≤ 414px mobile viewport the section reflows to a single column with no horizontal overflow.
6. `prefers-reduced-motion: reduce` users see the static poster/GIF still instead of looping motion.
7. On metered/slow connections (`Save-Data` or `effectiveType` `2g`/`slow-2g`) the video does not
   autoplay and the poster is shown.
8. An `IntersectionObserver` gates `play()`/`pause()` and fires a single `demo_viewed` OpenPanel
   event per page load.
9. OpenGraph/Twitter meta tags point at the poster and MP4 via absolute `https://cloudroof.eu/`
   URLs.
10. Only `public/index.html` and new files under `public/demo/` are changed; no unrelated files are
    modified.

## Out of scope

- Re-encoding or re-creating the canonical install script (`https://install.wgmesh.dev`); this issue
  only captures it on video.
- Redesigning `#pricing`, `#features`, or any other existing section beyond inserting the new hero
  above them.
- Building a separate `/demo` landing page or interactive (click-to-run) demo; this is a passive
  embedded media asset only.
- Adding a new analytics platform; engagement is tracked through the existing OpenPanel script.
- Changes to the developer-facing `wgmesh.dev/index.html` or `docs/` pages — this issue targets the
  cloudroof.eu homepage served from `public/index.html`.
- Subtitling/captions for the video (it has no audio track); a text transcript could be a follow-up
  issue if accessibility review requests it.
