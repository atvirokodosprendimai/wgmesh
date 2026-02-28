---
tldr: Autonomous company-in-a-repo that builds, markets, sells, and operates wgmesh as an AI service gateway — driven by an LLM control loop in GitHub Actions
---

# First Customer

An autonomous company that runs itself through GitHub. An LLM agent executes a recurring control loop — observing the real state of the product, infrastructure, market presence, revenue, and support — then creates and prioritises work across all business functions. The existing agent pipeline (Copilot specs → Goose implementation → auto-merge) handles development. New loops handle operations, go-to-market, billing, and support. Humans provide capital and intervention when the loop requests it.

The goal: first paying customer within 3 months. A homelab LLM operator paying for managed mesh + ingress to expose AI services over HTTPS.

## Target

wgmesh is technically mature but has no product layer, no company around it, and no revenue. The gap isn't code — it's everything else a business needs. This spec defines a system where that gap closes autonomously, with the LLM control loop as the operating brain and GitHub as the substrate.

## Behaviour

### The control loop

A scheduled GitHub Actions workflow (`company-loop.yml`) runs daily. It:

1. **Observes** — gathers signals from every business function
2. **Assesses** — an LLM evaluates state against the funnel and identifies the highest-leverage next actions
3. **Acts** — creates GitHub issues with function labels (`fn:dev`, `fn:ops`, `fn:gtm`, `fn:billing`, `fn:support`, `fn:legal`), which the appropriate pipeline picks up
4. **Reflects** — on next run, evaluates what happened since last loop, adjusts course

The loop doesn't follow a fixed plan. It evolves a funnel — from "no product" through "product exists" through "someone can buy it" through "someone did buy it" — adapting to whatever the real situation is on each run.

### The funnel

Stages the loop drives through, with transition criteria:

- **Stage 0: Foundation** — product doesn't exist yet
  - Exit when: service registration + managed ingress work end-to-end in a test environment
- **Stage 1: Dogfood** — product works but only internally
  - Exit when: wgmesh team uses it daily for own AI services, no critical bugs for 1 week
- **Stage 2: Presence** — product works but nobody knows about it
  - Exit when: landing page live, install one-liner works, quickstart guide published
- **Stage 3: Reachable** — people can find it but can't pay
  - Exit when: billing integration live, pricing page exists, signup flow works
- **Stage 4: Pipeline** — people can pay but nobody has
  - Exit when: first customer onboarded from personal network
- **Stage 5: Revenue** — first invoice paid
  - Exit when: payment received, retention signal (customer still active after 30 days)

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

**Go-to-market signals** (from web + social)
- Landing page: exists? deployed? analytics if available
- GitHub stars, forks, traffic (API: `repos/{owner}/{repo}/traffic`)
- Content published: blog posts, guides, comparison pages
- Social presence: accounts created, posts made

**Revenue signals** (from billing)
- Accounts created, active meshes, services registered
- Invoices sent, payments received, MRR
- Customer support tickets open/resolved

**Infrastructure cost signals**
- Hetzner spend (EU compute)
- Domain/DNS costs
- Any third-party service costs

### How the loop acts

The LLM outputs a structured assessment:
- Current funnel stage
- What's blocking advancement to next stage
- Top 3 actions ranked by leverage
- Issues to create (with function label, priority, acceptance criteria)
- Issues to close or deprioritise (situation changed)
- Requests for human intervention (if capital or decisions needed)

Issues flow into the existing pipeline:
- `fn:dev` → Copilot specs → Goose implements → auto-merge
- `fn:ops` → operations playbooks / deploy workflows
- `fn:gtm` → content generation, page deployment
- `fn:billing` → billing integration tasks
- `fn:support` → customer communication tasks
- `fn:legal` → compliance, terms of service, GDPR

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
| Domain | registrar TBD | Yes | `.dev` domain via EU registrar |
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

LLM-driven content and presence:
- Landing page: static site in `site/` directory, deployed to Hetzner via GitHub Actions
- Content generation: LLM writes blog posts, guides, comparison pages as PRs
- Install script: `curl -fsSL https://wgmesh.dev/install | sh` hosted on landing page
- Quickstart: "Expose Ollama in 5 minutes" guide
- Social: the loop can draft posts, but posting requires human approval (`needs-human`)

### Billing function (`fn:billing`)

Minimal viable billing:
- Stripe (or EU alternative) integration via API
- Account creation tied to mesh secret ownership
- Usage metering: nodes, services, bandwidth through ingress
- Invoice generation: monthly, automated
- Payment webhook → `repository_dispatch` → loop observes revenue
- For customer #1: can start with manual invoicing, automate in parallel

### Support function (`fn:support`)

GitHub Issues as support channel for first customer:
- Customer files issue or emails
- Email → GitHub Issue via webhook (Migadu → GitHub)
- Loop triages support issues, creates `fn:dev` bugs if needed
- Direct support from human for customer #1 (personal network)

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
└── loop-history/
    └── YYMMDD-assessment.json  # daily loop outputs for audit trail
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

- **LLM quality**: The loop's effectiveness depends entirely on the LLM's ability to assess state and prioritise. Bad assessments compound. Mitigation: loop history enables human audit, `needs-human` label as escape valve.
- **Signal availability**: Some signals (web analytics, social metrics) won't exist until those systems are set up. The loop must handle missing signals gracefully and prioritise creating them.
- **Billing complexity**: EU payment processing has VAT/tax implications. Stripe handles most of this but the legal entity question is a hard human dependency.
- **GitHub as substrate**: Running a full company through GitHub Issues is unconventional. Issue volume may become noisy. Mitigation: strict labeling, separate project boards per function.
- **Cost control**: Autonomous spending (Hetzner, domains, services) needs guardrails. The loop should track costs and flag when approaching thresholds. Human approves any new recurring spend.
- **Cold start**: The loop can't observe what doesn't exist yet. Early runs will mostly create foundational issues. This is expected — the funnel model handles it.

## Interactions

- Depends on existing agent pipeline (Copilot, Goose, auto-merge, board-sync)
- Extends pipeline with new function labels and workflows
- Evolves Lighthouse into the ingress product
- Builds on Chimney deploy patterns for edge infrastructure
- State files in `company/` are the loop's memory across runs

## Mapping

> [[.github/workflows/copilot-triage.yml]]
> [[.github/workflows/goose-build.yml]]
> [[.github/workflows/auto-merge.yml]]
> [[.github/workflows/spec-auto-approve.yml]]
> [[.github/workflows/board-sync.yml]]
> [[.github/workflows/chimney-deploy.yml]]
> [[cmd/lighthouse/main.go]]
> [[pkg/lighthouse/api.go]]
> [[deploy/chimney/bluegreen.sh]]

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
