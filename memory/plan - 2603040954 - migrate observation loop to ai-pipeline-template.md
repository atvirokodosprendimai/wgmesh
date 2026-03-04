---
tldr: Move company/ and observation loop from wgmesh into ai-pipeline-template, generalise for any project
status: active
---

# Plan: Migrate observation loop to ai-pipeline-template

## Context

- Spec: [[spec - ai pipeline template - autonomous product loop for ai-native startups]]
- Source repo: `atvirokodosprendimai/wgmesh` (`company/`, `.github/workflows/company-loop.yml`)
- Target repo: `atvirokodosprendimai/ai-pipeline-template`
- Strategy: move everything first, then generalise (preserve git history of moved files)

## Phases

### Phase 1 - Move files to ai-pipeline-template - status: done

All work in this phase happens in the `ai-pipeline-template` repo.

1. [x] Clone ai-pipeline-template locally, create branch `task/observation-loop`
   - => cloned to `/Users/coder/repo/ai-pipeline-template`
   - => branch `task/observation-loop` created from `a0771fe`
2. [x] Copy observation loop files from wgmesh into template:
   - `company/` → `company/` (all state files, scripts, loop-history)
   - `.github/workflows/company-loop.yml` → `.github/workflows/observation-loop.yml` (rename)
   - Function labels from wgmesh `.github/labels.yml` → merge into template `.github/labels.yml`
   - Commit as-is (faithful copy before any changes)
   - => commit `2bf13ff` — 17 files, 1015 insertions
3. [x] Add `company/` to template `.gitignore` exception if needed
   - Template already tracks `.github/` so workflows are fine
   - Verify state JSON files won't be ignored
   - => no .gitignore exists in template — no action needed

### Phase 2 - Generalise for any project - status: done

Still in ai-pipeline-template repo.

1. [x] Replace wgmesh-specific content in `company/system-prompt.md` with `__PLACEHOLDER__` markers:
   - Product name/description → `__PROJECT_NAME__`, `__PROJECT_DESCRIPTION__`
   - Funnel stage details → keep structure, make stage descriptions generic with placeholder examples
   - Infrastructure endpoints → `__HEALTH_ENDPOINTS__`
   - Remove wgmesh-specific references (Chimney, cloudroof, Lighthouse, managed ingress)
   - Keep opinionated structure: funnel stages, frugality, reciprocity, public/private boundary
   - => kept European-first constraint as-is (opinionated default)
2. [x] Generalise collector scripts:
   - `collect-github.sh` — removed hardcoded `atvirokodosprendimai/wgmesh` fallback, now requires `GITHUB_REPOSITORY`
   - `collect-infra.sh` — now reads endpoints from `company/health.json` dynamically
   - `collect-contributions.sh` — made language-agnostic (Go/Node/Python/Rust dep detection)
   - `sanitise.sh` — already generic, no changes needed
   - Removed `goose-build-context.sh` (build-specific, not observation)
3. [x] Replace secrets in `observation-loop.yml` with `__PLACEHOLDER__` patterns:
   - `OPENROUTER_API_KEY` → `__OBSERVER_API_KEY_SECRET__`
   - `PUSH_TOKEN` → kept as `PUSH_TOKEN` (GitHub standard)
   - LLM endpoint/model → `__OBSERVER_API_URL__`, `__OBSERVER_MODEL__`
   - => env var in workflow uses `OBSERVER_API_KEY` (safe pattern per GH Actions security)
4. [x] Reset state files to clean defaults:
   - `loop-state.json` → stage 0, run_count 0, no timestamps
   - `costs.json` → empty structure, no provider-specific entries
   - `metrics.json` → empty structure
   - `contributors.json` → empty entities array
   - `health.json` → empty endpoints array
   - Cleared `loop-history/` (kept `.gitkeep`)
5. [x] Commit: generalised observation loop with placeholders
   - => commit `04d3709` — 14 files changed, 82 insertions, 309 deletions

### Phase 3 - Update init.sh - status: open

1. [ ] Add observation loop section to `init.sh`:
   - Ask: "Enable observation loop? [y/n]" (default: y)
   - If yes, ask for:
     - Observer LLM provider (openrouter/openai/anthropic/other)
     - Observer model name
     - Observer API key secret name
     - Health check endpoints (comma-separated URLs)
     - Available capital (EUR/year) for costs.json
   - Replace `__PLACEHOLDER__` markers in `observation-loop.yml` and `system-prompt.md`
   - Seed `company/health.json` with provided endpoints
   - Seed `company/costs.json` with available capital
2. [ ] If observation loop declined, remove `company/` and `observation-loop.yml`
3. [ ] Update provider presets to include observer defaults:
   - openrouter: model `anthropic/claude-sonnet-4`, key `OPENROUTER_API_KEY`
   - openai: model `gpt-4o`, key `OPENAI_API_KEY`
   - anthropic: model `claude-sonnet-4-20250514`, key `ANTHROPIC_API_KEY`

### Phase 4 - Update documentation - status: open

1. [ ] Update `README.md`:
   - Add "Observation Loop" section explaining the observe → assess → act cycle
   - Update "How It Works" table to include observation phase
   - Update "File Manifest" with `company/` directory
   - Update "Prerequisites" to mention observer LLM key
   - Add "Full Loop" diagram showing both halves connected
2. [ ] Update pipeline flow diagram (`docs/pipeline-flow.d2`) to include observation loop
3. [ ] Commit and push branch, open PR

### Phase 5 - Clean up wgmesh - status: open

Back in wgmesh repo.

1. [ ] Remove `company/` directory from wgmesh
   - Keep a note in commit message pointing to ai-pipeline-template
2. [ ] Remove `.github/workflows/company-loop.yml`
3. [ ] Remove function labels from `.github/labels.yml` (fn:dev, fn:ops, fn:gtm, fn:billing, fn:support, fn:legal, needs-human)
4. [ ] Update `eidos/spec - first-customer - roadmap to first paying customer.md`:
   - Add note that the company loop now lives in ai-pipeline-template
   - Update any references to `company/` paths
5. [ ] Remove or archive company loop plans from `memory/`:
   - [[plan - 2602282207 - push subsections for autonomous company loop]] → mark completed/migrated
   - [[plan - 2603011216 - fix company loop and restructure goose build]] → mark completed/migrated
6. [ ] Commit: remove company loop (migrated to ai-pipeline-template)

### Phase 6 - Verify end-to-end - status: open

1. [ ] In ai-pipeline-template: run `init.sh` with observation loop enabled, verify all placeholders replaced
2. [ ] Trigger observation-loop.yml manually (stub mode, no API key) — verify stub assessment created
3. [ ] Trigger observation-loop.yml with API key — verify LLM assessment created
4. [ ] Create a test issue → verify Copilot picks it up → approve → verify Goose builds
5. [ ] Verify wgmesh CI still passes without company/ directory

## Verification

- ai-pipeline-template contains both action and observation pipelines
- `init.sh` configures both halves interactively
- Observation loop runs successfully (stub + LLM modes)
- Action pipeline still works (issue → spec → approve → build)
- No wgmesh-specific content remains in the template
- wgmesh repo is clean — no company/ directory, no company-loop workflow
- README documents the full loop

## Adjustments

## Progress Log

- 2603040954 — Phase 1 complete: cloned repo, copied all files, renamed workflow, merged labels. Commit `2bf13ff`.
- 2603041010 — Phase 2 complete: generalised all files, reset state, replaced placeholders. Commit `04d3709`.
