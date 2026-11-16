# Task ADR-0002-T9: Observe the local endpoint through one rule instead of eight writers (#828)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `LocalNode.ObserveEndpoint`
**Consumes:** every production claim goes through `Offer` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The node's announced endpoint does not flip between a STUN answer and a peer reflection on a multi-egress network: a peer reflection applies at once, a STUN-only change of a usable endpoint applies after three identical observations in a row.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/daemon.go` | edit | `daemon.LocalNode.ObserveEndpoint(source EndpointSource, endpoint string) bool` under the existing `endpointMu`; the `EndpointSource` type and its constants `interface` (a global IPv6 address), `reflection`, `stun` live in `pkg/daemon` with it (decision 10); the pending STUN value and its count are fields guarded by the same mutex |
| `pkg/discovery/dht.go` | edit | the six observation sites `:255`, `:271`, `:284`, `:420`, `:440`, `:457` call it (`interface` for the IPv6 ones, `stun` for the others) |
| `pkg/discovery/exchange.go` | edit | `applyObservedEndpoint` `:494` calls it with `reflection` |
| `pkg/daemon/local_endpoint_test.go` | add | tests below |

`pkg/daemon/daemon.go:1738` keeps `SetEndpoint`: it sets the initial `0.0.0.0:<port>`, not an observation. Selection: the seven call sites; reverting one to `SetEndpoint` is caught by the guard step S4.

## Ordered Steps

1. [S1] Add the five tests below with a compiling stub that applies every observation; run red; `TestEndpointDoesNotFlapOnAlternatingObservations` feeds 20 alternating STUN and reflection observations and counts changes (today 20).
2. [S2] Implement `ObserveEndpoint` (it sets the field under `endpointMu` itself and does not call `SetEndpoint`, which would deadlock and keep the `SetEndpoint` count above 1): any source applies at once while the current endpoint is empty or unspecified (`0.0.0.0`, `::`), so the first STUN result after start-up is not delayed; `interface` and `reflection` apply at once and reset the pending STUN count; `stun` equal to the current value resets the count; `stun` different from the current value applies only on the third consecutive identical observation.
3. [S3] Switch the seven sites. [proof: acceptance]
4. [S4] [proof: acceptance] Run `grep -rn "\.SetEndpoint(" --include=*.go . | grep -v _test.go | grep -v "func (n" | wc -l`; it prints 1 (the initial set). Record the count in the commit message.
5. [S5] Run the mutants with `adr-verify --mutant`: `stun` applies at once; a reflection resets nothing; the count is 1 instead of 3. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestEndpointDoesNotFlapOnAlternatingObservations|TestStunNeedsThreeConsecutiveObservations|TestReflectionAppliesAtOnce|TestInterfaceSourceAppliesAtOnce|TestFirstStunAppliesOverUnspecified)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] && [ "$(grep -rn "\.SetEndpoint(" --include=*.go . | grep -v _test.go | grep -v "func (n" | wc -l)" -eq 1 ] && go test -race ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestEndpointDoesNotFlapOnAlternatingObservations` | `pkg/daemon/local_endpoint_test.go` | 20 alternating STUN and reflection observations change the endpoint at most once | — | S1, S2 |
| `TestStunNeedsThreeConsecutiveObservations` | `pkg/daemon/local_endpoint_test.go` | two identical new STUN values change nothing, the third applies | — | S1, S2 |
| `TestReflectionAppliesAtOnce` | `pkg/daemon/local_endpoint_test.go` | one reflection applies immediately and discards a pending STUN value | — | S1, S2 |
| `TestInterfaceSourceAppliesAtOnce` | `pkg/daemon/local_endpoint_test.go` | a global IPv6 address from the interface applies at once | — | S1, S2 |
| `TestFirstStunAppliesOverUnspecified` | `pkg/daemon/local_endpoint_test.go` | with the current endpoint `0.0.0.0:51820` the first STUN observation applies at once | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the seven call sites; the acceptance fence counts the remaining `SetEndpoint` calls, which fails if one is added |
| 3 — the caller can discover it | doc comment; T11 amends the DHT core spec |
| 4 — it is used | the `[STUN] External endpoint changed` log line stops repeating each cycle on a dual-egress network |

## Mutation Log
- 2026-11-15 · 305dd6a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · stun applies at once: TestStunNeedsThreeConsecutiveObservations and TestEndpointDoesNotFlap... must go red · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · 305dd6a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a reflection (and interface) resets no pending STUN count: TestReflectionAppliesAtOnce must go red · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · 305dd6a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the confirmation count is 1 instead of 3 · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · 305dd6a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the first STUN observation over an unspecified endpoint is delayed: TestFirstStunAppliesOverUnspecified must go red · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · 305dd6a* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · applyObservedEndpoint reverts to SetEndpoint: the fence's SetEndpoint count must fail · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: stun applies at once · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: reflection/interface/equal-stun reset no pending count · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: count 1 not 3 · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: first STUN over unspecified delayed · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · r2: applyObservedEndpoint reverts to SetEndpoint · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · r2: any STUN counts as consecutive · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821
- 2026-11-15 · c8e3005* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · r2: a reflected unspecified address is accepted · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821

## Invariants

- A peer reflection still ignores loopback, private and link-local addresses and still does not downgrade a global IPv6 endpoint (the guards in `applyObservedEndpoint` stay).
- The WireGuard port in an observed endpoint still comes from the node's own configuration.
- `GetEndpoint` and `SetEndpoint` keep their signatures.

## Risks

- A real STUN-only change takes three refresh cycles (60 seconds each at `dht.go:410`), so up to three minutes; a reflection from a peer applies at once.
- Test doubles: none; the tests call `ObserveEndpoint` on a real `daemon.LocalNode` (ADR decision 10: `node.LocalNode` has no user; ADR-0005 merges the two types later).

## Stop Condition

Stop and ask if a test mesh shows that reflection is wrong more often than STUN (then the preference needs the observation counts the deferred multi-candidate state would give).

## Out of Scope

- Keeping both reflexive addresses and announcing them as candidates, the second half of #828 (deferred: docs/adr/BACKLOG.md).
- The initial endpoint at start-up (permanent: boundary: it is configuration, not an observation).

## Verification Log
- 2026-11-15 · 305dd6a* · exit 1 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:1629 · test-lock-sha256:4a8f3329837cc831d4d112de73b720438fea5d5ae0a4ebbc3cf73c901863e061 · test-lock-file:docs/adr/locks/4a8f3329837cc831d4d112de73b720438fea5d5ae0a4ebbc3cf73c901863e061.lock
  ```
  --- last 10 line(s) of stderr (of 16 after folding 16 raw)
  === RUN   TestReflectionAppliesAtOnce
      local_endpoint_test.go:64: pending STUN count survived a reflection
  --- FAIL: TestReflectionAppliesAtOnce (0.00s)
  === RUN   TestInterfaceSourceAppliesAtOnce
  --- PASS: TestInterfaceSourceAppliesAtOnce (0.00s)
  === RUN   TestFirstStunAppliesOverUnspecified
  --- PASS: TestFirstStunAppliesOverUnspecified (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.010s
  FAIL
  ```
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:13382
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:11933
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:13276
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:16144
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:14020
- 2026-11-15 · 305dd6a* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:8436
- 2026-11-15 · c8e3005* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:0 · test-lock-sha256:6451e99026c0bdb6060ab3e2ad90793e2da1dfc7d1fda8dd738d1cf53246e0b3 · test-lock-file:docs/adr/locks/6451e99026c0bdb6060ab3e2ad90793e2da1dfc7d1fda8dd738d1cf53246e0b3.lock · test-lock-kind:replace
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:11998
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:10804
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:9829
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:10286
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:10584
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:11190
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:10703
- 2026-11-15 · c8e3005* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:7193
- 2026-11-16 · 51e018f* · exit 0 · `set -o pipefail …` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:6805
- 2026-11-16 · c192dcd* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:41d029dd952a9c13d808561361f4e7f2283b7b68ae19f978740c39476ed7f821 · ms:1 · test-lock-sha256:ddaeca0658bd8f01ff52add798ee51db68ed101703a21296b492c9ee382aefa5 · test-lock-file:docs/adr/locks/ddaeca0658bd8f01ff52add798ee51db68ed101703a21296b492c9ee382aefa5.lock · test-lock-kind:replace
