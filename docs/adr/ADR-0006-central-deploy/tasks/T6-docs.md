# Task ADR-0006-T6: Amend the specs, write the release note and the backlog rows

**Depends-on:** T3, T4, T5, T7
**Covers:** none — no spec
**Estimated scope:** S (documents and one test)
**Owner:** unassigned
**Produces:** none
**Consumes:** `Mesh.Validate`, `StrictState` (T3); `NodeResult`, `DeployError`, `PersistFailureFatal` (T4); `resolveEndpoint` (T5); `restoreOwnConnectedRoutes` (T7)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The centralized specs, `docs/centralized-mode.md` and a release note describe quoting, validation, the per-node summary and the endpoint rule as built and name the two later flips, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md` | edit | line 20 says a node whose public IP differs gets no endpoint: the new order of rules, the per-node result, quoting |
| `eidos/spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md` | edit | the validation rules and their classes |
| `docs/centralized-mode.md` | edit | the example keeps its placeholders but says so; the `public_endpoint` override, the all-private rule and the DNS-name caveat; the summary text and the new first-deploy wording (line 201 says the first deploy fails for every node not yet in known_hosts: now it configures the others and lists each failed node); the connected-route restore after removing an own-network route (line 152-158); the two later flips |
| `docs/centralized-rollout.md` | add | operator and release note for N, N+1 and N+2: what class A and class B are, how to read the warnings as the go/no-go for `StrictState`, what a failed run leaves behind and that a second run converges, the persist warning (printed in a `Warnings:` block) and its flip; what the class B warnings that become refusals at N+1 mean for working states (listen_port 0, a bare IP in routable_networks, an interface name with a dot, an IPv6 ssh_host in an unbracketed endpoint), the `Restored connected route` line after an HA upgrade (#894 item 1) and that an older binary does not restore on later removals |
| `README.md` | edit | one link to the note, and the one-line description of the NAT rule (line 108: NAT is found by comparing the SSH host with the public IP) |
| `docs/adr/BACKLOG.md` | edit | rows for N+1 (`StrictState`), N+2 (`PersistFailureFatal`), `-only` and `-skip`, deleting the unreachable builders, issue #894 items 2 to 4 (PeerExchange start and stop race and DHTDiscovery restart; IPv6 entries in RoutableNetworks in the daemon and `GetCurrentRoutes`; transitive first insert with `Introducer` and shallow PeerStore copies, pointing items 2 and 4 at the daemon packages ADR-0002 and ADR-0005 govern) and the two copies of the connected-route restore (`pkg/daemon/routes.go` and `pkg/ssh/routes.go`), each naming ADR-0006 |
| `pkg/mesh/spec_central_test.go` | add | `TestSpecNamesCentralized` |

## Ordered Steps

1. [S1] Add `TestSpecNamesCentralized`: it reads the two specs, `docs/centralized-rollout.md` and `docs/adr/BACKLOG.md` and fails when `StrictState`, `PersistFailureFatal`, `public_endpoint`, `class A`, `class B` and `N+2` and `connected route` (note) or `ADR-0006` and `#894` (backlog) are missing. Run it red.
2. [S2] Amend the two specs and `docs/centralized-mode.md`.
3. [S3] Write `docs/centralized-rollout.md`; link it from the README.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0006 is reported unreceipted. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the note stops naming `PersistFailureFatal`; the backlog loses the N+2 row; the centralized spec keeps the old NAT sentence. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/mesh -run '^TestSpecNamesCentralized$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/centralized-rollout.md && ! adr-debt docs/adr | grep -q UNRECEIPTED
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesCentralized` | `pkg/mesh/spec_central_test.go` | the two specs, the release note and the backlog name the switches, `public_endpoint`, both classes and ADR-0006, and the old NAT sentence is gone from the centralized spec | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | the note is linked from the README and from `docs/centralized-mode.md` |
| 4 — it is used | nothing measures this yet |

## Mutation Log
- 2026-11-16 · e9b76ce* · mutant killed · exit 1 · `docs/centralized-rollout.md` · the release note no longer names PersistFailureFatal: TestSpecNamesCentralized must go red · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32
- 2026-11-16 · e9b76ce* · mutant killed · exit 1 · `docs/adr/BACKLOG.md` · the backlog row for PersistFailureFatal is gone: TestSpecNamesCentralized must go red · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32
- 2026-11-16 · e9b76ce* · mutant killed · exit 1 · `eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md` · the old NAT sentence without the private-address condition: TestSpecNamesCentralized must go red · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32

## Invariants

- The old sentence "gets no public endpoint" does not stay in a spec without the new condition.
- Every deferred row names ADR-0006 and a release, so `adr-debt` can resurface it.
- `docs/adr/BACKLOG.md` rows of other records are not edited (ADR-0002 T11 edits the same file: append, never rewrite).

## Risks

- The specs are descriptions, not gates; the test checks names, not meaning. Review owns the meaning.
- Merge order with ADR-0002 T11 on `README.md` and `docs/adr/BACKLOG.md`: both only add lines.

## Stop Condition

Stop and ask if T3, T4 or T5 changed a name this task's test requires.

## Out of Scope

- Flipping `StrictState` or `PersistFailureFatal` (deferred: docs/adr/BACKLOG.md).
- A change to `docs/envelope-rollout.md` (permanent: boundary: that note is ADR-0001's).

## Verification Log
- 2026-11-16 · e9b76ce* · exit 1 · `set -o pipefail …` · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32 · ms:802 · test-lock-sha256:018916d9d12b9e83685d781de77b734b5a1971e55d743b073d8851587383f4bd · test-lock-file:docs/adr/locks/018916d9d12b9e83685d781de77b734b5a1971e55d743b073d8851587383f4bd.lock
  ```
  --- last 10 line(s) of stderr (of 21 after folding 21 raw)
      spec_central_test.go:62: docs/centralized-mode.md does not name "PersistFailureFatal"
      spec_central_test.go:56: docs/centralized-rollout.md: cannot read: open /claude/scratch/2026-11-15/adr-g5/docs/centralized-rollout.md: no such file or directory
      spec_central_test.go:62: docs/adr/BACKLOG.md does not name "#894"
      spec_central_test.go:62: README.md does not name "centralized-rollout.md"
      spec_central_test.go:77: eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md keeps the old NAT sentence without the private-address condition: "  If the public IP differs from the SSH host address, the node is marked as behind NAT and given no public endpoint."
      spec_central_test.go:81: eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md does not state the all-private endpoint rule
  --- FAIL: TestSpecNamesCentralized (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	0.004s
  FAIL
  ```
- 2026-11-16 · e9b76ce* · exit 0 · `set -o pipefail …` · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32 · ms:2093
- 2026-11-16 · e9b76ce* · exit 0 · `set -o pipefail …` · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32 · ms:1716
- 2026-11-16 · e9b76ce* · exit 0 · `set -o pipefail …` · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32 · ms:1821
- 2026-11-16 · e9b76ce* · exit 0 · `set -o pipefail …` · acceptance-sha256:00c8b695db7412fa9136a548a5f8c56b58da7a9b3fb9232b32014412dfea9a32 · ms:1025
