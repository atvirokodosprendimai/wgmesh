# Task ADR-0007-T5: Replace the deprecated `Announce` with `AnnounceTraversal` (#836)

**Depends-on:** none
**Resolved:** ADR-0002 is merged on main (f3860c6, 2026-11-16; `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0)
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
| `pkg/discovery/dht.go` | edit | `d.server.Announce(randomID, 0, false)` (`:560` after ADR-0002) and `d.server.Announce(infohash, 0, false)` (`:804`) become `AnnounceTraversal(h)`; `d.server.Announce(infohash, port, false)` (`:726`) becomes `AnnounceTraversal(h, announceOpts(port)...)`; `announceOpts` new |
| `pkg/discovery/dht_announce_test.go` | add | the tests below |

Selection: the three call sites are the only users; `TestAnnounceCallsUseTraversal` parses `dht.go` and fails while any `.Announce(` call on the server remains.

## Ordered Steps

1. [S1] Add `dht_announce_test.go` with the three tests and a compiling stub of `announceOpts` that returns nil; run red (the parse test finds 3 deprecated calls).
2. [S2] [proof: acceptance] Read `anacrolix/dht/v2@v2.23.0/announce.go:76-90` again at the version in `go.mod` and confirm `Announce` is `AnnounceTraversal` plus `AnnouncePeer(AnnouncePeerOpts{Port, ImpliedPort})` when `port != 0 || implied`; if the version has changed, stop (Stop Condition).
3. [S3] Implement `announceOpts(port)`: for `port != 0` (the condition of the old `Announce`, so a negative port is announced too, as before; the port comes from a `uint16`) one option, `dht.AnnouncePeer(dht.AnnouncePeerOpts{Port: port})`; for 0 none (`ImpliedPort` is false at every site).
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
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `pkg/discovery/dht.go` · announce without port: TestAnnounceOptsCarryPortOnlyWhenNonZero red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `pkg/discovery/dht.go` · get_peers site announces: TestQueryAndBootstrapSitesPassNoAnnounceOption red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `pkg/discovery/dht.go` · bootstrap site announces: TestQueryAndBootstrapSitesPassNoAnnounceOption red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `pkg/discovery/dht.go` · announce site never announces: TestAnnounceCallsUseTraversal red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `pkg/discovery/dht.go` · deprecated API back: TestAnnounceCallsUseTraversal red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · announce without port: TestAnnounceOptsCarryPortOnlyWhenNonZero red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · get_peers site announces: TestQueryAndBootstrapSitesPassNoAnnounceOption red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · bootstrap site announces: TestQueryAndBootstrapSitesPassNoAnnounceOption red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · announce site never announces: TestAnnounceCallsUseTraversal red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · deprecated API back: TestAnnounceCallsUseTraversal red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21
- 2026-11-16 · a686785* · mutant killed · exit 1 · `pkg/discovery/dht.go` · old Announce announced iff port != 0: announceOpts(-1) case red · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21

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
- 2026-11-16 · 32664af* · exit 1 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:1642 · test-lock-sha256:bc98cbcbf9372d89ba5d77e8e085101c8892257618937b41759b231bba6add0b · test-lock-file:docs/adr/locks/bc98cbcbf9372d89ba5d77e8e085101c8892257618937b41759b231bba6add0b.lock
  ```
  --- last 10 line(s) of stderr (of 15 after folding 15 raw)
  === RUN   TestAnnounceOptsCarryPortOnlyWhenNonZero
      dht_announce_test.go:66: announceOpts(51820) has 0 options, want 1
  --- FAIL: TestAnnounceOptsCarryPortOnlyWhenNonZero (0.00s)
  === RUN   TestQueryAndBootstrapSitesPassNoAnnounceOption
      dht_announce_test.go:75: queryInfohash: 0 AnnounceTraversal calls, want 1
      dht_announce_test.go:75: attemptBootstrapLookup: 0 AnnounceTraversal calls, want 1
  --- FAIL: TestQueryAndBootstrapSitesPassNoAnnounceOption (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	0.022s
  FAIL
  ```
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:13809
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:12024
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:13195
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:13181
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:11107
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:10176
- 2026-11-16 · a686785* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:0 · test-lock-sha256:19746fdc90dde4e5e89ec6692ebd48af1636c6347252a4f1880d919431701c04 · test-lock-file:docs/adr/locks/19746fdc90dde4e5e89ec6692ebd48af1636c6347252a4f1880d919431701c04.lock · test-lock-kind:replace
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:12928
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:12041
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:12070
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:12038
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:11209
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:13720
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:9426
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:2f87d03d7e645b458084ca8ad1f14f764439a70eca620503988544c5fa9f2a21 · ms:9326
