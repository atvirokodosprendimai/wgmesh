---
tldr: What actions get the first paying customer for wgmesh/chimney.beerpub.dev as fast as possible
status: active
---

# Brainstorm: First Paying Customer for wgmesh

## Seed

chimney.beerpub.dev is the wgmesh project dashboard.
It shows the autonomous AI coding pipeline (Copilot spec → Goose impl → auto-merge), DORA metrics, a 3-year roadmap, and a capability matrix.
Three sponsorship tiers exist via GitHub Sponsors:
- **Contributor** $5/mo — badges, early access
- **Edge Node** $20/mo — 1 free CDN node *when launched*
- **Mesh Operator** $100/mo — 5 nodes, direct support, feature voting

The CDN product (zero-infrastructure anycast CDN on WireGuard mesh) is not yet GA — the $20 tier promises a node "when launched".
Payment today goes through GitHub Sponsors.
A secondary CTA points to "cloudroof.eu" (unclear what that does).
No paying customers yet.

Goal: one paying customer, as fast as possible.
Constraints: small team, no marketing budget, autonomous pipeline is the core differentiator.

Related specs: [[spec - chimney - dashboard server with github api proxy and two-layer cache]]
Related brainstorm: [[brainstorm - 2602211225 - chimney integration with table.beerpub.dev]]

## Ideas

- Direct outreach: DM 10–20 GitHub stargazers / watchers personally
- Post the dashboard link on Hacker News as a "Show HN"
- Post in WireGuard, homelab, self-hosting Discord/Reddit communities
- Make the autonomous pipeline the story — it's genuinely novel
- Write a short blog post: "I built a CDN with WireGuard and an AI that writes its own code"
- Share the pipeline dashboard on Twitter/X as a "building in public" thread
- Add a "founding member" time-limited tier (scarcity + early-adopter framing)
- Give founding members named credit on the dashboard / README
- Offer the first 10 paying customers a lifetime discount
- Reduce payment friction — GitHub Sponsors adds steps; consider Polar.sh or Stripe
- Make the $5/mo tier obviously worth it: add a concrete benefit that exists NOW (not "when launched")
- Add a waitlist with invite codes to build urgency and social proof
- Make a "try the mesh" demo — even a read-only one showing real peers
- Add a 1-click "become an Edge Node" CTA that lands directly on GitHub Sponsors
- Add a visible customer counter ("0 founding members" → update as people join)
- Clarify what cloudroof.eu is and whether it's the right destination for CTAs
- Replace "when launched" with a concrete beta date or milestone percentage
- Add a "why wgmesh" section above the fold — currently opaque to non-engineers
- Record a 2-min demo video showing the CDN in action (even if beta quality)
- Post on Indie Hackers about the autonomous pipeline approach
- Reach out to homelab YouTubers (NetworkChuck, Craft Computing, etc.) for a mention
- Create a GitHub Discussions post inviting early adopters to a beta program
- Add social proof: GitHub star count, uptime percentage, pipeline run count
- Make the roadmap progress visually obvious — "X% to beta CDN launch"
- Identify one specific ICP (e.g. "indie dev with 3+ VPS servers") and write directly for them
- Add a "#0001 founding member" badge/NFT concept for the very first paying customer
- Write a technical breakdown of the DHT discovery + NAT traversal — drives dev interest
- Submit to ProductHunt as "early access" — drives attention even pre-launch
- Email/newsletter to anyone who has starred the GitHub repo
- Host an async AMA in the GitHub Discussions about the architecture
- Add pricing anchoring: show what AWS CloudFront costs vs what wgmesh will cost
- Add an "edge node map" showing where CDN nodes exist/will exist
- Partner with another indie infra project for cross-promotion
- Create a "sponsor wall" on the dashboard that's visible to visitors
- Make the sponsorship tiers feel like investing in a project, not buying a product (community framing)
- Add a Discord or Matrix server and invite early supporters
- Write about the 503 postmortem + recovery as a transparency/trust signal
- Show the DORA metrics publicly — demonstrate shipping velocity
- Add a "what's blocking traction" section directly on the dashboard (already there?) and link it to roadmap
- Make the CTA for the $5 tier instant-gratification: "badge appears in 24h"
- Use the autonomous pipeline story as a differentiator in outreach: "this project ships itself"
- Add a changelog / "what shipped this week" section — builds confidence in delivery cadence
- Reach out to WireGuard creator or prominent WireGuard projects for a mention
- Create a "build with wgmesh" guide targeting homelab builders
- Write about NAT traversal techniques used — technical content drives organic discovery
- Add OpenGraph / Twitter card metadata to make the dashboard share well
- Make the dashboard mobile-friendly — shares happen on mobile
- Target companies with distributed edge workers (small CDN customers)
- Offer a "sponsored deployment" — pay $100, get a custom wgmesh edge node set up for you

## Clusters

### Reduce friction to paying

- GitHub Sponsors has multi-step friction — explore Polar.sh / Stripe as lower-friction alternatives
- 1-click CTA that lands directly on the correct sponsor tier
- Make the $5/mo tier have an immediate, tangible reward (not just "early access")
- Clarify cloudroof.eu and make it a coherent part of the funnel, not a mystery link
- Add visible customer counter so visitors know paying is possible
- "Founding member #0001" framing — makes the first purchase feel like a milestone

### Distribution — get in front of the right people

- Direct personal outreach to GitHub stargazers (highest conversion, lowest volume)
- Show HN: autonomous pipeline + WireGuard CDN is genuinely novel
- Homelab / WireGuard / self-hosting communities (Reddit r/selfhosted, r/homelab, Discord servers)
- Indie Hackers — "building in public" framing fits the autonomous pipeline story
- Twitter/X building-in-public thread showing the pipeline dashboard
- Email GitHub stargazers

### Tell the right story

- The autonomous pipeline is the hook — lead with that, not CDN features
- Write: "I built a CDN that ships itself" — blog post, tweet thread, HN submission
- Technical deep-dives (DHT, NAT traversal) drive developer discovery organically
- Transparency signals: 503 postmortem, DORA metrics, public roadmap — use them
- "What's shipping this week" changelog builds confidence in delivery velocity

### Make the product feel real NOW

- The "when launched" phrasing defers value — replace with beta date or milestone %
- "Try the mesh" demo, even read-only, makes the product tangible
- Edge node map (even a 1-node map) shows it's real
- Video demo: 2 min showing actual CDN routing in action

### Value proposition clarity

- Current page is engineering-focused and opaque to non-engineers
- Add "why this matters" / ICP section above the fold
- Pricing anchoring: compare to AWS CloudFront costs
- Identify one specific ICP and write the pitch for them
- Show GitHub star count, uptime %, pipeline run count as social proof

## Standouts

**1. Direct outreach to GitHub stargazers — highest-conversion path**
People who starred the repo already showed interest.
A personal DM explaining the project and asking if they'd be a founding member converts far better than any marketing.
Can be done today with no code changes.

**2. Reduce payment friction — GitHub Sponsors has too many steps**
GitHub Sponsors requires a GitHub account, navigating to the profile, selecting a tier.
Polar.sh or a Stripe payment link can be embedded directly in the page with a single click.
Every extra step loses customers; this is probably the single biggest conversion blocker.

**3. Reframe the $5/mo tier as something that exists NOW**
"Badges and early access" are weak when the product isn't live.
Change it to: "founding member — listed on the dashboard + Discord access + shapes the roadmap."
Instant social reward + community membership is tangible today.

**4. Show HN post — the autonomous pipeline is a genuine story**
"Show HN: I built a zero-infra CDN on WireGuard and an AI that writes and ships its own code"
This targets developers who care about both WireGuard and AI-augmented dev workflows.
Even if conversion is low, it drives awareness and GitHub stars which compound.

**5. Replace "when launched" with a concrete beta access offer**
The $20 Edge Node tier promises a node "when launched" — this defers all value.
Change it to: "beta access starts at X members" or "beta Q2 2026 — you're in queue."
Concrete > vague; urgency > someday.

## Next Steps

- 1 — Implement reduced-friction payment: swap GitHub Sponsors CTA for direct Polar.sh/Stripe link
- 2 — Rewrite $5 tier benefits (founding member frame, instant reward)
- 3 — Update $20 tier to show concrete beta timeline or milestone
- 4 — Draft direct outreach message for GitHub stargazers
- 5 — Draft Show HN post
- 6 — Clarify what cloudroof.eu is and whether it's still the right CTA destination
- 7 — Add founding member counter to dashboard (even at 0 — sets expectation)
