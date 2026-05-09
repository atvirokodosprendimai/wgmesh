# Envelope v1 Fixture

Regenerate with:

```bash
WGMESH_REGEN_FIXTURES=1 go test ./pkg/crypto/...
```

Regeneration writes to `testdata/compat/envelope/v<N>/`, where `N` is `EnvelopeCapabilityVersion`.
If that directory already exists, regeneration refuses to overwrite it. To replace an existing fixture while
developing, remove the directory first or run with `WGMESH_FORCE_REGEN=1`.

`SealEnvelope` uses a random nonce, so `envelope.bin` changes on each regeneration even when the decoded plaintext remains the same.
