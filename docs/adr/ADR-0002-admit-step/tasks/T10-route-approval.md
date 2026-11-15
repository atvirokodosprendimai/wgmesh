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
| `pkg/node/admit.go` | edit | R9 in `Decide`: for a `FirstHand` or `Restored` candidate, each route entry that is not approved is a warning at N and is listed in `Dropped` when `EnforceRouteBinding`; `Policy.AcceptRoutes func() []*net.IPNet` |
| `pkg/daemon/config.go` | edit | `AcceptRoutes []string` and its getter, parsed and validated like `AdvertiseRoutes` (same parser) |
| `pkg/daemon/daemon.go` | edit | `Policy.AcceptRoutes` closure over the config under its existing lock; the reload file accepts `accept-routes` as it accepts `advertise-routes` (`daemon.go:1665-1709`) |
| `main.go` | edit | `--accept-routes` beside `--advertise-routes` (`:333` for join, `:788`) |
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

- Approval bound to an advertiser key or operator-signed grants (permanent: boundary: announcements carry an unauthenticated public key, so a per-key rule would not hold).
- Deny-by-default when the list is unset (permanent: boundary: it would break every existing mesh at upgrade; the operator opts in by setting the list).
- Duplicate-subnet and default-route flapping in reconcile, #859 (permanent: boundary: apply-side route state belongs to the group that owns #859).

## Verification Log
