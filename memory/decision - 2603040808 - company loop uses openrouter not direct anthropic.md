# Decision — company loop uses OpenRouter, not direct Anthropic

Date: 2026-03-04

## Context

The plan ([[plan - 2602282207 - push subsections for autonomous company loop]], Phase 7) specified adding `ANTHROPIC_API_KEY` to GitHub secrets as a blocker.
The actual implementation in `company-loop.yml` uses `OPENROUTER_API_KEY` to call `openrouter.ai/api/v1/chat/completions`.
`goose-review.yml` supports both keys with OpenRouter as primary fallback.

## Options

1. **Use OpenRouter (as implemented)** — already wired, already in secrets, routes to Anthropic models via OpenRouter
2. **Switch to direct Anthropic** — would require rewriting the curl call in company-loop.yml

## Decision

Option 1 — keep OpenRouter.
Both `OPENROUTER_API_KEY` and `ANTHROPIC_API_KEY` are already configured in repo secrets.
The implementation matches Option 1 and the secret exists. No change needed.

## Consequence

- Plan Phase 7 action 1 (`Add ANTHROPIC_API_KEY`) was already satisfied — both keys present
- Plan text was inaccurate (referenced Anthropic, implementation uses OpenRouter)
- No code changes required
