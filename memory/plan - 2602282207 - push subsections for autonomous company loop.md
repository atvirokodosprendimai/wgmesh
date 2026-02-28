---
tldr: Phased plan to implement the autonomous company control loop per the first-customer spec
---

# Plan: Push Autonomous Company Loop

Source: [[spec - first-customer - roadmap to first paying customer]]
Push doc: [[push - 2602282207 - implement autonomous company loop]]

## Phase 1: Company state foundation

Bootstrap the `company/` directory with seed state so the loop has something to read on first run.

- [ ] Create `company/` directory structure
- [ ] Create `company/loop-state.json` — initial state: `{ "funnel_stage": 0, "stage_name": "Foundation", "last_run": null, "history": [] }`
- [ ] Create `company/health.json` — empty initial: `{ "last_check": null, "services": {} }`
- [ ] Create `company/costs.json` — empty initial: `{ "last_updated": null, "categories": {} }`
- [ ] Create `company/metrics.json` — empty initial: `{ "last_updated": null, "accounts": 0, "services": 0, "nodes": 0 }`
- [ ] Create `company/contributors.json` — seed from git log authors + go.mod dependencies + AI agents
- [ ] Create `company/loop-history/` directory with `.gitkeep`
- [ ] Commit

## Phase 2: LLM system prompt

Create the operational prompt the loop feeds to the LLM — distilled from the spec.

- [ ] Create `company/system-prompt.md` — funnel stages + transition criteria, output JSON schema, public/private rules, reciprocity principle, assessment format, issue creation format
- [ ] Define assessment output schema (funnel stage, blockers, top 3 actions, issues to create/close, contribution acknowledgments, human intervention requests)
- [ ] Commit

## Phase 3: State collection

Scripts that gather signals for the LLM. Start with what's observable today (GitHub API). Infra/billing signals come later as those systems exist.

- [ ] Create `company/scripts/collect-github.sh` — issues by label, open PRs, merge rate (last 7d), latest release, test pass rate, stars/forks, traffic, recent contributors
- [ ] Create `company/scripts/collect-contributions.sh` — git log contributors, AI agent activity from workflow runs, go.mod dependencies with GitHub sponsor status
- [ ] Create `company/scripts/collect-infra.sh` — stub that checks Chimney/Lighthouse health endpoints (expand later)
- [ ] Create `company/scripts/sanitise.sh` — strip patterns matching API keys, tokens, emails, IPs from collected state before passing to LLM
- [ ] Commit

## Phase 4: Control loop workflow

The core `company-loop.yml` — ties everything together.

- [ ] Create `.github/workflows/company-loop.yml`:
  - Triggers: schedule (daily 08:00 UTC), workflow_dispatch, repository_dispatch
  - Job 1: collect state (run collection scripts, output JSON)
  - Job 2: run LLM (load system prompt + state + previous loop-state, call Anthropic API, parse structured output)
  - Job 3: act on output (create/close issues with function labels, commit assessment to `company/loop-history/YYMMDD-assessment.md`, update `company/loop-state.json`, update `company/contributors.json`)
  - Job 4: notify if `needs-human` actions in output
- [ ] Add function labels to repo: `fn:dev`, `fn:ops`, `fn:gtm`, `fn:billing`, `fn:support`, `fn:legal`, `needs-human`
- [ ] Commit

## Phase 5: Board + pipeline integration

Connect new function labels to existing board sync and pipeline.

- [ ] Update `.github/workflows/board-sync.yml` — handle `fn:*` labels, route to appropriate board columns or a new "Company" board
- [ ] Ensure `fn:dev` + `needs-triage` issues flow into existing Copilot → Goose pipeline
- [ ] Commit

## Phase 6: Safety + secret scanning

Prevent the loop from leaking secrets to the public repo.

- [ ] Add secret pattern scanning step in `company-loop.yml` before any git commit (grep for AWS keys, GitHub tokens, API keys, emails, etc. — fail the workflow if found)
- [ ] Create `.github/hooks/pre-commit-secret-scan` (optional local hook)
- [ ] Commit

## Phase 7: First run + verify

Manual trigger, observe, fix.

- [ ] Add `ANTHROPIC_API_KEY` to GitHub secrets (`needs-human`)
- [ ] Trigger `company-loop.yml` via workflow_dispatch
- [ ] Verify: assessment created in `company/loop-history/`, loop-state updated, issues created with correct labels, no secrets in committed files
- [ ] Fix any issues found
- [ ] Commit fixes
