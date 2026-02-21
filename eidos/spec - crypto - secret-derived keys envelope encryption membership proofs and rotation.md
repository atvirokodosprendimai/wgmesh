---
tldr: One shared secret deterministically derives all network parameters via HKDF; AES-256-GCM envelopes carry time-bounded peer announcements; HMAC tokens prove membership; a signed rotation announcement allows live secret migration with a grace period.
category: core
---

# Crypto — key derivation, envelope encryption, membership proofs, and rotation

## Target

The cryptographic foundation of wgmesh: every key, port, subnet, and identity parameter
flows from a single shared secret. Five concerns: key derivation, peer-to-peer envelope
encryption, password-based file encryption, membership token proofs, and secret rotation.

## Behaviour

### Key derivation (`derive.go`)

`DeriveKeys(secret string) → DerivedKeys` produces all network parameters from one secret
(minimum 16 characters). All keys use HKDF-SHA256 with domain-separated `info` strings.

| Field | Algorithm | Use |
|---|---|---|
| `NetworkID [20]byte` | SHA256(secret)[0:20] | DHT infohash (BEP 5) |
| `GossipKey [32]byte` | HKDF(secret, "wgmesh-gossip-v1") | Symmetric envelope encryption |
| `MeshSubnet [2]byte` | HKDF(secret, "wgmesh-subnet-v1") | /16 subnet: `10.X.Y.0/16` |
| `MeshPrefixV6 [8]byte` | `0xfd` + HKDF(secret, "wgmesh-ipv6-prefix-v1", 7B) | ULA /64 prefix |
| `MulticastID [4]byte` | HKDF(secret, "wgmesh-mcast-v1") | LAN multicast group bytes |
| `PSK [32]byte` | HKDF(secret, "wgmesh-wg-psk-v1") | WireGuard PresharedKey |
| `GossipPort uint16` | `GossipPortBase + (HKDF(secret, "wgmesh-gossip-port-v1")[0:2] % 1000)` | In-mesh gossip port |
| `RendezvousID [8]byte` | SHA256(secret + "rv")[0:8] | GitHub Issue title discriminator |
| `MembershipKey [32]byte` | HKDF(secret, "wgmesh-membership-v1") | Membership token MAC key |
| `EpochSeed [32]byte` | HKDF(secret, "wgmesh-epoch-v1") | Dandelion++ relay rotation seed |

`GossipPortBase = 51821`, port range = 1000 → gossip port in `[51821, 52820]`.

### Mesh IP derivation

`DeriveMeshIP(meshSubnet, wgPubKey, secret)` → `10.X.Y.Z`:
- `hash = SHA256(wgPubKey + secret)`
- Third octet: `hash[0] XOR meshSubnet[1]`
- Fourth octet: `hash[1]`, clamped to `[1, 254]` (avoids .0 and .255)
- Result: `10.meshSubnet[0].<hash[0] XOR meshSubnet[1]>.<clamped hash[1]>`

`DeriveMeshIPv6(meshPrefixV6, wgPubKey, secret)` → `fdXX:…/64`:
- `hash = SHA256(wgPubKey + "|" + secret + "|ipv6")`
- Interface ID = `hash[0:8]` with `iid[0] = (iid[0] | 0x02) & 0xfe` (locally-administered bit)
- Result: `meshPrefixV6 || iid` (64-bit prefix + 64-bit host)

### Hourly-rotating network ID

`DeriveNetworkIDWithTime(secret, t)`:
- `hourEpoch = t.Unix() / 3600`
- `networkID = SHA256(secret + "||" + hourEpoch)[0:20]`

`GetCurrentAndPreviousNetworkIDs(secret)` returns both current and previous hour's IDs
for overlap during rotation (announced and queried under both during transitions).

---

### Envelope encryption (`envelope.go`)

AES-256-GCM symmetric encryption for all peer-to-peer messages.

**Seal:** `SealEnvelope(messageType, payload, gossipKey)`:
1. JSON-marshal payload.
2. Generate 12-byte random nonce.
3. AES-256-GCM encrypt (key = `GossipKey`).
4. Output: JSON-marshaled `Envelope{type, nonce, ciphertext}`.

**Open (raw):** `OpenEnvelopeRaw(data, gossipKey)`:
1. JSON-unmarshal `Envelope`.
2. AES-256-GCM decrypt.
3. Extract `{protocol, timestamp}` from plaintext JSON:
   - Reject if `protocol != "wgmesh-v1"`.
   - Reject if message age > 10 minutes (replay protection).
   - Reject if timestamp > 10 minutes in the future.

**Open (announcement):** `OpenEnvelope(data, gossipKey)`:
- Calls `OpenEnvelopeRaw`, then deserializes and validates the `PeerAnnouncement` payload.

**Validation limits (flood protection):**
- `MaxRoutableNetworks = 100` CIDRs per announcement.
- `MaxKnownPeers = 1000` transitive peers per announcement.
- Hostname ≤ 253 characters, printable ASCII only.
- WireGuard public key: valid base64, 32 decoded bytes.
- Endpoint: valid `host:port` with port in `[1, 65535]`.

---

### Password-based file encryption (`encrypt.go`)

PBKDF2-SHA256 (100,000 iterations) + AES-256-GCM for secrets at rest.

- `Encrypt(plaintext, password)`: random 32-byte salt → derive key → AES-256-GCM → base64(salt || nonce || ciphertext).
- `Decrypt(encoded, password)`: reverse.
- Used for encrypting state files and secrets stored on disk, not for peer-to-peer messages.

---

### Membership tokens (`membership.go`)

`GenerateMembershipToken(membershipKey, myPubkey)`:
- `HMAC-SHA256(membershipKey, pubkey || "|" || hourEpoch)`
- Proves the node possesses the shared secret to any verifier with the same secret.

`ValidateMembershipToken(membershipKey, theirPubkey, token)`:
- Accepts tokens for current hour, previous hour, and next hour (±1h clock skew tolerance).
- Used by the Lighthouse control plane for node admission.

---

### Secret rotation (`rotation.go`)

Allows a mesh operator to migrate all nodes to a new shared secret without downtime.

**Announcing rotation:**
`GenerateRotationAnnouncement(oldMembershipKey, newSecret, gracePeriod)`:
- SHA256(newSecret) as commitment (not the secret itself).
- Signed: `HMAC-SHA256(oldMembershipKey, newSecretHash || gracePeriod || timestamp)`.
- Timestamp valid ±1 hour.

**Validation:**
`ValidateRotationAnnouncement(oldMembershipKey, announcement)`:
- Timestamp check, then HMAC verification.

`VerifyNewSecret(newSecret, announcement)`:
- Confirms the operator's new secret matches the committed hash before accepting it.

**Grace period (`RotationState`):**
- Tracks old secret, new secret, start time, grace period duration, completion flag.
- `IsInGracePeriod()`: rotation in progress but not yet complete.
- `ShouldComplete()`: grace period elapsed — finalize by dropping old secret.
- During grace: nodes accept both old and new secret to handle staggered updates.

## Design

- One shared secret → all keys: no per-node key management, no PKI.
  Any node that knows the secret derives identical parameters and can communicate immediately.
- HKDF domain separation: each key uses a unique `info` string so leaking one key doesn't
  compromise others derived from the same secret.
- Mesh IP is deterministic from `(secret, pubkey)`: independent derivation by two nodes
  produces the same address. Collision resolution is handled by the daemon (see daemon support spec).
- AES-256-GCM provides authenticated encryption — decryption failure (wrong key, tampering)
  is a clean error, not a data leak.
- The 10-minute timestamp window prevents offline replay of captured packets.
  Short-lived enough to prevent significant replay window; long enough to tolerate clock drift.
- PBKDF2 (100k iterations) makes brute-forcing file-at-rest secrets expensive; this path
  is separate from the fast HKDF gossip key used for per-packet encryption.

## Interactions

- `pkg/daemon.Config.Keys` — consumers: all discovery, daemon reconciliation, epoch manager.
- `pkg/discovery.*` — `SealEnvelope`, `OpenEnvelope`, `OpenEnvelopeRaw`.
- `pkg/daemon.collision.go` — `DeriveMeshIP`.
- `pkg/daemon.epoch.go` — `EpochSeed`.
- `pkg/lighthouse.*` — `GenerateMembershipToken`, `ValidateMembershipToken`.
- `pkg/crypto.RotationState` — rotation lifecycle (used by daemon or CLI rotate command).

## Mapping

> [[pkg/crypto/derive.go]]
> [[pkg/crypto/envelope.go]]
> [[pkg/crypto/encrypt.go]]
> [[pkg/crypto/membership.go]]
> [[pkg/crypto/rotation.go]]
> [[pkg/crypto/password.go]]
