---
tldr: Push doc for implementing the first-customer spec — autonomous company control loop and supporting infrastructure
---

# Push: Implement Autonomous Company Loop

Source: [[spec - first-customer - roadmap to first paying customer]]

## Inventory

### Concern 1: Company state directory (`company/`)
- **Current state**: does not exist
- **Spec says**: `company/` with `loop-state.json`, `health.json`, `costs.json`, `metrics.json`, `contributors.json`, `loop-history/`
- **Action**: create directory structure with initial seed files (empty/default state)

### Concern 2: Control loop workflow (`company-loop.yml`)
- **Current state**: does not exist
- **Spec says**: scheduled daily + event-driven GitHub Actions workflow that observes state, calls LLM, creates issues, commits assessments
- **Action**: create `.github/workflows/company-loop.yml` with:
  - State collection jobs (GitHub API, infra health, contribution signals)
  - LLM assessment call (Anthropic API via secret)
  - Issue creation/management from LLM output
  - Commit loop state + assessment to `company/`
  - Secret sanitisation before LLM call
  - `needs-human` issue creation when intervention requested
- **Dependencies**: needs `ANTHROPIC_API_KEY` in GitHub secrets (human action)

### Concern 3: Function labels + board integration
- **Current state**: existing labels are `needs-triage`, `copilot-triaging`, `spec-ready`, `approved-for-build`, `goose-implementation`, `needs-review`
- **Spec says**: new labels `fn:dev`, `fn:ops`, `fn:gtm`, `fn:billing`, `fn:support`, `fn:legal`, `needs-human`
- **Action**: add labels to `sync-labels.yml` or create them in the loop's first run
- **Also**: update `board-sync.yml` to handle new function labels

### Concern 4: State collection scripts
- **Current state**: does not exist
- **Spec says**: parallel jobs querying GitHub API, infra endpoints, billing API, contribution signals
- **Action**: create collection scripts or inline workflow steps:
  - `collect-github-signals.sh` — issues by label, PRs, merge rate, releases, traffic, contributors
  - `collect-infra-signals.sh` — health endpoints, deploy status
  - `collect-contribution-signals.sh` — contributor activity, dependency health
  - Output: JSON files consumed by the LLM step

### Concern 5: LLM system prompt
- **Current state**: the spec itself is the system prompt conceptually, but no operational prompt exists
- **Spec says**: LLM receives system prompt (this spec) + state snapshot + loop history
- **Action**: create `company/system-prompt.md` — operational version of the spec distilled for the LLM, including funnel stages, transition criteria, output format, public/private rules, reciprocity principle

### Concern 6: Secret sanitisation
- **Current state**: no protection
- **Spec says**: pre-commit hook scans for secret patterns, state collection sanitises inputs
- **Action**: add secret scanning to the workflow (grep for API key patterns before commit) and optionally a pre-commit hook

### Concern 7: Contributor ledger
- **Current state**: does not exist
- **Spec says**: `company/contributors.json` — entities, contribution types, reciprocation status
- **Action**: seed with initial contributors (GitHub commit authors, dependencies from go.mod, AI agents), define schema

### Concern 8: Assessment output format
- **Current state**: does not exist
- **Spec says**: `company/loop-history/YYMMDD-assessment.md` — readable narrative, publicly visible
- **Action**: define template in system prompt, workflow writes LLM output as markdown

## Scope Assessment

This is a **large change** — 8 concerns, multiple new files, new workflow, new directory structure, LLM integration, label/board changes. Multi-pass mode is appropriate.

Recommended plan:
1. **Foundation**: create `company/` directory, seed state files, contributor ledger schema
2. **Collection**: state collection scripts for GitHub signals (most observable today)
3. **Loop core**: `company-loop.yml` workflow with LLM call, assessment output, issue creation
4. **Labels + board**: function labels, board-sync integration
5. **Safety**: secret sanitisation, pre-commit hook
6. **First run**: manual trigger, verify end-to-end, fix issues
