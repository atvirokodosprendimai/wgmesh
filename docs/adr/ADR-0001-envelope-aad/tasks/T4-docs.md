# Task ADR-0001-T4: Amend the crypto and exchange specs and the operator notes

**Depends-on:** T1, T2, T3
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `EnvelopeCapabilityVersion = 2`, `v2` fixture (T1); `nonceCache` (T3)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The crypto spec, the exchange spec and `docs/` describe the AAD-bound envelope, the legacy mode, the nonce cache and the three-release rollout as built, with the release note for N, and a test fails if either spec stops naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md` | edit | envelope section: `v` field, AAD bytes, legacy mode, capability version 2 |
| `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` | edit | GOODBYE gate, AAD `SendGoodbye`, nonce cache and the HELLO exemption |
| `docs/envelope-rollout.md` | add | operator note and release note for N: what each of N, N+1, N+2 changes, what an old node does with an AAD packet per reader (exchange logs "possibly DHT or wrong secret", LAN and gossip drop silently, `main.go` prints "wrong secret?"), when the legacy counter is meaningful |
| `pkg/crypto/spec_aad_test.go` | add | the specs and the note must name the rules |

## Ordered Steps

1. [S1] Add `pkg/crypto/spec_aad_test.go` with `TestSpecNamesAADRollout`: it reads both specs and `docs/envelope-rollout.md` and fails when `SealEnvelopeAAD`, `LegacyAAD`, `EnvelopeCapabilityVersion` (crypto spec), `nonce cache` and `GOODBYE` (exchange spec) or the strings `N+1` and `N+2` (note) are missing. Run it red.
2. [S2] Amend the crypto spec envelope section and the exchange spec.
3. [S3] Write `docs/envelope-rollout.md` including the release note for N.
4. [S4] Run the mutant that removes the AAD paragraph from the crypto spec, and the one that removes the nonce-cache paragraph from the exchange spec. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/crypto -run '^TestSpecNamesAADRollout$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/envelope-rollout.md
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesAADRollout` | `pkg/crypto/spec_aad_test.go` | both specs and the operator note name the AAD rule, the legacy flag, the nonce cache, the GOODBYE gate and the rollout | — | S1, S2, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | the note is linked from the README security section |
| 4 — it is used | nothing measures this yet |

## Mutation Log
- 2026-11-15 · 21b174a* · mutant killed · exit 1 · `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md` · the crypto spec stops naming SealEnvelopeAAD · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b
- 2026-11-15 · 21b174a* · mutant killed · exit 1 · `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` · the exchange spec stops naming the nonce cache · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b
- 2026-11-15 · 21b174a* · mutant killed · exit 1 · `docs/envelope-rollout.md` · the operator note stops naming ReplayedDatagrams · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b

## Invariants

- The documents describe what T1 to T3 built, not what N+1 or N+2 will do, except in the rollout note.

## Risks

- Documentation drifts from the code later; the test only checks that named terms exist.
- Test doubles: none.

## Stop Condition

Stop and ask if T1 to T3 changed a name this task documents.

## Out of Scope

- Documenting N+1 and N+2 as done (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-15 · 1f3c4a0* · exit 1 · `set -o pipefail …` · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b · ms:1036 · test-lock-sha256:67e6c78adfe4b3b1406f36490fa1229396c291b7c196a6eec1d5a58d93cfa6da · test-lock-file:docs/adr/locks/67e6c78adfe4b3b1406f36490fa1229396c291b7c196a6eec1d5a58d93cfa6da.lock
  ```
  --- last 10 line(s) of stderr (of 12 after folding 12 raw)
      spec_aad_test.go:40: eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md does not mention "LegacyAAD"
      spec_aad_test.go:40: eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md does not mention "EnvelopeCapabilityVersion"
      spec_aad_test.go:40: eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md does not mention "nonce cache"
      spec_aad_test.go:40: eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md does not mention "LegacyAAD"
      spec_aad_test.go:40: eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md does not mention "SealEnvelopeAAD"
      spec_aad_test.go:35: docs/envelope-rollout.md: open ../../docs/envelope-rollout.md: no such file or directory
  --- FAIL: TestSpecNamesAADRollout (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/crypto	0.003s
  FAIL
  ```
- 2026-11-15 · 21b174a* · exit 0 · `set -o pipefail …` · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b · ms:1759
- 2026-11-15 · 21b174a* · exit 0 · `set -o pipefail …` · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b · ms:1382
- 2026-11-15 · 21b174a* · exit 0 · `set -o pipefail …` · acceptance-sha256:407840be96a0ed192bfc7b4e370262d8b752101c7286a9416bf8a58bba6ea08b · ms:1872
