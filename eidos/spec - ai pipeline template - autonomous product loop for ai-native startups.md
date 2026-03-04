---
tldr: Full autonomous product pipeline — observe state, assess with LLM, create issues, implement with agents, verify — as a GitHub template repo for AI-native startups
---

# AI Pipeline Template

The product is the loop itself: an autonomous pipeline that observes a company's real state, decides what to do, and acts through AI agents — all running on GitHub Actions in a public (or private) repo.

Currently split across two locations:
- `atvirokodosprendimai/ai-pipeline-template` — the action half (issue → Copilot spec → human approve → Goose implement → PR)
- `atvirokodosprendimai/wgmesh/company/` — the observation half (collect state → LLM assess → create issues → commit assessment)

This spec defines the unified product after merging both halves into `ai-pipeline-template`.

## Target

AI-native startups (1–5 people) want their company to run autonomously between human decisions.
They want to drop a template into a repo and get:
- Daily state observation and LLM assessment
- Automatic issue creation from assessment
- Automatic spec writing from issues
- Automatic implementation from approved specs
- Human approval at two gates: spec review and PR merge

No custom infrastructure.
No SaaS dependency beyond GitHub and an LLM API key.
Everything runs in GitHub Actions, everything is committed to the repo.

## Behaviour

### The Loop

The pipeline forms a closed loop with two human checkpoints:

```
observe → assess → create issues → [spec agent] → HUMAN approves spec → [build agent] → HUMAN merges PR
    ↑                                                                                          |
    └──────────────────────────────────────────────────────────────────────────────────────────┘
```

Each phase:

1. **Observe** — collect signals from GitHub API, infrastructure health, git history, costs, revenue
2. **Assess** — LLM reads state + system prompt + recent history, produces structured JSON assessment
3. **Act (issues)** — create/close issues based on assessment, with function labels for routing
4. **Spec** — spec agent picks up `needs-triage` issues, writes a specification, opens a spec PR
5. **Approve** — human reviews spec PR (approve / request changes / close)
6. **Build** — build agent reads approved spec, implements code, opens a draft PR
7. **Merge** — human reviews implementation PR, merges to main
8. **Loop** — next observation run picks up the changed state

### Observation Loop

- Runs daily on schedule (configurable cron), plus manual trigger and webhook (`repository_dispatch`)
- Collects state via pluggable collector scripts (`scripts/collect-*.sh`)
- Merges into a single JSON snapshot
- Sends snapshot + system prompt + recent history to LLM
- LLM returns structured JSON assessment
- Assessment committed to `loop-history/YYYYMMDD-assessment.md`
- Loop state updated in `loop-state.json`
- Issues created/closed based on assessment output
- All output sanitised before commit (no secrets, no PII)
- Graceful fallback to stub assessment when LLM API unavailable

### Opinionated Defaults

The template ships with a company-oriented system prompt that includes:

- **Funnel stages** (Foundation → Dogfood → Presence → Reachable → Pipeline → Revenue) — users customise stages to their business
- **Frugality constraint** — runway tracking, survival mode when runway < 3 months
- **Reciprocity tracking** — contributors (human, AI, OSS dependencies) logged and flagged for reciprocation
- **Function labels** — `fn:dev`, `fn:ops`, `fn:gtm`, `fn:billing`, `fn:support`, `fn:legal`, `needs-human`
- **Public/private boundary** — rules for what can and cannot be committed to a public repo
- **Cost tracking** — category-level spend, available capital, monthly burn

Users edit `system-prompt.md` to match their company.
The structure (JSON schema, collector scripts, workflow) stays the same.

### Agent-Agnostic Roles

The pipeline defines three agent roles, not specific tools:

| Role | Responsibility | Default | Swappable to |
|------|---------------|---------|-------------|
| **Observer** | LLM that reads state and produces assessment | OpenRouter (any model) | Any chat-completion API |
| **Spec writer** | Agent that turns issues into specification PRs | GitHub Copilot coding agent | Claude Code, Cursor, any agent that can open PRs |
| **Implementer** | Agent that turns approved specs into code PRs | Goose | Claude Code, Aider, any agent that can open PRs |

Configuration via `pipeline.yml` or init script — user picks provider, model, and API key secret name.

### Action Pipeline

- Spec PR created by spec-writer agent when issue gets `needs-triage` label
- Human reviews spec PR using native GitHub review flow:
  - **Approve** → labels `approved-for-build`, triggers build agent
  - **Request changes** → re-assigns spec-writer agent
  - **Close** → rejection, no build
- Build agent reads approved spec + codebase context, implements, opens draft PR
- Build agent handles: toolchain detection, build verification, test execution
- Zero-change runs handled gracefully (no empty PR created)
- Concurrency control prevents parallel builds for the same issue

### State Files

```
loop-state.json       — funnel stage, run count, timestamps
costs.json            — available capital, monthly burn, category spend
metrics.json          — product + community + revenue signals
contributors.json     — contribution ledger (humans, agents, dependencies)
loop-history/         — daily assessment archive
scripts/
  collect-github.sh   — GitHub API signals
  collect-infra.sh    — infrastructure health
  collect-contrib.sh  — git authors, bot activity, dependencies
  sanitise.sh         — secret/PII scanner
system-prompt.md      — LLM operational instructions
```

### Init Script

`init.sh` bootstraps the template:
- Asks for project name, language, build commands, LLM provider
- Configures agent roles (spec-writer, implementer, observer)
- Sets API key secret names
- Populates system prompt with project-specific context
- Seeds state files with sensible defaults
- Creates label definitions

## Design

### Single Repo, Everything Committed

All state lives in the repo — no external database, no SaaS dashboard.
Assessments, state files, specs, and code are all git-tracked.
This means:
- Full audit trail via git history
- Anyone with repo access sees the full company state
- No vendor lock-in beyond GitHub Actions (portable to any CI)
- Fork = instant copy of the entire pipeline + history

### Two Human Gates

The loop is autonomous *between* human decisions.
Humans approve at exactly two points:
1. **Spec approval** — "yes, build this" / "no, change the spec" / "won't do"
2. **PR merge** — "yes, ship this" / "no, needs changes"

Everything else runs without human intervention.
The `needs-human` label escalates when the loop encounters decisions it can't make (cost approval, legal, strategy).

### Workflow Separation

Three independent workflows, loosely coupled via labels:

| Workflow | Trigger | Output |
|----------|---------|--------|
| `observation-loop.yml` | cron / manual / webhook | Assessment + issues |
| `copilot-triage.yml` (or equivalent) | `needs-triage` label | Spec PR |
| `goose-build.yml` (or equivalent) | `approved-for-build` label | Implementation PR |

Label-based coupling means workflows don't call each other directly.
Any workflow can be replaced independently.

### Sanitisation

Every output is scanned before commit:
- API keys, tokens, credentials → blocked
- PII (emails in non-public context) → blocked
- Private financial details → blocked
- Commit fails if sanitisation finds violations

## Verification

- `init.sh` completes without error for Go, Node, Python, Rust, and "other"
- Observation loop runs with stub (no API key) and produces valid assessment
- Observation loop runs with API key and produces LLM assessment
- Issue deduplication prevents duplicates across consecutive runs
- Spec-writer agent creates spec PR from issue
- Approve → build agent creates implementation PR
- Request changes → spec-writer agent revises
- Close → no build triggered
- Zero-change build produces no PR (graceful skip)
- Sanitisation blocks a planted secret in assessment output
- Full loop: observation creates issue → spec → approve → build → merge → next observation sees the change

## Friction

- **LLM assessment quality varies** — the system prompt needs tuning per-company; bad prompts produce generic assessments
- **Issue dedup is heuristic** — fuzzy keyword matching works but isn't perfect; occasional duplicates or missed matches
- **Agent availability** — Copilot coding agent requires GitHub Copilot subscription; Goose requires separate LLM key
- **Rate limits** — GitHub API rate limits affect state collection on repos with heavy activity
- **Cold start** — first few assessments are low-value until the LLM has history to compare against
- **Public repo tension** — the transparency model (assessments committed publicly) doesn't suit all companies

## Interactions

- Depends on: GitHub Actions, at least one LLM API (OpenRouter, Anthropic, OpenAI, etc.)
- Depends on: at least one spec-writing agent (Copilot, Claude Code, etc.)
- Depends on: at least one implementation agent (Goose, Claude Code, Aider, etc.)
- Supersedes: `wgmesh/company/` (observation loop migrates here)
- Related: [[spec - first-customer - roadmap to first paying customer]] (wgmesh uses this pipeline)

## Mapping

> `atvirokodosprendimai/ai-pipeline-template` (GitHub repo)
> `.github/workflows/goose-build.yml`
> `.github/workflows/copilot-triage.yml`
> `.github/workflows/approve-build.yml`
> `.github/workflows/sync-labels.yml`
> `wgmesh/.github/workflows/company-loop.yml` (to be migrated)
> `wgmesh/company/` (to be migrated)

## Future

{[!] Migration plan — move `wgmesh/company/` and `company-loop.yml` into `ai-pipeline-template`}
{[!] Rename observation workflow from `company-loop.yml` to `observation-loop.yml`}
{[!] Generalise `system-prompt.md` — extract wgmesh-specific references into a customisation section}
{[!] Add observation loop to `init.sh` setup flow}
{[!] Documentation — README update reflecting the full loop, not just the action half}
{[?] GitHub Action extraction — package core logic as reusable Actions when patterns stabilise}
{[?] Non-GitHub CI support — GitLab CI, Forgejo Actions}
{[?] Dashboard view — static site generated from loop-history for visual tracking}
{[?] Multi-repo observation — single loop observing an entire GitHub org}
