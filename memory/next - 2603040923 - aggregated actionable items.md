tldr: Aggregated actionable items across plans, specs, and todos

## Actionable Items

### 1 - Active Plan: [[plan - 2602282207 - push subsections for autonomous company loop]]
Phase 7 — First run + verify (4 remaining)
- 1.1 - Push branch to origin
- 1.2 - Trigger `company-loop.yml` via workflow_dispatch
- 1.3 - Verify: assessment created, loop-state updated, issues created, no secrets leaked
- 1.4 - Fix any issues found

### 2 - Active Plan: [[plan - 2603011216 - fix company loop and restructure goose build]]
Phase unknown (1 remaining)
- 2.1 - Verify `goose-review.yml` works with a PR that has review comments

### 3 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]]
Phase 2+ (7 remaining, starting at action 9)
- 3.1 - Promote cache counters; add Dragonfly and rate-limit gauges
- 3.2 - Add request metrics; wire panics and deploy-event counters
- 3.3 - Add deploy event ring buffer + `POST /api/deploy/events`
- 3.4 - Add `GET /api/deploy/status`
- 3.5 - Add CI deploy hook to `chimney-deploy.yml`
- 3.6 - Implement `POST /api/cache/invalidate`
- 3.7 - Register `/api/cache/invalidate` route

### 4 - Active Plan: [[plan - 2602221444 - chimney org dashboard and repo split]]
All 17 actions open — not yet started
- 4.1 - Org-level repo polling (actions 1–3)
- 4.2 - Dashboard redesign (actions 4–8)
- 4.3 - Spec updates (actions 9–10)
- 4.4 - Repo split and migration (actions 11–17)

### 5 - Planned Items (from specs)
Service CLI:
- 5.1 - {[!] `wgmesh service update <name>` — change port or protocol without remove+add}
- 5.2 - {[!] `--account` flag on `wgmesh join` to store API key during mesh join}

First-customer roadmap:
- 5.3 - {[!] Evaluate Mollie (NL) or Paddle (UK) as Stripe alternatives}
- 5.4 - {[!] Evaluate Mistral as EU-native LLM for the control loop}
- 5.5 - {[!] Multi-region edge proxies — 2-3 EU locations, then global}
- 5.6 - {[!] Self-serve signup and onboarding without human}
- 5.7 - {[!] EU LLM migration — evaluate Mistral for the control loop}
- 5.8 - {[!] EU billing migration — evaluate Mollie/Paddle replacing Stripe}
- 5.9 - {[!] Web dashboard for mesh and service management}
- 5.10 - {[!] Automated customer health scoring — loop detects churn risk}

### 6 - Postponed
- 6.1 - [p] Verify: `dpkg -i wgmesh_*.deb` installs binary + systemd unit (plan - 2603012134)

Which items to work on?
