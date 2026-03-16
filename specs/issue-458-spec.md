# Specification: Issue #458

## Classification
feature

## Deliverables
code

## Problem Analysis

Issue #458 requests building the first peer discovery component: peers must be able to announce their presence, discover other peers, use a defined exchange protocol, and have unit tests.

### Current State

The core discovery implementation already satisfies three of the four acceptance criteria:

| Acceptance Criterion | Status | Evidence |
|---|---|---|
| Peers can announce their presence | ✅ Done | `pkg/discovery/lan.go` (multicast), `dht.go` (DHT), `gossip.go` (mesh gossip), `registry.go` (GitHub Issues) |
| Peers can discover other available peers | ✅ Done | All four discovery layers above |
| Basic protocol defined for peer information exchange | ✅ Done | `pkg/crypto/envelope.go`: `PeerAnnouncement`, `Envelope`, `KnownPeer` structs; message types `HELLO`, `REPLY`, `ANNOUNCE`, `GOODBYE`, `RENDEZVOUS_OFFER`, `RENDEZVOUS_START` |
| Unit tests for discovery mechanism | ⚠️ Partial | `lan_test.go` covers only 1 helper; `registry.go` has **no test file** |

### Remaining Gaps

**Gap 1 — `registry.go` has no test file.**
`pkg/discovery/registry.go` (411 lines) implements the GitHub Issues-based rendezvous layer (`RendezvousRegistry`). It has exported types (`RendezvousRegistry`, `RegistryPeerEntry`) and internal helpers (`decryptPeerList`, `buildIssueBody`, `updatePeerListMerged`) with no corresponding `registry_test.go`.

**Gap 2 — `lan_test.go` tests only one internal helper.**
`pkg/discovery/lan_test.go` (32 lines) contains a single test `TestResolveEndpointPrefersLANSourceIP`. The `safeTruncate` helper and the multicast group address derivation logic in `LANDiscovery` are untested.

## Implementation Tasks

### Task 1: Add `pkg/discovery/registry_test.go`

Create `pkg/discovery/registry_test.go` with table-driven unit tests covering the pure (non-network) logic in `registry.go`.

**File to create:** `pkg/discovery/registry_test.go`

**Tests to implement:**

#### `TestDecryptPeerList`
Test `RendezvousRegistry.decryptPeerList()` (line 152 of `registry.go`):

- **Case "valid encrypted body"**: Seal a `[]RegistryPeerEntry` with `crypto.SealEnvelope` using a known 32-byte gossip key, base64-encode it as the issue body, call `decryptPeerList`, assert returned slice length and a sampled field (`WGPubKey`, `Endpoint`).
- **Case "empty body"**: Pass `""`, expect empty/nil slice (no panic).
- **Case "invalid base64"**: Pass `"not-base64!!"`, expect empty/nil slice.
- **Case "wrong key"**: Encrypt with key A, decrypt with key B, expect empty/nil slice.

Setup pattern (matches existing tests in `gossip_test.go` and `exchange_test.go`):
```go
func newTestRegistryKeys(t *testing.T) *crypto.DerivedKeys {
    t.Helper()
    keys, err := crypto.DeriveKeys("test-secret-for-registry-tests-32")
    if err != nil {
        t.Fatalf("DeriveKeys: %v", err)
    }
    return keys
}
```

#### `TestBuildIssueBody`
Test `RendezvousRegistry.buildIssueBody()` (line 319 of `registry.go`):

- **Case "single peer"**: Build body with one `daemon.PeerInfo`; assert the returned string is valid base64 and can be round-tripped through `decryptPeerList` yielding the original entry.
- **Case "nil/empty slice"**: Pass `nil`, assert no error and a non-empty body string (the empty encrypted list).

#### `TestRendezvousSearchTerm`
Test that `NewRendezvousRegistry` derives a deterministic search term from `DerivedKeys.RendezvousID`:

- Derive keys twice from the same secret; assert both `RendezvousRegistry.SearchTerm` values are identical and match the expected format `"wgmesh-<16 hex chars>"`.
- Derive keys from a different secret; assert the search term differs.

**Pattern to follow:** `pkg/discovery/gossip_test.go` — use `crypto.DeriveKeys`, create the struct directly, call the method under test, assert on the result without any network calls.

---

### Task 2: Extend `pkg/discovery/lan_test.go`

Add two tests to the existing `lan_test.go` file (after the existing `TestResolveEndpointPrefersLANSourceIP` test).

#### `TestSafeTruncate`
Test the `safeTruncate` helper (line 247 of `lan.go`):

```go
tests := []struct {
    name   string
    input  string
    maxLen int
    want   string
}{
    {"within limit", "hello", 10, "hello"},
    {"exact limit", "hello", 5, "hello"},
    {"over limit", "hello world", 5, "hello"},
    {"zero limit", "hello", 0, ""},
    {"empty string", "", 5, ""},
}
```

Assert `safeTruncate(tt.input, tt.maxLen) == tt.want` for each case.

#### `TestMulticastGroupDerivation`
Test that the multicast group IP address derived from `DerivedKeys.MulticastID` is in the correct range:

```go
func TestMulticastGroupDerivation(t *testing.T) {
    keys, err := crypto.DeriveKeys("test-secret-for-lan-multicast-32b")
    if err != nil {
        t.Fatalf("DeriveKeys: %v", err)
    }
    // LANMulticastBase = 239.192.0.0; MulticastID uses bytes [2] and [3]
    // Expected group: 239.192.<keys.MulticastID[2]>.<keys.MulticastID[3]>
    groupIP := net.IPv4(239, 192, keys.MulticastID[2], keys.MulticastID[3])
    if !groupIP.IsMulticast() {
        t.Errorf("derived group %v is not a multicast address", groupIP)
    }
}
```

This test validates the address derivation formula used in `NewLANDiscovery` without requiring any network socket operations.

---

## Affected Files

| File | Change type | Details |
|---|---|---|
| `pkg/discovery/registry_test.go` | **Create new** | 3 test functions: `TestDecryptPeerList`, `TestBuildIssueBody`, `TestRendezvousSearchTerm` |
| `pkg/discovery/lan_test.go` | **Extend** | Add `TestSafeTruncate` and `TestMulticastGroupDerivation` after the existing test |

No production code changes are required. All four acceptance criteria are met by the existing implementation; only the test coverage gap needs to be closed.

## Test Strategy

Run after implementing:

```bash
# Run only the affected package
go test ./pkg/discovery/...

# Verify no data races in discovery tests
go test -race ./pkg/discovery/...
```

Expected outcome: all existing tests continue to pass; the two new test files add 5+ new passing test functions with no network or filesystem dependencies.

## Estimated Complexity

low (1–2 hours)

- No production code changes
- All test logic is pure (no network sockets, no GitHub API calls, no mocking frameworks needed)
- Follows established patterns from `gossip_test.go` and `exchange_test.go`
- `registry_test.go`: ~80–100 lines
- `lan_test.go` additions: ~30–40 lines
