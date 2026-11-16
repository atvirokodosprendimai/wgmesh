# Task ADR-0004-T5: Make the DHT layer refresh its endpoint and announce at once on a wake (#829)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** `DHTDiscovery.OnResume`, `daemon.EndpointSTUNResume` accepted by `ObserveEndpoint`, a serialised `refreshSTUNOnce`, a guarded `NATType`
**Consumes:** `node.Resumer`, `ResumeGrace` (T4); `LocalNode.ObserveEndpoint` and the `EndpointSource` constants (added by ADR-0002, merged)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

Within seconds of a wake the node runs one STUN refresh, applies the new external endpoint at once, announces to its peers, and may punch again, instead of waiting up to a minute for `stunRefreshLoop` and two minutes for the announce cycle.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/daemon.go` | edit | add `EndpointSTUNResume EndpointSource = "stun-resume"` with a doc comment beside `EndpointInterface`, `EndpointReflection`, `EndpointSTUN` (`:139-149`); `ObserveEndpoint` (`:164`) already applies any source other than `EndpointSTUN` at once and clears the pending STUN observations (`:167-179`), so the code change is the constant and its doc. `NATType` (a `LocalNode` field, written unlocked today, see S4) gets a lock-guarded setter and getter on `LocalNode` under `endpointMu`; the reader in `shouldRelayPeerWithSubnets` (`:849`) uses the getter |
| `pkg/discovery/dht.go` | edit | `stunRefreshLoop` (`:409-461`) has no `refreshSTUNOnce` yet: its body becomes `refreshSTUNOnce(src daemon.EndpointSource)`, which the ticker and `OnResume` both call (the type is `EndpointSource`, not string). The IPv6 branch (`:415-423`, which uses `EndpointInterface` and `continue`s) stays inside the extracted function, and the two `ObserveEndpoint(daemon.EndpointSTUN, ...)` calls (`:437`, `:453`) take `src`; `discoverExternalEndpoint` at `Start` (`:255`, `:271`, `:284`) is not changed except for the `NATType` setter. `refreshSTUNOnce` takes a mutex (`stunMu`) so `OnResume` and the 60-second ticker cannot run it at once. `OnResume()`: safe before `Start` and after `Stop` (it reads `d.ctx` and `d.running` under `d.mu` and returns when not running); calls `refreshSTUNOnce(daemon.EndpointSTUNResume)`, `announceNow()`, and clears `d.rendezvousBackoff` under `d.mu` (`:77`, `:917-946`; `canAttemptRendezvous` is `:906`). **`announceNow()` is defined here**, because no single "announce now" exists: `announce()` (`:680`) goes to the infohash, `PeerExchange.SendAnnounce(remoteAddr)` (`exchange.go:1163`) goes to one address, and gossip (`gossip.go:166`) sends to one random peer per tick through `SendAnnounce`. `announceNow()` = `announce()` plus `SendAnnounce` to the mesh IP and gossip port (`g.port`) of every `GetActive` peer, guarded by `pe.conn == nil` and `!d.config.Gossip`; no new message type |
| `pkg/daemon/local_endpoint_test.go` | edit | `TestObserveEndpointAppliesStunResumeAtOnce` and `TestNATTypeAccessIsRaceFree` (the first test moved here from the `pkg/node` file of the first draft: `ObserveEndpoint` is in `pkg/daemon`, `node.LocalNode`, `pkg/node/types.go:36`, has no such method) |
| `pkg/discovery/dht_resume_test.go` | add | tests below |

Selection: the daemon finds `OnResume` by the `node.Resumer` type assertion on `d.dhtDiscovery` in T4's loop; a mutant that removes the method turns `TestDHTImplementsResumer` red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. The STUN call is a field (`stunFn`) that the test replaces, and the announce sends are fields the test replaces, so no network is used.
2. [S2] Extract `refreshSTUNOnce(src daemon.EndpointSource)` from the loop body without changing what the loop does (the loop test of ADR-0002 T9 stays green). [proof: acceptance]
3. [S3] Implement `OnResume`, `announceNow` and the back-off reset as described above; add `var _ node.Resumer = (*DHTDiscovery)(nil)`.
4. [S4] Add the `EndpointSTUNResume` constant and its doc. **The `NATType` race**: `d.localNode.NATType` is written unlocked at `dht.go:256`, `:272`, `:285`, `:421`, `:443` and read in `shouldRelayPeerWithSubnets` (`daemon.go:849`; `BACKLOG.md:18` records it as a known limit). `OnResume` now runs concurrently with the 60-second `stunRefreshLoop`, so the task serialises `refreshSTUNOnce` with `stunMu` **and** moves every access of `NATType` (the writers above and the reader; re-grep `\.NATType` and move every site found) behind a setter and getter under `endpointMu`. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: `OnResume` passes `EndpointSTUN`; it skips the announce; it does not reset the back-off; `refreshSTUNOnce` drops `stunMu`; the getter reads the field without the lock. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/discovery -run '^(TestOnResumeRefreshesSTUNAtOnce|TestOnResumeAnnouncesNow|TestOnResumeResetsRendezvousBackoff|TestDHTImplementsResumer|TestOnResumeBeforeStartAndAfterStopIsSafe|TestOnResumeRunsOneSTUNPassPerCall|TestOnResumeSerialisedWithStunRefreshLoop|TestOnResumeDropsAnswerAfterCancelDuringSTUN|TestOnResumeStopWhileSTUNBlockedKeepsEndpoint|TestOnResumeCoalescesWhileAPassRuns|TestAnnounceNowLogsOneSummaryLine)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 11 ] \
 && b=$(go test ./pkg/daemon -run '^(TestObserveEndpointAppliesStunResumeAtOnce|TestNATTypeAccessIsRaceFree)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 2 ] \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestOnResumeRefreshesSTUNAtOnce` | `pkg/discovery/dht_resume_test.go` | with a stubbed STUN that returns a new endpoint, `OnResume` changes the local endpoint at once; the ticker path with the same value does not (R7 stays) | — | S1, S3 |
| `TestOnResumeAnnouncesNow` | `pkg/discovery/dht_resume_test.go` | `OnResume` causes one `announce()` and one `SendAnnounce` per `GetActive` peer; none when `pe.conn == nil` or `Gossip` is off | — | S1, S3 |
| `TestOnResumeResetsRendezvousBackoff` | `pkg/discovery/dht_resume_test.go` | a peer in back-off may attempt a rendezvous after `OnResume` | — | S1, S3 |
| `TestDHTImplementsResumer` | `pkg/discovery/dht_resume_test.go` | the type satisfies `node.Resumer` and T4's assertion on the discovery field finds it | — | S1, S3 |
| `TestOnResumeBeforeStartAndAfterStopIsSafe` | `pkg/discovery/dht_resume_test.go` | `OnResume` on a never-started layer and on a stopped one returns without panic, STUN call or announce | — | S1, S3 |
| `TestOnResumeRunsOneSTUNPassPerCall` | `pkg/discovery/dht_resume_test.go` | without a ticker, N sequential `OnResume` calls run exactly N STUN passes and the first one applies the new endpoint at once | — | S1, S3, S4 |
| `TestOnResumeSerialisedWithStunRefreshLoop` | `pkg/discovery/dht_resume_test.go` | with a 2 ms ticker and many `OnResume` calls, the stubbed STUN query is never in flight twice (a mutant without `stunMu` turns it red); run under `-race` in the final fence | — | S1, S3, S4 |
| `TestOnResumeDropsAnswerAfterCancelDuringSTUN` | `pkg/discovery/dht_resume_test.go` | a layer cancelled during the query neither applies the endpoint nor announces | — | S1, S3 |
| `TestOnResumeStopWhileSTUNBlockedKeepsEndpoint` | `pkg/discovery/dht_resume_test.go` | `Stop` while the query is blocked leaves the endpoint unchanged when it returns | — | S1, S3 |
| `TestOnResumeCoalescesWhileAPassRuns` | `pkg/discovery/dht_resume_test.go` | many `OnResume` calls during one running pass start no second pass but still clear the back-off; a call after the pass starts the next | — | S1, S3 |
| `TestAnnounceNowLogsOneSummaryLine` | `pkg/discovery/dht_resume_test.go` | failures and invalid mesh IPs are summarised in one log line per wake | — | S1, S3 |
| `TestObserveEndpointAppliesStunResumeAtOnce` | `pkg/daemon/local_endpoint_test.go` | `ObserveEndpoint(EndpointSTUNResume, ep)` applies at once; `ObserveEndpoint(EndpointSTUN, ep)` still waits for three equal values | — | S1, S4 |
| `TestNATTypeAccessIsRaceFree` | `pkg/daemon/local_endpoint_test.go` | concurrent setter and getter calls of `NATType` complete under `-race` | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | T4's loop type-asserts `node.Resumer` on `d.dhtDiscovery`; the mutant that drops the method turns `TestDHTImplementsResumer` red |
| 3 — the caller can discover it | the log lines `[STUN] External endpoint changed` and `[Resume]` |
| 4 — it is used | every wake; the S6 step of T4 times it |

## Mutation Log
- 2026-11-16 · 1ccb7e8* · mutant killed · exit 1 · `pkg/discovery/dht.go` · OnResume passes EndpointSTUN · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d
- 2026-11-16 · 1ccb7e8* · mutant killed · exit 1 · `pkg/discovery/dht.go` · OnResume skips the announce · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d
- 2026-11-16 · 1ccb7e8* · mutant killed · exit 1 · `pkg/discovery/dht.go` · OnResume does not reset the back-off · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d
- 2026-11-16 · 1ccb7e8* · mutant survived · exit 0 · `pkg/discovery/dht.go` · refreshSTUNOnce drops stunMu · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · 1ccb7e8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · GetNATType reads without the lock · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d
- 2026-11-16 · 1ccb7e8* · mutant killed · exit 1 · `pkg/discovery/dht.go` · OnResume runs before Start · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d

## Invariants

- The ticker path of `stunRefreshLoop` behaves as ADR-0002 T9 leaves it: three equal observations for a STUN-only change.
- No new message type and no new wire byte; `announceNow` sends what the periodic announce and `SendAnnounce` send.
- `pkg/daemon` does not import `pkg/discovery`.
- No field, variable or counter accessed by more than one goroutine is bare: `atomic.*` or a lock on every access (M 2026-11-16); the task has a `-race` test for each such field it adds. Here: `NATType` (setter and getter under `endpointMu`), `refreshSTUNOnce` (`stunMu`), `rendezvousBackoff` (`d.mu`); `TestOnResumeSerialisedWithStunRefreshLoop` and `TestNATTypeAccessIsRaceFree` are the `-race` tests; `OnResume` starts one pass at a time (`resumeBusy` under `d.mu`), a burst only clears the back-off.

## Risks

- Applying a STUN result at once after a wake can publish a wrong endpoint if the network is not up yet; the next ticker pass and any peer reflection correct it (ADR-0002 R7 rank order).
- Moving `NATType` behind accessors touches every reader of the field in `pkg/daemon`, `pkg/discovery` and tests; the grep in S4 finds them, and `BACKLOG.md:18` (the known limit) is closed or reworded by T7's release note.
- `BACKLOG.md:18` says ADR-0005 owns the local node; the `NATType` accessors here are the minimal guard for the one new concurrent writer (`OnResume`) and are handed to ADR-0005 as they are.

## Stop Condition

Stop and ask if the `NATType` accessors turn out to need a lock order against `endpointMu` that `ObserveEndpoint` cannot keep, or if `ObserveEndpoint` no longer applies a non-`EndpointSTUN` source at once.

## Out of Scope

- Rebinding the transport socket or restarting WireGuard on a wake (deferred: docs/adr/BACKLOG.md).
- The multi-candidate local endpoint (permanent: boundary: ADR-0002 backlog row).

## Verification Log
- 2026-11-16 · 1ccb7e8* · exit 1 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:6138 · test-lock-sha256:3eba1d9befd4b8efb74c932afd8c6b8625bdc1397668bf8d8a30e55d9fb2c8ff · test-lock-file:docs/adr/locks/3eba1d9befd4b8efb74c932afd8c6b8625bdc1397668bf8d8a30e55d9fb2c8ff.lock
  ```
  --- last 10 line(s) of stderr (of 49 after folding 50 raw)
  --- PASS: TestOnResumeBeforeStartAndAfterStopIsSafe (0.00s)
      --- PASS: TestOnResumeBeforeStartAndAfterStopIsSafe/never_started (0.00s)
      --- PASS: TestOnResumeBeforeStartAndAfterStopIsSafe/stopped (0.00s)
  === RUN   TestOnResumeRacesWithStunRefreshLoop
  2026/11/16 12:20:47 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
      dht_resume_test.go:248: stunRefreshLoop never refreshed on its ticker
  --- FAIL: TestOnResumeRacesWithStunRefreshLoop (5.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	5.011s
  FAIL
  ```
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:12738
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:12364
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:13313
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:13227
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:12750
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:13617
- 2026-11-16 · 1ccb7e8* · exit 0 · `set -o pipefail …` · acceptance-sha256:576986cfc655550ae8f98308a0171ec8b8a34f7478835ae2f80a5b8056b36d0d · ms:8261
