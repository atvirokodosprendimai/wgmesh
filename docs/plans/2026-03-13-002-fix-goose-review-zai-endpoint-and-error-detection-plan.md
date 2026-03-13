---
title: "fix: Goose Review Z.ai GLM endpoint path and error detection"
type: fix
status: active
date: 2026-03-13
---

# fix: Goose Review Z.ai GLM endpoint path and error detection

## Problem

The Goose Review workflow fails to use Z.ai GLM-5 due to an incorrect API endpoint path, then hangs for 45 minutes before timing out.

**Error from run 23044653223:**
```
Request failed: Resource not found (404): {"path":"/v4/v1/chat/completions"}
```

**Root cause:** The workflow sets `OPENAI_HOST="https://api.z.ai/api/coding/paas/v4"`. Goose (using the OpenAI provider) appends its default `/v1/chat/completions` path, resulting in the double-versioned path `/v4/v1/chat/completions` which 404s. Z.ai uses `/v4/` (not `/v1/`) as its API version prefix.

**Secondary issues:**
1. The error detection grep doesn't catch generic "404" or "Not Found" responses — only `model.*not.*found`. So the cascade fallback doesn't trigger cleanly.
2. Even after the 404, Goose hangs internally (doesn't exit) for 45 minutes until the `timeout 2700` kills it. The cascade can't fall through to the next provider.

## Proposed Solution

### Fix 1: Correct Z.ai endpoint with `OPENAI_BASE_PATH`

Goose supports `OPENAI_BASE_PATH` to override the default `/v1` path prefix. Set it so the final URL becomes `https://api.z.ai/api/coding/paas/v4/chat/completions`:

```bash
# goose-review.yml — Z.ai GLM-5 cascade step
export OPENAI_API_KEY="$ZAI_API_KEY"
export OPENAI_HOST="https://api.z.ai"
export OPENAI_BASE_PATH="/api/coding/paas/v4"
run_goose "openai" "glm-5" "Z.ai GLM-5" && goose_ok=true
unset OPENAI_API_KEY OPENAI_HOST OPENAI_BASE_PATH
```

### Fix 2: Broaden error detection grep

Add `404`, `Not Found`, and generic HTTP errors to the grep pattern so the cascade triggers on Z.ai endpoint failures:

```bash
# Before:
if grep -qiE '(add more credits|insufficient.*(funds|credits|balance)|rate.limit|unauthorized|api.key.not.found|model.*not.*found|not.*available)' /tmp/goose-review-output.log; then

# After:
if grep -qiE '(add more credits|insufficient.*(funds|credits|balance)|rate.limit|unauthorized|api.key.not.found|model.*not.*found|not.*available|404.*not.found|resource.not.found|connection.refused|503|502|500)' /tmp/goose-review-output.log; then
```

### Fix 3: Add per-provider timeout (defense in depth)

The main `timeout 2700` (45min) is too generous for a single provider attempt when the cascade has 4 providers. Add a shorter per-provider timeout so a hanging Goose doesn't block the entire cascade:

```bash
# In run_goose():
timeout 600 goose run \    # 10 minutes per provider (was 2700 for entire cascade)
```

Keep the outer `timeout 2700` as a safety net but don't rely on it for individual provider failures.

## Acceptance Criteria

- [ ] Z.ai GLM-5 endpoint resolves correctly (no more `/v4/v1/` double-versioning)
- [ ] Error detection catches 404, 500, 502, 503, and "Not Found" responses
- [ ] Per-provider timeout prevents a single hanging provider from consuming the full 45-minute budget
- [ ] `unset OPENAI_BASE_PATH` after Z.ai attempt to avoid leaking into OpenRouter attempts
- [ ] Cascade still falls through correctly when Z.ai fails (to GPT-5.4 → Mistral → Venice)

## Files to Modify

1. `.github/workflows/goose-review.yml` — Z.ai env vars (line 231-235), error grep (line 201), timeout (line 196)

## Sources

- Failed run logs: run 23044653223 — `404: /v4/v1/chat/completions`
- Current in-progress run: 23047538114
- Z.ai docs: base URL is `https://api.z.ai/api/paas/v4/` ([docs](https://docs.z.ai/guides/overview/quick-start))
- Goose `OPENAI_BASE_PATH`: [Goose provider config](https://block.github.io/goose/docs/getting-started/providers/)
- Related plan: `docs/plans/2026-03-12-002-fix-goose-review-dispatch-race-and-workflow-gaps-plan.md`
