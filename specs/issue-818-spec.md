# Specification: Issue #818 — Build 'cloudroof vs Headscale' comparison landing page with trial signup CTA

## Classification
feature

## Problem Analysis

wgmesh's product surface is shipped under the `cloudroof` brand (the landing/managed-ingress offering at cloudroof.eu), and its strongest structural differentiator versus every other WireGuard-mesh tool is the absence of a coordination server. `FEATURE_MATRIX.md` already enumerates this in plain language: the cost of running Tailscale's protocol on your own is "trust Tailscale's control plane **or run Headscale**." That single sentence is the wedge for a `cloudroof vs Headscale` landing page — Headscale is the exact alternative the self-hosting buyer evaluates, and there is currently **no page that targets that comparison head-to-head with a conversion path**.

Current state of the relevant artifacts:

- **`docs/comparison/`** contains `tailscale.md` and `README.md` but **no `headscale.md`**. The Headscale positioning today lives only as one column in the `FEATURE_MATRIX.md` competitive table and a passing mention in `STRATEGY.md` ("we are not Headscale-as-a-service"). There is no long-form, defensible, Headscale-specific comparison document.
- **`FEATURE_MATRIX.md`** already carries the auditable, public, non-controversial facts needed for a head-to-head Headscale table: coordination-server model (Headscale = self-hosted control plane; wgmesh = none, DHT), bootstrap UX (account+login vs one secret), NAT traversal (Headscale still needs DERP relays; wgmesh uses DHT-driven + Dandelion++), works-fully-offline (Headscale ❌ needs control plane; wgmesh ✅ multicast), Magic DNS, mobile clients, SSO/OIDC (Headscale 🟡 OIDC), open-source license (both, BSD-3 vs MIT), self-hostable end-to-end, encrypted state at rest, EU/GDPR posture, vendor lock-in, time-to-mesh, CLI-only operation, and "maintained by" (community vs solo+AI). These rows are the source of truth for the comparison section.
- **`public/index.html`** is the generic homepage. It has a Polar.sh sponsorship section and a Buttondown `meshletter` embed form posting to `https://buttondown.com/api/emails/embed-subscribe/meshletter`, plus an exit-intent modal using the same endpoint. It has **no trial signup CTA** and no "vs Headscale" framing.
- **`public/vpn-alternative.md`** and `public/tailscale-alternative.html`/`.md` (issue #801) target adjacent intents ("self-hosted VPN" and "Tailscale alternative") but neither targets the self-hosted-control-plane buyer specifically, which is the Headscale audience: an operator who already rejected SaaS Tailscale and is now weighing "stand up Headscale myself" against "run cloudroof."
- **Trial attribution plumbing:** `pkg/promo` already implements `Campaign`, `Source` (an enum: `SourceDiscord … SourceUnknown`), `Store`, `GenerateCode`, and `Redemption`. There is no `Source` value for landing-page-driven signups, so trial conversions coming from this funnel cannot be attributed today.

The gap, concretely: a self-hosting buyer who arrives from a "Headscale alternative," "self-hosted Tailscale control plane," or Headscale-HN/Reddit thread lands on the generic homepage, sees sponsorship tiers (not a trial), and has no single page that (a) states the head-to-head structural difference (no control plane at all, vs Headscale's self-hosted-but-still-a-control-plane), (b) hands a one-command quickstart, and (c) offers a trial signup. This issue closes that gap with one new static page, a minimal attribution hook in `pkg/promo`, and reciprocal cross-links.

A genuine concern worth noting (not blocking): there is no `docs/comparison/headscale.md` long-form source yet, unlike the Tailscale page which had one. To keep every claim defensible, this spec requires the page's comparison rows to be sourced **directly from `FEATURE_MATRIX.md`** (which already has a Headscale column), and treats the optional creation of a `docs/comparison/headscale.md` as a soft dependency / out-of-scope-but-recommended item. The page is buildable today without the long-form doc.

## Proposed Approach

Add a new static, conversion-focused landing page at `public/cloudroof-vs-headscale.html` (and a Markdown mirror at `public/cloudroof-vs-headscale.md` for SEO/indexing parity with `vpn-alternative.md` and `tailscale-alternative.md`). Build it as self-contained single-file HTML/CSS in the same style as `public/index.html` and `public/tailscale-alternative.html` — no build step, no JS framework — so it deploys through the existing static-asset pipeline and opens correctly via `file://`.

Page structure (top to bottom):

1. **Hero** — H1 targeting "cloudroof vs Headscale" (e.g., "cloudroof vs Headscale: a WireGuard mesh with no control plane to operate"). A one-line differentiator framing the structural difference: Headscale removes Tailscale-the-company but keeps the coordination-server architecture (a control plane you must host, run, back up, and keep available); cloudroof/wgmesh removes the control plane entirely via DHT discovery. Primary trial CTA plus a secondary "See the 60-second demo" link that scrolls to the existing demo embed (reuse `/demo/wgmesh-60s-demo.webm|mp4|gif` already in `public/demo/`).
2. **Trial signup CTA band** — primary conversion element. Inline email-capture form posting to the Buttondown `meshletter` list (identical mechanism to `public/index.html` and the exit-intent modal), with a hidden attribution field (see plumbing below) so conversions from this page are taggable in Buttondown. On submit success, show an inline success state mirroring the `#wgmesh-exit-modal .wgmesh-exit-success` pattern. The form must remain fully functional with JS disabled: a plain `<form method="post" target="popupwindow" action="…embed-subscribe/meshletter">` that falls back to Buttondown's hosted confirmation. The email input must be `<input type="email" name="email" required placeholder="you@example.com">` (matching the existing convention).
3. **Head-to-head comparison table** — a focused cloudroof/wgmesh-vs-Headscale table whose rows are copied verbatim in spirit from the Headscale column of `FEATURE_MATRIX.md`. Required rows, each factual and already defensible: coordination-server model (none/DHT vs self-hosted control plane), discovery mechanism (DHT + LAN multicast + gossip vs control plane + DERP), NAT traversal (DHT-driven vs needs DERP relays), works fully offline/LAN (✅ vs ❌ needs control plane), trust/account model (one shared secret vs account + login + OIDC), bootstrap UX, routable subnets, MagicDNS (❌ vs ✅), mobile clients (❌ vs ✅ via Tailscale client), SSO/OIDC (❌ vs 🟡), open-source license (MIT vs BSD-3), self-hostable end-to-end (✅ no server vs ✅ but you operate the server), encrypted state at rest (AES-256-GCM vs 🟡 DB-level), EU/GDPR-friendly posture (✅ vs depends on host), vendor lock-in (low vs low), time-to-mesh LAN (<5s vs ~30s), time-to-mesh Internet (15–60s vs 5–10s), CLI-only operation, and maintained by (solo+AI vs community). The honest "where cloudroof loses" rows (mobile, MagicDNS, SSO/OIDC, slower first-connection) **must be included** — do not omit them; the FEATURE_MATRIX explicitly says "the honest column is where wgmesh is behind. Don't hide it on the landing page."
4. **"The Headscale trap" / why-people-switch section** — three to four concrete reasons grounded in existing public positioning: (a) Headscale still means you operate a control plane — install, upgrade, back up the DB, keep it online, expose it securely; cloudroof removes that entire surface; (b) no DERP-relay operational burden (Headscale's NAT traversal still needs DERP relays, which you also self-host); (c) works on air-gapped / LAN-only sites where a control plane cannot exist; (d) EU-first posture (Hetzner, deSEC) and `STRATEGY.md`'s explicit line that cloudroof is "not Headscale-as-a-service." Keep every claim sourced to `FEATURE_MATRIX.md` or `STRATEGY.md`; no new unverified claims (no invented Headscale limitations, no invented user counts).
5. **Quickstart band** — the canonical install/join one-liners from `README.md` and `public/vpn-alternative.md`:
   ```bash
   wgmesh init --secret
   wgmesh join --secret "wgmesh://v1/<your-secret>"
   ```
   Link the "full walkthrough" anchor to `docs/quickstart.md` (relative `../docs/quickstart.md`).
6. **Secondary trial CTA** repeated at the bottom (same form action + attribution tag as #2) for scroll-through visitors.
7. **Footer** — reuse the homepage footer convention: copyright, "Powered by WireGuard®", Polar.sh attribution, and links to GitHub / Discord / docs.

SEO / metadata requirements:

- In the `.md` mirror, YAML front-matter with `title`, `description`, `og:image`, `og:type` (parity with `vpn-alternative.md`). In the `.html`, matching `<meta>` tags: a `<title>` and meta description targeting "cloudroof vs Headscale", `og:type=website`, `og:image` (reuse `https://github.com/atvirokodosprendimai/wgmesh/raw/main/docs/img/social-card.png` used by `vpn-alternative.md`, or `https://cloudroof.eu/demo/wgmesh-60s-demo-poster.jpg` used by the homepage), and matching `twitter:` cards.
- `<link rel="canonical">` to avoid duplicate-content penalties vs the sibling landing pages.
- A clearly marked internal link to the long-form comparison. Because `docs/comparison/headscale.md` does not yet exist, link to `docs/comparison/README.md` (or the `FEATURE_MATRIX.md` section) as the source of the comparison facts, and add a `TODO`/comment noting that once `docs/comparison/headscale.md` exists, the link target should be updated to it. (Creating that long-form doc is explicitly out of scope here but recommended as a follow-up.)

Conversion / attribution plumbing (small, additive, no behavior change to existing sources):

- Add a new `Source` value to `pkg/promo` for landing-page-driven trials so future promo campaigns generated from this funnel are attributable. Add `SourceLandingHeadscale Source = "landing-headscale"` to the const block in `pkg/promo/types.go` (currently `SourceDiscord … SourceUnknown`). This is a string-const addition. Wherever `Source` is switched on or enumerated (search the package for `SourceUnknown` as the default fallback), ensure the new source is accepted; if any test enumerates the full const block, update it to include the new source.
- Add a Buttondown hidden attribution field on **both** the hero and footer CTA forms on the new page, e.g. `<input type="hidden" name="tag" value="cloudroof-vs-headscale">`. Buttondown accepts a `tag` field on the embed-subscribe endpoint to tag subscribers; this is the only attribution mechanism available without a backend and matches the existing dependency-free form pattern used by `public/index.html`. Document in the page's top HTML comment that the `tag` value is load-bearing for attribution so a future maintainer doesn't strip it.

Cross-linking (single-anchor additions, no restructuring):

- Add one link from `public/index.html` to the new page in the `Explore Further` links section (and a top nav link if a nav exists) — labeled e.g. "cloudroof vs Headscale".
- Add a reciprocal link from `public/vpn-alternative.md` and from `public/tailscale-alternative.html` ("Comparing self-hosted control planes? → cloudroof vs Headscale") so the three comparison funnels interlink.
- If `docs/comparison/headscale.md` is created later as a follow-up, add a reciprocal link from it to this page; this is a soft dependency, not part of this issue's acceptance criteria.

Accessibility & content-safety constraints (matching repo conventions in `public/demo/README.md` and the issue #801 spec):

- Page must be keyboard-navigable, with `aria-labelledby` on each major `<section>`, and the email input must have a visible `<label>`.
- No secrets, customer PII, real public/peer keys, production IPs, or exact revenue/ARR figures anywhere. All example mesh addresses use RFC1918 or `100.64.0.0/10`. Example emails use the existing `you@example.com` placeholder. Do not invent Headscale pricing or quote specific customer counts; if cost framing is used, phrase it qualitatively ("scales with infrastructure, not per-user") consistent with `docs/comparison/tailscale.md`'s cost section, without fabricating exact Headscale TCO numbers.
- Trial language must not invent pricing tiers. If a managed trial is not generally available, the CTA captures the email for "trial access when your turn comes up" rather than promising an instant paid trial.

## Acceptance Criteria

- A new file `public/cloudroof-vs-headscale.html` exists, is valid HTML5 (passes `npx --yes html-validator-cli public/cloudroof-vs-headscale.html`, or if offline, `tidy -q -e` exits without fatal errors), and renders with no external build step (opens correctly via `file://`).
- A Markdown mirror `public/cloudroof-vs-headscale.md` exists with YAML front-matter (`title`, `description`, `og:image`, `og:type`) and a canonical note.
- The page contains a head-to-head cloudroof/wgmesh-vs-Headscale comparison table whose rows are consistent with the Headscale column in `FEATURE_MATRIX.md`, and which **does not omit** the documented honest gaps (no mobile clients, no MagicDNS, no SSO/OIDC, slower Internet time-to-mesh).
- Two trial-signup CTA forms (hero + footer) are present, both posting to the Buttondown `meshletter` embed endpoint (`https://buttondown.com/api/emails/embed-subscribe/meshletter`) with a hidden attribution `tag` field, and both degrade gracefully (core signup works with JS disabled).
- The quickstart section reproduces the canonical `wgmesh init --secret` / `wgmesh join --secret "wgmesh://v1/<your-secret>"` commands verbatim and links to `docs/quickstart.md`.
- `pkg/promo/types.go` contains a new `SourceLandingHeadscale Source = "landing-headscale"` constant and `go build ./...` passes.
- Existing promo tests still pass: `go test ./pkg/promo/...` is green, and any test enumerating the `Source` const block is updated to include the new source.
- `public/index.html`, `public/vpn-alternative.md`, and `public/tailscale-alternative.html` each contain exactly one new link to the new page; no other content on those pages is changed.
- A Lighthouse/desktop pass (manual or CI) on the new page scores ≥ 90 for Performance, Accessibility, Best Practices, and SEO on the static HTML; the page has a `<link rel="canonical">` and `og:`/`twitter:` social tags.
- No secrets, PII, production IPs/keys, or revenue figures appear in either new file. Grep check returns nothing load-bearing: `grep -nE '(sk-|ghp_|password|BEGIN.*PRIVATE KEY)' public/cloudroof-vs-headscale.*` is empty; example emails use the existing `you@example.com` placeholder.
- Copy claims are defensible: every comparative assertion on the page maps to a statement already present in `FEATURE_MATRIX.md`, `STRATEGY.md`, `docs/comparison/tailscale.md`, or `public/vpn-alternative.md`. No new unsubstantiated claims (no invented Headscale limitations, no fabricated Headscale pricing or customer counts).

## Out of scope

- Creating the long-form `docs/comparison/headscale.md` reference document. It is recommended as a follow-up so this page can link to richer objective content, but the page is buildable today using `FEATURE_MATRIX.md` as the single source of truth.
- Building a real managed-trial backend / account system. The CTA captures email via Buttondown; actual trial provisioning via `pkg/promo` codes is a separate follow-up. This page only needs the attribution `Source` constant and the Buttondown `tag`, not a redemption endpoint.
- Rewriting or redesigning `public/index.html` beyond adding one nav/footer link.
- Mobile clients, SSO/OIDC, or MagicDNS — these are existing documented gaps; the page must acknowledge them, not solve them.
- Paid ad campaigns, A/B testing infrastructure, or analytics SDK integration beyond what the homepage already uses.
- Creating or editing the 60-second demo video asset — owned by `public/demo/`; this page only embeds the existing asset.
- Changes to Polar.sh sponsorship tiers or pricing — the page is about trial signup, not sponsorship pricing.
- Positioning changes to the broader `cloudroof` brand identity — this page uses the existing brand as already referenced in `STRATEGY.md` and the homepage.
