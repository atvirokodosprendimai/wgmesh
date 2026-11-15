# Task ADR-0004-T5: Make the DHT layer refresh its endpoint and announce at once on a wake (#829)

**Depends-on:** T4
**External prerequisite:** ADR-0002 T9 merged on main (it adds `LocalNode.ObserveEndpoint`)
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** `DHTDiscovery.OnResume`, the `stun-resume` source accepted by `ObserveEndpoint`
**Consumes:** `node.Resumer`, `ResumeGrace` (T4); `LocalNode.ObserveEndpoint`, added by the ADR-0002 task named in the prerequisite line
**Data dependency:** hermetic
**Proof map:** v1

## Goal

Within seconds of a wake the node runs one STUN refresh, applies the new external endpoint at once, announces to its peers, and may punch again, instead of waiting up to a minute for `stunRefreshLoop` and two minutes for the announce cycle.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/dht.go` | edit | `OnResume()` on `DHTDiscovery`; the body of one `stunRefreshLoop` pass (`:409-464`) becomes `refreshSTUNOnce(source string)` that the ticker and `OnResume` both call, with source `stun` or `stun-resume`; `OnResume` also clears the rendezvous back-off state (`recordRendezvousAttempt`, `canAttemptRendezvous`) |
| `pkg/discovery/exchange.go` | edit | an exported `AnnounceNow()` that runs the same send the periodic announce runs; no new message type |
| `pkg/node/types.go` | edit | `ObserveEndpoint` applies the source `stun-resume` at once (the one case added to ADR-0002's R7) |
| `pkg/discovery/dht_resume_test.go` | add | tests below |

Selection: the daemon finds `OnResume` by the `node.Resumer` type assertion in T4's loop; a mutant that removes the method turns `TestDHTImplementsResumer` red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. The STUN call is a field (`stunFn`) that the test replaces, so no network is used.
2. [S2] Extract `refreshSTUNOnce(source)` from the loop body without changing what the loop does (the loop test of ADR-0002 T9 stays green). [proof: acceptance]
3. [S3] Implement `OnResume`: `refreshSTUNOnce("stun-resume")`, `AnnounceNow()`, reset the back-off.
4. [S4] Add the `stun-resume` case to `ObserveEndpoint`; add `var _ node.Resumer = (*DHTDiscovery)(nil)`.
5. [S5] Run the mutants with `adr-verify --mutant`: `OnResume` passes source `stun`; it skips the announce; it does not reset the back-off. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/discovery -run '^(TestOnResumeRefreshesSTUNAtOnce|TestOnResumeAnnouncesNow|TestOnResumeResetsRendezvousBackoff|TestDHTImplementsResumer)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 4 ] \
 && b=$(go test ./pkg/node -run '^TestObserveEndpointAppliesStunResumeAtOnce$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestOnResumeRefreshesSTUNAtOnce` | `pkg/discovery/dht_resume_test.go` | with a stubbed STUN that returns a new endpoint, `OnResume` changes the local endpoint at once; the ticker path with the same value does not (R7 stays) | — | S1, S3 |
| `TestOnResumeAnnouncesNow` | `pkg/discovery/dht_resume_test.go` | `OnResume` causes one announce send | — | S1, S3 |
| `TestOnResumeResetsRendezvousBackoff` | `pkg/discovery/dht_resume_test.go` | a peer in back-off may attempt a rendezvous after `OnResume` | — | S1, S3 |
| `TestDHTImplementsResumer` | `pkg/discovery/dht_resume_test.go` | the type satisfies `node.Resumer` and T4's assertion finds it | — | S1, S4 |
| `TestObserveEndpointAppliesStunResumeAtOnce` | `pkg/node/liveness_test.go` | `ObserveEndpoint("stun-resume", ep)` applies at once; `ObserveEndpoint("stun", ep)` still waits for three equal values | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | T4's loop type-asserts `node.Resumer`; the mutant that drops the method turns `TestDHTImplementsResumer` red |
| 3 — the caller can discover it | the log lines `[STUN] External endpoint changed` and `[Resume]` |
| 4 — it is used | every wake; the S6 step of T4 times it |

## Mutation Log

## Invariants

- The ticker path of `stunRefreshLoop` behaves as ADR-0002 T9 leaves it: three equal observations for a STUN-only change.
- No new message type and no new wire byte; `AnnounceNow` sends what the periodic announce sends.
- `pkg/daemon` does not import `pkg/discovery`.

## Risks

- Applying a STUN result at once after a wake can publish a wrong endpoint if the network is not up yet; the next ticker pass and any peer reflection correct it (ADR-0002 R7 rank order).
- The `stun-resume` case is an amendment of ADR-0002's R7 and needs M's answer to Open Question 5 first.

## Stop Condition

Stop and ask if ADR-0002 T9 shipped `ObserveEndpoint` with no source parameter that can carry `stun-resume`, or if Open Question 5 is answered "obey R7".

## Out of Scope

- Rebinding the transport socket or restarting WireGuard on a wake (deferred: docs/adr/BACKLOG.md).
- The multi-candidate local endpoint (permanent: boundary: ADR-0002 backlog row).

## Verification Log
