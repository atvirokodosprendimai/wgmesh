# Task ADR-0005-T4: A corrupt identity file is not treated as absent (#870)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `ErrStateMissing`, `ErrStateCorrupt`, `RefuseCorruptState` (package variable, `false` at N)
**Consumes:** `saveLocalNode` writes atomically (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`initLocalNode` generates a key only when the identity file is missing. A file that is unreadable JSON, empty, or lacks a public or private key is moved aside, never overwritten, and at N+1 stops the daemon with a message that names the file.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/helpers.go` | edit | `loadLocalNode` returns `ErrStateMissing` (wraps `os.ErrNotExist`) or `ErrStateCorrupt` (wraps the JSON error; also for zero bytes and for an empty `WGPubKey` or `WGPrivateKey`); `quarantineState(path) (string, error)` renames the file to `path + ".corrupt-" + unix` |
| `pkg/daemon/daemon.go` | edit | `initLocalNode` (`:362-363`, `:403`): `ErrStateMissing` generates as today; `ErrStateCorrupt` calls `quarantineState`, logs the new path, and continues to generation; with `RefuseCorruptState` it returns an error naming the file and leaves it untouched; any other error (permission) is returned, not treated as absent |
| `pkg/daemon/identity_state_test.go` | add | tests below |

Selection: `initLocalNode` is called by `Run` and `RunWithDHTDiscovery` (`daemon.go:219`, `:1529`); reverting the typed branch to "any error means absent" turns `TestInitLocalNodeCorruptMovesAside` red.

Ordering note: ADR-0002 T5 adds a `stateFile` field set where `initLocalNode` builds the path (`daemon.go:361`); this task follows it and reuses that field.

## Ordered Steps

1. [S1] Add the five tests with a compiling stub (the two error values and `RefuseCorruptState` exist, `loadLocalNode` still returns the raw error) and run them red.
2. [S2] Implement the typed errors and `quarantineState`; `{}` and a file with a public key but no private key are corrupt.
3. [S3] Change `initLocalNode`; the log line at N says that a new identity is being created and where the old file went.
4. [S4] Run the mutants with `adr-verify --mutant`: a corrupt file overwritten without being moved; `RefuseCorruptState` ignored; a permission error treated as missing. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestLoadLocalNodeDistinguishesMissingAndCorrupt|TestInitLocalNodeCorruptMovesAside|TestInitLocalNodeRefusesWhenEnforced|TestInitLocalNodeMissingGeneratesKey|TestInitLocalNodeUnreadableIsNotAbsent)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLoadLocalNodeDistinguishesMissingAndCorrupt` | `pkg/daemon/identity_state_test.go` | table: no file is missing; garbage, zero bytes, `{}`, no private key are corrupt; a good file loads | — | S1, S2 |
| `TestInitLocalNodeCorruptMovesAside` | `pkg/daemon/identity_state_test.go` | at N the corrupt bytes are found unchanged in `<iface>.json.corrupt-*` and a new valid file exists | — | S1, S3 |
| `TestInitLocalNodeRefusesWhenEnforced` | `pkg/daemon/identity_state_test.go` | with `RefuseCorruptState` the error names the file, no new key is generated, the file is byte-identical | — | S1, S3 |
| `TestInitLocalNodeMissingGeneratesKey` | `pkg/daemon/identity_state_test.go` | pin: no file still creates an identity (starts green) | — | S1, S3 |
| `TestInitLocalNodeUnreadableIsNotAbsent` | `pkg/daemon/identity_state_test.go` | a file the process cannot read returns an error and generates nothing (skipped as root) | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `initLocalNode` on every start; the mutants turn the tests red |
| 3 — the caller can discover it | the log line names the moved file; T11 writes the recovery steps in the release note |
| 4 — it is used | every start with a damaged identity file |

## Mutation Log

## Invariants

- A node with a valid file starts exactly as before; the persisted mesh IP rule (`meshIPInSubnet`) is not touched.
- The corrupt file is never deleted and never overwritten; the new identity goes to the original path.
- `RefuseCorruptState` is `false` at N; the flip is a one-line change recorded as a backlog row.

## Risks

- A stricter "corrupt" than the old loader: a file valid before (keys present, odd extra fields) is still valid; only missing keys are new. A hand-written state file with only `mesh_ip` was never usable.
- Test doubles: `localNodeStateDir` pointed at a temp directory (tests must not run in parallel, as its doc comment says).

## Stop Condition

Stop and ask if ADR-0002 T5 is not merged (the `stateFile` field is missing), or if `quarantineState`'s rename can cross filesystems on a supported layout.

## Out of Scope

- Setting `RefuseCorruptState = true` (deferred: docs/adr/BACKLOG.md).
- Validating that the public key matches the private key (permanent: boundary: a damaged file that still parses with both keys is not what #870 reports; a mismatch would show at the first handshake).

## Verification Log
