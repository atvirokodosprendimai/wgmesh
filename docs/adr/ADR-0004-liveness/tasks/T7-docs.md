# Task ADR-0004-T7: Amend the specs, write the release notes and the backlog rows

**Depends-on:** T3, T5, T6
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** none
**Consumes:** `RestoredAt` and `PeerInfo.LastContact` in the cache file (T3); `DHTDiscovery.OnResume` and `daemon.EndpointSTUNResume` (T5); `daemon.EnforceRelayProbe`, `wgmesh_relay_route_age_seconds` (T6)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The peer store and daemon support specs and a release note describe `Assess`, the evidence times, the wake grace, the restore grace, probe-only relayed peers, the two switches and the three metrics as built, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md` | edit | `Assess`, `Alive`, the four evidence times, `GetProbeable`, `ObserveDataPlane` |
| `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` | edit | the restore grace, `LastContact` in the cache file, the resume loop and grace, probe-only relayed peers |
| `eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md` | edit | the `stun-resume` source (`daemon.EndpointSTUNResume`) and `DHTDiscovery.OnResume`; the spec already names `ObserveEndpoint` and is read by `spec_admit_test.go:21`, so this edit lands in a section that test reads |
| `README.md`, `docs/envelope-rollout.md` | edit | one link each to the new note |
| `docs/liveness-rollout.md` | add | operator and release note for N, N+1 and N+2: what each switch does; `wgmesh_liveness_disagree_total{site}` read as the N+1 go/no-go; `wgmesh_relay_route_age_seconds` read before N+2; `wgmesh_resume_detected_total`; the keepalive cost of probe-only peers; the 60-second wake grace; what a restored peer looks like in `daemon.status`; the `NATType` guard added by T5 (`BACKLOG.md:18` known limit, closed or reworded) |
| `docs/adr/BACKLOG.md`, `docs/admit-rollout.md` | edit | rows for N+1, N+2, OS power events through cgo or a helper, and rebinding the transport socket on a wake; `BACKLOG.md:19` and `docs/admit-rollout.md:24` say "ADR-0004 M5 will add / is planned to add a `stun-resume` source": once T5 has landed they say it adds one (this is the one exception to the rule that this task does not edit ADR-0002's lines: ADR-0002 R7 and these two sentences describe the same case, and M confirmed it on 2026-11-15 22:46); `BACKLOG.md:18` is reworded: T5 closed the `NATType` half (every access behind `GetNATType`/`SetNATType`), the `Policy.Local` read stays open |
| `eidos/spec - daemon health - dual signal health monitoring with eviction and temporary offline.md` | edit | line 60 names `buildDesiredPeerConfigs`, which T2 deleted (`planPeers` and `buildDesiredPeerConfigsWithHandshakes` replace it); `checkPeerHealth` is split into the wg read and `evaluatePeerHealth`; the health skip for relay-routed peers (T6) and the wake grace (T4) |
| `docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md` | edit | the R7 row (line 85) and the T11 note (line 190) say ADR-0004 M5 "will add" / "planned" a `stun-resume` source: they say it adds one (the second exception to the rule above, same case, M 2026-11-15 22:46) |
| `pkg/node/spec_liveness_test.go` | add | the specs, the note and the backlog must name the pieces; modelled on `pkg/node/spec_admit_test.go` (same file reading and missing-name reporting) |

## Ordered Steps

1. [S1] Add `pkg/node/spec_liveness_test.go` with `TestSpecNamesLiveness`, modelled on `pkg/node/spec_admit_test.go`: it reads the three specs, `docs/liveness-rollout.md` and `docs/adr/BACKLOG.md` and fails when `Assess`, `ObserveDataPlane` and `GetProbeable` (peer store spec), `ResumeGrace` and `probeOnly` (daemon spec), `EndpointSTUNResume` and `OnResume` (discovery spec), `EnforceLiveness`, `EnforceRelayProbe`, `N+1`, `N+2`, `wgmesh_liveness_disagree_total` and `wgmesh_relay_route_age_seconds` (note) or `ADR-0004` (backlog) are missing. Run it red.
2. [S2] Amend the two specs.
3. [S3] Write `docs/liveness-rollout.md`.
4. [S4] Edit `docs/adr/BACKLOG.md` and the `will add` / `is planned to add` sentences (`BACKLOG.md:19`, `docs/admit-rollout.md:24`); run `adr-debt docs/adr` and paste that no deferred row of ADR-0004 is reported unreceipted.
5. [S5] Run the mutants with `adr-verify --mutant`: the peer store spec stops naming `Assess`; the note stops naming the disagreement counter; the backlog loses the N+2 row. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^TestSpecNamesLiveness$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/liveness-rollout.md && grep -c 'ADR-0004' docs/adr/BACKLOG.md | grep -qv '^0$' && ! grep -rnE 'ADR-0004 M5 (is planned to|will) add|planned .stun-resume|will add a .stun-resume' docs/admit-rollout.md docs/adr/BACKLOG.md docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md && adr-debt docs/adr
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesLiveness` | `pkg/node/spec_liveness_test.go` | the three specs, the note and the backlog name the pieces above | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the note |
| 2 — something selects it | README and `docs/envelope-rollout.md` link the note; `adr-debt` reads the backlog rows |
| 3 — the caller can discover it | the README link |
| 4 — it is used | the N+1 and N+2 releases read the note and the rows |

## Mutation Log

## Invariants

- The note states no number that was not measured; the wake time comes from T4 S6 with its date and host, or it says "not measured".
- The release note states that `BACKLOG.md:18` (`NATType` written without a lock) is closed by T5's guard, or says what is left of it.

## Risks

- The specs drift from the code again; the test fails on a missing name only, not on a wrong description.

## Stop Condition

Stop and ask if T3, T5 or T6 renamed a symbol this task's test names, or if ADR-0002 T11 edited the same spec sections in a way that cannot be merged line by line.

## Out of Scope

- Flipping `EnforceLiveness` or `EnforceRelayProbe` (deferred: docs/adr/BACKLOG.md).
- ADR-0002's own spec amendments (permanent: boundary: ADR-0002 T11 owns them; this task edits other sections of the same files). The one exception is the wording of the `stun-resume` sentence in `BACKLOG.md:19` and `docs/admit-rollout.md:24` (S4).

## Verification Log
