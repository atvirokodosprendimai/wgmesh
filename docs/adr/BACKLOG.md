# ADR backlog

Deferred work from accepted decisions, so `adr-debt` can resurface it.

- ADR-0001 follow-up N+1: switch `SealEnvelope` to the AAD-bound form (`SealEnvelopeAAD`) in the release after the one that ships ADR-0001 T1 to T4. Needs a release note: nodes older than N cannot open the new packets.
- ADR-0001 follow-up N+2: refuse legacy (nil-AAD) envelopes in `OpenEnvelopeRaw`, one release after N+1. `EnvelopeCapabilityVersion` becomes 3 and `testdata/compat/envelope/v1` changes from "must open" to "must be refused with the legacy error"; the fixture files stay byte-identical.
- ADR-0001 deferred: change the mesh IP collision winner rule and bind a mesh IP to its public key (members can still claim other members' mesh IPs; see issue #864). Taken up by ADR-0002 (Proposed), which stages it as warn at N and enforce at N+1 and N+2.
- ADR-0001 deferred: a replay cache in the gossip reader (gossip is unicast, `pkg/discovery/gossip.go:222`) and the legacy counter in `daemon.status`.
- ADR-0001 deferred: stop a replayed HELLO from being answered every time (HELLO is exempt from the nonce cache because the retry loop at `pkg/discovery/exchange.go:527` resends identical bytes; a fix needs a per-HELLO sequence or token).
- ADR-0002 follow-up N+1: set `node.EnforceIPBinding = true` (claim order bound-first, refusal of a claim for a bound member's mesh IP) in the release after the one that ships ADR-0002 T1 to T10; needs a release note and `wgmesh_admit_warnings_total` at zero for a release.
- ADR-0002 follow-up N+2: set `node.EnforceRouteBinding = true` (drop route entries inside the mesh network or a member's mesh IP, and networks the receiver's `accept-routes` list does not approve) one release after N+1, with a release note.
- ADR-0002 deferred: keep and announce several reflexive local endpoints on a multi-egress network, the second half of issue #828 (needs a candidate list in the node state).
- ADR-0002 deferred: `install-service` validates `--advertise-routes` at install time with the parser the daemon uses, item 1 of issue #875.
- ADR-0007 deferred: #873 items 3 to 6 (`mesh list` default state path versus legacy mode, `status` saying "config only" or querying the RPC socket, `install-service` checking `systemctl is-active`, SIGHUP `log-level` applied through `configureLogging`); four independent fixes, item 3 changes a default path.
- ADR-0007 deferred: a deadline for the 37 `cmdExecutor` calls (`ip`, `iptables`, `systemctl`); `systemctl start` can take longer than the 10 seconds chosen for `wg`, so the calls are split first.
- ADR-0007 deferred: coverage of `pkg/discovery` (`registry.go`, `init.go` have no test file), `pkg/ssh/client.go`, `cmd/analytics-dashboard`, the SSH-driven functions of `pkg/wireguard/apply.go` and `persist.go`, and any coverage target or CI floor (issue #833), after the numbers of ADR-0007 T7.
