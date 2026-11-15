---
tldr: Verify a private security report, fold the reporter's overlapping fork PRs into one reviewed branch, fix the remainder, close absorbed PRs without merging them directly.
status: active
---

# Plan: security report hardening (branch task/security-report-1115)

## Context

- Private report received 2026-11-15, verified against 44ad8f4 by a Sonnet translator, 4 Haiku reviewers and an Opus synthesis (workflow wf_763a6eb4-af4).
  - kv notes in the wgmesh project: `known/sec-report-1115-ci`, `-aad`, `-minor`; state `now/wgmesh-secreport`.
- Open GitHub PRs checked 2026-11-15: 8 fork PRs by `ootada` (#842-#849), 4 same-repo PRs by `nycterent` (#819, #820, #831, #832).
  - #844 systemd secret argv, #845 probe read bound, #849 route and announcement validation overlap the report.
  - #842 (mesh deploy deadlock) fixes issue #838. Not security. Leave for normal review.
  - #843, #846, #847, #848 are unrelated bug fixes. Leave for normal review.
  - #832 (lint gate) touches envelope.go, exchange.go, systemd.go, daemon.go, deploy.go: rebase conflict risk for every phase below.
- Open issues checked 2026-11-15 against the findings:
  - #839 DHT overwrites PeerStore entries with empty endpoints. Same class as claim 4 (Update takes any source) and the missing Validate on the exchange path.
  - #830 collision nonce IPs are not persisted. Related to claim 4 (IP claims); DeriveMeshIPWithCollisionCheck has no non-test callers.
  - #828 endpoint flapping: applyObservedEndpoint lets any peer reflection overwrite our endpoint. Issue text only, not verified by us.
  - #827 is fixed by PR #831 (mesh IP stability). Touches daemon.go and subnet.go.
  - #837 flush close error, #836 deprecated Announce, #833 low coverage (wireguard 9.1%, discovery 19.5%), #829 macOS sleep/wake: unrelated. #833 supports the missing-tests finding.
  - #840 has no actionable content (one reproduction line).
  - #838 is a real report for the deadlock PR #842.
- Nothing in the PRs or issues covers: auto-merge trust gate, envelope AAD, nonce cache, SSH host key, transitive MeshIP overwrite.

### Seed
"dar sake supilde kazka ka rado i .../issues tai irgi perziurek su musu radiniais"
"kai isspresi 1 pr, uzdaryk, nemergink tiesiogiai. padaryk nauja branch/pr sitam reikalui"

## Phases

### Phase 1 - Fold the reporter's overlapping PRs - status: open

1. [ ] Cherry-pick #844 (systemd secret out of argv), authorship kept
   - commit fd36d7b from refs/pull/844/head
   - verify: `go test ./pkg/daemon -run Systemd -race`
2. [ ] Cherry-pick #845 (probe request line cap), then cap the client read at daemon.go:1095
   - #845 covers the server only
   - verify: probe tests, `go test -race ./pkg/daemon`
3. [ ] Cherry-pick #849 (route validation, exchange Validate)
   - commit 31dd790
   - does not reject 0.0.0.0/0 and does not stop transitive overwrite: phase 2
   - verify: `go test -race ./pkg/daemon ./pkg/discovery`

### Phase 2 - Fixes nobody proposed - status: open

1. [ ] auto-merge.yml: skip PRs whose head repo is not this repo (:104), bind merge to head sha (:344), bind approval to commit_id (:326, :152)
   - not requiring a label: only spec PRs carry approved-for-build
   - bot self-approval stays, that is a policy decision for the maintainer
   - verify: extract the gate to a small node script with a test for a fork PR and a moved head sha, as other workflow handlers do
2. [ ] PeerStore.Update: ignore MeshIP and route changes from transitive sources for an already-known key; reject 0.0.0.0/0 and ::/0 in Validate
   - relates to issue #839 and #830
   - verify: store test where a transitive update must not overwrite MeshIP
3. [ ] SSH host key verification (pkg/ssh/client.go:47)
   - changes behaviour for operators: first connect to an unknown host fails
   - needs a decision: ship in this PR with a release note, or separate PR

### Phase 3 - Wire change, separate PR - status: open

1. [ ] Spec first: envelope AAD (version and type) and a bounded nonce cache
   - three-release plan: readers accept both, writers seal with AAD, readers refuse nil AAD
   - a packet that opened only with nil AAD must never trigger GOODBYE
   - not in this branch

### Phase 4 - Close out - status: open

1. [ ] Push branch, open one PR to main, wait for CI
2. [ ] Close #844, #845, #849 with a thank-you comment naming the commit in our PR; do not merge them directly
3. [ ] Reply to the reporter: needs the maintainer's word

## Verification

- `go build ./... && go vet ./... && go test -race ./...` green on the branch.
- Each fixed finding has a test that fails when the fix is reverted.
- `git log` shows the reporter's authorship on cherry-picked commits.

## Adjustments

_Plans are best-effort at planning time._

## Progress Log

- 2611151637 plan written, branch task/security-report-1115 created from main 44ad8f4.
