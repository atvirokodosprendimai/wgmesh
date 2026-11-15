package crypto

// EnvelopeCapabilityVersion documents the compatibility contract for encrypted
// peer announcement envelopes.
//
// Version meanings:
//   - v1: initial AES-256-GCM envelope containing nonce and ciphertext.
//   - v2: readers also open envelopes whose message type is bound to the GCM
//     tag (additional data "wgmesh-v1\x00"+type, Envelope.V == 2) and mark
//     older envelopes Envelope.LegacyAAD. Writers switch to the bound form in a
//     later release; see docs/adr/ADR-0001-envelope-aad.
const EnvelopeCapabilityVersion = 2
