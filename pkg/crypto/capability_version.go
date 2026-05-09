package crypto

// EnvelopeCapabilityVersion documents the compatibility contract for encrypted
// peer announcement envelopes.
//
// Version meanings:
//   - v1: initial AES-256-GCM envelope containing nonce and ciphertext.
const EnvelopeCapabilityVersion = 1
