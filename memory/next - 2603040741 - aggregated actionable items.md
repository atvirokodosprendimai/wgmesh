# Next — aggregated actionable items

Generated: 2026-03-04

## Actionable Items

1 - Active Plan: [[plan - 2602282207 - push subsections for autonomous company loop]] (Phase 7 — needs human)
  - 1.1 - Add `ANTHROPIC_API_KEY` to GitHub secrets (`needs-human`)
  - 1.2 - Push branch to origin
  - 1.3 - Trigger `company-loop.yml` via workflow_dispatch
  - 1.4 - Verify: assessment created, loop-state updated, issues created, no secrets leaked

2 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]] (Phase 3 — open)
  - 2.1 - Promote cache counters; add Dragonfly and rate-limit gauges
  - 2.2 - Add request metrics; wire panics and deploy-event counters

3 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]] (Phase 4 — open)
  - 3.1 - Add deploy event ring buffer + `POST /api/deploy/events`
  - 3.2 - Add `GET /api/deploy/status`
  - 3.3 - Add CI deploy hook to `chimney-deploy.yml`

4 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]] (Phase 5 — open)
  - 4.1 - Implement `POST /api/cache/invalidate`
  - 4.2 - Register `/api/cache/invalidate` route

5 - Active Plan: [[plan - 2602221444 - chimney org dashboard and repo split]] (all phases open)
  - 5.1 - Org-level GitHub polling and API endpoints (actions 1–3)
  - 5.2 - Dashboard redesign for TV display (actions 4–8)
  - 5.3 - Spec updates (actions 9–10)
  - 5.4 - Repo split and migration (actions 11–15)

6 - Current Branch: `task/join-account-flag` (completed, not merged)
  - 6.1 - Merge or create PR for `--account` flag on join/install-service

7 - Planned Items (specs)
  - 7.1 - {[!]} `wgmesh service update <name>` command — spec - service cli
  - 7.2 - {[!]} Evaluate Mollie/Paddle as Stripe alternatives — spec - first-customer
  - 7.3 - {[!]} Evaluate Mistral as EU-native LLM — spec - first-customer
  - 7.4 - {[!]} Multi-region edge proxies — spec - first-customer
  - 7.5 - {[!]} Self-serve signup and onboarding — spec - first-customer
  - 7.6 - {[!]} Web dashboard for mesh/service management — spec - first-customer
  - 7.7 - {[!]} Automated customer health scoring — spec - first-customer

8 - Postponed
  - 8.1 - [p] Verify `dpkg -i wgmesh_*.deb` installs binary + systemd unit — plan - distributable packages

Which items to work on?
