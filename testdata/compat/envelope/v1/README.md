# Envelope v1 Fixture

Regenerate with:

```bash
WGMESH_REGEN_FIXTURES=1 go test ./pkg/crypto/...
```

`SealEnvelope` uses a random nonce, so `envelope.bin` changes on each regeneration even when the decoded plaintext remains the same.
