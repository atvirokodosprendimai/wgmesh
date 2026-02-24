---
tldr: Add INVALIDATE_TOKEN to GitHub repo secrets so POST /api/cache/invalidate is usable
---

# Todo: add INVALIDATE_TOKEN GitHub secret

## What

Add `INVALIDATE_TOKEN` secret to the `atvirokodosprendimai/wgmesh` GitHub repo
(and/or directly to the Hetzner origin server `.env` file).

## Why

`POST /api/cache/invalidate` (Phase 5) returns 503 if `INVALIDATE_TOKEN` env var
is empty. Without the secret, the entire cache invalidation API is dead code.

## How

1. Generate a random token: `openssl rand -hex 32`
2. Add as `INVALIDATE_TOKEN` secret in GitHub repo settings
3. Add to origin server `.env` at `/opt/chimney/.env` on nbg1/fsn1:
   `INVALIDATE_TOKEN=<value>`
4. Optionally set `INVALIDATE_ALL_ALLOWED=true` if full-cache flush is needed

## Optional follow-on

Wire `table.beerpub.dev` to call `POST /api/cache/invalidate` with a targeted
prefix after detecting a new deploy (see question: should CI call cache invalidate
after deploy).
