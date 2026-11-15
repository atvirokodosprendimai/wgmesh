# Task ADR-0006-T5: Decide a node's endpoint in one pure function, and skip the probe for private hosts (#866)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `resolveEndpoint`, `endpointDecision`
**Consumes:** `NodeResult`, `deployNode` (T4); `pkg/ssh/sshtest` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The three writes at `deploy.go:139-147` become one table-tested function: an endpoint already in the state wins, a mesh whose SSH hosts are all private IP addresses gets `<ssh host>:<listen port>` for every node, the old public-IP rule applies otherwise, and a node with a private SSH host is not probed with curl.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/mesh/endpoint.go` | add | `resolveEndpoint(node *Node, allPrivate bool, publicIP string, probed bool) endpointDecision` with `Endpoint`, `BehindNAT`, `Reason`; `isPrivateHost(host string) bool` (`net.ParseIP` then `IsPrivate`; a DNS name or loopback is not private); the endpoint is built with `net.JoinHostPort` |
| `pkg/mesh/deploy.go` | edit | `detectEndpoints` computes `allPrivate` once over the non-failed nodes, skips `ssh.DetectPublicIP` for a private SSH host, calls `resolveEndpoint` for every node including the local one, and prints the `Reason` |
| `pkg/mesh/endpoint_test.go` | add | tests below |

Selection: `detectEndpoints` calls `resolveEndpoint`; reverting the call restores the old writes and turns `TestAllLANMeshRendersEndpoints` red.

## Ordered Steps

1. [S1] Add the tests with a stub `resolveEndpoint` that reproduces today's rule; run red (the all-private rows and the probe-skip test fail on assertions).
2. [S2] Implement `isPrivateHost` and `resolveEndpoint` in the order of the ADR's Decision item 4; for the local node there is no probe and `publicIP` is empty.
3. [S3] Wire it into `detectEndpoints`; a node that failed in `detect` (T4) is excluded from `allPrivate` and keeps whatever endpoint the state holds.
4. [S4] Render the config for two private nodes and assert both peers carry an `Endpoint` (`TestAllLANMeshRendersEndpoints`).
5. [S5] Run the mutants with `adr-verify --mutant`: `allPrivate` ignored; the operator-set endpoint is overwritten; the probe is not skipped; `"%s:%d"` replaces `JoinHostPort`; a DNS name is classed private. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/mesh -run '^(TestResolveEndpointTable|TestPrivateHostClassification|TestDetectEndpointsSkipsProbeForPrivateHost|TestAllLANMeshRendersEndpoints|TestMixedMeshKeepsTodaysRule)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestResolveEndpointTable` | `pkg/mesh/endpoint_test.go` | rows: operator-set wins when public IP equals SSH host and when it differs; all-private gives `192.168.1.10:51820` for the local node too; mixed mesh with a private host gives none; public IP equal to SSH host gives `<host>:<port>`; probe failed gives NAT, no endpoint; an IPv6 literal host gives `[fd00::1]:51820` | — | S1, S2 |
| `TestPrivateHostClassification` | `pkg/mesh/endpoint_test.go` | `10.0.0.1`, `172.16.0.1`, `192.168.0.1`, `fd00::1` are private; `8.8.8.8`, `100.64.0.1`, `127.0.0.1`, `169.254.1.1`, `node.example.com` and the empty string are not | — | S1, S2 |
| `TestDetectEndpointsSkipsProbeForPrivateHost` | `pkg/mesh/endpoint_test.go` | against a server on `127.0.0.1` classed private through the test seam for the address, no recorded command contains `curl`; for a node classed public, one does | — | S1, S3 |
| `TestAllLANMeshRendersEndpoints` | `pkg/mesh/endpoint_test.go` | two nodes with private SSH hosts: each generated config has the other peer with `Endpoint = <ssh host>:51820` | — | S1, S3, S4 |
| `TestMixedMeshKeepsTodaysRule` | `pkg/mesh/endpoint_test.go` | one private and one public SSH host: the peer config for the private node has no `Endpoint`, as on `b8b83f3` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `detectEndpoints` calls `resolveEndpoint`; the revert turns `TestAllLANMeshRendersEndpoints` red |
| 3 — the caller can discover it | the deploy prints the reason per node; T6 documents the rule and `public_endpoint` |
| 4 — it is used | every `wgmesh -deploy` |

## Mutation Log
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/endpoint.go` · an all-private mesh gets no endpoints: TestAllLANMeshRendersEndpoints must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/endpoint.go` · public_endpoint no longer wins: the operator-wins row must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a private host is probed again: TestDetectEndpointsSkipsProbeForPrivateHost must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/endpoint.go` · an IPv6 SSH host is not bracketed: the IPv6 row must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/endpoint.go` · a DNS name is classed private: TestPrivateHostClassification must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f
- 2026-11-16 · 0563b3e* · mutant killed · exit 1 · `pkg/mesh/endpoint.go` · rule (c) removed: the public-IP-equals-host row must go red · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f

## Invariants

- A mesh with at least one non-private or DNS-named SSH host gets the endpoints it gets on `b8b83f3`, except that an endpoint already in the state is no longer overwritten (M's decision 4).
- No endpoint is written to the state file; detection stays in memory.
- A node's endpoint never comes from the address of the machine running the deploy.

## Risks

- A seam is needed to treat a loopback test server as private; it is a package variable for the classifier used only by the test, and `TestPrivateHostClassification` exercises the real classifier.
- An SSH host reached through a bastion gets an endpoint nobody can reach; the tunnel then starts only from the other side, as without an endpoint (ADR Risks).
- Test doubles: the fake server of T2.

## Stop Condition

Stop and ask if a test shows the local node must not receive an endpoint, or if the all-private rule changes the endpoint of a mixed mesh.

## Out of Scope

- Candidate lists of several endpoints per node (permanent: boundary: the centralized state has one `public_endpoint`).
- Resolving a DNS-named SSH host to classify it (permanent: boundary: the operator's resolver is not consulted; `public_endpoint` is the override).
- The daemon's STUN and endpoint rules (permanent: boundary: ADR-0002 owns them).

## Verification Log
- 2026-11-16 · 0563b3e* · exit 1 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:1086 · test-lock-sha256:fd1d4c1fb93a75a77bb36d5f84041b5207328c7617f160d9375f8d73baa3dc0d · test-lock-file:docs/adr/locks/fd1d4c1fb93a75a77bb36d5f84041b5207328c7617f160d9375f8d73baa3dc0d.lock
  ```
  --- last 10 line(s) of stderr (of 108 after folding 108 raw)
            Restarting wg-quick@wg0 service
            ✓ Deployed successfully

  --- FAIL: TestAllLANMeshRendersEndpoints (0.01s)
  === RUN   TestMixedMeshKeepsTodaysRule
      endpoint_test.go:203: the private host was probed: ["hostname" "hostname -f" "curl -s -4 --max-time 5 ifconfig.me || curl -s -4 --max-time 5 icanhazip.com || true" "which wg" "wg show wg0 dump 2>/dev/null || true" "cat > /etc/wireguard/wg0.conf && chmod 600 /etc/wireguard/wg0.conf" "systemctl enable wg-quick@wg0" "systemctl restart wg-quick@wg0"]
  --- FAIL: TestMixedMeshKeepsTodaysRule (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	0.043s
  FAIL
  ```
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:11947
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:11536
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:12586
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:11529
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:11499
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:11185
- 2026-11-16 · 0563b3e* · exit 0 · `set -o pipefail …` · acceptance-sha256:1bd81b1f349b8acf76dffd883dd2608cf65915d9b7c77ab0030ebacb7b44b57f · ms:9308
