# Task ADR-0002-T7: One endpoint rule set: guessed endpoints rank last and are not re-announced (#839)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `node.ResolveEndpoint`, `PeerInfo.EndpointGuessed`, the lowest endpoint rank
**Consumes:** every production claim goes through `Offer` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

An endpoint built from the sender's address and a guessed port is stored below every advertised one, replaced by the first advertisement, and never put into `KnownPeers`; an empty first-hand endpoint never clears a known one; and no endpoint is ever built from the sender's UDP port.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/endpoint.go` | add | `ResolveEndpoint(advertised string, sender net.IP, fallbackPort int, preferSender bool) (endpoint string, guessed bool)`; one place for the three rules that differ today |
| `pkg/node/types.go` | edit | `PeerInfo.EndpointGuessed bool` |
| `pkg/node/store.go` | edit | `endpointMethodRank` returns 10 (below `cache`, 20) when the stored endpoint is guessed; the merge copies `EndpointGuessed` with the endpoint |
| `pkg/discovery/exchange.go` | edit | `resolvePeerEndpoint` (`:1058`) calls `ResolveEndpoint` with `daemon.DefaultWGPort`; `getKnownPeers` (`:1140`) skips the endpoint of a guessed peer |
| `pkg/discovery/lan.go` | edit | `resolveEndpoint` (`:235`) calls it with `preferSender` true (LAN keeps the sender's IP on purpose) |
| `pkg/discovery/gossip.go` | edit | `:306-310` uses it |
| `pkg/discovery/gossip.go` (standalone announce), `pkg/daemon/cache.go` | edit | added after review: a guessed endpoint is neither announced by the standalone gossip builder nor persisted by `SavePeerCache` (it would come back as a rank-cache advertisement, the #839 failure) |
| `pkg/node/endpoint_test.go`, `pkg/discovery/exchange_test.go`, `pkg/node/admit_test.go` | add / edit | tests below |

Selection: the three sources reach `ResolveEndpoint` through the functions they already call; reverting `resolvePeerEndpoint` to its old body turns `TestGuessedEndpointNotAnnounced` red.

## Ordered Steps

1. [S1] Add the six tests below with compiling stubs and run them red; `TestNoEndpointFromSenderPort` and `TestEmptyFirstHandEndpointKeepsKnown` describe behaviour that holds today (`resolvePeerEndpoint` never reads `sender.Port`; `store.go:117` guards the empty case), so they start green and pin it; the other four fail.
2. [S2] Implement `ResolveEndpoint` and replace the three local functions with calls to it; the exchange path keeps an advertised real host, LAN keeps the sender host.
3. [S3] Add `EndpointGuessed` and the rank; make `getKnownPeers` skip a guessed endpoint.
4. [S4] Reproduce the report once on a branch, if a test mesh is available: join two nodes with distinct `--listen-port` values and read `[DHT] SUCCESS! Found wgmesh peer ... at "..."`; write what was seen into the commit message, or write "not reproduced, no test mesh". The issue's claim that the DHT record carries the exchange port (52654) as the WireGuard endpoint is not reproduced by reading `5edcdd0`: `grep -n "pe.port\|Keys.GossipPort" pkg/discovery/exchange.go` shows the exchange port used to bind the socket (`:137-147`), to derive a control endpoint (`:743`) and to log. [proof: human: needs two real nodes; the gate is hermetic]
5. [S5] Run the mutants with `adr-verify --mutant`: the guess is stored at rank 30; `getKnownPeers` stops skipping; the exchange fallback uses the sender's UDP port instead of `DefaultWGPort` (`ResolveEndpoint` takes no port, so the mutant lives in the exchange wrapper); the merge forgets `EndpointGuessed`; a non-LAN guess is not marked guessed; LAN stops preferring the sender; the rank mutant (`return 10` to 30) is recorded in the round-2 log. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/node -run '^(TestResolveEndpointKeepsAdvertisedPort|TestResolveEndpointMarksGuess|TestGuessedEndpointLosesToAdvertisement|TestEmptyFirstHandEndpointKeepsKnown|TestNoEndpointFromSenderPort)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] \
 && b=$(go test ./pkg/discovery -run '^TestGuessedEndpointNotAnnounced$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && go test -race ./pkg/node ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestResolveEndpointKeepsAdvertisedPort` | `pkg/node/endpoint_test.go` | an advertised `host:51821` keeps port 51821 whatever the sender port is; an unspecified host takes the sender IP with the advertised port | — | S1, S2 |
| `TestResolveEndpointMarksGuess` | `pkg/node/endpoint_test.go` | an empty advertisement with a sender IP gives `sender:fallbackPort` and `guessed` true | — | S1, S2 |
| `TestGuessedEndpointLosesToAdvertisement` | `pkg/node/admit_test.go` | a guessed endpoint is replaced by an advertised one from any source, including `cache` and a transitive mention | — | S1, S3 |
| `TestEmptyFirstHandEndpointKeepsKnown` | `pkg/node/admit_test.go` | a first-hand offer with an empty endpoint leaves a known endpoint and its method unchanged | — | S1 |
| `TestNoEndpointFromSenderPort` | `pkg/node/endpoint_test.go` | for senders with ports 52654 and 1, no returned endpoint contains the sender's port unless the peer advertised it | — | S1, S2 |
| `TestGuessedEndpointNotAnnounced` | `pkg/discovery/exchange_test.go` | `getKnownPeers` lists the peer with an empty `WGEndpoint` when its endpoint is guessed | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `resolvePeerEndpoint`, `resolveEndpoint` and the gossip path call `ResolveEndpoint`; reverting one turns a named test red |
| 3 — the caller can discover it | doc comment; T11 amends the peer store and exchange specs |
| 4 — it is used | a guessed endpoint is not re-announced in `KnownPeers` (the peer keeps its key with an empty endpoint); `daemon.status` still shows the guess, and `pkg/daemon/cache.go` does not persist it |

## Mutation Log
- 2026-11-15 · dacefb2* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · getKnownPeers stops skipping a guessed endpoint: TestGuessedEndpointNotAnnounced must go red · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2
- 2026-11-15 · dacefb2* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the exchange path falls back to the sender UDP port · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2
- 2026-11-15 · dacefb2* · mutant killed · exit 1 · `pkg/node/store.go` · the merge forgets the guessed flag · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2
- 2026-11-15 · dacefb2* · mutant killed · exit 1 · `pkg/node/endpoint.go` · ResolveEndpoint does not mark the sender+fallback endpoint as guessed · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2
- 2026-11-15 · dacefb2* · mutant killed · exit 1 · `pkg/discovery/lan.go` · LAN takes the advertised host instead of the sender IP · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2

## Invariants

- `DefaultWGPort` stays the daemon's constant and is passed in; `pkg/node` does not import `pkg/daemon`.
- LAN keeps the sender IP over an advertised host; exchange keeps an advertised real host.
- A candidate with no endpoint never lowers or clears a known one.

## Risks

- A peer known only by a guess has no endpoint to announce; transitive discovery of it waits for its own advertisement. This is the intended trade (a wrong address is worse than none) and is why `getKnownPeers` keeps the key.
- Mixed fleet: a node before this release still announces its own guesses as advertised endpoints, so the wrong address keeps spreading until every node runs it (T11 release note); an empty port in an advertisement is now a guess in every mode (it was kept as is in the exchange mode); `daemon.status` and `peers.list` show a guess like any endpoint (no new field; BACKLOG row written by T11); `registry.go` copies `p.Endpoint` unguarded but has no production caller of `UpdatePeerListWithAll`.
- Test doubles: none.

## Stop Condition

Stop and ask if a source needs the guessed endpoint to establish the first WireGuard handshake (then the guess stays configured but is still not announced).

## Out of Scope

- Making discovery advertise the WireGuard listen port in the DHT record (no code was found that does the opposite; see S4).
- The local node's own endpoint (T9).

## Verification Log
- 2026-11-15 · dacefb2* · exit 1 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:976 · test-lock-sha256:8a4ab011b1ad2b641e9da438746c9beb4d2747a60df5655e7119a3b4718ba262 · test-lock-file:docs/adr/locks/8a4ab011b1ad2b641e9da438746c9beb4d2747a60df5655e7119a3b4718ba262.lock
  ```
  --- last 10 line(s) of stderr (of 34 after folding 35 raw)
      endpoint_test.go:44: preferSender=true: ResolveEndpoint("") = "203.0.113.1:51820", false; want 203.0.113.1:51820, true
  --- FAIL: TestResolveEndpointMarksGuess (0.00s)
  === CONT  TestResolveEndpointKeepsAdvertisedPort
      endpoint_test.go:31: preferSender keeps sender IP over a real host: ResolveEndpoint("198.51.100.7:51821") = "198.51.100.7:51821", false; want "203.0.113.1:51821", false  (x2)
  --- FAIL: TestResolveEndpointKeepsAdvertisedPort (0.00s)
  === CONT  TestEmptyFirstHandEndpointKeepsKnown
  --- PASS: TestEmptyFirstHandEndpointKeepsKnown (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.004s
  FAIL
  ```
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:11164
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:15612
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:12498
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:12534
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:12500
- 2026-11-15 · dacefb2* · exit 0 · `set -o pipefail …` · acceptance-sha256:130fd28db59c2b10cd54704a26c0a049f6e5ab4f0f088ba58cc3f9eadbafedb2 · ms:7919
