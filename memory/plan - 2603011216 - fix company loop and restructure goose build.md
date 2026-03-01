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

### Phase 1 - Merge company-loop fix to main - status: open

1. [ ] Create PR for `task/fix-company-loop-workflow` → `main`
2. [ ] Merge (or get it merged) to unblock the daily schedule

### Phase 2 - Extract Goose task builder script - status: open

Extract the task-building logic from `goose-build.yml` into a standalone script that reads the recipe.

1. [ ] Create `company/scripts/goose-build-task.sh`
   - reads recipe YAML via `yq` for prompt, settings, checks
   - reads `.goosehints` for project context
   - reads `AGENTS.md` for conventions
   - builds `/tmp/goose-task.md` from recipe prompt + codebase context + spec file
   - standalone: can be run locally with `./company/scripts/goose-build-task.sh specs/issue-42-spec.md`
2. [ ] Create `company/scripts/goose-validate.sh`
   - reads recipe YAML checks via `yq`
   - runs each check command in sequence
   - reports pass/fail per check
   - outputs structured results (exit code + summary)
3. [ ] Create `company/scripts/goose-run.sh`
   - reads recipe settings (model, max_turns, retries)
   - retry loop with backoff (currently inline in workflow)
   - calls goose CLI with task file
   - on retry, builds fix instructions from validation errors
   - outputs metrics JSON
4. [ ] Update recipe `wgmesh-implementation.yaml`
   - ensure all settings are canonical: model, max_turns, retries, checks
   - add `context_files` field listing `.goosehints`, `AGENTS.md`

### Phase 3 - Slim down goose-build.yml - status: open

Replace inline logic with script calls. Keep GitHub-specific orchestration (PR creation, auto-merge, memory, metrics, artifact upload).

1. [ ] Replace "Build codebase context" + "Build Goose instructions" steps with call to `goose-build-task.sh`
2. [ ] Replace "Run Goose" step (retry loop) with call to `goose-run.sh`
3. [ ] Replace "Validate implementation" step with call to `goose-validate.sh`
4. [ ] Keep: checkout, Go setup, Goose install, mem0, spec extraction, branch creation, commit/push, PR creation, metrics — these are GitHub-specific
5. [ ] Add `yq` install step (needed for recipe parsing)
6. [ ] Test: trigger manually with a test issue to verify the refactored workflow works

### Phase 4 - Update goose-review.yml - status: open

Apply same pattern: review workflow should also use recipe/scripts where applicable.

1. [ ] Refactor `goose-review.yml` to use `goose-run.sh` for the Goose invocation
   - review-specific task building stays in workflow (reads PR threads)
   - but Goose invocation + retry uses shared script

## Verification

- `company-loop.yml` runs successfully on main (daily schedule or manual trigger)
- `goose-build.yml` triggers and completes with a test issue (manual workflow_dispatch)
- Scripts are independently runnable: `./company/scripts/goose-build-task.sh <spec-file>` produces a valid task file
- Recipe YAML is the single source of truth for prompt, model, checks, retries
- No prompt duplication between recipe and workflow

## Adjustments

## Progress Log

