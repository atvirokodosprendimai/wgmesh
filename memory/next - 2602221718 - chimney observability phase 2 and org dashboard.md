---
created: 2602221718
---

# Next — chimney observability phase 2 and org dashboard

1 - Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]]
  - 1.1 - Phase 2 / action 6: Replace log.Print* with slog + OTEL log bridge
  - 1.2 - Phase 2 / action 7: Add panic recovery middleware
  - 1.3 - Phase 2 / action 8: Add request log middleware
  - 1.4 - Phase 3 / action 9: Promote cache counters to OTEL instruments
  - 1.5 - Phase 3 / action 10: Add request metrics + panics/deploy counters
  - 1.6 - Phase 4 / action 11–13: Deploy status endpoint + ring buffer + CI hook
  - 1.7 - Phase 5 / action 14–15: Cache invalidation API

2 - Active Plan: [[plan - 2602221444 - chimney org dashboard and repo split]]
  - 2.1 - Phase 1 / action 1: Add GITHUB_ORG env var + /orgs/{org}/repos polling
  - 2.2 - Phase 1 / action 2: Expose GET /api/github/org/repos
  - 2.3 - Phase 1 / action 3: Add GET /api/github/org/activity
  - 2.4 - Phase 2 / action 4–8: TV screen dashboard redesign
  - 2.5 - Phase 3 / action 9–10: Eidos spec update
  - 2.6 - Phase 4 / action 11–17: Repo split to atvirokodosprendimai/chimney
