# Task ADR-0002-T6: Bind mesh IPs and routes to what the key derives, in warn mode, with frozen compat cases (#864)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** L (cross-boundary)
**Owner:** unassigned
**Produces:** `node.EnforceRouteBinding`, rules R4 and R5 in `Decide`, `testdata/compat/admit/v1`
**Consumes:** `Policy`, `PeerStore.SetPolicy` (T1); `Claim`, `Bound`, `ResolveClaims`, `EnforceIPBinding`, the daemon-built `Policy` and `wgmesh_admit_warnings_total` (T4)
**Data dependency:** hermetic for the gate; the grinding measurement in S6 runs real key generation (no network)
**Proof map:** v1

## Goal

A claim for a mesh IP that another key holds by derivation, and a route entry inside the mesh network or inside another member's mesh IP, are counted at release N and refused when the two switches are flipped; rotated, nonce and custom-subnet nodes are admitted in every phase.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/binding.go` | add | `EnforceRouteBinding`; the route check (`inside or equal to MeshNet`, `inside or equal to another member's mesh IP`; supernets such as `0.0.0.0/0` and `::/0` are kept) |
| `pkg/node/admit.go` | edit | R4 and R5 in `Decide`: a first-hand claim from a key not yet in the store, for an IP held by a bound claim of another key, is `Refuse` with reason `ip-held-by-bound` when `EnforceIPBinding`, else a warning; a key already stored is never refused by R5 (R4 only orders it); offending route entries are listed in `Dropped` when `EnforceRouteBinding`, else a warning |
| `pkg/daemon/daemon.go` | edit | add `MeshNet` (from `CustomSubnet` or `10.<b0>.0.0/16`) to the `Policy` T4 builds |
| `testdata/compat/admit/v1/cases.json`, `README.md` | add | frozen recordings: rotated address, collision-nonce address, custom-subnet address, node with no routes, a hijack case, and a rotated node already in the store against a newcomer whose natural address equals the rotated one (the rotated node stays admitted and stored in every phase); per case the decision in warn mode and in enforce mode |
| `pkg/node/binding_test.go`, `pkg/node/admit_compat_test.go` | add | tests below |

Selection: `SetPolicy` (T4's `TestDaemonInstallsAdmitPolicy` fails when the call is deleted) selects every rule here; `TestAdmitCompatFixtures` fails when a rule refuses a frozen case.

## Ordered Steps

1. [S1] Write `testdata/compat/admit/v1/cases.json` by hand from the shapes the code produces today (`DeriveMeshIP`, `DeriveMeshIPWithNonce(.., 1)`, `DeriveMeshIPInSubnet`, an address inside `10.0.0.0/8` that no key derives to); add the tests below with compiling stubs; run red. Freeze the fixture: it is never regenerated.
2. [S2] Implement R4/R5 in `Decide` and `binding.go` with both switches false; every case where enforcement would act calls `Policy.OnWarn(rule)` with one of `ip-held-by-bound`, `route-inside-mesh`, `route-inside-member-ip`, `winner-differs`.
3. [S3] Add `MeshNet` to the daemon's policy; `OnWarn` and the counter come from T4. [proof: acceptance]
4. [S4] `TestAdmitCompatFixtures` runs every case in four phases (both switches off, `EnforceIPBinding` only, both on, and the order of N+1 then N+2) and fails if a rotated, nonce or custom-subnet case is refused in any of them, or if the hijack case is admitted when its switch is on.
5. [S5] Run the mutants with `adr-verify --mutant`: R4 refuses an unbound claim without checking that a bound claim exists; the route check refuses supernets; `OnWarn` is not called in warn mode; R5 also refuses a key that is already stored. [proof: mutation]
6. [S6] Measure once how many key pairs it takes to find one whose derived default-subnet address equals a chosen victim address, with `WGMESH_MEASURE=1 go test ./pkg/node -run TestMeasureBindingGrinding -v`; write the count, the date and the machine into `testdata/compat/admit/v1/README.md` and the commit message. [proof: human: a measured number replaces the estimate in Open Question 1; the gate does not run it]

7. [S7] With the claim rules in place, re-run `BenchmarkOfferAtCap` with a REPLY carrying 1000 `KnownPeers` mentions against a full store, and record ns/op, the date and the machine in the commit message; if one message holds the write lock for longer than a WireGuard keepalive tick would tolerate, say so and stop. [proof: human: a measurement; the gate does not run it]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/node -run '^(TestIPClaimWarnsAtReleaseN|TestIPClaimRefusedWhenEnforced|TestRouteInsideMeshWarnsThenDrops|TestRouteSupernetIsKept|TestRotatedAddressAdmittedInEveryPhase|TestAdmitCompatFixtures|TestRotatedNodeInStoreIsNeverRefused)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] \
 && test -f testdata/compat/admit/v1/cases.json && go test -race ./pkg/node ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestIPClaimWarnsAtReleaseN` | `pkg/node/binding_test.go` | with both switches false, a claim for a bound member's IP is stored as today and `OnWarn("ip-held-by-bound")` is called once | — | S1, S2 |
| `TestIPClaimRefusedWhenEnforced` | `pkg/node/binding_test.go` | with `EnforceIPBinding` true the same claim is refused with reason `ip-held-by-bound` and the store is unchanged | — | S1, S2 |
| `TestRouteInsideMeshWarnsThenDrops` | `pkg/node/binding_test.go` | a route equal to a member's mesh IP `/32` or inside `MeshNet` is kept with a warning at N and dropped, the rest kept, when `EnforceRouteBinding` is true | — | S1, S2 |
| `TestRouteSupernetIsKept` | `pkg/node/binding_test.go` | `0.0.0.0/0` and `::/0` and a `10.0.0.0/8` supernet survive both switches | — | S1, S2 |
| `TestRotatedAddressAdmittedInEveryPhase` | `pkg/node/binding_test.go` | an unbound address with no competing claim is admitted with both switches on | — | S1, S2 |
| `TestAdmitCompatFixtures` | `pkg/node/admit_compat_test.go` | every frozen case gives its recorded decision in all four phases | — | S1, S4 |
| `TestRotatedNodeInStoreIsNeverRefused` | `pkg/node/binding_test.go` | a stored rotated node keeps being admitted after a newcomer with the same bound address arrives, with both switches on | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests and the fixture |
| 2 — something selects it | `SetPolicy` in the daemon (T4); deleting it turns `TestDaemonInstallsAdmitPolicy` red |
| 3 — the caller can discover it | the counter (T4) is on the metrics endpoint; T10 writes the release note that names it |
| 4 — it is used | the counter value is the go/no-go for N+1 and N+2 |

## Mutation Log

## Invariants

- Both switches are false in this release; nothing in production code sets them.
- A rotated, nonce or custom-subnet address is never refused for being unbound; it loses only to a bound claim for the same address.
- No byte of the envelope or announcement changes; the secret never enters `pkg/node`.
- Route supernets are not refused: WireGuard matches AllowedIPs by longest prefix, so they cannot take a more specific address.

## Risks

- A legitimate mesh with overlapping routes counts warnings from N; that is the reason the route switch flips one release after the IP one.
- Grinding (Open Question 1): the binding does not stop a member who generates a key that derives to the victim's address.
- Test doubles: none for the rules; the daemon tests use the real `Policy` closures over a hand-built `Config`.

## Stop Condition

Stop and ask if a frozen case that older releases emit would be refused in enforce mode, or if building `MeshNet` for the default subnet needs more than the two bytes `Keys.MeshSubnet` already holds.

## Out of Scope

- Route approval for routes outside the mesh network (deferred: docs/adr/BACKLOG.md).
- Flipping either switch (deferred: docs/adr/BACKLOG.md).
- Signature-based binding (permanent: boundary: separate decision, see the ADR).

## Verification Log
