# Task ADR-0001-T1: Bind the envelope type to the GCM tag, keep legacy envelopes opening

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `SealEnvelopeAAD`, `Envelope.LegacyAAD`, `EnvelopeCapabilityVersion = 2`, `testdata/compat/envelope/v2`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

An envelope sealed with `SealEnvelopeAAD` cannot be opened after its `type` is changed, and envelopes sealed the old way still open and say that they are legacy.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/crypto/envelope.go` | edit | `Envelope.V`, `Envelope.LegacyAAD`, `SealEnvelopeAAD`, AAD selection in `OpenEnvelopeRaw` |
| `pkg/crypto/capability_version.go` | edit | `EnvelopeCapabilityVersion` 1 to 2 |
| `pkg/crypto/envelope_aad_test.go` | add | tests below |
| `testdata/compat/envelope/v2/envelope.bin`, `expected.json`, `README.md` | add | frozen AAD-bound fixture, beside the frozen `v1` |
| `pkg/crypto/envelope_compat_test.go` | edit | regeneration helper must write an AAD-bound envelope for `v2` |

Selection: `OpenEnvelopeRaw` is what every reader calls (`exchange.go`, `lan.go`, `gossip.go`, `registry.go`, `main.go`); the `v` field is what selects the mode, and `TestAADEnvelopeCannotBeDowngraded` fails if the nil-AAD branch accepts an AAD-bound packet.

## Ordered Steps

1. [S1] Add `pkg/crypto/envelope_aad_test.go` with the four tests below and run them red (the functions they call do not exist yet).
2. [S2] Add `V int` (`json:"v,omitempty"`) and `LegacyAAD bool` (`json:"-"`) to `Envelope`.
3. [S3] Add `SealEnvelopeAAD(messageType, payload, gossipKey)`: same as `SealEnvelope` but `gcm.Seal` with additional data `"wgmesh-v1\x00"+messageType` and `V` = 2. `SealEnvelope` stays legacy (a package variable `sealWithAAD`, false, that N+1 flips).
4. [S4] In `OpenEnvelopeRaw` choose the additional data from `envelope.V`: 2 uses the AAD above, 0 uses nil and sets `LegacyAAD`, any other value is refused with an error naming the value. A package variable `acceptLegacyAAD` (true) lets N+2 refuse legacy.
5. [S5] Bump `EnvelopeCapabilityVersion` to 2. Generate `testdata/compat/envelope/v2` once with `WGMESH_REGEN_FIXTURES=1 go test ./pkg/crypto/...`, commit it, and never regenerate it. `v1` must stay byte-identical.
6. [S6] Run the mutants with `adr-verify --mutant` (see Mutation Log) and the whole `pkg/crypto` package with `-race`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/crypto -run '^(TestEnvelopeTypeIsAuthenticated|TestLegacyEnvelopeOpensAndIsMarked|TestAADEnvelopeCannotBeDowngraded|TestEnvelopeCompatibilityFixtures)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go test -race ./pkg/crypto -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestEnvelopeTypeIsAuthenticated` | `pkg/crypto/envelope_aad_test.go` | for every `MessageType*` constant: seal as X, rewrite `type` to Y, open fails | — | S1, S3, S4 |
| `TestLegacyEnvelopeOpensAndIsMarked` | `pkg/crypto/envelope_aad_test.go` | an envelope sealed with nil AAD opens and `LegacyAAD` is true; an AAD one has it false | — | S2, S4 |
| `TestAADEnvelopeCannotBeDowngraded` | `pkg/crypto/envelope_aad_test.go` | removing `v` from an AAD envelope makes open fail | — | S4 |
| `TestEnvelopeCompatibilityFixtures` | `pkg/crypto/envelope_compat_test.go` | `v1` (legacy) and `v2` (AAD) both open with current code | — | S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `v` on the envelope selects the mode inside `OpenEnvelopeRaw`; deleting the `case 2` branch turns `TestEnvelopeTypeIsAuthenticated` red |
| 3 — the caller can discover it | `SealEnvelopeAAD` is exported with a doc comment; the spec is amended in T4 |
| 4 — it is used | nothing seals with AAD in production until N+1; T2 counts legacy opens so remaining legacy senders become visible |

## Mutation Log

## Invariants

- `v1` fixtures keep opening unchanged.
- The signatures of `SealEnvelope`, `OpenEnvelope` and `OpenEnvelopeRaw` do not change.
- The payload `protocol` string stays `wgmesh-v1`.
- An envelope with an unknown `v` is refused, never opened with nil AAD.

## Risks

- A fixture generated twice differs (random nonce); the regeneration helper refuses to overwrite an existing version, which is the intended guard.
- Test doubles: none; the tests use real AES-GCM and real fixture files.

## Stop Condition

Stop and ask if `v1` stops opening or if any caller of `OpenEnvelopeRaw` outside `pkg/crypto` needs a signature change.

## Out of Scope

- Using `LegacyAAD` in the exchange handler (T2).
- Flipping writers to AAD or refusing legacy (deferred: docs/adr/BACKLOG.md).

## Verification Log
