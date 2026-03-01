---
tldr: Fix company-loop jq failure on main and deeply restructure goose-build.yml so Goose recipe is the portable artifact
status: active
---

# Plan: Fix Company Loop and Restructure Goose Build

## Context

- Spec: [[spec - first-customer - roadmap to first paying customer]]
- Prior plan: [[plan - 2602282207 - push subsections for autonomous company loop]]
- Recipe: `.github/goose-recipes/wgmesh-implementation.yaml`
- Goosehints: `.goosehints`

**Problem 1:** `company-loop.yml` on `main` fails with `jq: invalid JSON text passed to --argjson`.
The fix already exists on `task/fix-company-loop-workflow` (rewrites as single job with `--slurpfile`).
Need to merge this to main.

**Problem 2:** `goose-build.yml` has the entire Goose prompt, retry logic, validation, and task-building inline (~1000 lines).
The recipe file (`.github/goose-recipes/wgmesh-implementation.yaml`) exists but isn't used.
Goal: make the recipe + standalone scripts the source of truth, workflow becomes thin orchestration.

## Phases

### Phase 1 - Merge company-loop fix to main - status: completed

1. [x] Create PR for `task/fix-company-loop-workflow` → `main`
   - => PR #354 already existed
   - => addressed 2 Copilot review comments: fetch-depth: 0 and read -r
2. [x] Merge (or get it merged) to unblock the daily schedule
   - => merged as `1c709e7` via squash

### Phase 2 - Extract Goose task builder script - status: completed

Extract the task-building logic from `goose-build.yml` into a standalone script that reads the recipe.

1. [x] Create `company/scripts/goose-build-task.sh`
   - => reads recipe via yq for prompt, context_files, checks
   - => generates codebase type context from pkg/*/
   - => includes memory context if MEMORY_FILE env set
   - => standalone: `./company/scripts/goose-build-task.sh specs/issue-42-spec.md`
2. [x] Create `company/scripts/goose-validate.sh`
   - => reads checks from recipe, runs each, outputs JSON summary with diff stats
3. [x] Create `company/scripts/goose-run.sh`
   - => reads provider, model, max_turns, retries from recipe
   - => full retry loop with backoff, rate-limit detection, fix instructions on retry
   - => outputs /tmp/goose-metrics.json
4. [x] Update recipe `wgmesh-implementation.yaml`
   - => added context_files: [.goosehints, AGENTS.md]
   - => expanded prompt with real-types guidance
   - => commit: `815b923`

### Phase 3 - Use native `goose run --recipe` - status: completed

Research revealed Goose has native recipe execution with `goose run --recipe`.
Recipe already supports retry+checks, model settings, extensions, and parameters.

1. [x] Rewrite recipe as self-sufficient artifact
   - => added `extensions: [{type: builtin, name: developer}]`
   - => added `parameters: [{key: spec_file, input_type: file}]` with `{{ spec_file }}` in prompt
   - => split `instructions` (system) from `prompt` (initial message) per Goose spec
   - => deleted `goose-run.sh` and `goose-validate.sh` (native retry handles this)
   - => renamed `goose-build-task.sh` → `goose-build-context.sh` (just codebase types)
   - => commit: `bc29185`
2. [x] Rewrite `goose-build.yml`
   - => 1016 → 481 lines (53% reduction)
   - => single `goose run --recipe` call replaces inline retry loop + task builder
   - => `GOOSE_MODE=auto`, `GOOSE_DISABLE_SESSION_NAMING=true` for CI
   - => all untrusted inputs via env vars (injection safety)
   - => commit: `23cd09c`
3. [x] Update `goose-review.yml`
   - => created `wgmesh-review.yaml` recipe
   - => workflow passes review feedback as recipe parameter
   - => added Go setup step for recipe retry checks
   - => branch ref uses env var (injection safety)
   - => commit: `96dc9ae`

### Phase 4 - Test and verify - status: open

1. [ ] Test: trigger `goose-build.yml` manually with a test issue
2. [ ] Verify recipe executes correctly with `goose run --recipe` locally
3. [ ] Verify `goose-review.yml` works with a PR that has review comments

## Verification

- `company-loop.yml` runs successfully on main (daily schedule or manual trigger)
- `goose-build.yml` triggers and completes with a test issue (manual workflow_dispatch)
- Scripts are independently runnable: `./company/scripts/goose-build-task.sh <spec-file>` produces a valid task file
- Recipe YAML is the single source of truth for prompt, model, checks, retries
- No prompt duplication between recipe and workflow

## Adjustments

- 2603011315 — Phase 3 rewritten after Goose docs research. Goose has native `goose run --recipe` with retry+checks, parameters, extensions. Our Phase 2 scripts (`goose-run.sh`, `goose-validate.sh`) duplicate native features. New plan: make recipe self-sufficient, delete redundant scripts, keep only `goose-build-task.sh` for codebase context generation.

## Progress Log

- 2603011230 — Phase 1 complete. PR #354 merged to main. Company loop unblocked.
- 2603011245 — Phase 2 complete. Three scripts + recipe update in `815b923`.
- 2603011315 — Goose docs research: native `goose run --recipe` makes goose-run.sh and goose-validate.sh redundant. Adjusted Phase 3.
- 2603011345 — Phase 3 complete. Recipe rewritten, goose-build.yml 53% smaller, goose-review.yml uses recipe, redundant scripts deleted.
