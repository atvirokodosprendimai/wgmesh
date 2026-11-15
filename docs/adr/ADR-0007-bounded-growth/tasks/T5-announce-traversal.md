# Task ADR-0007-T5: Replace the deprecated `Announce` with `AnnounceTraversal` (#836)

**Depends-on:** none
**Blocked-on:** the last task of ADR-0002 (its documentation task) is merged on main: `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0, because the files this task edits are also edited by ADR-0002 tasks
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** `announceOpts(port int) []dht.AnnounceOpt`
**Consumes:** none
**Data dependency:** needs two nodes on a network for step S5 only (the unit tests are hermetic); the sign-off line records the command and the result
**Proof map:** v1

## Goal

`pkg/discovery/dht.go` calls `AnnounceTraversal` at its three sites, with the announce option present exactly when the port is non-zero, and behaves as before.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/dht.go` | edit | `d.server.Announce(randomID, 0, false)` (`:544`) and `d.server.Announce(infohash, 0, false)` (`:787`) become `AnnounceTraversal(h)`; `d.server.Announce(infohash, port, false)` (`:710`) becomes `AnnounceTraversal(h, announceOpts(port)...)`; `announceOpts` new |
| `pkg/discovery/dht_announce_test.go` | add | the tests below |

Selection: the three call sites are the only users; `TestAnnounceCallsUseTraversal` parses `dht.go` and fails while any `.Announce(` call on the server remains.

## Ordered Steps

1. [S1] Add `dht_announce_test.go` with the three tests and a compiling stub of `announceOpts` that returns nil; run red (the parse test finds 3 deprecated calls).
2. [S2] [proof: acceptance] Read `anacrolix/dht/v2@v2.23.0/announce.go:76-90` again at the version in `go.mod` and confirm `Announce` is `AnnounceTraversal` plus `AnnouncePeer(AnnouncePeerOpts{Port, ImpliedPort})` when `port != 0 || implied`; if the version has changed, stop (Stop Condition).
3. [S3] Implement `announceOpts(port)`: for `port > 0` one option, `dht.AnnouncePeer(dht.AnnouncePeerOpts{Port: port})`; for 0 none (`ImpliedPort` is false at every site).
4. [S4] Replace the three calls; `go vet ./pkg/discovery` clean; `grep -rn nolint --include=*.go .` still empty.
5. [S5] On two nodes with one mesh secret on a network that reaches the public DHT, start both, and record that each logs `[DHT] Announced to N nodes` with N > 0 and finds the other (`[DHT] Query complete, discovered M peer addresses`), with the command, hosts and date. [proof: human: the announce traversal needs the public DHT, which a hermetic test cannot reach]
6. [S6] Run the mutants with `adr-verify --mutant`: `announceOpts` returning no option for a non-zero port; the query site passing an option. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestAnnounceCallsUseTraversal|TestAnnounceOptsCarryPortOnlyWhenNonZero|TestQueryAndBootstrapSitesPassNoAnnounceOption)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 3 ] && ! grep -nE '\.Announce\(' pkg/discovery/dht.go && go vet ./pkg/discovery && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestAnnounceCallsUseTraversal` | `pkg/discovery/dht_announce_test.go` | parsing `dht.go`, no selector call named `Announce` remains and exactly three `AnnounceTraversal` calls exist | — | S1, S4 |
| `TestAnnounceOptsCarryPortOnlyWhenNonZero` | `pkg/discovery/dht_announce_test.go` | `announceOpts(0)` is empty and `announceOpts(51820)` has one option | — | S1, S3 |
| `TestQueryAndBootstrapSitesPassNoAnnounceOption` | `pkg/discovery/dht_announce_test.go` | the two port-0 call sites pass no option (parse of the argument list) | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the three tests |
| 2 — something selects it | the three call sites in the bootstrap, announce and query paths; the parse test fails if one is left |
| 3 — the caller can discover it | the DHT core spec already names the announce path; no new surface |
| 4 — it is used | every DHT bootstrap, announce and query |

## Mutation Log

## Invariants

- The announced port, the query-only behaviour of `queryInfohash` and the bootstrap lookup of `bootstrap` are unchanged; the DHT bootstrap nodes are not touched.
- `anacrolix/dht/v2` stays at the version in `go.mod`; no `go.mod` change.

## Risks

- A later version of the library changes `AnnounceTraversal` semantics; this task pins the equivalence for the current version only.
- The hermetic tests cannot show propagation; step S5 is human.

## Stop Condition

Stop and ask if the library version in `go.mod` is not `v2.23.0` at execution, or if `Announce` and `AnnounceTraversal` differ in the version present (re-read S2), or if `AGENTS.md` still forbids DHT changes without a test that this task cannot run.

## Out of Scope

- Any change to the DHT bootstrap nodes (permanent: boundary: the project's review list in `CLAUDE.md`).
- Changing announce intervals or the rendezvous logic (permanent: boundary: not part of #836).

## Verification Log
