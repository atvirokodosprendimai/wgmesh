---
title: "Fix PR #439 Copilot review comments on solution docs"
type: fix
status: active
date: 2026-03-13
---

# Fix PR #439 Copilot review comments on solution docs

## Overview

PR #439 (merged) added solution docs for v0.2.1 release issues. Copilot left 3 review comments identifying documentation inaccuracies and a fragile shell snippet. All fixes are documentation-only — no code changes.

## Acceptance Criteria

- [x] **Comment 1** — `custom-subnet-silent-fallback-and-missed-callsites.md`: Align prose and code snippet to both reference `DeriveMeshIPWithNonce` (the actual fallback function in collision.go).
- [ ] **Comment 2** — `goreleaser-dual-tag-same-commit-conflict.md` Root Cause: Update to reflect that `release.yml` uses `git describe --tags --exact-match HEAD`, not bare `git describe --tags`. Clarify that `--exact-match` can still pass when multiple tags point at HEAD, and recommend validating the exact tag name matches the intended release.
- [ ] **Comment 3** — `goreleaser-dual-tag-same-commit-conflict.md` cleanup script: Make the `for` loop resilient to `set -e` by adding `|| true` to each command so the snippet is safe to paste into strict-mode scripts.

## Files to Modify

1. `docs/solutions/logic-errors/custom-subnet-silent-fallback-and-missed-callsites.md` — line 52 code snippet
2. `docs/solutions/integration-issues/goreleaser-dual-tag-same-commit-conflict.md` — Root Cause section + cleanup script

## Sources

- PR review: https://github.com/atvirokodosprendimai/wgmesh/pull/439#pullrequestreview-3942272425
- Actual `git describe` usage: `.github/workflows/release.yml:55`
