# Specification: Issue #652 - Investigate and fix CI failure blocking merge pipeline

## Summary

Fix CI pipeline failures blocking PR merges by resolving four root causes: (1) phantom "CodeQL" workflow reference in `auto-merge.yml`, (2) 11 workflows using expired `secrets.PUSH_TOKEN` PAT, (3) raw `PUSH_TOKEN` fetch() calls in `auto-merge.yml`, and (4) `pr-review-merge.sh` referencing non-existent `spec-validation.yml`.

## Context

The wgmesh CI/CD pipeline relies on GitHub Actions workflows for automated testing, code review, and merging. Recent changes migrated some workflows from the deprecated `secrets.PUSH_TOKEN` PAT to GitHub App tokens (PRs #659, #661), but 11 workflows were not migrated, causing runtime failures when attempting API operations requiring write access.

Additionally, the `auto-merge.yml` workflow triggers on `workflow_run` events from a non-existent "CodeQL" workflow, causing PRs to wait for a 10-minute cron fallback instead of merging promptly after status checks pass.

The `pr-review-merge.sh` script also references a `spec-validation.yml` workflow that doesn't exist, creating race conditions in the spec PR review flow.

## Requirements

- All GitHub Actions workflows must use valid authentication (GitHub App tokens, not expired PATs)
- All workflow triggers must reference existing workflows
- All script references to workflows must be accurate
- CI pipeline must reliably merge PRs after all required checks pass
- No phantom or non-existent workflow references

## Acceptance Criteria

- `go build ./...` passes
- `go test ./...` passes (all existing tests including `company/scripts/pr-review-merge_test.sh`)
- `go vet ./...` passes
- No workflow file references `secrets.PUSH_TOKEN` (verified via `grep -r 'secrets.PUSH_TOKEN' .github/workflows/` returns empty)
- No workflow file references "CodeQL" (verified via `grep -r 'CodeQL' .github/workflows/` returns empty)
- `auto-merge.yml` `workflow_run` trigger lists only workflows that exist (verified via matching actual workflow `name:` fields)
- `pr-review-merge.sh` does not reference `spec-validation.yml` (verified via `grep 'spec-validation' company/scripts/pr-review-merge.sh` returns empty)
- Every migrated workflow follows the same app-token pattern: `Generate app token` step → `steps.app-token.outputs.token` (verified via consistency with `goose-build.yml`, `goose-triage.yml`, `spec-auto-approve.yml`)

## Out of scope

- Migrating workflows already updated in PRs #659 and #661 (`goose-build.yml`, `goose-triage.yml`, `spec-auto-approve.yml`, `e2e-stalled-watcher.yml`, `e2e-verifier.yml`, `auto-merge.yml` app-token generation step)
- Creating new workflows or changing CI pipeline logic beyond authentication fixes
- Modifying workflow business logic or conditions
- Changes to Docker build, test, or deployment processes
