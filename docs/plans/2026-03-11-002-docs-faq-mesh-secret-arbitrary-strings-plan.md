---
title: "docs: FAQ — why arbitrary strings work as mesh secrets"
type: docs
status: completed
date: 2026-03-11
---

# docs: FAQ — why arbitrary strings work as mesh secrets

## Overview

A user created a `.env` file with `MESH_SECRET=somethingimadeup` — no `wgmesh init`, no `wgmesh://v1/` prefix, just a hand-typed string. They ran docker compose and it formed a working mesh. Their surprise:

> "I thought the secret carries some encrypted info. Why does a made-up string work? And without the `wgmesh://v1/` prefix too!"

Another user in the same chat confirmed: with a *different* secret, they see only 1 peer (themselves) — so mesh isolation works correctly.

This plan adds FAQ documentation explaining how secrets work and a warning when the secret looks user-chosen rather than auto-generated.

## Problem Statement / Motivation

**User mental model mismatch:** Users expect the secret to be a structured token (like a JWT, API key, or signed blob) that encodes mesh metadata. In reality, it's just a passphrase — raw input keying material for HKDF-SHA256. Every mesh parameter (subnet, encryption keys, DHT identity, gossip port) is *derived* deterministically from this one string.

**Why it "just works":**
- `parseSecret()` strips the `wgmesh://v1/` prefix if present — but it's optional cosmetics
- `DeriveKeys()` accepts any string >= 16 characters
- HKDF doesn't care about the source of the string — it produces valid cryptographic output regardless

**The security gap:** HKDF is not a password-based KDF. It's designed for already-random input (RFC 5869 §4). A guessable passphrase means an attacker can brute-force the mesh secret offline — compute HKDF for candidate strings and compare against observed NetworkIDs or gossip traffic. There's no computational cost amplification (no iterations, no memory hardness).

**What works correctly:** Different secrets produce entirely different NetworkIDs, subnets, gossip keys, and PSKs. Two meshes with different secrets are cryptographically isolated — they literally can't discover each other.

## Proposed Solution

1. **FAQ entry** in `docs/FAQ.md` answering "How do mesh secrets work?"
2. **Warning on stderr** when the secret doesn't look auto-generated

## Acceptance Criteria

- [x] FAQ section in `docs/FAQ.md`: "How do mesh secrets work?"
  - Why any string >= 16 chars works
  - What the secret derives (all 10 parameters, briefly)
  - Why different secrets = completely separate meshes
  - Why user-chosen passphrases are risky (HKDF vs password KDFs)
  - Recommendation to use `wgmesh init --secret`
- [x] Warning printed to stderr when secret lacks `wgmesh://v1/` prefix
  - Suggests the secret is user-chosen rather than auto-generated
  - Recommends `wgmesh init --secret` for cryptographically strong generation
  - Does NOT block operation — informational only
  - Only prints once (at daemon startup, not on every reload)

## Technical Details

### What the secret derives (from `pkg/crypto/derive.go`)

From one secret string, HKDF-SHA256 with domain-separated info strings produces:

| Parameter | Purpose |
|-----------|---------|
| NetworkID | DHT discovery (infohash) |
| GossipKey | AES-256-GCM symmetric encryption |
| MeshSubnet | Deterministic 10.x.y.0/16 |
| IPv6 prefix | ULA /64 for mesh |
| MulticastID | LAN discovery group |
| PSK | WireGuard PresharedKey |
| GossipPort | 51821 + (derived % 1000) |
| RendezvousID | DHT rendezvous |
| MembershipKey | HMAC membership proofs |
| EpochSeed | Dandelion++ relay rotation |

### Current validation

- `MinSecretLength = 16` in `pkg/crypto/derive.go:16` — byte-length only
- No entropy estimation, no dictionary check, no strength scoring

### Where to add the warning

- `pkg/daemon/config.go` in `NewConfig()`, after `parseSecret()` and before `DeriveKeys()`
- Condition: `!strings.HasPrefix(opts.Secret, "wgmesh://")` (the original input, not the parsed value)
- Output: `log.Printf("[WARN] Secret does not use wgmesh:// format — it may be weak. Use 'wgmesh init --secret' for a cryptographically strong secret.")`
- Also consider: warning if parsed secret length < 32 (auto-generated are 43 chars)

### Future consideration (not in scope)

- Pre-hash user-chosen secrets through Argon2id before HKDF (proper password-based hardening)
- Entropy estimation with reject/warn thresholds
- Fixed application-specific HKDF salt instead of nil

## Sources

- `pkg/crypto/derive.go:54-116` — DeriveKeys with HKDF-SHA256
- `pkg/crypto/derive.go:16` — MinSecretLength = 16
- `pkg/daemon/config.go:185-200` — parseSecret (URI stripping)
- `service.go:345-374` — normalizeSecret (duplicate URI parser)
- RFC 5869 §4 — "HKDF is not designed to slow down dictionary attacks"
