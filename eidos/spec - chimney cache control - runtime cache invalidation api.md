---
tldr: POST /api/cache/invalidate accepts a path pattern and removes matching entries from both Dragonfly and in-memory cache, letting table.beerpub.dev force-refresh stale GitHub data after a deploy without waiting for TTL expiry.
category: feature
---

# Chimney cache control — runtime cache invalidation API

## Target

An endpoint that lets table.beerpub.dev (or a CI step) evict specific entries from chimney's two-layer cache.
Most useful immediately after a deploy — e.g. force-refresh open PRs and recent CI runs without waiting for the 30s TTL.

## Behaviour

### `POST /api/cache/invalidate`

Request body:

```json
{
  "path": "/pulls",
  "reason": "post-deploy refresh"
}
```

Fields:
- `path` — GitHub API path prefix (e.g. `/pulls`, `/issues`, `/actions/runs`). Matches any cache key that starts with the given path prefix (after stripping the leading `/`).
- `reason` — optional string; logged for observability.
- `all` — boolean; if `true`, clears the entire cache regardless of `path`. Requires `INVALIDATE_ALL_ALLOWED=true` env var to be set (opt-in safety guard).

**Authentication**: bearer token from `INVALIDATE_TOKEN` env var (same pattern as `DEPLOY_TOKEN`).
Returns 401 on missing/incorrect token.

**Effect:**
1. Scan in-memory `memCache` for keys whose path component starts with the given prefix; delete matching entries.
2. If Dragonfly is connected: SCAN `chimney:*` keys, delete those matching the prefix.
3. Return a count of evicted entries.

Response:

```json
{
  "evicted": 7,
  "path": "/pulls",
  "reason": "post-deploy refresh"
}
```

### Matching semantics

Cache keys are stored as `ghPath + "?" + rawQuery` (e.g. `/pulls?state=open&per_page=100`).
A prefix match on `path` matches all cache entries for that GitHub API path regardless of query string.

Examples:
- `"path": "/pulls"` evicts `/pulls?state=open`, `/pulls?state=closed&per_page=10`, etc.
- `"path": "/actions/runs"` evicts all workflow run cache entries.
- `"path": "/"` evicts all GitHub proxy cache entries (but not `__pipeline_summary__`).

The `__pipeline_summary__` internal key is treated separately:
it is only evicted if `path` is explicitly `/__pipeline_summary__` or `all` is `true`.

### `POST /api/cache/warm` (optional companion)

After invalidation, immediately pre-fetch the most-used paths to avoid a cold-cache spike:
- `/pulls?state=open&per_page=100`
- `/issues?state=open&per_page=100`
- `/actions/runs` (last 10)

This is a fire-and-forget background goroutine; the response returns immediately.
The warm endpoint shares the same `INVALIDATE_TOKEN` auth.

## Design

- **Prefix match over exact key**: a post-deploy invalidation wants to clear all PR-related entries, not enumerate each exact URL. Prefix matching on the GitHub path handles query-string variants automatically.
- **SCAN not KEYS**: Dragonfly's `SCAN` iterates incrementally with a cursor; `KEYS *` is O(N) and blocks. For ≤500 cache entries this is fine either way, but SCAN is the correct pattern.
- **`all` requires opt-in env var**: clearing the entire cache is a high-impact operation. Requiring `INVALIDATE_ALL_ALLOWED=true` prevents accidental nukes from a misconfigured table.beerpub.dev request.
- **`INVALIDATE_TOKEN` separate from `DEPLOY_TOKEN`**: different callers (CI pipeline vs table.beerpub.dev frontend) should use separate tokens. This lets them be rotated independently and audited separately.
- **`/api/cache/warm` as optional companion**: a cold cache after invalidation causes a burst of GitHub API requests. Pre-warming the hot paths reduces that spike. The warm step is best-effort — if it fails, the cache naturally repopulates on the next request.
- **Response includes count**: `"evicted": 7` gives immediate feedback that the right entries were matched. Zero evictions may indicate a path typo.

## Interactions

- `cmd/chimney/main.go` — registers `/api/cache/invalidate` and (optionally) `/api/cache/warm`.
- `INVALIDATE_TOKEN` env var — shared secret for authorization.
- `INVALIDATE_ALL_ALLOWED` env var — opt-in guard for full cache clear.
- Dragonfly — SCAN + DEL for matching keys.
- In-memory `memCache` — direct map key deletion.
- table.beerpub.dev — `POST /api/cache/invalidate` after user clicks "Refresh" or after a deploy event is observed.
- `chimney-deploy.yml` — optionally calls invalidate after deploy notification.
- `/api/metrics` — `chimney_cache_invalidations_total` counter (extend the metrics spec).

## Mapping

> [[cmd/chimney/main.go]]
