# Task ADR-0002-T10: Approve non-mesh routes by an operator list on the receiving node (#864)

**Depends-on:** T6
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `Policy.AcceptRoutes`, `Config.AcceptRoutes`, rule R9, the `--accept-routes` flag
**Consumes:** `EnforceRouteBinding`, rule R5 route part in `Decide` (T6); `Policy`, the daemon-built policy and `OnWarn` (T4)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A node whose operator sets `accept-routes` drops announced non-mesh networks that the list does not approve (counted as `route-not-approved` at release N, dropped when `EnforceRouteBinding` is flipped at N+2); with the list unset nothing changes; two HA nodes may advertise the same approved network; and a default route stays advertisable and is approved only by a `/0` entry.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/binding.go` | edit | `Approved(network string, list []*net.IPNet) bool`: an empty list approves everything; otherwise the network must be equal to or inside one entry (a `/0` entry approves everything, a `10.0.0.0/8` entry does not approve `0.0.0.0/0`) |
| `pkg/node/admit.go` | edit | R9 (in `bindClaim`, which `Decide` calls): for a `FirstHand` or `Restored` candidate, each route entry that is not approved is a warning at N and is listed in `Dropped` when `EnforceRouteBinding`; `Policy.AcceptRoutes func() []*net.IPNet` |
| `pkg/daemon/config.go` | edit | `AcceptRoutes []string` (the daemon reads it through the unexported `acceptRoutes()`; no exported getter), parsed and validated like `AdvertiseRoutes` (same parser) |
| `pkg/daemon/daemon.go` | edit | `Policy.AcceptRoutes` closure over the config under its existing lock; the reload file accepts `accept-routes` as it accepts `advertise-routes` (`daemon.go:1665-1709`) |
| `main.go` | edit | `--accept-routes` beside `--advertise-routes` (and `pkg/daemon/systemd.go`: `install-service` writes it into the unit) (`:333` for join, `:788`) |
| `pkg/node/binding_test.go`, `pkg/daemon/config_test.go`, `pkg/daemon/reload_test.go`, `pkg/daemon/routes_test.go` | edit | tests below |

Selection: the flag and the config field select the list, `Policy.AcceptRoutes` hands it to `Decide`; deleting the closure turns `TestAcceptRoutesReloadApplies` red.

## Ordered Steps

1. [S1] Add the seven tests below with compiling stubs (`Approved` returns true) and run red; `TestHAExitNodesDropEachOthersDefault` and `TestHAAdvertisersBothApproved` describe behaviour that holds today and start green as pins.
2. [S2] Implement `Approved` and R9; no other rule changes. Second-hand candidates carry no routes and are not touched.
3. [S3] Add the config field, the flag and the reload key, with one parser shared with `advertise-routes`. [proof: acceptance]
4. [S4] Confirm by reading `pkg/routes/routes.go:116-131` and `reconcile` that `ExcludeOwn` is still applied after admit and is unchanged; record `git diff --stat pkg/routes` (empty) in the commit message. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: `Approved` ignores the list; `Approved` approves `0.0.0.0/0` for a `10.0.0.0/8` entry; R9 refuses a network already advertised by another member; `AcceptRoutes` closure returns nil. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/node -run '^(TestAcceptRoutesEmptyIsOpen|TestAcceptRoutesContainmentApproves|TestDefaultRouteNeedsSlashZeroEntry|TestHAAdvertisersBothApproved|TestRouteNotApprovedWarnsThenDrops)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] \
 && b=$(go test ./pkg/daemon -run '^(TestAcceptRoutesReloadApplies|TestHAExitNodesDropEachOthersDefault)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 2 ] \
 && grep -q 'accept-routes' main.go && [ -z "$(git diff --stat HEAD -- pkg/routes)" ] && go test -race ./pkg/node ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestAcceptRoutesEmptyIsOpen` | `pkg/node/binding_test.go` | with no list every network, including `0.0.0.0/0`, is approved and nothing is warned or dropped | — | S1, S2 |
| `TestAcceptRoutesContainmentApproves` | `pkg/node/binding_test.go` | `192.168.5.0/24` is approved by an entry `192.168.0.0/16` and not by `10.0.0.0/8` | — | S1, S2 |
| `TestDefaultRouteNeedsSlashZeroEntry` | `pkg/node/binding_test.go` | `0.0.0.0/0` is approved by `0.0.0.0/0` only, `::/0` by `::/0` only | — | S1, S2 |
| `TestHAAdvertisersBothApproved` | `pkg/node/binding_test.go` | two members advertising the same approved network are both kept; neither is refused for the other's advertisement | — | S1, S2 |
| `TestRouteNotApprovedWarnsThenDrops` | `pkg/node/binding_test.go` | with a list, an unapproved network is kept and warned (`route-not-approved`) at N and dropped, the rest kept, when `EnforceRouteBinding` is true | — | S1, S2 |
| `TestAcceptRoutesReloadApplies` | `pkg/daemon/reload_test.go` | after the reload file sets `accept-routes`, a later offer is judged by the new list | — | S1, S3 |
| `TestHAExitNodesDropEachOthersDefault` | `pkg/daemon/routes_test.go` | a node that advertises `0.0.0.0/0` does not install a peer's `0.0.0.0/0`, and a node that does not advertise it installs the peer's | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `Policy.AcceptRoutes` is built in the daemon from the config; deleting the closure turns `TestAcceptRoutesReloadApplies` red |
| 3 — the caller can discover it | the `--accept-routes` flag help text and `docs/admit-rollout.md` (T11); the acceptance fence greps `main.go` for the flag |
| 4 — it is used | `wgmesh_admit_warnings_total{rule="route-not-approved"}` |

## Mutation Log
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/node/binding.go` · Approved ignores the list: TestAcceptRoutesContainmentApproves must go red · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · b22d90a* · mutant inconclusive · exit 1 · `pkg/node/binding.go` · a supernet is approved by a narrower entry: TestDefaultRouteNeedsSlashZeroEntry must go red · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/node/binding.go` · R9 touches second-hand candidates: TestRouteNotApprovedWarnsThenDrops (SecondHand subtest) must go red · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the AcceptRoutes closure is not installed: TestAcceptRoutesReloadApplies must go red · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/node/binding.go` · IPv4 entry approves an IPv4-mapped IPv6 network · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the reload file does not apply accept-routes · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · b22d90a* · mutant killed · exit 1 · `pkg/node/binding.go` · a supernet is approved by a narrower entry: TestDefaultRouteNeedsSlashZeroEntry must go red · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: Approved ignores the list · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: a supernet is approved by a narrower entry · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant survived · exit 0 · `pkg/node/binding.go` · r2: R9 judges second-hand candidates · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: the closure is not installed · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: families cross-approve · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: the reload file does not apply accept-routes · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/node/binding.go` · r2: R9 judges an unparsable entry · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 15af86a* · mutant killed · exit 1 · `pkg/daemon/config.go` · r2: a separators-only accept-routes is accepted as an open list · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 1d170e9* · mutant killed · exit 1 · `pkg/node/binding.go` · r3: R9 judges second-hand candidates (one decision point now) · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 1d170e9* · mutant killed · exit 1 · `pkg/node/binding.go` · r3: supernet approved by narrower entry · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9
- 2026-11-15 · 1d170e9* · mutant killed · exit 1 · `pkg/node/binding.go` · r3: R9 judges an unparsable entry · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9

## Invariants

- No byte on the wire changes; an advertiser needs no upgrade.
- An unset or empty list approves every network, as today.
- Approval is per network, never per advertiser; HA networks advertised by several members stay approved for each.
- A default route is never refused for being a default or a supernet; it needs a `/0` entry only when a list is set.
- `routes.ExcludeOwn` and `reconcile` are unchanged; admit does not decide HA failover.

## Risks

- An operator lists too little and drops legitimate routes at N+2; the counter at N is the warning, and an unset list stays open.
- Routes already stored drop only when the peer next announces; documented in the note (T11).
- Test doubles: none; the HA test uses the real `routes` package and a hand-built peer store.

## Stop Condition

Stop and ask if reading `reconcile` shows that a peer's default route is installed before `ExcludeOwn` runs (then admit and apply disagree and the HA decision of 2026-11-15 is at risk).

## Out of Scope

- Approval bound to an advertiser key or operator-signed grants (permanent: boundary: M decision 8, 2026-11-15, network only; announcements also carry an unauthenticated public key).
- Deny-by-default when the list is unset (permanent: boundary: M decision 7, 2026-11-15, open as today; the operator opts in by setting the list).
- Duplicate-subnet and default-route flapping in reconcile, #859 (permanent: boundary: apply-side route state belongs to the group that owns #859).

## Verification Log
- 2026-11-15 · b22d90a* · exit 1 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:772 · test-lock-sha256:4bb5ac584aeb0a1e931a6f14cd9ee89637c1f8f0e47eb545ebd769bc8955528e · test-lock-file:docs/adr/locks/4bb5ac584aeb0a1e931a6f14cd9ee89637c1f8f0e47eb545ebd769bc8955528e.lock
  ```
  --- last 10 line(s) of stderr (of 33 after folding 33 raw)
      binding_test.go:393: decision = {Verdict:0 Reason: Dropped:[] Warnings:[]}, want AdmitPartial dropping [172.16.0.0/12 10.1.0.0/16]
  === RUN   TestRouteNotApprovedWarnsThenDrops/restored_is_judged_and_second-hand_is_not
      binding_test.go:412: restored: decision = {Verdict:0 Reason: Dropped:[] Warnings:[]}, want AdmitPartial dropping [172.16.0.0/12 10.1.0.0/16]
  --- FAIL: TestRouteNotApprovedWarnsThenDrops (0.00s)
      --- FAIL: TestRouteNotApprovedWarnsThenDrops/warn_at_N (0.00s)
      --- FAIL: TestRouteNotApprovedWarnsThenDrops/drop_when_enforced (0.00s)
      --- FAIL: TestRouteNotApprovedWarnsThenDrops/restored_is_judged_and_second-hand_is_not (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.003s
  FAIL
  ```
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:8757
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7800
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7183
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:8295
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:8362
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7853
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:5454
- 2026-11-15 · b22d90a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7158
- 2026-11-15 · 15af86a* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:1 · test-lock-sha256:6337196e9bfa28f37030dee1c0de2b412be3f81cffe318ec0ae9c34d75f29545 · test-lock-file:docs/adr/locks/6337196e9bfa28f37030dee1c0de2b412be3f81cffe318ec0ae9c34d75f29545.lock · test-lock-kind:replace
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:8482
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7776
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7225
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:6582
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:6923
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7148
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7898
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:9025
- 2026-11-15 · 15af86a* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:4574
- 2026-11-15 · 1d170e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7591
- 2026-11-15 · 1d170e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:8200
- 2026-11-15 · 1d170e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:9097
- 2026-11-15 · 1d170e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:7176
- 2026-11-16 · 51e018f* · exit 0 · `set -o pipefail …` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:4692
- 2026-11-16 · 51e018f* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:1 · test-lock-sha256:21de1d04bed6d52ab7ccc8f43cc8450528414ebc69e347b12653b63e995a7003 · test-lock-file:docs/adr/locks/21de1d04bed6d52ab7ccc8f43cc8450528414ebc69e347b12653b63e995a7003.lock · test-lock-kind:replace
- 2026-11-16 · c192dcd* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:30aa3e5d966d6b92f32928f5edcd76e04b189f1c9b16f94d1b6bbf5fb3d9c9f9 · ms:0 · test-lock-sha256:7ff68cc3d9b281948bd7656a84adb357b791a71d9845c26c799d0edaa9400122 · test-lock-file:docs/adr/locks/7ff68cc3d9b281948bd7656a84adb357b791a71d9845c26c799d0edaa9400122.lock · test-lock-kind:replace
