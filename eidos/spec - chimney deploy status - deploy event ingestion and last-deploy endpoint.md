---
tldr: chimney exposes a POST /api/deploy/events endpoint that receives deploy notifications from the CI pipeline (SHA, PR, slot, outcome); GET /api/deploy/status returns the latest state so table.beerpub.dev can show "last deployed X ago" without GitHub API calls.
category: feature
---

# Chimney deploy status — deploy event ingestion and last-deploy endpoint

## Target

Closes the loop between the CI deploy pipeline and the running chimney process.
`chimney-deploy.yml` POSTs a deployment event to chimney on completion;
chimney retains the last N events in memory and exposes them via a status endpoint.
table.beerpub.dev reads the status without touching the GitHub API.

## Behaviour

### Deploy event ingestion (`POST /api/deploy/events`)

The CI pipeline POSTs a JSON body on every deploy completion:

```json
{
  "sha":        "3051037",
  "pr_number":  321,
  "pr_title":   "blue/green deploy rewrite",
  "slot":       "blue",
  "outcome":    "success",
  "duration_s": 47,
  "deployed_at": "2026-02-21T10:13:00Z",
  "origin_ips": ["1.2.3.4", "5.6.7.8"]
}
```

Fields:
- `sha` — commit SHA (short or full).
- `pr_number`, `pr_title` — optional; omitted for direct-to-main deploys.
- `slot` — active blue/green slot after deploy (`blue` or `green`).
- `outcome` — `success` or `failure`.
- `duration_s` — deploy duration in seconds.
- `deployed_at` — RFC3339 UTC timestamp.
- `origin_ips` — IPs of the origins that were provisioned/updated.

**Authentication**: bearer token checked against a `DEPLOY_TOKEN` env var.
Token is a shared secret set in GitHub Actions secrets and in chimney's environment.
Requests with missing or incorrect token receive 401.

The event is prepended to an in-memory ring (max **50 events**); oldest is dropped when full.
The ring is goroutine-safe.

### Status endpoint (`GET /api/deploy/status`)

Returns the current deployment picture:

```json
{
  "last_deploy": {
    "sha": "3051037",
    "pr_number": 321,
    "pr_title": "blue/green deploy rewrite",
    "slot": "blue",
    "outcome": "success",
    "duration_s": 47,
    "deployed_at": "2026-02-21T10:13:00Z",
    "age_s": 3720,
    "origin_ips": ["1.2.3.4", "5.6.7.8"]
  },
  "recent_deploys": [...],
  "total_deploys": 12,
  "success_rate_pct": 91.7
}
```

`age_s` is computed at request time (`now - deployed_at`).
`recent_deploys` is the last 10 events from the ring.
`success_rate_pct` is computed over all retained events.

No authentication required — the status is public information (no secrets exposed).

### CI integration (`chimney-deploy.yml`)

After the bootstrap step completes (healthz returns 200), add a `notify-chimney` step:

```yaml
- name: Notify chimney of deploy
  run: |
    curl -sf -X POST https://chimney.beerpub.dev/api/deploy/events \
      -H "Authorization: Bearer ${{ secrets.CHIMNEY_DEPLOY_TOKEN }}" \
      -H "Content-Type: application/json" \
      -d '{
        "sha": "${{ github.sha }}",
        "pr_number": ${{ github.event.pull_request.number || 0 }},
        "slot": "${{ steps.deploy.outputs.active_slot }}",
        "outcome": "success",
        "duration_s": ${{ steps.deploy.outputs.duration_s }},
        "deployed_at": "${{ steps.deploy.outputs.deployed_at }}",
        "origin_ips": ${{ steps.deploy.outputs.origin_ips_json }}
      }'
```

On deploy failure, a failure event should be posted from the `if: failure()` step instead.

## Design

- **In-memory ring, no persistence**: deploy events don't need to survive chimney restarts — the CI pipeline is the source of truth. If chimney restarts, the next deploy repopulates the ring. The GitHub Actions run history is always available as a fallback.
- **Shared secret over JWT**: DEPLOY_TOKEN is a simple shared secret (lower operational complexity). Since this endpoint is POST-only and the payload is not sensitive (just metadata), the threat model is replay prevention, not full auth. A short-lived JWT would be overkill.
- **Separate from GitHub API cache**: the deploy status doesn't go through the cache layer — it's push-based and always fresh. No TTL logic needed.
- **`age_s` computed at response time**: avoids storing a mutable field in the ring; derived on read.
- **`/api/deploy/status` is unauthenticated**: the information (SHA, slot, outcome) is already public via GitHub Actions. No secrets are exposed. Simplifies table.beerpub.dev's integration (no token management in the frontend).

## Interactions

- `chimney-deploy.yml` — adds `notify-chimney` step; reads `CHIMNEY_DEPLOY_TOKEN` from GitHub Actions secrets.
- `DEPLOY_TOKEN` env var — set in chimney's deployment environment.
- `cmd/chimney/main.go` — registers `/api/deploy/events` and `/api/deploy/status` routes.
- table.beerpub.dev — polls `/api/deploy/status` (e.g. every 60s) to show last-deploy card.
- `/api/logs/stream` — deploy events optionally emit a structured log line to the ring buffer.

## Mapping

> [[cmd/chimney/main.go]]
> [[.github/workflows/chimney-deploy.yml]]
