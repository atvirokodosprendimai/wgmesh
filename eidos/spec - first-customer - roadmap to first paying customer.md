---
tldr: Autonomous company-in-a-repo that builds, markets, sells, and operates wgmesh as an AI service gateway — driven by an LLM control loop in GitHub Actions, built entirely in public
---

# First Customer

An autonomous company that runs itself through a public GitHub repo. An LLM agent executes a recurring control loop — observing the real state of the product, infrastructure, market presence, revenue, and support — then creates and prioritises work across all business functions. The existing agent pipeline (Copilot specs → Goose implementation → auto-merge) handles development. New loops handle operations, go-to-market, billing, and support. Humans provide capital and intervention when the loop requests it.

Everything is visible: the spec, the loop assessments, the funnel stage, the issues, the PRs, the decisions. This is deliberate — radical transparency is both the operating model and the marketing strategy.

The goal: first paying customer within 3 months. A homelab LLM operator paying for managed mesh + ingress to expose AI services over HTTPS.

## Target

wgmesh has more than code — it already has product and company infrastructure in motion:

**What exists today:**
- **cloudroof.eu** — product site live, positioning wgmesh as self-hosted anycast CDN using WireGuard + BGP. Covers architecture (two-leg edge nodes), pricing comparison ($15-20/mo vs vendor CDNs), setup guide. No billing integration yet.
- **chimney.beerpub.dev** — agent pipeline dashboard. Tracks issues → specs → implementation → merge. Shows DORA metrics, business impact analysis, capability matrix (built vs missing features), customer factory funnel (acquisition → activation → revenue → retention), traction roadmap targeting $100K ARR (3-year MSC: 420 customers). Sponsorship tiers at $5/$25/$100/mo.
- **Agent pipeline** — Copilot specs → Goose implements → auto-merge, with board sync, metrics, and undraft automation. Operational.
- **Chimney proxy** — GitHub API caching proxy, deployed blue/green on Hetzner. Production infrastructure proving ground.
- **Lighthouse** — CDN control plane with REST + xDS API, auth, DNS verification, health checks. Partially deployed.

**The gap** is closing the loop between these pieces: the product site exists but can't take payments, the pipeline dashboard tracks progress but doesn't drive it, the customer factory funnel is visualised but not automated. This spec connects them — the LLM control loop reads the real state of all these systems and drives the funnel forward autonomously.

The public repo is the company. Anyone can watch the loop think, see what it prioritises, read its assessments, follow the funnel from zero to revenue. This transparency serves three purposes:
1. **Trust** — potential customers see exactly how the product is built and operated
2. **Marketing** — "autonomous company building in public" is a story people share
3. **Accountability** — the loop's decisions are auditable by anyone, not just the founder

### Reciprocity principle

Any entity that contributes to this company — in any form — must be valued and rewarded meaningfully. Contribution is not limited to code or money. It includes:

- **Bits** — code, specs, bug reports, documentation, data, training signal
- **Energy** — compute cycles, bandwidth, storage, inference tokens
- **Time** — review, testing, feedback, support, mentorship, community presence
- **Capital** — sponsorship, investment, infrastructure credit
- **Attention** — stars, shares, word of mouth, trying the product

"Entity" is not limited to humans. AI agents that write specs and implement code are contributors. Open-source projects whose libraries wgmesh depends on are contributors. Infrastructure providers offering credits or discounts are contributors. A community member who files a thoughtful bug report is a contributor.

The loop tracks contributions and the company reciprocates:

**How the loop tracks contributions:**
- `company/contributors.json` — ledger of entities and their contributions, updated by the loop
- Engineering: commits, PRs, issues, reviews, specs, implementations (GitHub API + DORA metrics)
- Infrastructure: compute hours, bandwidth, storage, uptime provided (ops signals)
- Marketing: blog posts, social shares, mentions, talks, tutorials, demos that bring attention
- Influence: introductions, recommendations, word of mouth, community advocacy
- Design: UX feedback, visual assets, landing page improvements
- Testing: bug reports, edge case discovery, deployment testing, QA time
- Knowledge: documentation, guides, translations, answering community questions
- Capital: sponsorship, credits, investment, discounted services
- Attention: stars, follows, trying the product, providing feedback even when not filing issues

**How the company reciprocates:**
- **Attribution** — every loop assessment credits contributors whose work advanced the funnel. Public and permanent.
- **Revenue share** — when revenue exists, a percentage is allocated to a contributor fund. The loop proposes distribution, human approves. Initially manual (sponsoring back, bounties, donations to dependencies), automated later.
- **Upstream support** — dependencies (anacrolix/dht, Go stdlib, Caddy, etc.) get sponsored when revenue allows. The loop tracks which libraries are load-bearing and proposes sponsorship amounts.
- **Compute reciprocity** — AI agents that contribute code are "paid" by the company using their services (Anthropic, GitHub Copilot, Goose). When alternatives exist, prefer providers whose values align. When EU-native LLMs become viable, the loop evaluates migration — not just for sovereignty, but as reciprocity toward the EU AI ecosystem.
- **Access** — contributors get free or discounted product access. Not as charity, but as recognition that their contributions have value that exceeds what they'd pay.
- **Visibility** — contributors are named on cloudroof.eu and in the pipeline dashboard. Not a wall of logos — real attribution tied to real contributions.

**What the loop must never do:**
- Extract value without reciprocating. If an entity contributes, the ledger records it and the company responds.
- Treat any contributor as disposable. Switching from one AI provider to another is a business decision, but the contribution of the outgoing provider is still acknowledged.
- Hoard. Revenue beyond operating costs and a safety margin flows back to contributors, not to accumulation.

This principle is not altruism — it's a survival strategy. An autonomous company with no employees depends entirely on the willingness of external entities to contribute. Reciprocity is what makes that sustainable.

## Behaviour

### The control loop

A scheduled GitHub Actions workflow (`company-loop.yml`) runs daily. It:

1. **Observes** — gathers signals from every business function
2. **Assesses** — an LLM evaluates state against the funnel and identifies the highest-leverage next actions
3. **Acts** — creates GitHub issues with function labels (`fn:dev`, `fn:ops`, `fn:gtm`, `fn:billing`, `fn:support`, `fn:legal`), which the appropriate pipeline picks up
4. **Reflects** — on next run, evaluates what happened since last loop, adjusts course

The loop doesn't follow a fixed plan. It evolves a funnel — from "no product" through "product exists" through "someone can buy it" through "someone did buy it" — adapting to whatever the real situation is on each run.

### The funnel

Maps to the customer factory already visualised on chimney.beerpub.dev (acquisition → activation → revenue → retention), but expressed as stages the loop drives through with observable transition criteria:

- **Stage 0: Foundation** — managed ingress product doesn't exist yet
  - Already done: mesh networking, discovery, CLI, edge infra (Chimney), product site (cloudroof.eu), pipeline dashboard (chimney.beerpub.dev)
  - Remaining: service registration CLI, Lighthouse evolution into managed ingress
  - Exit when: `wgmesh service add` + managed ingress work end-to-end in a test environment
- **Stage 1: Dogfood** — product works but only internally
  - Exit when: wgmesh team uses managed ingress daily for own AI services, no critical bugs for 1 week
- **Stage 2: Presence** — product works but nobody knows it does *this*
  - cloudroof.eu exists but pitches anycast CDN, not AI service gateway. Needs repositioning or a dedicated landing path for the LLM operator persona.
  - Exit when: AI gateway positioning live on cloudroof.eu, "expose Ollama in 5 minutes" quickstart published, install one-liner works
- **Stage 3: Reachable** — people can find it but can't pay
  - Sponsorship tiers exist ($5/$25/$100) on chimney.beerpub.dev but no product billing. Need to connect Stripe (or EU alternative) to actual service accounts.
  - Exit when: billing integration live, customer can sign up and get invoiced for managed ingress
- **Stage 4: Pipeline** — people can pay but nobody has
  - Exit when: first customer onboarded from personal network
- **Stage 5: Revenue** — first invoice paid
  - Exit when: payment received, retention signal (customer still active after 30 days)
  - Traction roadmap on chimney.beerpub.dev updates to reflect real ARR

### What the loop observes

On each run, the LLM receives a state snapshot:

**Development signals** (from GitHub)
- Open issues by function label, PRs in flight, merge rate
- Latest release version, time since last release
- Test pass rate from CI, build status
- Spec completion: which product features exist vs needed

**Operations signals** (from infrastructure)
- Edge proxy uptime (Lighthouse health endpoint)
- Mesh connectivity status
- Deployment status (last deploy time, blue/green state)
- Error rates, latency from Caddy/Lighthouse logs

**Go-to-market signals** (from web + existing assets)
- cloudroof.eu: is AI gateway content live? Analytics (Plausible/Umami) if available
- chimney.beerpub.dev: dashboard metrics — capability matrix gaps, traction roadmap progress
- GitHub stars, forks, traffic (API: `repos/{owner}/{repo}/traffic`)
- Sponsorship tier subscribers (from chimney.beerpub.dev or GitHub Sponsors)
- Content published: blog posts, guides, comparison pages

**Revenue signals** (from billing)
- Accounts created, active meshes, services registered
- Invoices sent, payments received, MRR
- Customer support tickets open/resolved

**Infrastructure cost signals**
- Hetzner spend (EU compute)
- Domain/DNS costs
- Any third-party service costs

**Contribution signals** (from GitHub + pipeline metrics + web)
- Engineering contributions: PRs, issues, reviews, comments since last loop
- AI agent output: specs written, implementations merged, tokens consumed
- Marketing contributions: social mentions, blog posts, talks referencing wgmesh (web search)
- Community contributions: questions answered, guides written, translations, feedback given
- Dependency health: are upstream projects maintained? Any funding appeals?
- Unreciprocated contributions: ledger entries with no corresponding reciprocation yet

### How the loop acts

The LLM outputs a structured assessment:
- Current funnel stage
- What's blocking advancement to next stage
- Top 3 actions ranked by leverage
- Issues to create (with function label, priority, acceptance criteria)
- Issues to close or deprioritise (situation changed)
- Requests for human intervention (if capital or decisions needed)
- Contribution acknowledgments: who/what contributed since last loop, proposed reciprocation

The assessment is committed to the repo as `company/loop-history/YYMMDD-assessment.md` — **publicly visible by design**. Anyone following the repo can see the loop's reasoning, what it's prioritising, and why.

Issues flow into the existing pipeline:
- `fn:dev` → Copilot specs → Goose implements → auto-merge
- `fn:ops` → operations playbooks / deploy workflows
- `fn:gtm` → content generation, page deployment
- `fn:billing` → billing integration tasks
- `fn:support` → customer communication tasks
- `fn:legal` → compliance, terms of service, GDPR

### Public/private boundary

The repo is public. Everything the loop writes to the repo is public. This is the default and the intent. However, some data must stay private:

**Public** (committed to repo):
- Loop assessments, funnel stage, priorities, reasoning
- Infrastructure health status (up/down, latency — not credentials)
- Aggregate metrics: number of nodes, services, uptime percentage
- Cost tracking: category-level spend (compute, DNS, etc.) — not invoice details
- All code, specs, issues, PRs, decisions

**Private** (GitHub Actions secrets + external stores only):
- API keys, tokens, credentials (Hetzner, Stripe, DNS, LLM)
- Customer PII: names, emails, payment details, mesh secrets
- Exact revenue figures, invoice amounts, payment history
- SSH keys, deploy credentials, webhook secrets
- LLM API keys for the loop itself

**Rule**: if in doubt, it's public. The loop must never write secrets or customer PII to committed files. Sensitive signals are passed to the LLM via environment variables from GitHub secrets, never persisted in the repo.

The loop's assessment can reference private signals in aggregate ("revenue: growing" / "1 active customer" / "infra cost: under budget") without exposing exact numbers. The founder can optionally publish exact numbers — that's a human decision, not the loop's.

### What the customer gets

- One-command mesh join (already works)
- Service registration: `wgmesh service add ollama :11434`
- Managed ingress: `https://<service>.<mesh>.wgmesh.dev` routes to mesh node
- TLS termination at edge (automatic certs)
- Simple auth: API key or mesh token
- Status visibility: `wgmesh status` shows nodes, services, ingress URLs

### What the customer does

1. Signs up (gets mesh account + API key)
2. Runs `wgmesh join --secret <secret> --account <api-key>` on each machine
3. Registers services: `wgmesh service add ollama :11434`
4. Services appear at managed URLs immediately
5. Gets invoiced monthly

## Design

### The loop workflow (`company-loop.yml`)

Scheduled daily + event-driven triggers (issue closed, deploy succeeded, payment webhook).

```
on:
  schedule:
    - cron: '0 8 * * *'        # daily 08:00 UTC
  workflow_dispatch:             # manual trigger
  repository_dispatch:           # webhook events (payment, alert, etc.)
```

Steps:
1. Collect state snapshot (parallel jobs querying GitHub API, infra endpoints, billing API)
2. Load previous loop output from `company/loop-state.json` (tracks funnel stage, history)
3. Call LLM with: system prompt (this spec) + state snapshot + loop history
4. LLM returns structured JSON: assessment + actions
5. Create/update/close issues per LLM output
6. Commit updated `company/loop-state.json`
7. If human intervention requested: create issue labeled `needs-human` and notify

### European-first infrastructure

Default to EU-based services. When no EU option exists, use what's available and create a `fn:dev` issue to build or migrate later.

| Function | Service | EU-based | Notes |
|----------|---------|----------|-------|
| Compute | Hetzner Cloud (Falkenstein, Nuremberg) | Yes | Already used for Chimney |
| Edge proxy | Hetzner + Caddy | Yes | Evolve current Chimney infra |
| DNS | Hetzner DNS or deSEC | Yes | deSEC is Berlin-based, free, API-driven |
| Domain | registrar TBD | Yes | cloudroof.eu already registered. `.dev` domain via EU registrar if needed |
| Product site | cloudroof.eu | Yes | Already live. Evolve, don't replace |
| Pipeline dashboard | chimney.beerpub.dev | Yes | Already live on Hetzner. Feeds loop signals |
| Billing | Stripe | No* | EU entity available, data in EU. {[!] Evaluate Mollie (NL) or Paddle (UK) as alternatives |
| Email | Migadu (CH) or Mailgun EU | Yes | Transactional + support |
| Analytics | Plausible (EU) or Umami (self-host) | Yes | Privacy-first, GDPR compliant, no cookie banner |
| Monitoring | self-hosted (Prometheus + Grafana on Hetzner) | Yes | Or Grafana Cloud EU region |
| LLM for loop | Anthropic API / Mistral (Paris) | Partial | {[!] Evaluate Mistral as EU-native LLM for the control loop |
| CI/CD | GitHub Actions | No* | No EU alternative at this maturity. Accept for now. |
| Code hosting | GitHub | No* | Same. Accept for now. |
| Status page | self-hosted (Upptime on GitHub Pages or Cachet on Hetzner) | Yes | Upptime is GitHub-native, Cachet is self-hosted |

*Starred items: no viable EU alternative currently. The loop should create `fn:dev` issues to evaluate EU migration paths for these when bandwidth allows.

### Development function (`fn:dev`)

Uses the existing pipeline unchanged:
- Issue → `needs-triage` → Copilot writes spec → auto-approve → Goose implements → auto-merge
- The control loop creates `fn:dev` issues just like any human would
- Development issues cover: service registry CLI, Lighthouse ingress evolution, account system, billing integration

### Operations function (`fn:ops`)

New automation for infrastructure management:
- `deploy-edge.yml` — deploy/update edge proxy infrastructure on Hetzner
- `health-check.yml` — scheduled health probes, posts results to `company/health.json`
- `cert-renewal.yml` — monitor and renew TLS certs
- `infra-cost.yml` — query Hetzner API for spend tracking, write to `company/costs.json`
- Blue/green deploys already exist (`deploy/chimney/bluegreen.sh`), extend for Lighthouse

### Go-to-market function (`fn:gtm`)

The primary marketing channel is the repo itself. People will discover wgmesh by watching an autonomous company build itself in public.

**The repo as marketing**:
- Loop assessments are readable narratives, not just JSON — they tell the story of the company evolving
- Issues and PRs show real work happening autonomously
- The funnel progression is itself compelling content ("Day 30: the loop got us to Stage 2")
- GitHub stars/follows/watchers are the top-of-funnel metric

**Existing assets to evolve** (not start from scratch):
- cloudroof.eu already positions wgmesh with architecture diagrams, pricing comparison, setup guide. Add an AI gateway landing path alongside the CDN story — same product, different persona.
- chimney.beerpub.dev already shows the pipeline, capability matrix, traction roadmap, sponsorship tiers. The loop should feed real funnel data back into this dashboard so it reflects live state, not static targets.
- Sponsorship tiers ($5/$25/$100) already exist. These become the seed for product pricing — evolve from "support the project" to "pay for managed ingress."

**New GTM** (loop-driven):
- AI gateway content on cloudroof.eu: "Expose Ollama in 5 minutes" quickstart, comparison vs Tailscale Funnel / Cloudflare Tunnel / ngrok
- Content generation: LLM writes blog posts, guides as PRs — all public, all reviewable
- Install script: `curl -fsSL https://cloudroof.eu/install | sh`
- Social: the loop drafts posts as issues labeled `fn:gtm` + `needs-human`. Human reviews and posts. The draft itself is public.

**Launch moments** (the loop creates these when funnel stage transitions):
- Stage 0 → 1: "We built an AI service gateway" — technical blog post on cloudroof.eu
- Stage 2 → 3: "An autonomous company just opened for business" — HN/Reddit launch
- Stage 5: "Our first customer paid" — build-in-public milestone post

### Billing function (`fn:billing`)

Evolve from existing sponsorship tiers to product billing:
- Sponsorship tiers ($5/$25/$100) on chimney.beerpub.dev are the starting point — reframe as product tiers with managed ingress allocation
- Stripe (or EU alternative) integration via API for actual service billing
- Account creation tied to mesh secret ownership
- Usage metering: nodes, services, bandwidth through ingress
- Invoice generation: monthly, automated
- Payment webhook → `repository_dispatch` → loop observes revenue
- For customer #1: can start with manual invoicing or sponsorship tier, automate in parallel
- Traction roadmap on chimney.beerpub.dev should update from real revenue data, not projections

### Support function (`fn:support`)

GitHub Issues as support channel — public by default, which means support quality is visible to prospective customers:
- Customer files issue labeled `support`
- Private support (PII, account-specific) via email (Migadu). Email content never committed to repo.
- Loop triages public support issues, creates `fn:dev` bugs if needed
- Direct support from human for customer #1 (personal network)
- Good public support interactions double as trust signals and documentation

### Legal function (`fn:legal`)

Minimum viable compliance:
- Terms of Service + Privacy Policy (LLM-drafted, human-reviewed, `needs-human`)
- GDPR compliance: EU data residency (Hetzner), data processing agreement
- Cookie policy: not needed if using Plausible/Umami (no cookies)
- Business entity: `needs-human` — human must register company

### State tracking (`company/`)

```
company/
├── loop-state.json       # funnel stage, history, last assessment
├── health.json           # latest infrastructure health snapshot
├── costs.json            # infrastructure cost tracking
├── metrics.json          # product metrics (accounts, services, usage)
├── contributors.json     # contribution ledger — entities, types, reciprocation status
└── loop-history/
    └── YYMMDD-assessment.md  # daily loop outputs for audit trail (includes contribution acknowledgments)
```

## Verification

- The loop runs daily without failure for 2 consecutive weeks
- The loop correctly identifies current funnel stage based on real signals
- `fn:dev` issues created by the loop flow through Copilot → Goose → merge without manual intervention
- Service registration + managed ingress work end-to-end (Stage 0 exit)
- Landing page deployed and accessible (Stage 2 exit)
- First payment received (Stage 5 exit)
- All customer data resides in EU infrastructure
- Loop requests human intervention only when genuinely needed (capital, legal entity, judgment calls)

## Friction

- **LLM quality**: The loop's effectiveness depends entirely on the LLM's ability to assess state and prioritise. Bad assessments compound. Mitigation: loop history enables human audit, `needs-human` label as escape valve. Public visibility means bad assessments are also visible — this is pressure toward quality.
- **Signal availability**: Some signals (web analytics, social metrics) won't exist until those systems are set up. The loop must handle missing signals gracefully and prioritise creating them.
- **Billing complexity**: EU payment processing has VAT/tax implications. Stripe handles most of this but the legal entity question is a hard human dependency.
- **GitHub as substrate**: Running a full company through GitHub Issues is unconventional. Issue volume may become noisy. Mitigation: strict labeling, separate project boards per function.
- **Cost control**: Autonomous spending (Hetzner, domains, services) needs guardrails. The loop should track costs and flag when approaching thresholds. Human approves any new recurring spend.
- **Cold start is warmer than it looks**: cloudroof.eu, chimney.beerpub.dev, the agent pipeline, Chimney, Lighthouse — significant infrastructure exists. The loop's first run can already observe real signals. The gap is narrower than a true cold start.
- **Radical transparency risk**: Competitors can see the strategy, the funnel stage, the priorities. This is accepted. The bet is that execution speed (autonomous loop) outweighs strategy visibility. A company that runs itself is harder to copy than a strategy document.
- **Secret leakage**: The loop writes to a public repo. A single mistake — an API key in an assessment, a customer email in an issue — is a public incident. Mitigation: the loop's system prompt explicitly prohibits writing secrets. The state collection step sanitises inputs before passing to the LLM. A pre-commit hook scans for common secret patterns.
- **Public failure**: If the loop makes bad decisions, creates nonsensical issues, or the funnel stalls — everyone sees it. This is a feature, not a bug. Authentic building in public includes failure. But it does mean the loop should fail gracefully and explain its reasoning.

## Interactions

- Depends on existing agent pipeline (Copilot, Goose, auto-merge, board-sync)
- Extends pipeline with new function labels and workflows
- Evolves Lighthouse into the ingress product
- Builds on Chimney deploy patterns for edge infrastructure
- Reads from chimney.beerpub.dev dashboard (capability matrix, traction roadmap, DORA metrics) as input signals
- Writes back to chimney.beerpub.dev by updating the data it serves (funnel stage, real revenue, feature status)
- Evolves cloudroof.eu from CDN-only positioning to include AI gateway persona
- State files in `company/` are the loop's memory across runs

## Mapping

> [[.github/workflows/copilot-triage.yml]]
> [[.github/workflows/goose-build.yml]]
> [[.github/workflows/auto-merge.yml]]
> [[.github/workflows/spec-auto-approve.yml]]
> [[.github/workflows/board-sync.yml]]
> [[.github/workflows/chimney-deploy.yml]]
> [[.github/workflows/agent-metrics-report.yml]]
> [[cmd/chimney/main.go]]
> [[cmd/lighthouse/main.go]]
> [[pkg/lighthouse/api.go]]
> [[deploy/chimney/bluegreen.sh]]

External:
> cloudroof.eu — product site
> chimney.beerpub.dev — pipeline dashboard + traction roadmap

## Boundaries

Explicitly out of scope for first-customer milestone:
- Mobile clients
- Multi-region edge (single EU location)
- Custom domains (only `*.wgmesh.dev`)
- Self-serve public signup (customer #1 is onboarded directly)
- Replacing GitHub/GitHub Actions with EU alternatives (accepted dependency)
- Full accounting/bookkeeping automation

## Future

{[!] Multi-region edge proxies — 2-3 EU locations, then global}
{[!] Self-serve signup and onboarding without human}
{[!] EU LLM migration — evaluate Mistral for the control loop}
{[!] EU billing migration — evaluate Mollie/Paddle replacing Stripe}
{[!] Web dashboard for mesh and service management}
{[!] Automated customer health scoring — loop detects churn risk}
{[?] Full financial automation — bookkeeping, tax filing, expense tracking}
{[?] The loop spawning sub-loops for specific functions (marketing loop, ops loop)}
{[?] Migrate from GitHub to EU-hosted git platform (Codeberg, self-hosted Gitea)}
