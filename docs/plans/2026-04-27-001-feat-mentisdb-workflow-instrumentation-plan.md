---
title: "feat: instrument 16 wgmesh GitHub Actions workflows with MentisDB thought-append"
type: feat
status: active
date: 2026-04-27
---

# feat: instrument 16 wgmesh GitHub Actions workflows with MentisDB thought-append

## Overview

Phase 1 of the multi-repo MentisDB CI rollout. Wire 16 lifecycle workflows in `atvirokodosprendimai/wgmesh` to append structured thoughts to MentisDB at `mem.beerpub.dev`, mirroring the instrumentation just shipped in the sister repo `ai-pipeline-template` (PR #610 + heal #611). Three workflows already integrate (`agent-metrics-report.yml` weekly Summary, `release.yml` TaskComplete/Mistake on tag publish, `goose-build.yml` ActionTaken/Mistake on each build). The remaining 16 lifecycle workflows run silently — there is no agent-memory record of PR triage, spec validation, build dispatch, merge closure, infrastructure sync, or pages deployment.

After this work, every meaningful CI event in wgmesh writes a typed thought into the `wgmesh` chain, restoring continuity for downstream agents that read MentisDB to reason about the product's pipeline state.

---

## Problem Frame

wgmesh is the primary product repo per `ai-pipeline-template/.github/workflows/observation-loop.yml` (it is the one repo that gets full GitHub-signal collection + Product Codebase Summary in the daily strategic loop). Yet 16 of its 20 lifecycle workflows produce no MentisDB record beyond `gh run` logs that age out and lack semantic structure.

Memory note `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/reference_mentisdb_ci_integration_pattern.md` already defines the canonical step shape, thought-type table, fatality decision rules, and chain naming convention. This plan is a mechanical rollout of that pattern across the remaining wgmesh workflows; no architectural decisions are open. The sister repo's PR #610 ships exactly the same pattern across an equivalent set of workflows (10 there → 16 here), so the implementer has a verified precedent to mirror.

Skip `sync-labels.yml` per pattern-doc convention (mirrors the ai-pipeline-template skip). Label-sync events are mechanical and high-frequency — they would flood the chain with noise.

---

## Requirements Trace

- R1. Every lifecycle workflow in scope appends a thought to MentisDB on terminal job state (`success`, `failure`, `cancelled`), via `if: always()` so the append fires regardless of preceding step outcomes.
- R2. All appends use `chain_key: "wgmesh"` to keep this repo's CI activity in one queryable chain (per pattern doc — one chain per repo).
- R3. Append failures are non-fatal across all 16 workflows — `|| echo "::warning::mentisdb append failed (non-fatal)"`. Workflow primary outcome (build pass/fail, merge success, etc.) is unaffected by MentisDB hiccups.
- R4. `thought_type`, `importance`, `tags`, and `content` are chosen per the pattern doc table for each workflow's event class. Specifically: `Correction` is reserved for "successful auto-fix from review with refs/relations to a Mistake thought" — do **not** use it for plain spec-validation success (use `ActionTaken` instead, per the lesson learned in ai-pipeline-template PR #610 review).
- R5. No new secrets are introduced. Reuse existing org-level `MENTISDB_URL` / `MENTISDB_USER` / `MENTISDB_PASSWORD` — already provisioned for wgmesh per the org secrets inventory (visibility: selected → ai-pipeline-template + wgmesh).
- R6. Existing wired workflows (`agent-metrics-report.yml`, `release.yml`, `goose-build.yml`) are not modified.
- R7. `sync-labels.yml` is not instrumented (low-signal noise per pattern doc convention).
- R8. Post-merge verification: at least one instrumented workflow per cluster fires (via natural trigger or `gh workflow run`), and the resulting thought lands in MentisDB chain `wgmesh` with the run_url matching the dispatched run.

---

## Scope Boundaries

- Not modifying the three already-wired workflows (`agent-metrics-report`, `release`, `goose-build`).
- Not instrumenting `sync-labels.yml` (low signal, high frequency).
- Not changing the existing pattern (chain naming, thought_type table, step shape) — this plan consumes it.
- Not adding new MentisDB secrets, rotating existing ones, or changing org secret visibility.
- Not extracting a reusable composite action — pattern doc + ai-pipeline-template precedent both keep inline. Composite extraction is a separate concern, deferred to its own future plan once the rollout count and shape stabilizes.
- Not instrumenting any additional secondary repo (lighthouse, chimney, coroot-cicd, etc.) — those are separate phases requiring secret-visibility widening first.

### Deferred to Follow-Up Work

- Composite-action extraction (`.github/actions/mentis-thought/`) — defer until 13+ workflows in any single repo OR a breaking MentisDB API change forces a sweep. Coordinate with sister repos when undertaken.
- Phase 2 secondary-repo onboarding (coroot-cicd, chimney, tvcentras) — separate plan, requires `gh secret set ... --visibility selected --repos` widening first.
- `sync-labels.yml` instrumentation — revisit if labels-sync events are ever judged worth memorializing (default: not).

---

## Context & Research

### Relevant Code and Patterns

- `.github/workflows/agent-metrics-report.yml` — wgmesh-side reference impl (one of 3 already wired). Mirror its env-block + jq + curl shape.
- `.github/workflows/release.yml` — release-event reference (TaskComplete on success, Mistake on failure, fatal append for tag publish).
- `.github/workflows/goose-build.yml` — high-frequency reference (ActionTaken on success, non-fatal append).
- Sister-repo plan: `ai-pipeline-template/docs/plans/2026-04-27-001-feat-mentisdb-workflow-instrumentation-plan.md` — same shape for 10 workflows, structurally identical to this plan's 16.
- Sister-repo PR #610 commit `a612392` — squash-merged 10-workflow instrumentation. Diff is the canonical "what good looks like" for this work.
- Sister-repo heal commit `190844e` (PR #611) — unrelated terraform fix; not relevant to this plan but documents a Hetzner gotcha worth knowing for the workflows that touch hetzner-integration.

### Institutional Learnings

- Pattern doc (authoritative): `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/reference_mentisdb_ci_integration_pattern.md` — step shape, thought-type table, fatality rules, chain naming convention, onboarding checklist.
- `ai-pipeline-template/docs/solutions/integration-issues/github-workflow-placeholder-validation-failures.md` — `secrets` context is **not** allowed in `if:` expressions; always use `env:` block. This plan correctly uses `env:` only.
- `ai-pipeline-template/docs/solutions/integration-issues/github-app-reviews-dont-trigger-workflows.md` — App-actor reviews via GITHUB_TOKEN do not fire `pull_request_review` workflows. Affects `approve-build.yml` here (same caveat as ai-pipeline-template's). Surface as advisory, not as a defect introduced by this plan.
- `ai-pipeline-template/docs/solutions/runtime-errors/coroot-disk-full-outage-recovery.md` — `if: always()` is the only reliable execution guarantee when prior steps fail. This plan uses it on every step.
- `ai-pipeline-template/docs/solutions/integration-issues/autonomous-review-merge-bootstrap.md` — log-cleanliness patterns for curl/gh in workflows. Mirror the silent-but-show-error curl pattern.
- Memory note `feedback_mentisdb_no_rest_auth.md` — REST has zero built-in auth; nginx Basic Auth is the gate. Step shape must include `-u "$MENTISDB_USER:$MENTISDB_PASSWORD"`.

### External References

- None required — pattern is fully internal and battle-tested in 3 wgmesh workflows + 11 ai-pipeline-template workflows + ai-pipeline-template's heartbeat/observation-loop production runs.

---

## Key Technical Decisions

- **Per-workflow inline step over reusable composite action.** Same rationale as the sister-repo plan: pattern doc + multi-repo precedent both use inline steps. Inline keeps per-workflow content/tag choices visible at the point of use. Composite extraction deferred (see Scope Boundaries).
- **Non-fatal append for all 16 workflows.** None are release events or low-frequency metrics rollups (those are already wired and use fatal append). This batch is operational lifecycle events where the build/merge outcome is the primary signal and MentisDB observability is supplementary. Append: `|| echo "::warning::mentisdb append failed (non-fatal)"`.
- **`if: always()` placement.** Append step must run regardless of preceding step outcomes so failure thoughts get recorded. Place as the last step in each job. Explicit exception: none in this batch (no equivalent of ai-pipeline-template's `health-check.yml` failure-only gate).
- **Single chain `wgmesh` for all 16 workflows.** Pattern doc: one chain per repo. Workflow identity is preserved via `agent_id` (workflow filename without extension) and tags.
- **`agent_id` / `agent_name` convention.** Use the workflow's filename minus `.yml` extension (e.g., `bot-pr-review-merge`, `dns-update`, `pages`). Matches existing `agent-metrics-report` / `release` / `goose-build` convention.
- **Tag schema:** `["wgmesh", "<event-type>", $outcome]`. `<event-type>` is the action-flavored noun the workflow performs, NOT necessarily the agent_id. Per ai-pipeline-template review: agent_id ≠ tag is intentional design (tag describes the event semantic; agent_id describes the workflow identity).
- **Importance values from pattern doc table.** Do not invent new importance scores. ActionTaken=0.5, TaskComplete=0.7, Mistake=0.7-0.8, Correction=0.6 (rare), Insight=0.4.
- **`Correction` is reserved.** Per pattern doc and ai-pipeline-template review: only use TYPE=Correction when the workflow literally fixes a prior Mistake thought (with refs/relations to it). For all 16 workflows in this plan, success cases use ActionTaken (or TaskComplete for terminal merges/builds). No workflow in this batch uses Correction.

---

## Open Questions

### Resolved During Planning

- **Q: Build a reusable composite action?** No. Same answer as ai-pipeline-template plan — pattern doc + multi-repo precedent both use inline. Composite extraction deferred to a separate plan.
- **Q: Should `auto-merge.yml` (general PR auto-merge) and `bot-pr-review-merge.yml` both append?** Yes. They handle different actor classes (`auto-merge` is general; `bot-pr-review-merge` is bot-authored). Both events warrant chain entries with distinct tags (`auto-merge` vs `pr-review-merge`).
- **Q: Should `goose-review.yml` use TYPE=Correction on success?** No. Use ActionTaken. Goose review is a quality gate that produces a review verdict, not an auto-fix. Correction is only for workflows that literally fix a prior Mistake.
- **Q: Should `spec-auto-approve.yml` use Correction?** No. Use ActionTaken on success. This is the lesson directly carried from ai-pipeline-template PR #610 review (where `spec-validation.yml` was initially set to Correction and had to be heal-fixed to ActionTaken). Pre-empt the same mistake here.
- **Q: Cross-repo chain references?** No. Each repo's chain is self-contained. Cross-repo correlation happens at query time (`tags_any=["release"]` across multiple chains), not via chain links.

### Deferred to Implementation

- **Tag choice per workflow:** The pattern doc gives the structure. Exact tag strings are best chosen at the point of edit so they reflect the workflow's actual semantics. Implementer should mirror the wording used in the workflow's existing job/step names.
- **Whether to include PR number, issue number, branch name, or commit SHA in `content`:** Decide per workflow based on what makes the search result useful. Default: include the GitHub run URL, plus the most relevant identifier from the trigger event.
- **`pages.yml` content shape:** GitHub Pages deployment may not produce a meaningful "content" string from event context. Use a generic "Pages deploy <outcome>" if no better identifier is available; supplement with the deployed URL if the workflow exposes it as an output.

---

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification. The shape is identical to the sister-repo plan; only chain_key and per-workflow specifics vary.*

Each instrumented workflow gains exactly one new step at the end of its existing job (or each job, when there are multiple parallel jobs whose outcomes both matter). The shape is constant; the contents vary:

```yaml
- name: Append <event> to MentisDB
  if: always()
  env:
    MENTISDB_URL: ${{ secrets.MENTISDB_URL }}
    MENTISDB_USER: ${{ secrets.MENTISDB_USER }}
    MENTISDB_PASSWORD: ${{ secrets.MENTISDB_PASSWORD }}
    OUTCOME: ${{ job.status }}
    RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
    # plus event-context vars: PR_NUMBER, PR_TITLE, ISSUE_NUMBER, ISSUE_LABEL, etc.
  run: |
    set -euo pipefail
    case "$OUTCOME" in
      success)   TYPE="<success-type>"; IMP="<success-imp>" ;;
      failure)   TYPE="Mistake";        IMP="0.8"           ;;
      cancelled) TYPE="Mistake";        IMP="0.6"           ;;
      *)         TYPE="Mistake";        IMP="0.7"           ;;
    esac
    PAYLOAD=$(jq -nc \
      --arg type "$TYPE" \
      --arg outcome "$OUTCOME" \
      --arg run_url "$RUN_URL" \
      --arg imp "$IMP" \
      '{
        chain_key: "wgmesh",
        agent_id: "<workflow-id>",
        agent_name: "<workflow-id>",
        thought_type: $type,
        content: ("<event description with identifiers> — " + $outcome + " (" + $run_url + ")"),
        tags: ["wgmesh", "<event-type>", $outcome],
        importance: ($imp | tonumber)
      }')
    curl --fail-with-body --silent --show-error --max-time 15 \
      -u "$MENTISDB_USER:$MENTISDB_PASSWORD" \
      -X POST -H 'Content-Type: application/json' \
      -d "$PAYLOAD" "$MENTISDB_URL/v1/thoughts" \
      || echo "::warning::mentisdb append failed (non-fatal)"
```

Per-workflow variation lives only in: `<event>` (step name), `<workflow-id>` (filename stem), `<success-type>` / `<success-imp>` (per pattern table), `<event description>`, `<event-type>` tag string, identifiers in content.

---

## Implementation Units

- U1. **PR lifecycle workflows (3 files)**

**Goal:** Instrument the three workflows that handle PR lifecycle (open → review/merge → undraft) so each PR's outcome is recorded.

**Requirements:** R1, R2, R3, R4, R5, R6

**Dependencies:** None (org secrets already provisioned).

**Files:**
- Modify: `.github/workflows/bot-pr-review-merge.yml`
- Modify: `.github/workflows/copilot-undraft.yml`
- Modify: `.github/workflows/auto-merge.yml`

**Approach:**
- For `bot-pr-review-merge.yml`: append at end of the review-merge job. `agent_id="bot-pr-review-merge"`. Success → `TaskComplete` (imp 0.7), failure → `Mistake` (imp 0.8). Content includes PR number + author. Tags: `["wgmesh", "pr-review-merge", $outcome]`.
- For `copilot-undraft.yml`: append at end of the undraft job. `agent_id="copilot-undraft"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). Content includes PR number. Tags: `["wgmesh", "spec-undraft", $outcome]`.
- For `auto-merge.yml`: append at end of the auto-merge job. `agent_id="auto-merge"`. Success → `TaskComplete` (imp 0.7), failure → `Mistake` (imp 0.8). Content includes PR number + title. Tags: `["wgmesh", "auto-merge", $outcome]`.
- All three: non-fatal append.

**Patterns to follow:**
- `.github/workflows/agent-metrics-report.yml` and `.github/workflows/release.yml` — env block, jq payload, curl invocation.
- Sister-repo PR #610 commit `a612392` — same pattern applied to the equivalent ai-pipeline-template workflows.

**Test scenarios:**
- Happy path: real bot PR opens → `bot-pr-review-merge` fires → `TaskComplete` thought lands with PR# in content (verifiable by querying chain with `tags_any=["pr-review-merge"]`).
- Happy path: Copilot draft PR opens → `copilot-undraft` fires → `ActionTaken` thought lands.
- Happy path: PR ready for auto-merge → `auto-merge` fires → `TaskComplete` thought lands.
- Edge case: workflow's `if:` gate skips the job (e.g., bot-pr-review-merge filter excludes the PR's author) — append step still runs (`if: always()`) and records the gate-out as success of an empty job. Acceptable.
- Error path: simulate MentisDB unavailability via `gh workflow run` against a test branch with bad URL — confirm workflow exits 0 with `::warning::` in logs.
- Integration: full PR lifecycle (bot opens → undraft → review-merge → merge) → all three thoughts present in chain with consistent timing.

**Verification:**
- All three workflows pass `actionlint` post-edit.
- After merge, the next bot PR triggers each workflow and corresponding thoughts land in the chain.

---

- U2. **Spec workflow cluster (4 files)**

**Goal:** Instrument the four workflows that drive the spec → approve → build pipeline (parallel to ai-pipeline-template's spec/build cluster, but with wgmesh-specific extras `spec-command` for slash-command handling and `spec-auto-approve` for auto-approval logic).

**Requirements:** R1, R2, R3, R4, R5, R6

**Dependencies:** None.

**Files:**
- Modify: `.github/workflows/approve-build.yml`
- Modify: `.github/workflows/spec-auto-approve.yml`
- Modify: `.github/workflows/spec-command.yml`
- Modify: `.github/workflows/spec-merged-build.yml`

**Approach:**
- For `approve-build.yml`: `agent_id="approve-build"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). Content includes PR # + reviewer + review state. Tags: `["wgmesh", "spec-approval", $outcome]`. **Caveat:** trigger is `pull_request_review` — Copilot/App reviews via GITHUB_TOKEN won't fire it (per `github-app-reviews-dont-trigger-workflows.md`). Document in residual notes; not a defect introduced by this plan.
- For `spec-auto-approve.yml`: `agent_id="spec-auto-approve"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). **Do NOT use Correction** — pre-empt the lesson from ai-pipeline-template PR #610 (where spec-validation was initially mis-set to Correction). Content includes PR # + auto-approve decision. Tags: `["wgmesh", "spec-auto-approve", $outcome]`.
- For `spec-command.yml`: `agent_id="spec-command"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). Content includes the slash-command invoked + issue/PR #. Tags: `["wgmesh", "spec-command", $outcome]`.
- For `spec-merged-build.yml`: `agent_id="spec-merged-build"`. Success → `ActionTaken` (imp 0.5; the actual build happens elsewhere — goose-build is already wired), failure → `Mistake` (imp 0.7). Content includes spec PR # + dispatched build target. Tags: `["wgmesh", "spec-merge-build", $outcome]`.
- All four: non-fatal append.

**Patterns to follow:**
- Same as U1.

**Test scenarios:**
- Happy path: spec PR opened with slash-command → `spec-command` runs → `ActionTaken` lands.
- Happy path: spec PR validated → `spec-auto-approve` runs → `ActionTaken` lands (NOT Correction — verify thought_type explicitly).
- Happy path: spec PR approved (human review) → `approve-build` runs → `ActionTaken` lands.
- Happy path: spec PR merged → `spec-merged-build` dispatches downstream → `ActionTaken` lands.
- Edge case: `approve-build` triggered by Copilot review — silently skipped per App-actor limitation. Append step doesn't fire because workflow doesn't fire. Document in residual.
- Error path: spec validation chain fails at any stage → `Mistake` thought lands at the failing workflow; main workflow surfaces failure normally.
- Integration: full spec lifecycle → all four thoughts present in chain with correct outcome ordering.

**Verification:**
- All four workflows pass actionlint.
- A spec PR run produces 4 thoughts (one per workflow in the spec cluster) tagged correctly.
- spec-auto-approve thought has `thought_type="ActionTaken"` (NOT Correction) — verifiable via direct chain query.

---

- U3. **Issue lifecycle workflows (3 files)**

**Goal:** Instrument the three workflows that handle issue triage, impl-PR closure, and issue housekeeping so the issue → spec → impl → close arc is recorded end-to-end.

**Requirements:** R1, R2, R3, R4, R5, R6

**Dependencies:** None.

**Files:**
- Modify: `.github/workflows/copilot-triage.yml`
- Modify: `.github/workflows/impl-merged-close.yml`
- Modify: `.github/workflows/close-resolved-issues.yml`

**Approach:**
- For `copilot-triage.yml`: `agent_id="copilot-triage"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). Content includes issue # + triggering label. Tags: `["wgmesh", "issue-triage", $outcome]`.
- For `impl-merged-close.yml`: `agent_id="impl-merged-close"`. Success → `TaskComplete` (imp 0.7) — closing an impl PR completes a unit of work, failure → `Mistake` (imp 0.7). Content includes PR # + linked issue # (extracted from PR title via grep `Issue #N`). Tags: `["wgmesh", "impl-close", $outcome]`.
- For `close-resolved-issues.yml`: `agent_id="close-resolved-issues"`. Success → `ActionTaken` (imp 0.5; this is housekeeping, not a "task complete" event), failure → `Mistake` (imp 0.7). Content includes count of issues closed (if exposed as job output) or just outcome. Tags: `["wgmesh", "issue-housekeeping", $outcome]`.
- All three: non-fatal append.

**Patterns to follow:**
- Same as U1.
- For `impl-merged-close.yml` ISSUE_NUM extraction: copy the pipefail-safe pattern from ai-pipeline-template's `impl-merged-close.yml`:
  ```bash
  ISSUE_NUM=$(echo "$PR_TITLE" | grep -oE 'Issue #[0-9]+' | grep -oE '[0-9]+' | head -1 || echo "unknown")
  ```
  Verified correct under `set -euo pipefail` per the sister-repo PR #610 review (correctness + reliability + testing reviewers all confirmed).

**Test scenarios:**
- Happy path: file an issue with `bug` label → `copilot-triage` fires → `ActionTaken` thought lands with issue # in content.
- Happy path: bot opens impl PR → human merges → `impl-merged-close` fires → `TaskComplete` thought lands.
- Happy path: scheduled cleanup runs → `close-resolved-issues` fires → `ActionTaken` thought lands with summary.
- Edge case: PR title without `Issue #N` → ISSUE_NUM extraction returns `"unknown"`, payload still validates and lands.
- Edge case: triage workflow's `if:` gate skips the job (e.g., issue already has `copilot-triaging` label) — `if: always()` still runs the append, recording a no-op run as success.
- Error path: `gh` token expired → workflow fails → `Mistake` thought lands; original failure surfaces in logs.
- Integration: full issue → triage → spec → build → impl → close cycle on a test issue → all U3 thoughts present plus the U2 spec-pipeline thoughts.

**Verification:**
- All three workflows pass actionlint.
- A real issue lifecycle produces all three thoughts tagged correctly.

---

- U4. **Build / deploy workflows (3 files)**

**Goal:** Instrument the three workflows that build/deploy artifacts (container, pages, DNS) so deploy-side observability has a chain entry per dispatch.

**Requirements:** R1, R2, R3, R4, R5, R6

**Dependencies:** None.

**Files:**
- Modify: `.github/workflows/docker-build.yml`
- Modify: `.github/workflows/pages.yml`
- Modify: `.github/workflows/dns-update.yml`

**Approach:**
- For `docker-build.yml`: `agent_id="docker-build"`. Success → `TaskComplete` (imp 0.7) — image successfully built/pushed, failure → `Mistake` (imp 0.8). Content includes commit SHA + image tag/digest if exposed, plus build duration if available. Tags: `["wgmesh", "docker-build", $outcome]`.
- For `pages.yml`: `agent_id="pages"`. Success → `TaskComplete` (imp 0.6) — pages successfully deployed, failure → `Mistake` (imp 0.7). Content includes deploy URL + commit SHA. Tags: `["wgmesh", "pages-deploy", $outcome]`.
- For `dns-update.yml`: `agent_id="dns-update"`. Success → `ActionTaken` (imp 0.5; DNS sync is incremental, not a release), failure → `Mistake` (imp 0.7). Content includes record count or zone updated. Tags: `["wgmesh", "dns-update", $outcome]`.
- All three: non-fatal append.

**Patterns to follow:**
- `.github/workflows/release.yml` — release-event reference for build/deploy idiom (TaskComplete on success, fatal append for tag publish — but adjust to non-fatal for these higher-frequency events).

**Test scenarios:**
- Happy path: push to main → `docker-build` runs → `TaskComplete` thought lands with image tag in content.
- Happy path: docs change to main → `pages` runs → `TaskComplete` thought lands with deploy URL in content.
- Happy path: DNS config change → `dns-update` runs → `ActionTaken` thought lands.
- Error path: docker build fails (registry auth, network) → `Mistake` thought lands; CI status surfaces failure.
- Edge case: workflow has multiple parallel jobs (e.g., docker-build for amd64+arm64) — append step in EACH job. Each emits its own thought. Acceptable: queries with `tags_any=["docker-build"]` get one thought per arch per build.
- Integration: a single push to main may trigger multiple of these workflows simultaneously — confirm chain receives one thought per workflow per dispatch.

**Verification:**
- All three workflows pass actionlint.
- The next push to main triggers each workflow and produces the expected thought.

---

- U5. **Infra / board / quality workflows (3 files)**

**Goal:** Instrument the three workflows that manage infrastructure sync, project board sync, and goose review (pre-build quality gate).

**Requirements:** R1, R2, R3, R4, R5, R6

**Dependencies:** None.

**Files:**
- Modify: `.github/workflows/hetzner-integration.yml`
- Modify: `.github/workflows/board-sync.yml`
- Modify: `.github/workflows/goose-review.yml`

**Approach:**
- For `hetzner-integration.yml`: `agent_id="hetzner-integration"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.8) — Hetzner failures matter (per recent PR #611 hcloud-volume gotcha, infra plan failures should be loud in the chain). Content includes resource type / change summary if exposed. Tags: `["wgmesh", "hetzner-sync", $outcome]`.
- For `board-sync.yml`: `agent_id="board-sync"`. Success → `ActionTaken` (imp 0.5), failure → `Mistake` (imp 0.7). Content includes count of items synced (if exposed) or just outcome. Tags: `["wgmesh", "board-sync", $outcome]`.
- For `goose-review.yml`: `agent_id="goose-review"`. Success → `ActionTaken` (imp 0.5; review verdict produced), failure → `Mistake` (imp 0.7). **Do NOT use Correction** (review pass produces a verdict, doesn't fix a Mistake). Content includes PR # + review summary if exposed. Tags: `["wgmesh", "goose-review", $outcome]`.
- All three: non-fatal append.

**Patterns to follow:**
- Same as U1.

**Test scenarios:**
- Happy path: scheduled hetzner-integration run → `ActionTaken` thought lands.
- Happy path: PR opened → `board-sync` runs → `ActionTaken` thought lands.
- Happy path: spec PR opened → `goose-review` runs → `ActionTaken` thought lands.
- Error path: hetzner-integration fails (Hetzner API down, terraform error) → `Mistake` thought lands with imp 0.8 (high importance — infra failures are loud).
- Edge case: goose-review timeout or empty verdict → workflow may succeed-with-warnings; thought_type still ActionTaken; content reflects the situation.
- Integration: a spec PR triggers `goose-review` + `spec-command` + `spec-auto-approve` + `approve-build` (potentially) — confirm all four thoughts present and tagged distinctly.

**Verification:**
- All three workflows pass actionlint.
- Next natural trigger of each produces an expected thought.

---

- U6. **Post-merge verification dispatch + chain query**

**Goal:** Confirm the rollout works end-to-end by dispatching one workflow per cluster after merge and querying the wgmesh chain for the resulting thoughts.

**Requirements:** R8

**Dependencies:** U1, U2, U3, U4, U5 all merged to `main`.

**Files:**
- No file changes. Verification is operational.

**Approach:**
- After merge, manually trigger via `gh workflow run` (or wait for natural triggers) at least one workflow per cluster:
  - U1 cluster: any of the three (auto-merge fires on the merge of this PR itself, providing natural verification).
  - U2 cluster: dispatch `spec-command` against an existing closed spec PR or wait for next bot spec.
  - U3 cluster: file a test issue with `needs-triage` label, then close manually.
  - U4 cluster: trigger `pages` via `gh workflow run pages.yml` or wait for next docs commit.
  - U5 cluster: dispatch `goose-review` against an existing PR, or wait for `hetzner-integration` scheduled run.
- For each, run the verification curl from `reference_mentisdb_ci_integration_pattern.md` Verification queries section, scoped to `chain_key=wgmesh` and the relevant tag, and confirm the run_url in content matches the dispatched run.
- Cross-check no `::warning::mentisdb append failed` lines in any of the dispatched runs (would indicate connectivity/auth/payload issue worth investigating, even though non-fatal).

**Test scenarios:**
- Test expectation: none — operational verification, no code changes.

**Verification:**
- For each of the five clusters, at least one thought appears in the wgmesh chain with the run_url matching a dispatched run.
- No workflow failed due to mentisdb append (Actions tab green; warnings in logs investigated if present but workflow itself green).
- The cross-repo continuity goal is met: querying `chain_key=wgmesh tags_any=["pr-review-merge","spec-approval","docker-build","goose-review","hetzner-sync"]` returns ≥5 distinct event-type thoughts within 24h of merge.

---

## System-Wide Impact

- **Interaction graph:** Each instrumented workflow gains one trailing step. No cross-workflow coupling, no shared state. Failure of the append step does not cascade.
- **Error propagation:** Append failures are absorbed by `|| echo "::warning::..."`. The workflow's primary outcome (build pass/fail, merge success, deploy verdict, etc.) is unaffected.
- **State lifecycle risks:** None. MentisDB writes are append-only thoughts. Risk if a workflow retries: duplicate thoughts. Mitigation: pattern doc accepts this; queries can dedup by `run_url`.
- **API surface parity:** No external API surface change. Only consumer of new thoughts is MentisDB, already serving wgmesh's existing 3 instrumented workflows + ai-pipeline-template's 11.
- **Integration coverage:** Integration is a single curl POST. No unit tests meaningful. End-to-end verification via U6.
- **Unchanged invariants:** `agent-metrics-report.yml`, `release.yml`, `goose-build.yml` are not modified. `sync-labels.yml` remains uninstrumented by design. Org secret `MENTISDB_*` values, visibility, and rotation cadence unchanged. Workflow trigger conditions, permissions blocks, concurrency groups unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| MentisDB outage during rollout would emit `::warning::` on every workflow run across both repos. | Non-fatal append + `--max-time 15` curl timeout. Workflows still pass. ai-pipeline-template's `mentisdb-smoketest.yml` daily canary surfaces sustained outage as its own failure. |
| `approve-build.yml` triggered by Copilot review silently skips per App-actor limitation. | Pre-existing limitation, not introduced by this plan. Document in residual review findings. Future migration to `workflow_run` on `spec-auto-approve` is a separate concern. |
| 16 workflows × ~30-40 LOC = ~500 LOC duplication; pattern doc + sister repo also inline. | Composite-action extraction explicitly deferred. Re-evaluate at 13+ workflows in any single repo OR breaking MentisDB API change. |
| Pre-emptive Correction misuse: implementer might default to Correction on success for `spec-auto-approve` or `goose-review`. | Plan explicitly documents the lesson from ai-pipeline-template PR #610 review. Implementer must use ActionTaken on success for all 16 workflows; Correction is reserved for "fix to a Mistake thought". |
| Implementer might forget `if: always()` on a step, causing failure thoughts to never land. | Per-unit Approach explicitly states `if: always()`. U6 verification on intentionally failed test runs catches missing thoughts. |
| `pages.yml` and `dns-update.yml` may have multi-job structures that need per-job append (not just per-workflow). | Implementer should inspect each workflow's job count before adding append. If multi-job, append in each terminal job; if single-job, append once. Plan does not mandate one-step-per-workflow rigidly. |

---

## Documentation / Operational Notes

- After rollout, update `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/reference_mentisdb_ci_integration_pattern.md` "Onboarding a new repo" section to reflect: 19 wgmesh workflows instrumented (was 3), and 11 ai-pipeline-template workflows instrumented (was 3). Defer the memory update to post-merge.
- No runbook changes — failure modes are the same as before, just now also visible in MentisDB.
- No monitoring changes — existing GitHub Actions failure notifications remain primary alert channel; MentisDB is supplementary.
- Phase 2 (secondary repos: coroot-cicd, chimney, tvcentras) is a separate plan, requires `gh secret set ... --visibility selected --repos ai-pipeline-template,wgmesh,<new-repo>` widening first. Defer.

---

## Sources & References

- **Pattern doc (authoritative):** `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/reference_mentisdb_ci_integration_pattern.md`
- **Sister-repo plan:** `ai-pipeline-template/docs/plans/2026-04-27-001-feat-mentisdb-workflow-instrumentation-plan.md`
- **Sister-repo PR #610** (squash commit `a612392`) — 10-workflow instrumentation, structurally identical
- **Sister-repo heal PR #611** (commit `190844e`) — terraform-side fix unrelated to this plan
- **wgmesh reference impls:** `.github/workflows/agent-metrics-report.yml`, `.github/workflows/release.yml`, `.github/workflows/goose-build.yml`
- **Org secrets inventory:** `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/reference_org_secrets_inventory.md`
- **Auth gotcha:** `~/.claude/projects/-Users-oldroot-Repos-ai-pipeline-template/memory/feedback_mentisdb_no_rest_auth.md`
- **Lesson from sister-repo review:** `ai-pipeline-template/docs/residual-review-findings/task-le-volume-and-review-batch.md` — items res-001 (agent_id↔tag), res-002 (App-actor trigger gap), res-003 (composite-action threshold)
