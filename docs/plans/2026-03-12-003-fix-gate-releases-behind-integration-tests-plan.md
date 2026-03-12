---
title: "fix: Gate releases behind integration tests"
type: fix
status: completed
date: 2026-03-12
---

# fix: Gate releases behind integration tests

## Overview

v0.2.0 was released with failing Hetzner integration tests (tiers 1,2,4,5 failed).
The release pipeline published 12 artifacts (binaries, packages, Docker images, Homebrew tap) while integration tests were still running and failing.
This must never happen again.

## Problem Statement

`release.yml` and `hetzner-integration.yml` both trigger independently on `v*.*.*` tag push.
They run in parallel with no dependency — GoReleaser publishes artifacts before integration tests complete.

**Current flow (broken):**
```
tag push v*.*.*
  ├── release.yml → GoReleaser publishes artifacts (fast, ~3 min)
  └── hetzner-integration.yml → 7 tiers on Hetzner VMs (~60-90 min)
      (no dependency between them)
```

**Evidence:**
- v0.2.0 tag push: run [22995657773](https://github.com/atvirokodosprendimai/wgmesh/actions/runs/22995657773) — tiers 1,2,4,5 failed
- v0.2.0-rc1 (March 1): also failed — pre-existing issue
- Last successful integration test: Feb 19 on main

## Proposed Solution

Convert release.yml from a direct tag-push trigger to a `workflow_run` trigger that waits for integration tests to complete successfully.

**New flow:**
```
tag push v*.*.*
  └── hetzner-integration.yml → 7 tiers on Hetzner VMs (~60-90 min)
      └── on success → release.yml (workflow_run trigger) → GoReleaser publishes
```

## Acceptance Criteria

- [x] Release artifacts are never published when integration tests fail
- [x] Release artifacts are never published when integration tests are still running
- [x] Release workflow only triggers after successful completion of `hetzner-integration.yml`
- [x] Manual releases remain possible via `workflow_dispatch` (with explicit bypass flag)
- [x] Tag-triggered integration tests that fail produce a clear notification (existing GitHub Actions UI is sufficient)
- [x] The gate applies to all `v*.*.*` tags, not just specific ones

## Implementation

### Phase 1: Gate release behind integration tests

#### 1.1 Modify `release.yml` trigger

Replace the direct tag-push trigger with a `workflow_run` trigger:

```yaml
# .github/workflows/release.yml
name: Release

on:
  workflow_run:
    workflows: ["Hetzner Integration Tests"]
    types: [completed]
  workflow_dispatch:
    inputs:
      skip_integration_check:
        description: 'Skip integration test gate (emergency only)'
        required: false
        default: 'false'
        type: boolean

permissions:
  contents: write

jobs:
  release:
    name: GoReleaser
    runs-on: ubuntu-latest
    # Only run if:
    # 1. Integration tests completed successfully on a tag, OR
    # 2. Manual dispatch with explicit bypass
    if: >-
      (github.event_name == 'workflow_run'
       && github.event.workflow_run.conclusion == 'success'
       && startsWith(github.event.workflow_run.head_branch, 'v'))
      || (github.event_name == 'workflow_dispatch'
          && github.event.inputs.skip_integration_check == 'true')
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          # workflow_run checks out the default branch by default,
          # need to checkout the tag
          ref: ${{ github.event.workflow_run.head_branch || github.ref }}

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PUSH_TOKEN: ${{ secrets.PUSH_TOKEN }}
```

**Key considerations:**
- `workflow_run` receives the triggering workflow's metadata, including `head_branch` (the tag name)
- `startsWith(head_branch, 'v')` ensures we only release on version tags, not `workflow_dispatch` runs of integration tests
- `workflow_dispatch` with `skip_integration_check` provides emergency escape hatch
- Must explicitly checkout the tag ref since `workflow_run` defaults to the default branch

#### 1.2 Verify `hetzner-integration.yml` tag trigger is correct

The existing trigger is already correct:
```yaml
on:
  push:
    tags:
      - 'v*.*.*'
```

No changes needed to `hetzner-integration.yml`.

### Phase 2: Fix failing integration tests

Integration tests have been failing since v0.2.0-rc1 (March 1).
The gate is useless if tests always fail.

- [ ] Download tier logs from the v0.2.0 run to diagnose failures
- [ ] Identify root cause of tier 1, 2, 4, 5 failures
- [ ] Fix the test infrastructure or code issues
- [ ] Verify a clean run on main before next release

**Note:** This is a separate effort tracked in its own issue/plan. The gate should be deployed first — a failing gate that blocks bad releases is correct behavior.

### Phase 3: Verify the gate works

- [ ] Push a test tag (e.g., `v0.2.1-rc1`) to verify:
  - Integration tests trigger on the tag
  - Release does NOT trigger immediately
  - Release triggers only after integration tests pass
- [ ] If integration tests fail, verify release is NOT triggered
- [ ] Test `workflow_dispatch` bypass works for emergency releases

## Technical Considerations

### `workflow_run` limitations

1. **Ref context:** `workflow_run` events run in the context of the default branch, not the tag. The `head_branch` field contains the tag name. We must explicitly `ref:` checkout the tag.

2. **Conclusion values:** `workflow_run.conclusion` can be `success`, `failure`, `cancelled`, `skipped`, `timed_out`, `action_required`, `stale`, or `neutral`. Only `success` should trigger release.

3. **Concurrency:** If multiple tags are pushed quickly, multiple integration test runs may overlap. The existing `concurrency: hetzner-integration` with `cancel-in-progress: true` handles this — only the latest tag's tests run. The release will only trigger for the tag whose tests actually complete successfully.

### Edge cases

| Scenario | Expected Behavior |
|----------|-------------------|
| Tag push, tests pass | Release triggers automatically |
| Tag push, tests fail | No release, investigate failures |
| Tag push, tests cancelled (new tag pushed) | No release for cancelled tag |
| `workflow_dispatch` integration test | No release (not a tag) |
| Emergency release needed, tests broken | Use `workflow_dispatch` with `skip_integration_check: true` |
| Integration tests time out (120min) | No release, conclusion = `timed_out` |

## Risk Analysis

| Risk | Mitigation |
|------|------------|
| Integration tests always fail, blocking all releases | Emergency bypass via `workflow_dispatch`; Phase 2 fixes tests |
| `workflow_run` ref checkout fails | Explicit `ref:` parameter in checkout step |
| GoReleaser needs the tag ref, not HEAD | `fetch-depth: 0` + explicit tag checkout ensures full history |
| Multiple rapid tag pushes cause confusion | Concurrency group on integration tests handles this |

## Sources

- Release workflow: `.github/workflows/release.yml`
- Integration tests: `.github/workflows/hetzner-integration.yml`
- Failed v0.2.0 run: https://github.com/atvirokodosprendimai/wgmesh/actions/runs/22995657773
- GitHub docs: [workflow_run event](https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows#workflow_run)
