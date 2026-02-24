---
tldr: Should chimney-deploy.yml POST to /api/cache/invalidate after a successful deploy to force-refresh stale GitHub proxy entries?
---

# Question: should CI call cache invalidate after deploy?

## Context

`POST /api/cache/invalidate` (Phase 5) can flush Dragonfly + in-memory cache
entries by prefix. The chimney-deploy.yml workflow already POSTs to each origin
after deploy (the deploy event hook added in Phase 4).

## The question

Should chimney-deploy.yml also call `POST /api/cache/invalidate` on each origin
after a successful deploy — e.g., invalidating `/pulls` or all entries — so the
dashboard immediately reflects the new state?

## Arguments for

- Eliminates a class of stale-data bugs: post-deploy the dashboard may serve
  cached PR/issue data from before the deploy for up to `maxAge` (currently 5min)
- Convex: one CI step addition, permanent elimination of stale-cache confusion
- The infrastructure is already in place (INVALIDATE_TOKEN, the endpoint, the
  origin IP loop in chimney-deploy.yml)

## Arguments against

- Cache serves a real purpose (rate limit protection); flushing on every deploy
  may cause a brief spike of GitHub API calls
- Could be scoped: only invalidate specific prefixes (e.g., `/pulls`, `/releases`)
  rather than `all: true`

## Decision criteria

- If deploys are infrequent (< daily): flush all, the rate limit spike is trivial
- If deploys are frequent: flush targeted prefixes only
