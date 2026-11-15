# Envelope v2 Fixture

Frozen. `envelope.bin` is an ANNOUNCE envelope with `"v":2`: its message type is bound to the GCM tag
(additional data `"wgmesh-v1\x00" + type`). `TestEnvelopeCompatibilityFixtures` opens it with current code and
requires that it opens as AAD-bound (not legacy) and that the file carries `"v":2`.

Generated once with `WGMESH_REGEN_FIXTURES=1 go test ./pkg/crypto/...` when `EnvelopeCapabilityVersion` became 2.
Never regenerate it: a fixture exists to prove that bytes written by an older build still open. See
`docs/adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md`.
