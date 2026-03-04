---
tldr: Phased plan to implement the autonomous company control loop per the first-customer spec
status: completed-migrated
---

> **Migrated:** Company loop implementation moved to [ai-pipeline-template](https://github.com/atvirokodosprendimai/ai-pipeline-template).
> See [[plan - 2603040954 - migrate observation loop to ai-pipeline-template]].

# Plan: Push Autonomous Company Loop

Source: [[spec - first-customer - roadmap to first paying customer]]
Push doc: [[push - 2602282207 - implement autonomous company loop]]

## Phase 1: Company state foundation

Bootstrap the `company/` directory with seed state so the loop has something to read on first run.

- [x] Create `company/` directory structure
- [x] Create `company/loop-state.json` — initial state: stage 0 Foundation
- [x] Create `company/health.json` — chimney + cloudroof endpoints
- [x] Create `company/costs.json` — with runway tracking, frugality principle
- [x] Create `company/metrics.json` — product + community + revenue sections
- [x] Create `company/contributors.json` — seeded: founder, Copilot, Goose, GitHub Actions, Hetzner, anacrolix/dht, go-redis, x/crypto, Caddy, Anthropic
- [x] Create `company/loop-history/` directory with `.gitkeep`
- [x] Fix `.gitignore` — added `!company/**/*.json` exception
- [x] Commit: `84d0413`

## Phase 2: LLM system prompt

Create the operational prompt the loop feeds to the LLM — distilled from the spec.

- [x] Create `company/system-prompt.md` — funnel stages, transition criteria, output JSON schema, public/private rules, reciprocity principle, assessment format, frugality constraints, writing style guide
- [x] Commit: `d7e1a47`

## Phase 3: State collection

Scripts that gather signals for the LLM.

- [x] Create `company/scripts/collect-github.sh` — issues by fn label, PRs, merge rate, releases, CI, stars/forks, contributors
- [x] Create `company/scripts/collect-contributions.sh` — git authors, bot activity, dependency count, unreciprocated count
- [x] Create `company/scripts/collect-infra.sh` — health checks for chimney + cloudroof
- [x] Create `company/scripts/sanitise.sh` — blocks API keys, tokens, private keys, PII emails
- [x] Commit: `e25b2af`

## Phase 4: Control loop workflow

The core `company-loop.yml` — ties everything together.

- [x] Create `.github/workflows/company-loop.yml`:
  - 3 jobs: collect-state → assess (LLM call) → act (commit + issues)
  - Daily 08:00 UTC + manual + webhook triggers
  - Graceful stub when ANTHROPIC_API_KEY missing
  - Sanitise step before commit
  - Issue creation with dedup check
  - needs-human issue creation
- [x] Commit: `f4af103`

## Phase 5: Board + pipeline integration

- [x] Add function labels to `.github/labels.yml`: fn:dev, fn:ops, fn:gtm, fn:billing, fn:support, fn:legal, needs-human
- [x] Update `.github/workflows/board-sync.yml` — fn:* labels route to Triage
- [x] fn:dev + needs-triage flows into existing Copilot → Goose pipeline (unchanged)
- [x] Commit: `8b9b3ef`

## Phase 6: Safety + secret scanning

- [x] Sanitise step in company-loop.yml (Phase 4)
- [x] Create `.github/hooks/pre-commit-secret-scan` — optional local hook
- [x] Commit: `c975ceb`

## Phase 7: First run + verify

Manual trigger, observe, fix.

- [x] Add `ANTHROPIC_API_KEY` to GitHub secrets (`needs-human`)
  => Both `OPENROUTER_API_KEY` and `ANTHROPIC_API_KEY` already present. Implementation uses OpenRouter. See [[decision - 2603040808 - company loop uses openrouter not direct anthropic]].
- [ ] Push branch to origin
- [ ] Trigger `company-loop.yml` via workflow_dispatch
- [ ] Verify: assessment created, loop-state updated, issues created, no secrets leaked
- [ ] Fix any issues found
