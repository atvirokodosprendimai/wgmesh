# Task ADR-0005-T10: Make `PeerExchange` and `DHTDiscovery` restart after `Stop` (#863, the part PR #884 left)

**Depends-on:** T9
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `listenLoop(stopCh, conn)`; a fresh context per `DHTDiscovery.Start`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

After `Stop` then `Start`, the exchange has exactly one listen loop and the DHT layer's loops see a live context that still ends when the daemon's context ends. `PeerExchange.Stop` returns only after its listen loop has ended (a `sync.WaitGroup`, as the LAN and gossip layers do since #884), so a following `Start` never meets the old loop; every access to `running`, `stopCh` and `conn` of the exchange is under `pe.mu` or goes through the loop's arguments (M 2026-11-16: no bare shared state). `DHTDiscovery.Start` marks itself running only after its sub-layers started; a startup error stops what was already started and leaves `running` false (issue #894 item 2, review of 2026-11-15, verified by the reviewer under `-race` for the exchange part).

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/exchange.go` | edit | `Start` passes `stopCh` and `conn` to `go pe.listenLoop(...)` and adds the loop to a `sync.WaitGroup`; `listenLoop` uses its arguments, not `pe.stopCh` and `pe.conn`, and reads `running` under `pe.mu`; `Stop` closes the channel and the connection and then waits for the group, bounded by a timeout so a stuck read cannot hang the daemon (the connection close makes the blocked read return); an `atomic.Int32` `loops` counts live loops (test seam, one line) |
| `pkg/discovery/dht.go` | edit | `DHTDiscovery` keeps `parentCtx`; `Start` derives `ctx, cancel` under `d.mu` through `beginRun()` that returns the "already running" error; `running` is set true only after the exchange, gossip and the DHT server started, and on a startup error the layers already started are stopped and `running` stays false (the leaked LAN layer of #894) |
| `pkg/discovery/restart_test.go` | add | tests below |

Selection: `DHTDiscovery.Start` and `PeerExchange.Start` are the only starters; the exchange test fails if the loop reads the struct fields again, the DHT test fails if the context is not recreated.

Ordering note: follows T9 (same file) and ADR-0002 T9 (`dht.go`); `Start` and `Stop` hunks at `dht.go:104-215` are not edited by ADR-0002.

## Ordered Steps

1. [S1] Add the four tests with the `loops` counter present (so the build holds) and run red: after `Start`, `Stop`, `Start` the count is 2 with the old loop alive, and `d.ctx.Err()` is non-nil after a restart.
2. [S2] Pass the arguments to `listenLoop`; read `running` under the lock.
3. [S3] Add `beginRun` and `parentCtx`; `Stop` keeps cancelling the current `ctx`.
4. [S4] [proof: human: a full DHT layer restart against the real network was not run, it needs a bootstrap and STUN; the tests cover the context and the exchange] Record whether a restart was run on a development host, with date, in the sign-off.
5. [S5] Run the mutants with `adr-verify --mutant`: `listenLoop` reads `pe.stopCh` again; `beginRun` keeps the old context; `beginRun` derives from `context.Background()` instead of the parent; `Stop` does not wait for the loop; `Start` sets `running` before the layers started. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestPeerExchangeRestartKeepsOneLoop|TestPeerExchangeStopEndsTheLoop|TestDHTBeginRunGivesLiveContext|TestDHTRestartContextFollowsParent|TestPeerExchangeStopWaitsForLoop|TestDHTStartFailureLeavesNotRunning)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 6 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestPeerExchangeRestartKeepsOneLoop` | `pkg/discovery/restart_test.go` | Start, Stop, Start on a loopback port: `loops` is 1 once the old loop has ended (waits up to 3 s for it) | — | S1, S2 |
| `TestPeerExchangeStopEndsTheLoop` | `pkg/discovery/restart_test.go` | after the second Stop `loops` is 0 | — | S1, S2 |
| `TestDHTBeginRunGivesLiveContext` | `pkg/discovery/restart_test.go` | after Stop, `beginRun` returns nil and `d.ctx.Err()` is nil; a second `beginRun` while running returns the "already running" error | — | S1, S3 |
| `TestDHTRestartContextFollowsParent` | `pkg/discovery/restart_test.go` | cancelling the parent context ends the restarted context | — | S1, S3 |
| `TestPeerExchangeStopWaitsForLoop` | `pkg/discovery/restart_test.go` | right after `Stop` returns the `loops` counter is 0 (no polling): `Stop` waited for the loop; clean under `-race` with Start and Stop in a 20-iteration loop (the reviewer's reproduction of #894 item 2) | — | S1, S2 |
| `TestDHTStartFailureLeavesNotRunning` | `pkg/discovery/restart_test.go` | a startup error of a sub-layer leaves `running` false and the already started layers stopped | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `Start` of both types calls the new code; the mutants turn the tests red |
| 3 — the caller can discover it | doc comments on `Start` say "restart-safe"; T11 amends the DHT core and exchange specs |
| 4 — it is used | no code restarts them today; #829 would (another group) |

## Mutation Log

## Invariants

- A first `Start` behaves as before (same ports, same goroutines, same logs).
- `Stop` of a stopped layer is still a no-op; `Start` of a running one still errors.
- Daemon shutdown (parent context) still ends every goroutine.
- The LAN and gossip fixes of PR #884 are not touched.

## Risks

- The `loops` counter is a test seam in production code; one atomic add per loop start and end.
- The exchange test waits for a goroutine to end (up to 3 s) and uses a real UDP port; a busy host could delay it, so the wait is a poll with a deadline, not a sleep.
- Test doubles: loopback UDP port from the config keys; the DHT test calls `beginRun` without networking.

## Stop Condition

Stop and ask if `DHTDiscovery.Start` is changed by ADR-0002 T9 so that the context is used before `running` is set.

## Out of Scope

- Deciding when to restart after sleep or wake (permanent: boundary: #829, another group).
- `EpochManager.Stop` and `rpc.Server.Stop` restart behaviour (deferred: docs/adr/BACKLOG.md).

## Verification Log
