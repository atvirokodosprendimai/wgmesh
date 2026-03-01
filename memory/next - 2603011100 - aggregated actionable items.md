# Next — 2603011100

## Actionable Items

### 1 — Active Plan: [[plan - 2602282207 - push subsections for autonomous company loop]]
Phase 5 (Validate) — all code phases complete, needs manual steps:
- 1.1 — Add `ANTHROPIC_API_KEY` to GitHub secrets (`needs-human`)
- 1.2 — Push branch to origin
- 1.3 — Trigger `company-loop.yml` via workflow_dispatch
- 1.4 — Verify: assessment created, loop-state updated, issues created, no secrets leaked
- 1.5 — Fix any issues found

### 2 — Open Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]]
Phase 3 (Observability) actions 9–10, Phase 4 (Deploy Status) actions 11–13, Phase 5 (Cache Control) actions 14–15:
- 2.1 — Promote cache counters; add Dragonfly and rate-limit gauges (action 9)
- 2.2 — Add request metrics; wire panics and deploy-event counters (action 10)
- 2.3 — Add deploy event ring buffer + `POST /api/deploy/events` (action 11)
- 2.4 — Add `GET /api/deploy/status` (action 12)
- 2.5 — Add CI deploy hook to `chimney-deploy.yml` (action 13)
- 2.6 — Implement `POST /api/cache/invalidate` (action 14)
- 2.7 — Register `/api/cache/invalidate` route (action 15)

### 3 — Open Plan: [[plan - 2602221444 - chimney org dashboard and repo split]]
Phase 1 (Backend) actions 1–3, Phase 2 (Frontend) actions 4–8, Phase 3 (Specs) actions 9–10, Phase 4 (Repo Split) actions 11–17:
- 3.1 — Add `GITHUB_ORG` env var; implement `/orgs/{org}/repos` polling (action 1)
- 3.2 — Expose `GET /api/github/org/repos` (action 2)
- 3.3 — Add `GET /api/github/org/activity` (action 3)
- 3.4–3.8 — Frontend redesign, auto-refresh, legibility, smoke test (actions 4–8)
- 3.9–3.10 — Spec updates (actions 9–10)
- 3.11–3.17 — Repo split and migration (actions 11–17)

### 4 — Planned Items (spec - first-customer)
- 4.1 — Evaluate Mollie/Paddle as EU billing alternatives
- 4.2 — Evaluate Mistral as EU-native LLM for the control loop
- 4.3 — Multi-region edge proxies (2–3 EU locations, then global)
- 4.4 — Self-serve signup and onboarding without human
- 4.5 — Web dashboard for mesh and service management
- 4.6 — Automated customer health scoring
