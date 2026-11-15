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
| `pkg/crypto/capability_version.go` | edit | `EnvelopeCapabilityVersion` 1 to 2, and a line in its comment saying what 2 adds |
| `pkg/crypto/envelope_aad_test.go` | add | tests below |
| `testdata/compat/envelope/v2/envelope.bin`, `expected.json`, `README.md` | add | frozen AAD-bound fixture, beside the frozen `v1` |
| `pkg/crypto/envelope_compat_test.go` | edit | regeneration helper seals `v2` with AAD; the replay asserts each version's own mode |

Selection: `OpenEnvelopeRaw` is what selects the mode, through the `v` field. `exchange.go` calls it directly; `lan.go`, `gossip.go`, `registry.go` and `main.go` reach it through `OpenEnvelope`, which calls it. `TestAADEnvelopeCannotBeDowngraded` fails if the nil-AAD branch accepts an AAD-bound packet, and deleting the `case 2` branch turns `TestEnvelopeTypeIsAuthenticated` red.

## Ordered Steps

1. [S1] Add `pkg/crypto/envelope_aad_test.go` with the three new tests, and add compiling stubs (`SealEnvelopeAAD` that returns an error, the `Envelope.LegacyAAD` field) so the first run is red on assertions and not on a build error. Tighten `TestEnvelopeCompatibilityFixtures` as described in the Tests table. Run them red.
2. [S2] Add `V int` (`json:"v,omitempty"`) and `LegacyAAD bool` (`json:"-"`) to `Envelope`.
3. [S3] Implement `SealEnvelopeAAD(messageType, payload, gossipKey)`: same as `SealEnvelope` but `gcm.Seal` with additional data `"wgmesh-v1\x00"+messageType` and `V` = 2. `SealEnvelope` stays legacy, behind a package variable `sealWithAAD` (false) that release N+1 flips.
4. [S4] In `OpenEnvelopeRaw` choose the additional data from `envelope.V`: 2 uses the AAD above, 0 uses nil and sets `LegacyAAD`, any other value is refused with an error naming the value. A package variable `acceptLegacyAAD` (true) lets release N+2 refuse legacy.
5. [S5] Bump `EnvelopeCapabilityVersion` to 2 and update its comment. Generate `testdata/compat/envelope/v2` once with `WGMESH_REGEN_FIXTURES=1 go test ./pkg/crypto/...`, commit it, and never regenerate it. `v1` must stay byte-identical (`git diff --exit-code testdata/compat/envelope/v1`).
6. [S6] Run the mutants with `adr-verify --mutant`: delete the `case 2` branch; make the nil-AAD branch also accept `v` = 2. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/crypto -run '^(TestEnvelopeTypeIsAuthenticated|TestLegacyEnvelopeOpensAndIsMarked|TestAADEnvelopeCannotBeDowngraded|TestEnvelopeCompatibilityFixtures)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && git diff --exit-code HEAD -- testdata/compat/envelope/v1 && test -f testdata/compat/envelope/v2/envelope.bin && go test -race ./pkg/crypto -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestEnvelopeTypeIsAuthenticated` | `pkg/crypto/envelope_aad_test.go` | for every `MessageType*` constant: seal as X, rewrite `type` to Y, open fails | — | S1, S3, S4 |
| `TestLegacyEnvelopeOpensAndIsMarked` | `pkg/crypto/envelope_aad_test.go` | an envelope sealed with nil AAD opens and `LegacyAAD` is true; an AAD one has it false | — | S1, S2, S4 |
| `TestAADEnvelopeCannotBeDowngraded` | `pkg/crypto/envelope_aad_test.go` | removing `v` from an AAD envelope, or setting `v` to another value, makes open fail | — | S1, S4 |
| `TestEnvelopeCompatibilityFixtures` | `pkg/crypto/envelope_compat_test.go` | every fixture directory present: `v1` opens with `LegacyAAD` true; `v2` opens with `LegacyAAD` false and its `envelope.bin` carries `"v":2`; the `v2` directory must exist | — | S1, S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `v` on the envelope selects the mode inside `OpenEnvelopeRaw`; deleting the `case 2` branch turns `TestEnvelopeTypeIsAuthenticated` red |
| 3 — the caller can discover it | `SealEnvelopeAAD` is exported with a doc comment; the spec is amended in T4 |
| 4 — it is used | `SendGoodbye` uses it from T2; the other writers only at release N+1 |

## Mutation Log
- 2026-11-15 · 23cec9a* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · type binding removed on seal and open: a retyped AAD envelope must still be refused · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c
- 2026-11-15 · 23cec9a* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · unknown v opened with nil additional data instead of refused · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c
- 2026-11-15 · 23cec9a* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · legacy envelope no longer marked LegacyAAD, so the GOODBYE gate in T2 would have nothing to read · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c
- 2026-11-15 · 23cec9a* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · acceptLegacyAAD false no longer refuses legacy envelopes · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c
- 2026-11-15 · d8082c6* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · SealEnvelope ignores the sealWithAAD switch, so release N+1 could not flip it · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c
- 2026-11-15 · d8082c6* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · invalid UTF-8 message types are sealed and then fail to open · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c

## Invariants

- `v1` fixtures keep opening unchanged and are byte-identical.
- The signatures of `SealEnvelope`, `OpenEnvelope` and `OpenEnvelopeRaw` do not change.
- The payload `protocol` string stays `wgmesh-v1`.
- An envelope with an unknown `v` is refused, never opened with nil AAD.
- `sealWithAAD` and `acceptLegacyAAD` are switches for later releases and seams for tests; nothing in production code sets them in this release.

## Risks

- A fixture generated twice differs (random nonce); the regeneration helper refuses to overwrite an existing version, which is the intended guard.
- Test doubles: none; the tests use real AES-GCM and real fixture files.

## Stop Condition

Stop and ask if `v1` stops opening or if any caller of `OpenEnvelope` or `OpenEnvelopeRaw` outside `pkg/crypto` needs a signature change.

## Out of Scope

- Using `LegacyAAD` in the exchange handler (T2).
- Flipping writers to AAD or refusing legacy (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-15 · d91955e* · exit 1 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:716 · test-lock-sha256:9901acf87c835b5ca0bd02b937675a4b5174cf2b6ac28d0959eeedc05ad9c2b7 · test-lock-file:docs/adr/locks/9901acf87c835b5ca0bd02b937675a4b5174cf2b6ac28d0959eeedc05ad9c2b7.lock
  ```
  --- last 10 line(s) of stderr (of 18 after folding 18 raw)
      envelope_aad_test.go:112: SealEnvelopeAAD not implemented
  --- FAIL: TestAADEnvelopeCannotBeDowngraded (0.00s)
  === RUN   TestEnvelopeCompatibilityFixtures
  === RUN   TestEnvelopeCompatibilityFixtures/v1
      envelope_compat_test.go:183: v1 fixture did not open as a legacy envelope
  --- FAIL: TestEnvelopeCompatibilityFixtures (0.00s)
      --- FAIL: TestEnvelopeCompatibilityFixtures/v1 (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/crypto	0.003s
  FAIL
  ```
- 2026-11-15 · 23cec9a* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:8356
- 2026-11-15 · 23cec9a* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:9816
- 2026-11-15 · 23cec9a* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:8743
- 2026-11-15 · 23cec9a* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:9065
- 2026-11-15 · d8082c6* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:16675
- 2026-11-15 · d8082c6* · exit 0 · `set -o pipefail …` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:10359
- 2026-11-15 · d8082c6* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:acd930e62c31a50d666fbfcbdbfc76e08a0163d34ae963182a9af9d31e27186c · ms:1 · test-lock-sha256:58abe5fdc63c0b8e0c50103184ddffafc4d2d3944082e752f1fd7bc7909c35d0 · test-lock-file:docs/adr/locks/58abe5fdc63c0b8e0c50103184ddffafc4d2d3944082e752f1fd7bc7909c35d0.lock · test-lock-kind:replace
