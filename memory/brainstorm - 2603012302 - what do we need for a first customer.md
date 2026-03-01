---
tldr: Brainstorm on the shortest path from current state to first paying customer for wgmesh
status: complete
---

# Brainstorm: What do we need for a first customer?

## Seed

wgmesh is a WireGuard mesh networking tool with DHT discovery, NAT traversal, and an autonomous company control loop.
The [[spec - first-customer - roadmap to first paying customer]] defines the vision but the question is: what's the *minimum* path from where we are today to someone paying?

**What exists today:**
- Working wgmesh binary (builds, packages: .deb/.rpm/Nix/Homebrew as of v0.2.0-rc1)
- Autonomous company control loop (daily LLM assessments via OpenRouter)
- Chimney dashboard at chimney.beerpub.dev (pipeline visibility)
- Agent pipeline (Goose builds, Copilot reviews, auto-merge)
- cloudroof.eu domain (product site placeholder)
- Show HN draft and stargazer DM templates ready
- European-first infrastructure (Hetzner, deSEC, Migadu)
- Lighthouse CDN control plane (code exists, not yet productised)

**What's missing (from spec):**
- `wgmesh service add` CLI (#372 — in triage)
- Managed ingress product (the thing people would pay for)
- Billing integration (Polar.sh identified, not wired)
- Landing page repositioned for AI gateway / homelab use case
- Cost tracking (costs.json has nulls)
- No real users yet beyond dogfooding

**Constraint:** Near-zero budget, solo founder + AI agents, EU-first.

## Ideas

- Ship the Show HN post now — get attention before product is "ready"
- Find one person manually who needs mesh networking and offer free setup in exchange for feedback
- Offer wgmesh as a "bring your own nodes" CDN — users provide servers, we provide the mesh
- Make `nix run github:atvirokodosprendimai/wgmesh` the 30-second try-it path
- Create a 2-minute demo video showing mesh setup between 2 VPS nodes
- Polar.sh integration for founding member subscriptions ($5/mo tier)
- Target homelab communities (r/homelab, r/selfhosted) with "zero-infra mesh" positioning
- Position as "Tailscale alternative for people who want to own their infrastructure"
- Build a one-click Docker Compose that sets up a 3-node mesh locally for evaluation
- Write a tutorial: "Connect your homelab to your VPS with wgmesh in 5 minutes"
- Skip billing entirely — offer free beta, collect email, charge later
- Partner with a small hosting provider to offer wgmesh as a network add-on
- Make the dashboard public and impressive — let it sell the engineering quality
- GitHub Sponsors as the simplest possible payment path (already exists on GitHub)
- Create a Discord/Matrix community and personally invite interested stargazers
- Focus on a single vertical: "AI inference mesh" — route LLM requests across homelab GPUs
- Offer managed mesh setup as a service ($50/mo for up to 10 nodes, you handle config)
- Open source the full thing, monetise support/managed hosting
- Create a comparison page: wgmesh vs Tailscale vs Netmaker vs Nebula
- Make the landing page a live dashboard showing a real mesh in action
- Write a blog post: "How I run a company with zero employees using AI agents"
- Ship founding member perks: name on dashboard, roadmap votes, private channel
- Find a YC/indie hacker founder who needs cross-cloud networking and offer to set it up
- Create a GitHub Action that deploys wgmesh across your infrastructure
- "Edge function mesh" — deploy functions at the edge of your own network
- Just ask on Twitter/HN/Reddit: "What would you pay for in a mesh networking tool?"
- Build a Terraform/Pulumi provider for wgmesh
- Target small DevOps consultancies who manage multiple client networks
- Offer free for open-source projects, paid for commercial
- Launch on Product Hunt with the "AI-built company" angle
- Integrate with Coolify/CapRover for self-hosted PaaS networking

## Clusters

### Outreach and attention (get eyeballs)
- Ship the Show HN post now
- Target homelab/selfhosted communities
- Write the "AI agents run my company" blog post — this is the *story* people share
- Launch on Product Hunt
- Ask publicly: "what would you pay for?"
- Comparison page vs Tailscale/Netmaker/Nebula
- Make the dashboard public and impressive

### Try-it path (reduce friction to zero)
- `nix run github:atvirokodosprendimai/wgmesh` one-liner
- Docker Compose 3-node evaluation setup
- 2-minute demo video
- "Connect homelab to VPS in 5 minutes" tutorial
- GitHub Action for automated deployment

### Revenue mechanics (how money flows in)
- Polar.sh founding member subscriptions
- GitHub Sponsors (simplest, already exists)
- Skip billing, do free beta, collect emails
- Managed mesh setup as a service ($50/mo)
- Free for OSS, paid for commercial

### Positioning (why wgmesh, not X)
- "Tailscale but you own everything"
- "AI inference mesh" for homelab GPU routing
- "Zero-infra anycast CDN"
- "Edge function mesh"
- BYOB CDN (bring your own boxes)

### Community and relationships (one person at a time)
- Find one person manually, offer free setup for feedback
- Discord/Matrix community, invite stargazers
- DM stargazers with founding member pitch
- Find a YC/indie founder who needs cross-cloud networking
- Target small DevOps consultancies

### Ecosystem integrations (reach users where they are)
- Terraform/Pulumi provider
- Coolify/CapRover integration
- GitHub Action for mesh deployment
- Docker Compose for evaluation

## Standouts

1. **Ship the Show HN post with the "AI agents run my company" angle**
   The tech is interesting but the *story* is what gets shared.
   A working autonomous company loop is genuinely novel.
   Combines attention-getting with authenticity — the dashboard proves it's real.

2. **Create a 5-minute try-it path (Docker Compose or nix run)**
   Attention without a try-it path is wasted.
   The fastest path: `docker compose up` spins up 3 nodes that mesh automatically.
   Or `nix run github:atvirokodosprendimai/wgmesh` for Nix users.

3. **Polar.sh for founding member revenue**
   Simplest payment path that's already identified.
   $5/mo founding member with name-on-dashboard, roadmap votes.
   No complex billing — just a link that works.

4. **Find ONE person manually and do their setup for free**
   The first customer doesn't need to find you — you find them.
   r/homelab, r/selfhosted, HN "Ask" threads about mesh networking.
   Free setup, real feedback, case study for the landing page.

5. **"Connect your homelab to your VPS in 5 minutes" tutorial**
   Solves a real, specific, common problem.
   SEO-friendly, shareable, proves the product works.
   This is the content that converts drive-by visitors to users.

## Next Steps

**Selected:** 3 (Polar.sh) and 4 (find one person manually).

- => #376 created for Polar.sh setup (needs-human)
- => [[outreach - 2602240805 - stargazer dm template]] updated with personalised messages for all 5 targets
- Swap sponsor links for Polar.sh checkout URLs once #376 is resolved
- Send outreach now with GitHub Sponsors links (don't wait for Polar.sh)
