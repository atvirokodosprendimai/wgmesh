---
title: "fix: Auto-verify network bug fixes via e2e workflow"
type: fix
status: active
date: 2026-05-06
origin: https://github.com/atvirokodosprendimai/wgmesh/issues/568
---

# fix: Auto-verify network bug fixes via e2e workflow

## Summary

Replace the human-typed `verified` close path for network-class bug fixes with an automated e2e verifier. When a bug-fix PR labeled `type: bug` and touching `pkg/daemon/`, `pkg/discovery/`, or `pkg/rpc/` merges, a `workflow_run`-driven verifier runs a fast Hetzner integration subset against the merge commit, then flips the linked issue between `verified` (close), `e2e-failed` (reopen + reporter ping), and `e2e-stalled` (timeout watcher). Predicate-only unit tests no longer satisfy the L2 gate on network paths — an `*_integration_test.go` file change is required.

---

## Problem Frame

PR #559 introduced an `awaiting-verification` gate that requires the original issue reporter to type `verified` / `confirmed` / `fixed` before the issue closes. For network-behavior bugs (relay flap, hole-punch fail, NAT traversal, peer discovery, WireGuard handshake), unit tests of decision predicates cannot reproduce the bug class. The current gate accepts any `func TestXxx(t *testing.T)` as proof-of-fix, so PR #564 (NAT relay stability) merged with only `pkg/daemon/relay_test.go` predicate tests and issue #556 has been sitting in `awaiting-verification` indefinitely with no human comment.

The pipeline is supposed to be autonomous; the human-in-loop close requirement re-inserts a verification step that the original PR #559 design tried to eliminate. The reproduction surface (Hetzner VMs behind simulated NATs) already exists in `.github/workflows/hetzner-integration.yml` but only triggers on `push: tags v*`, so it never gates a merge.

---

## Requirements

- R1. Network-bug PR cannot reach `awaiting-verification` with only unit tests; an `*_integration_test.go` change is required when the diff touches `pkg/daemon/`, `pkg/discovery/`, or `pkg/rpc/`.
- R2. `awaiting-verification` is automatically resolved by e2e workflow conclusion, not by human comment.
- R3. Issue #556 closes automatically after the new flow ships and runs against its merge commit, or surfaces a real failure that warrants reopen.
- R4. No human comment is required on the green path.
- R5. Pulse report exposes `e2e-stalled` and `e2e-failed` open-issue counts.
- R6. Existing non-network bug fixes keep the current `awaiting-verification` → reporter-comment close path; this plan only changes the network-path branch.

---

## Scope Boundaries

- Operator-side network reproduction stays out — automating away that step is the whole point.
- General test-infra rewrite stays out — keep change surgical and gate-focused.
- The full 7-tier Hetzner run (~120 min) is not the verifier; a fast subset is extracted.
- Tier 5 (NAT Simulation) is currently `all SKIP — not yet implemented` per the workflow comment; this plan does not implement those scenarios, but the verifier subset must exercise the relay/partition paths that #556 actually hit (Tier 4 partitions + relay sub-tests in Tier 2).

### Deferred to Follow-Up Work

- Tier 5 NAT-Simulation scenarios that physically reproduce a `node1 → NAT → internet → NAT → node2` topology — separate issue, blocked on `testlab/cloud/` chaos primitives.
- Migrating *all* bug fixes (not just network) to e2e verification — out of scope; this plan keeps the existing reporter-comment path for non-network bugs.

---

## Context & Research

### Relevant Code and Patterns

- `.github/workflows/impl-merged-close.yml` — current gate trigger on `pull_request: closed`.
- `scripts/workflows/impl-merged-close-handler.js` — owns L2 (new test func) and L3 (repro keyword) gate logic. Extracted Node module with companion `*.test.js`.
- `scripts/workflows/impl-merged-close-handler.test.js` — table-driven Vitest suite; new path-detection logic must extend this file.
- `.github/workflows/verify-comment-close.yml` — current `issue_comment`-triggered closer. Not deleted; kept for non-network bugs.
- `.github/workflows/hetzner-integration.yml` — Hetzner runner; matrix over tiers 1-7. `workflow_dispatch` accepts `tiers` and `vm_count`. Already builds a single linux/arm64 binary, ships via artifact, runs each tier on its own VM prefix.
- `testlab/cloud/test-cloud.sh` — entrypoint with `--tiers` flag; verifier reuses it with a reduced tier set.
- `scripts/pulse.sh` — already counts `awaiting-verification` at line 394 via `gh issue list --label awaiting-verification`. Same shape extends to `awaiting-tests`, `e2e-failed`, `e2e-stalled`.

### Institutional Learnings

- `memory/MEMORY.md` — "Sync Board Status workflow fails on every PR/issue, token missing `read:project` scope" — be explicit about which token (`PUSH_TOKEN` vs `GITHUB_TOKEN`) the new workflow uses; reuse `PUSH_TOKEN` from `impl-merged-close.yml` for label writes.
- `memory/MEMORY.md` — "Idempotence in infrastructure operations" — the verifier label-flip step must tolerate re-runs (`gh label create … --force`, `try { removeLabel } catch`).
- Process-bug history: wgmesh#540 was auto-closed when only one of two affected paths was fixed, then re-reported 6 days later. The new gate must not regress that lesson — verification, not just merge, gates closure.

### External References

- GitHub Actions `workflow_run` event docs: triggers from another workflow's `completed` conclusion; payload exposes `workflow_run.head_sha`, `workflow_run.conclusion`, `workflow_run.pull_requests`. Used for the post-merge verifier-conclusion → label-flip handoff so the verifier and the closer stay decoupled.

---

## Key Technical Decisions

- **Verifier triggers on `pull_request: closed` (merged) with path + label filter, not on `workflow_run`.** A direct `pull_request: closed` filter on the verifier mirrors `impl-merged-close.yml` and keeps the merge-commit SHA reachable via `pull_request.merge_commit_sha`. `workflow_run` is reserved for the closer, which observes the verifier's conclusion.
- **Verifier subset = Tier 1 + Tier 2 + Tier 4 (≈30 min wall, 5 VMs each).** Tier 1 = topology, Tier 2 = peer lifecycle (relay paths), Tier 4 = partitions. Skip Tier 3 (55 min chaos), Tier 5 (SKIPs only), Tier 6 (35 min chaos monkey), Tier 7 (37 min soak). Justification: #556's reproduction is relay-flap + partition-handling, exactly what 1+2+4 exercise. The full matrix still runs on tag pushes.
- **Closer trigger = `workflow_run: completed` from `e2e-verifier`, not `issue_comment`.** A new handler reads `workflow_run.head_sha`, looks up the originating PR via `workflow_run.pull_requests[0]`, finds the linked issue from PR title `Issue #N`, then applies labels by conclusion: `success` → `verified` + close, `failure` → `e2e-failed` + reopen + ping, otherwise log and exit.
- **Stalled-watcher = scheduled cron, not in-band timeout.** A separate workflow runs every 30 min, lists issues with `awaiting-verification` older than 6h with no `verified`/`e2e-failed` label, adds `e2e-stalled`. Pulse picks it up.
- **Network-path detection lives in `impl-merged-close-handler.js`, not a separate workflow.** The handler already inspects `pulls.listFiles` for `*_test.go` content; extend it to detect `pkg/daemon/`, `pkg/discovery/`, `pkg/rpc/` touches and require an `*_integration_test.go` file in the same diff. Keeps gate logic in one place.
- **Closer keeps the existing `verify-comment-close.yml` workflow alive for non-network bugs.** Non-network bug fixes still go reporter-comment → close. Only the network-path branch becomes e2e-driven. Avoids regressing UX for shell/docs/build bug reporters who do not have a Hetzner repro.
- **`PUSH_TOKEN` (not `GITHUB_TOKEN`) for label writes.** Existing convention from `impl-merged-close.yml`. Required because `GITHUB_TOKEN` cannot trigger downstream workflows; verifier conclusion + closer chain depend on push-token writes.

---

## Open Questions

### Resolved During Planning

- *Should the verifier run on PR open or only on merge?* — Only on merge. Pre-merge runs would burn Hetzner spend on every push; PR #559's gate is already a code-review-time predicate-test gate. Verification is a post-merge concept.
- *Should the verifier subset include Tier 7 soak?* — No. 37 min soak doubles wall time without exercising the relay-flap pattern that fails the bugs we are gating. Soak stays on tag pushes.

### Deferred to Implementation

- Exact `concurrency` group for the verifier — must coexist with `hetzner-integration` group on `workflow_dispatch` runs without one cancelling the other. May need a per-PR group key.
- Exact phrasing of the autoclose comment posted on `verified` flip — copy stays close to existing `verify-comment-close.yml` template, but the `e2e-verified` provenance link replaces the human-author link.
- Whether the stalled-watcher should also reopen and ping reporters or only label — leaning label-only (pulse surfaces it; humans still own the recovery decision), but defer until the watcher fires its first alert.

---

## Implementation Units

### U1. Add network-path detection to `impl-merged-close-handler.js` (L4 gate)

**Goal:** Reject PRs that touch `pkg/daemon/`, `pkg/discovery/`, or `pkg/rpc/` and add only non-integration unit tests; require an `*_integration_test.go` file in the diff.

**Requirements:** R1.

**Dependencies:** None.

**Files:**
- Modify: `scripts/workflows/impl-merged-close-handler.js`
- Modify: `scripts/workflows/impl-merged-close-handler.test.js`
- Modify: `.github/workflows/impl-merged-close.yml` (top-of-file policy comment block reflects the new L4 gate)

**Approach:**
- Add `NETWORK_PATH_PREFIXES = ['pkg/daemon/', 'pkg/discovery/', 'pkg/rpc/']` as a module constant.
- New helper `touchesNetworkPaths(prFiles)` returns boolean.
- New helper `hasIntegrationTest(prFiles)` returns boolean iff any file matches `_integration_test.go$` and has `f.status !== 'removed'`.
- Extend the gate decision: if `touchesNetworkPaths && !hasIntegrationTest`, fail with new message `L4 — PR touches pkg/{daemon,discovery,rpc}/ and must add at least one *_integration_test.go file`.
- L4 fails alongside existing L2/L3 — same `awaiting-tests` outcome, augmented `failedGates` array.
- Export `touchesNetworkPaths`, `hasIntegrationTest`, and `NETWORK_PATH_PREFIXES` for unit tests.

**Patterns to follow:**
- Existing `detectNewTestFuncs({github, context, core, pr})` shape — receive the same `prFiles` payload via `github.paginate(github.rest.pulls.listFiles, …)`. Pass `prFiles` once into the gate decision so we don't re-paginate.
- Existing `removeLabels` + `addLabels` ordering — leave intact.
- Existing `failedGates` array assembly pattern in the L2/L3 branch.

**Test scenarios:**
- Happy path: PR diff touches `pkg/daemon/relay.go` and `pkg/daemon/relay_integration_test.go` → L4 passes, gate proceeds to L2/L3.
- Edge case: PR diff touches only `cmd/main.go` and adds `cmd/main_test.go` → L4 not applicable, gate proceeds.
- Edge case: PR diff touches `pkg/discovery/lan.go` only, with `pkg/discovery/lan_test.go` (unit, not integration) → L4 fails, `awaiting-tests` applied, message lists L4.
- Edge case: PR diff touches `pkg/rpc/server.go` and `pkg/discovery/exchange_integration_test.go` (cross-package integration test counts) → L4 passes.
- Error path: `pulls.listFiles` paginated payload empty → L4 not applicable (same as no network touch); existing L2 still fails with no-new-test message.
- Integration: full handler invocation with mocked `github` client returning network paths + only unit tests → asserts `addLabels` called with `awaiting-tests` and `createComment` body contains `L4`.

**Verification:**
- `node --test scripts/workflows/impl-merged-close-handler.test.js` (or whatever test runner the existing `.test.js` already uses) passes with new cases.
- Existing test cases still pass without modification.

### U2. New `e2e-verifier.yml` workflow

**Goal:** Run a fast Hetzner integration subset against `merge_commit_sha` whenever a `type: bug` PR touching network paths merges.

**Requirements:** R2, R3.

**Dependencies:** U1 (gate must not let predicate-only PRs through to verification anyway).

**Files:**
- Create: `.github/workflows/e2e-verifier.yml`

**Approach:**
- Trigger: `pull_request: closed`. Filter in `if:` to `merged == true && contains(labels, 'type: bug') && (one of paths-filter)`. Use `dorny/paths-filter@v3` (or inline via the `paths` directive on `pull_request`) to gate on `pkg/daemon/**`, `pkg/discovery/**`, `pkg/rpc/**`.
- Checkout `${{ github.event.pull_request.merge_commit_sha }}` so the run is pinned to the merged tree.
- Reuse the build job shape from `hetzner-integration.yml` (build linux/arm64 binary once, share via artifact).
- Tier matrix: `tiers: ['1','2','4']`.
- Reuse `testlab/cloud/test-cloud.sh --tiers 1,2,4 --vms 5`.
- `concurrency` group: `e2e-verifier-pr-${{ github.event.pull_request.number }}` so retries cancel in-flight; do not collide with `hetzner-integration` group.
- `timeout-minutes: 60` per tier (vs 120 in tag run; tier 3 not in subset, so 60 is comfortable).
- Pass through `WGMESH_RUN_ID`, `VM_PREFIX=wgmesh-vfy-pr${PR}-t${TIER}`, `WGMESH_PPROF=1` consistent with hetzner-integration.yml.
- Output the PR number to a workflow-level output so the closer can correlate (already available via `workflow_run.pull_requests`, but explicit output simplifies the closer).

**Patterns to follow:**
- `.github/workflows/hetzner-integration.yml` — phase 1 build, phase 2 tier matrix, phase 3 results aggregation.
- Existing `concurrency`/`cancel-in-progress` pattern.

**Test scenarios:**
- Happy path: PR labeled `type: bug` touching `pkg/daemon/relay.go` merges → verifier runs against merge SHA → all three tiers pass → workflow conclusion `success`.
- Edge case: PR labeled `type: bug` touching only `cmd/` → workflow does not trigger (paths filter).
- Edge case: PR labeled `type: feature` touching `pkg/daemon/` → workflow does not trigger (label filter).
- Error path: tier fails on a real chaos scenario → workflow conclusion `failure`; logs uploaded; closer (U3) handles label flip.
- Integration: `workflow_run` payload from a successful `e2e-verifier` run reaches the closer (U3) with `workflow_run.head_sha` and `workflow_run.pull_requests[0].number` populated.

Test expectation: this is workflow YAML; behavioral verification happens via U3's handler tests and a one-shot manual trigger against #556 (U7). No direct unit test of the YAML.

**Verification:**
- `actionlint .github/workflows/e2e-verifier.yml` is clean.
- A `workflow_dispatch` smoke trigger with `vm_count: 1` and `tiers: 1` completes without VM-leak (orphan cleanup runs at start and end).

### U3. New `e2e-verify-close.yml` workflow + handler

**Goal:** Observe `e2e-verifier` conclusion and apply labels/close/reopen on the linked issue.

**Requirements:** R2, R3, R4.

**Dependencies:** U2.

**Files:**
- Create: `.github/workflows/e2e-verify-close.yml`
- Create: `scripts/workflows/e2e-verify-close-handler.js`
- Create: `scripts/workflows/e2e-verify-close-handler.test.js`

**Approach:**
- Trigger: `workflow_run: completed`, filtered to `workflows: [e2e-verifier]`.
- Single job runs an `actions/github-script@v8` step that delegates to the new handler module, mirroring the `impl-merged-close.yml` → `impl-merged-close-handler.js` split.
- Handler reads:
  - `context.payload.workflow_run.conclusion` (`success` | `failure` | `cancelled` | `timed_out` | …)
  - `context.payload.workflow_run.head_sha`
  - `context.payload.workflow_run.pull_requests[0].number`
  - `context.payload.workflow_run.html_url` (artifact link surface)
- Handler resolves the linked issue via the PR title (existing pattern: `Issue #(\d+)` regex).
- Action by conclusion:
  - `success` → remove `awaiting-verification`/`awaiting-tests`/`e2e-failed`/`e2e-stalled`, add `verified`, close issue, post comment with verifier run URL.
  - `failure` → remove `awaiting-verification`, add `e2e-failed`, reopen issue if closed, post comment with run URL + artifact link.
  - other → log and exit (no label change).
- Label creation guarded by `gh label create … --force` in the workflow's setup step (idempotent; mirrors `impl-merged-close.yml`).
- Use `PUSH_TOKEN`, not `GITHUB_TOKEN`.

**Patterns to follow:**
- `.github/workflows/impl-merged-close.yml` — label-create idempotence, handler-delegation, github-script invocation.
- `scripts/workflows/impl-merged-close-handler.js` — module shape (`async function handler({github, context, core})`), exported helpers for unit testing, repro of `removeLabels` helper.

**Test scenarios:**
- Happy path: `workflow_run.conclusion === 'success'`, PR title `impl: Issue #556 - …` → asserts `addLabels(['verified'])`, `update({state: 'closed'})`, `removeLabel('awaiting-verification')`, `createComment` body contains run URL.
- Edge case: PR title lacks `Issue #N` → handler logs and returns without API writes.
- Edge case: PR was for an issue that is already closed and `verified` → handler is idempotent (re-applies same label set without throwing).
- Error path: `workflow_run.conclusion === 'failure'` → asserts `addLabels(['e2e-failed'])`, `update({state: 'open'})` if previously closed, comment includes artifact-link URL.
- Error path: `workflow_run.conclusion === 'cancelled'` → handler logs and exits without label changes.
- Integration: simulated `workflow_run` payload with empty `pull_requests` array → handler logs and exits cleanly.

**Verification:**
- New `*-handler.test.js` passes with the same runner as `impl-merged-close-handler.test.js`.
- `actionlint` clean on the new workflow.

### U4. Stalled-verification watcher

**Goal:** Surface PRs/issues stuck in `awaiting-verification` past a 6h budget with no verifier conclusion.

**Requirements:** R5.

**Dependencies:** U3 (label vocabulary established).

**Files:**
- Create: `.github/workflows/e2e-stalled-watcher.yml`
- Create: `scripts/workflows/e2e-stalled-watcher.js`
- Create: `scripts/workflows/e2e-stalled-watcher.test.js`

**Approach:**
- Trigger: `schedule: cron '*/30 * * * *'` plus `workflow_dispatch`.
- Handler queries `gh issue list --label awaiting-verification --state open --json number,labels,updatedAt`.
- For each issue: if `now - updatedAt > 6h` and labels do not include `e2e-failed` or `e2e-stalled` or `verified`, add `e2e-stalled` label. Idempotent.
- Surface count to `$GITHUB_STEP_SUMMARY` for visibility.

**Patterns to follow:**
- Cron-triggered workflow shape from `.github/workflows/pulse.yml`.
- Idempotent label add (try/catch wrap on already-applied label).

**Test scenarios:**
- Happy path: issue with `awaiting-verification` label and `updatedAt` 7h ago → handler adds `e2e-stalled`.
- Edge case: issue with `awaiting-verification` and `e2e-failed` already applied → no-op.
- Edge case: issue with `awaiting-verification` and `updatedAt` 5h ago → no-op.
- Edge case: issue with `awaiting-verification` and `verified` (race window) → no-op.
- Integration: list of 3 issues, two stalled and one fresh → exactly two `addLabels` calls.

**Verification:**
- Handler unit tests pass.
- One real cron firing in dry-run mode (logs intended actions without writes) shows expected detection.

### U5. Pulse counts for awaiting-tests, e2e-failed, e2e-stalled

**Goal:** Pulse report exposes the new label counts so operators can see the verification pipeline state at a glance.

**Requirements:** R5.

**Dependencies:** U3, U4 (label vocabulary established).

**Files:**
- Modify: `scripts/pulse.sh`

**Approach:**
- Around line 394, where `AWAITING_VERIFICATION_OPEN` is computed, add three parallel `gh issue list --label <name> --json number --jq 'length'` calls populating `AWAITING_TESTS_OPEN`, `E2E_FAILED_OPEN`, `E2E_STALLED_OPEN`.
- Render in the report block alongside `awaiting-verification`. Keep the existing rendering shape; just append three lines.

**Patterns to follow:**
- Existing `awaiting-verification` block at `scripts/pulse.sh:394`.

**Test scenarios:**
- Happy path: pulse run with one issue per label → all four counts render correctly.
- Edge case: zero issues for a given label → renders `0`, not blank.
- Error path: `gh` call transient failure for one label → corresponding count stays empty (existing pattern), other counts unaffected.

Test expectation: shell script — covered by a one-shot manual `bash scripts/pulse.sh` run in U7's backfill smoke.

**Verification:**
- Local `bash scripts/pulse.sh` produces a report with the four label counts visible.

### U6. Document network-bug verification policy

**Goal:** Update agent-facing instructions so Goose and Copilot generate integration tests for network-path bug fixes upfront, not as a post-merge fixup.

**Requirements:** R1.

**Dependencies:** U1.

**Files:**
- Modify: `.github/copilot-instructions.md`
- Modify: `.github/AGENTS.md` (if it exists and has a similar policy section)

**Approach:**
- Add a "Network-path bug fixes" section describing the L4 gate: any PR touching `pkg/daemon/`, `pkg/discovery/`, or `pkg/rpc/` and labeled `type: bug` must add an `*_integration_test.go` file.
- Cross-reference `.github/workflows/impl-merged-close.yml` policy comment.
- Keep the existing Spec-Only Triage Mode and CI/CD sections untouched.

**Patterns to follow:**
- Existing section structure of `copilot-instructions.md` (no formal heading scheme — just narrative blocks).

**Test scenarios:**
None — pure documentation. Test expectation: none — no behavioral change.

**Verification:**
- Markdown renders cleanly on GitHub (preview).
- A subsequent Goose-driven bug-fix PR for a fake network bug surfaces the integration-test requirement in its PR description.

### U7. Backfill verifier against #556

**Goal:** Validate the end-to-end flow against the originating bug. After ship, manually `workflow_dispatch` the verifier on PR #564's merge commit and observe the closer's behavior on issue #556.

**Requirements:** R3.

**Dependencies:** U1 through U5.

**Files:**
- None new. Uses `workflow_dispatch` UI on `e2e-verifier.yml`.

**Approach:**
- Add a `workflow_dispatch` input `merge_commit_sha` (string, optional) to `e2e-verifier.yml`. When set, checkout that ref instead of `pull_request.merge_commit_sha`. Otherwise fall back to the PR-derived SHA.
- Add a `workflow_dispatch` input `pr_number` (string, optional) so the closer can correlate when triggered manually.
- After ship, dispatch with `merge_commit_sha = <PR #564 merge SHA>`, `pr_number = 564`, observe closer behavior on #556.

**Patterns to follow:**
- Existing `inputs` block on `hetzner-integration.yml`.

**Test scenarios:**
- Happy path: dispatch with #564's merge SHA → verifier conclusion `success` → closer adds `verified` to #556, removes `awaiting-verification`, closes #556 with autoclose comment linking the run.
- Error path: dispatch with #564's merge SHA → verifier fails (NAT topology actually broken) → closer adds `e2e-failed` to #556, posts artifact link. This is the real diagnostic the user asked for: "I do not know if it works."
- Edge case: dispatch with no `pr_number` → closer logs and exits without label changes (PR-correlation is required for the close path).

Test expectation: behavioral verification happens via the live dispatch. No new unit tests.

**Verification:**
- One run of either outcome, recorded in pulse the next morning, closes the loop on #568's acceptance criterion 3.

---

## System-Wide Impact

- **Interaction graph:** `impl-merged-close.yml` (gate) → either `awaiting-tests` (block) or `awaiting-verification` (open). For network bugs, `e2e-verifier.yml` runs in parallel post-merge → `e2e-verify-close.yml` consumes its conclusion → `verified` close OR `e2e-failed` reopen. `e2e-stalled-watcher.yml` separately catches no-conclusion stalls. `pulse.yml` reads label counts every morning.
- **Error propagation:** Verifier failure does not crash the closer; closer exits cleanly on `cancelled`/`timed_out`. Label-write failures are caught and logged (existing `removeLabels` pattern). The reporter-comment path in `verify-comment-close.yml` stays operational for non-network bugs and as a manual override.
- **State lifecycle risks:** Race between verifier and reporter-comment closer if the reporter types `verified` while the verifier is mid-run. Mitigation: the new closer is idempotent on `verified` label add (label create + `update({state: 'closed'})` are both already idempotent in the existing `verify-comment-close.yml` shape).
- **API surface parity:** No CLI/API changes. All new surface is GitHub Actions workflows, gh-token-scoped writes, and label additions.
- **Integration coverage:** The verifier itself *is* the integration coverage. Unit tests for U1, U3, U4 cover gate logic and handler logic; live workflow dispatch in U7 covers the end-to-end chain.
- **Unchanged invariants:** `verify-comment-close.yml` continues to handle non-network bug closes via reporter comment. `impl-merged-close.yml`'s L2/L3 gates are unchanged; only the new L4 is added. `hetzner-integration.yml` tag-push run is unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Verifier consumes Hetzner spend on every network-bug merge | Subset to ~30 min wall (tiers 1, 2, 4 only); `concurrency` group cancels stale runs; `cleanup-orphans` step + safety-net teardown identical to existing pattern. |
| `workflow_run` event drops `pull_requests[0]` when the PR base is from a fork | Closer falls back to scanning the workflow run's commit message for `Issue #N` if `pull_requests` is empty. Documented in handler. |
| `PUSH_TOKEN` lacks `read:project` (per memory: Sync Board Status precedent) | Use only `issues: write` and `pull-requests: read`; do not touch project boards. |
| Tier 3/6/7 partial coverage gap — soak/chaos bugs slip past verifier | Tag-push run still gates releases on full matrix; this plan only changes per-merge verification, not release gating. |
| L4 false positive on legitimate non-test PRs that touch `pkg/{daemon,discovery,rpc}` (e.g., refactor) | L4 only fires when issue carries `type: bug` label. Refactor PRs without bug labels skip L4 entirely. |
| Verifier flakiness reopens issues spuriously | `e2e-failed` label flip is reversible (re-running the verifier on the same SHA flips back to `verified` on success). Reporter can also still type `verified` on the legacy `verify-comment-close.yml` path. |

---

## Documentation / Operational Notes

- After ship, dispatch verifier against PR #564 merge SHA per U7 to validate end-to-end behavior on issue #556.
- Update `memory/MEMORY.md` once the feature ships with a one-line entry under "CI/workflow gaps" pointing to the new `e2e-verifier`/`e2e-verify-close` pair as the post-merge bug-fix verification path. (Out of scope for the implementation PR; tracked via this plan.)
- Coroot dashboard `table.beerpub.dev` is not affected; this is a GitHub-only flow.

---

## Sources & References

- Origin issue: [atvirokodosprendimai/wgmesh#568](https://github.com/atvirokodosprendimai/wgmesh/issues/568)
- Closed-but-stuck instance: [atvirokodosprendimai/wgmesh#556](https://github.com/atvirokodosprendimai/wgmesh/issues/556)
- Predicate-only fix PR: [atvirokodosprendimai/wgmesh#564](https://github.com/atvirokodosprendimai/wgmesh/pull/564)
- Predecessor gate PR: [atvirokodosprendimai/wgmesh#559](https://github.com/atvirokodosprendimai/wgmesh/pull/559)
- Existing files: `.github/workflows/impl-merged-close.yml`, `.github/workflows/verify-comment-close.yml`, `.github/workflows/hetzner-integration.yml`, `scripts/workflows/impl-merged-close-handler.js`, `scripts/pulse.sh:394`.
