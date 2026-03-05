---
type: next
created: 2603040854
---

# Next — Aggregated Actionable Items

## 1 — Active Plans

### 1.1 — [[plan - 2602282207 - push subsections for autonomous company loop]] (Phase 7)
- Push branch to origin
- Trigger `company-loop.yml` via workflow_dispatch
- Verify: assessment created, loop-state updated, issues created, no secrets leaked
- Fix any issues found

### 1.2 — [[plan - 2603011216 - fix company loop and restructure goose build]] (Phase 4)
- Verify `goose-review.yml` works with a PR that has review comments

### 1.3 — [[plan - 2602211419 - chimney integration observability deploy status and cache control]] (Phase 3)
- Promote cache counters; add Dragonfly and rate-limit gauges
- Add request metrics; wire panics and deploy-event counters
- Then Phases 4–5 (deploy status endpoint, cache invalidation API)

### 1.4 — [[plan - 2602221444 - chimney org dashboard and repo split]] (Phase 1)
- Org repo discovery, TV dashboard, spec update, repo split

## 2 — Planned Items ({[!]})

### 2.1 — Service CLI
- `wgmesh service update <name>` — change port or protocol without remove+add
- `--account` flag on `wgmesh join` to store API key during mesh join (current branch `task/join-account-flag`)

### 2.2 — First Customer Roadmap
- Evaluate Mollie (NL) or Paddle (UK) as Stripe alternatives for EU billing
- Evaluate Mistral as EU-native LLM for the control loop
- Multi-region edge proxies — 2-3 EU locations, then global
- Self-serve signup and onboarding without human
- EU LLM migration — evaluate Mistral for the control loop
- EU billing migration — evaluate Mollie/Paddle replacing Stripe
- Web dashboard for mesh and service management
- Automated customer health scoring — loop detects churn risk
