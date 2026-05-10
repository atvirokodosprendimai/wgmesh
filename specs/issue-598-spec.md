# Specification: Issue #598

## Classification
ops

## Deliverables
code

## Problem Analysis

As of 2026-05-10, the following open issues lack an `fn:*` routing label that the Copilot
pipeline requires to route work to the correct function agent:

| Issue | Title (abbreviated) | Missing |
|-------|--------------------|----|
| #595  | bot-pr-review-merge.yml treats Copilot 'COMMENTED' as approval | `fn:*` |
| #578  | [Feature] optional flag: private/public key instead of --secret | `fn:*` |
| #573  | Triage workflow does not fire on issue reopen | `fn:*` |
| #571  | Tier 5 NAT Simulation: implement and gate releases | `fn:*` |
| #568  | Auto-verify network bug fixes via e2e workflow | `fn:*` |
| #540  | [Bug] key rotation changes node IP address | `fn:*` |
| #539  | Start(key, fd) for Android VPN API | `fn:*` |
| #233  | [Bug] missing mem0 stats on dashboard | `fn:*` |
| #171  | fix: TestCreateInterface_Darwin fails after wg binary path caching | `fn:*` |
| #79   | fix: GitHub Actions workflows stuck in 'action required' on Copilot PRs | `fn:*` |

Additionally, issue **#233** ("missing mem0 stats on dashboard") refers to a "dashboard" and
"mem0" integration that do not exist anywhere in the wgmesh codebase (confirmed via CLAUDE.md and
repository search). There is no dashboard component in wgmesh (operator surface is CLI only), and
no mem0 client or stats integration. The issue is obsolete/misrouted and should be closed.

## Proposed Approach

Use the `gh` CLI to apply labels to each issue in a single script execution and close the one
obsolete issue.

**Label assignments:**

| Issue | Label to add | Rationale |
|-------|-------------|-----------|
| #595  | `fn:dev`    | CI/CD workflow code fix (GitHub Actions bot logic) |
| #578  | `fn:dev`    | Feature: new CLI flag + daemon key-handling code |
| #573  | `fn:dev`    | CI/CD workflow code fix (copilot-triage.yml trigger) |
| #571  | `fn:dev`    | Testing infrastructure: Tier 5 NAT simulation in hetzner-integration.yml |
| #568  | `fn:dev`    | CI/CD automation: e2e verifier workflow + issue-close pipeline |
| #540  | `fn:dev`    | Daemon bug: IP allocation persistence after secret rotation |
| #539  | `fn:dev`    | Platform feature: Android VPN API fd-based Start() in daemon |
| #233  | `fn:dev`    | Temporarily assigned before closing as obsolete (no dashboard exists) |
| #171  | `fn:dev`    | Test fix: TestCreateInterface_Darwin mock path matching |
| #79   | `fn:dev`    | CI/CD fix: pull_request_target swap for safe workflows |

## Implementation Tasks

### Task 1: Apply `fn:dev` label to all ten unlabeled issues

Execute the following shell commands using the `gh` CLI. These commands are idempotent — adding a
label that already exists is a no-op.

```bash
for issue in 595 578 573 571 568 540 539 233 171 79; do
  gh issue edit "$issue" --repo atvirokodosprendimai/wgmesh --add-label "fn:dev"
done
```

Expected outcome: each issue gains the `fn:dev` label. The `gh` command will output a URL for
each updated issue.

### Task 2: Close issue #233 as obsolete

After the label is applied (Task 1), close issue #233 with an explanation comment:

```bash
gh issue comment 233 --repo atvirokodosprendimai/wgmesh \
  --body "Closing as obsolete: the wgmesh codebase has no dashboard UI and no mem0 integration — the operator surface is the CLI only. There is no component that could produce \"mem0 stats\". If you intended to report a different issue against a different repository, please file a new issue with a clear reproduction path."

gh issue close 233 --repo atvirokodosprendimai/wgmesh --reason "not planned"
```

Expected outcome: issue #233 is closed with a clear explanation comment visible to the reporter.

## Verification

After executing Tasks 1 and 2:

1. Run `gh issue list --repo atvirokodosprendimai/wgmesh --state open --json number,labels` and
   confirm that every open issue has at least one `fn:*` label in its `labels` array.
2. Run `gh issue view 233 --repo atvirokodosprendimai/wgmesh` and confirm it is closed.

A one-liner verification:

```bash
gh issue list --repo atvirokodosprendimai/wgmesh --state open --json number,labels \
  | jq '[.[] | select((.labels | map(.name) | any(startswith("fn:"))) | not) | .number]'
```

This must return an empty array `[]` for the acceptance criteria to be satisfied.

## Affected Files

None — this task applies GitHub labels and closes one issue via the `gh` CLI only. No repository
files are modified.

## Test Strategy

The verification query above (`jq` filter for issues missing any `fn:*` label) serves as the
acceptance test. It must return `[]`.

## Estimated Complexity
low
