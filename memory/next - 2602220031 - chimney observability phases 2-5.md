# Next - chimney observability phases 2-5

Generated: 2602220031

## Active Plan

[[plan - 2602211419 - chimney integration observability deploy status and cache control]]

Phase 1 (OTEL traces) is complete.
Phases 2–5 remain open.

---

## Actionable Items

1 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]]

  **Phase 2 - Structured logging and panic recovery**
  - 1.1 - Replace `log.Print*` with slog + OTEL log bridge (action 6)
  - 1.2 - Add panic recovery middleware (action 7)
  - 1.3 - Add request log middleware (action 8)

  **Phase 3 - OTEL metrics**
  - 1.4 - Promote cache counters; add Dragonfly and rate-limit gauges (action 9)
  - 1.5 - Add request metrics; wire panics and deploy-event counters (action 10)

  **Phase 4 - Deploy status endpoint and CI hook**
  - 1.6 - Add deploy event ring buffer + `POST /api/deploy/events` (action 11)
  - 1.7 - Add `GET /api/deploy/status` (action 12)
  - 1.8 - Add CI deploy hook to `chimney-deploy.yml` (action 13)

  **Phase 5 - Cache invalidation API**
  - 1.9 - Implement `POST /api/cache/invalidate` (action 14)
  - 1.10 - Register `/api/cache/invalidate` route (action 15)
