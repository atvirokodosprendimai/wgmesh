# Task ADR-0002-T6: Bind mesh IPs and routes to what the key derives, in warn mode, with frozen compat cases (#864)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** L (cross-boundary)
**Owner:** unassigned
**Produces:** `node.EnforceRouteBinding`, rule R5 in `Decide` (R4, the claim order, stays in `Winner` from T4 and is reused through `Bound`), `testdata/compat/admit/v1`
**Consumes:** `Policy`, `PeerStore.SetPolicy` (T1); `Claim`, `Bound`, `ResolveClaims`, `EnforceIPBinding`, the daemon-built `Policy` and `wgmesh_admit_warnings_total` (T4)
**Data dependency:** hermetic for the gate; the grinding measurement in S6 runs real key generation (no network)
**Proof map:** v1

## Goal

A claim for a mesh IP that another key holds by derivation, and a route entry inside the mesh network or inside another member's mesh IP, are counted at release N and refused when the two switches are flipped; rotated, nonce and custom-subnet nodes are admitted in every phase.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/binding.go` | add | `EnforceRouteBinding`; the route check (`inside or equal to MeshNet`, `inside or equal to another member's mesh IP`; supernets such as `0.0.0.0/0` and `::/0` are kept) |
| `pkg/node/admit.go` | edit | R5 in `Decide` (R4 stays in `Winner`): a first-hand claim from a key not yet in the store whose own claim is unbound, for an IP held by a bound claim of another key, is `Refuse` with reason `ip-held-by-bound` when `EnforceIPBinding`, else a warning; a key already stored is never refused by R5 (R4 only orders it); offending route entries are listed in `Dropped` when `EnforceRouteBinding`, else a warning |
| `pkg/daemon/daemon.go` | edit | add `MeshNet` (from `CustomSubnet` or `10.<b0>.0.0/16`) to the `Policy` T4 builds |
| `testdata/compat/admit/v1/cases.json`, `README.md` | add | frozen recordings: rotated address, collision-nonce address, custom-subnet address, node with no routes, a hijack case, and a rotated node already in the store against a newcomer whose natural address equals the rotated one (the rotated node stays admitted and stored in every phase); per case the decision in warn mode and in enforce mode |
| `pkg/node/binding_test.go`, `pkg/node/admit_compat_test.go` | add | tests below |

Selection: `SetPolicy` (T4's `TestDaemonInstallsAdmitPolicy` fails when the call is deleted) selects every rule here; `TestAdmitCompatFixtures` fails when a rule refuses a frozen case.

## Ordered Steps

1. [S1] Write `testdata/compat/admit/v1/cases.json` by hand from the shapes the code produces today (`DeriveMeshIP`, `DeriveMeshIPWithNonce(.., 1)`, `DeriveMeshIPInSubnet`, an address inside `10.0.0.0/8` that no key derives to); add the tests below with compiling stubs; run red. Freeze the fixture: it is never regenerated.
2. [S2] Implement R5 in `Decide` and `binding.go` with both switches false; every case where enforcement would act calls `Policy.OnWarn(rule)` with one of `ip-held-by-bound`, `route-inside-mesh`, `route-inside-member-ip` (`winner-differs` is emitted by T4's `Winner`, not here).
3. [S3] Add `MeshNet` to the daemon's policy; `OnWarn` and the counter come from T4. [proof: acceptance]
4. [S4] `TestAdmitCompatFixtures` runs every case in four phases (both switches off, `EnforceIPBinding` only, both on, and the order of N+1 then N+2) and fails if a rotated, nonce or custom-subnet case is refused in any of them, or if the hijack case is admitted when its switch is on.
5. [S5] Run the mutants with `adr-verify --mutant`: R4 refuses an unbound claim without checking that a bound claim exists; the route check refuses supernets; `OnWarn` is not called in warn mode; R5 also refuses a key that is already stored. [proof: mutation]
6. [S6] Measure once how many key pairs it takes to find one whose derived default-subnet address equals a chosen victim address, with `WGMESH_MEASURE=1 go test ./pkg/node -run TestMeasureBindingGrinding -v`; write the count, the date and the machine into `testdata/compat/admit/v1/README.md` and the commit message. [proof: human: a measured number replaces the estimate in Open Question 1; the gate does not run it]

7. [S7] With the claim rules in place, run the new `BenchmarkOfferReplyAtCap` (and `BenchmarkOfferAtCap` for comparison) with a REPLY carrying 1000 `KnownPeers` mentions against a full store, and record ns/op, the date and the machine in the commit message; if one message holds the write lock for longer than a WireGuard keepalive tick would tolerate, say so and stop. [proof: human: a measurement; the gate does not run it]

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
| 1 — exists | the seven tests and the fixture |
| 2 — something selects it | `SetPolicy` in the daemon (T4); deleting it turns `TestDaemonInstallsAdmitPolicy` red |
| 3 — the caller can discover it | the counter (T4) is on the metrics endpoint; T11 writes the release note that names it |
| 4 — it is used | the counter value is the go/no-go for N+1 and N+2 |

## Mutation Log
- 2026-11-15 · bbd6579* · mutant killed · exit 1 · `pkg/node/binding.go` · R4/R5 act without checking that a bound claim holds the IP · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · bbd6579* · mutant killed · exit 1 · `pkg/node/binding.go` · the route check refuses supernets: TestRouteSupernetIsKept must go red · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · bbd6579* · mutant killed · exit 1 · `pkg/node/binding.go` · OnWarn is not called in warn mode: TestIPClaimWarnsAtReleaseN must go red · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · bbd6579* · mutant killed · exit 1 · `pkg/node/binding.go` · R5 also refuses a key already stored: TestRotatedNodeInStoreIsNeverRefused must go red · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: R5 acts without a bound holder · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant inconclusive · exit 1 · `pkg/node/binding.go` · r2: the route check refuses a shorter prefix whose base is inside the mesh · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: OnWarn not called · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: R5 refuses a stored key · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: a holder bound at nonce 1..10 is not seen · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: IPv4-mapped IPv6 route counted inside the IPv4 mesh · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: a bound claimant on a bound holder is refused · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b
- 2026-11-15 · 6206e06* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: the route check refuses a shorter prefix whose base is inside the mesh: TestRouteSupernetIsKept must go red · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b

## Invariants

- Both switches are false in this release; nothing in production code sets them.
- A rotated, nonce or custom-subnet address is never refused for being unbound; it loses only to a bound claim for the same address.
- No byte of the envelope or announcement changes; the secret never enters `pkg/node`.
- Route supernets are not refused: WireGuard matches AllowedIPs by longest prefix, so they cannot take a more specific address.

## Risks

- A legitimate mesh with overlapping routes counts warnings from N; that is the reason the route switch flips one release after the IP one.
- Grinding (Open Question 1): the binding does not stop a member who generates a key that derives to the victim's address.
- Known limits found in review (2026-11-15), kept: the `ip-held-by-bound` refusal needs the claimant's own claim unbound (two keys that derive the same address are ordered by `Winner`, not refused); an IPv6 `/128` route equal to a member's `MeshIPv6` is not caught (only `MeshIP` is compared); route warnings also count second-hand mentions and re-count on every announcement, so the N counter over-counts on gossip-heavy meshes; `Policy.Local` reads `daemon.LocalNode` without a lock inside `Offer` (the daemon reads it unlocked elsewhere; ADR-0005 owns that); an unbound squatter that arrives first stays stored until the collision pass orders it by `Winner`.
- Test doubles: none for the rules; the daemon tests use the real `Policy` closures over a hand-built `Config`.

## Stop Condition

Stop and ask if a frozen case that older releases emit would be refused in enforce mode, or if building `MeshNet` for the default subnet needs more than the two bytes `Keys.MeshSubnet` already holds.

## Out of Scope

- Approval of routes outside the mesh network by an operator list (T10).
- Flipping either switch (deferred: docs/adr/BACKLOG.md).
- Signature-based binding (permanent: boundary: separate decision, see the ADR).

## Verification Log
- 2026-11-15 · bbd6579* · exit 1 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:851 · test-lock-sha256:bcfcb034d4a9886b726651708d4c9667b60e0df232ba2729c80ac4bdc40bf14a · test-lock-file:docs/adr/locks/bcfcb034d4a9886b726651708d4c9667b60e0df232ba2729c80ac4bdc40bf14a.lock
  ```
  --- last 10 line(s) of stderr (of 110 after folding 110 raw)
      --- FAIL: TestRouteInsideMeshWarnsThenDrops/drop_when_enforced (0.00s)
  === RUN   TestRouteSupernetIsKept
  --- PASS: TestRouteSupernetIsKept (0.00s)
  === RUN   TestRotatedAddressAdmittedInEveryPhase
  --- PASS: TestRotatedAddressAdmittedInEveryPhase (0.00s)
  === RUN   TestRotatedNodeInStoreIsNeverRefused
  --- PASS: TestRotatedNodeInStoreIsNeverRefused (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.005s
  FAIL
  ```
- 2026-11-15 · bbd6579* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:4963
- 2026-11-15 · bbd6579* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5254
- 2026-11-15 · bbd6579* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5178
- 2026-11-15 · bbd6579* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5057
- 2026-11-15 · bbd6579* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:3516
- 2026-11-15 · 6206e06* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:0 · test-lock-sha256:40b6b0c34e075026a3f8586b5d1ad837b95e1f2640c28837895bc62ce73147e4 · test-lock-file:docs/adr/locks/40b6b0c34e075026a3f8586b5d1ad837b95e1f2640c28837895bc62ce73147e4.lock · test-lock-kind:replace
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5177
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:4784
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5243
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5055
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:6335
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5332
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:4879
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:3692
- 2026-11-15 · 6206e06* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:5223
- 2026-11-16 · 51e018f* · exit 0 · `set -o pipefail …` · acceptance-sha256:79b3563d600fbf57d5ba0192535a9efd3ed0a3089290c81450c624e672cf4e4b · ms:3431
