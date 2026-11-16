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
| `pkg/daemon/helpers.go` | edit | `loadLocalNode` returns `ErrStateMissing` (wraps `os.ErrNotExist`) or `ErrStateCorrupt` (fixed text, never the JSON decoder's text, which can quote a byte of the file; also for zero bytes and for an empty `WGPubKey` or `WGPrivateKey`); `quarantineState(path) (string, error)` hard-links the file to `path + ".corrupt-" + unix` (then `-1` .. `-1000` when the name is taken; `os.Link` cannot replace an existing name), sets the copy to 0600 and removes the original: no `os.Rename`, which the T3 guard forbids and which would silently replace an existing name |
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
n=$(go test ./pkg/daemon -run '^(TestLoadLocalNodeDistinguishesMissingAndCorrupt|TestInitLocalNodeCorruptMovesAside|TestInitLocalNodeRefusesWhenEnforced|TestInitLocalNodeMissingGeneratesKey|TestInitLocalNodeUnreadableIsNotAbsent|TestCorruptStateErrorsCarryNoFileContent|TestQuarantineStateNeverTakesAnExistingName|TestQuarantineStateGivesUpWithoutTouchingAnything)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 8 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLoadLocalNodeDistinguishesMissingAndCorrupt` | `pkg/daemon/identity_state_test.go` | table: no file is missing; garbage, zero bytes, `{}`, no private key are corrupt; a good file loads | — | S1, S2 |
| `TestInitLocalNodeCorruptMovesAside` | `pkg/daemon/identity_state_test.go` | at N the corrupt bytes are found unchanged in `<iface>.json.corrupt-*` and a new valid file exists | — | S1, S3 |
| `TestInitLocalNodeRefusesWhenEnforced` | `pkg/daemon/identity_state_test.go` | with `RefuseCorruptState` the error names the file, no new key is generated, the file is byte-identical | — | S1, S3 |
| `TestInitLocalNodeMissingGeneratesKey` | `pkg/daemon/identity_state_test.go` | pin: no file still creates an identity (starts green) | — | S1, S3 |
| `TestInitLocalNodeUnreadableIsNotAbsent` | `pkg/daemon/identity_state_test.go` | a state path that cannot be read as a file (a non-empty directory, so it also runs as root) returns an error that is neither missing nor corrupt, generates nothing and quarantines nothing | — | S1, S3 |
| `TestCorruptStateErrorsCarryNoFileContent` | `pkg/daemon/identity_state_test.go` | a file `{"wg_pubkey":"AAAA","wg_private_key":"QUJD"DEF"}` gives errors (from the loader and from `initLocalNode` with `RefuseCorruptState`) that contain no file bytes and wrap no JSON error | — | S1, S2 |
| `TestQuarantineStateNeverTakesAnExistingName` | `pkg/daemon/identity_state_test.go` | with the current-second name already taken the copy gets the next name, the old copy is untouched, the new one is 0600 | — | S1, S2 |
| `TestQuarantineStateGivesUpWithoutTouchingAnything` | `pkg/daemon/identity_state_test.go` | with `base` and `base-1..base-1000` taken it returns an error and changes nothing | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `initLocalNode` on every start; the mutants turn the tests red |
| 3 — the caller can discover it | the log line names the moved file; T11 writes the recovery steps in the release note |
| 4 — it is used | every start with a damaged identity file |

## Mutation Log
- 2026-11-16 · 25cdb4c* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the corrupt file is not moved aside: TestInitLocalNodeCorruptMovesAside must go red · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc
- 2026-11-16 · 25cdb4c* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · RefuseCorruptState ignored: TestInitLocalNodeRefusesWhenEnforced must go red · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc
- 2026-11-16 · 25cdb4c* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a permission error is treated as absent: TestInitLocalNodeUnreadableIsNotAbsent must go red · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc
- 2026-11-16 · 25cdb4c* · mutant killed · exit 1 · `pkg/daemon/helpers.go` · an identity without keys is valid: TestLoadLocalNodeDistinguishesMissingAndCorrupt must go red · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc
- 2026-11-16 · 195438a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · the corrupt file is not moved aside: TestInitLocalNodeCorruptMovesAside must go red · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3
- 2026-11-16 · 195438a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · RefuseCorruptState ignored: TestInitLocalNodeRefusesWhenEnforced must go red · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3
- 2026-11-16 · 195438a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a read error is treated as absent: TestInitLocalNodeUnreadableIsNotAbsent must go red · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3
- 2026-11-16 · 195438a* · mutant killed · exit 1 · `pkg/daemon/helpers.go` · an identity without keys is valid: TestLoadLocalNodeDistinguishesMissingAndCorrupt must go red · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3
- 2026-11-16 · 195438a* · mutant killed · exit 1 · `pkg/daemon/helpers.go` · the decoder text is back in the error: TestCorruptStateErrorsCarryNoFileContent must go red · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3

## Invariants

- A node with a valid file starts exactly as before; the persisted mesh IP rule (`meshIPInSubnet`) is not touched.
- The corrupt file is never deleted and never overwritten; the new identity goes to the original path.
- `RefuseCorruptState` is `false` at N; the flip is a one-line change recorded as a backlog row.

## Risks

- A stricter "corrupt" than the old loader: a file valid before (keys present, odd extra fields) is still valid; only missing keys are new. A hand-written state file with only `mesh_ip` was never usable.
- Test doubles: `localNodeStateDir` pointed at a temp directory (tests must not run in parallel, as its doc comment says).
- Behaviour taken on (panel 2026-11-16): at N an unreadable file (permission, a directory) stops the daemon (`initLocalNode` returns the error); the old code generated a new key. Before N+1 sets `RefuseCorruptState`, an operator also needs a way to know: the log lines name the file and say how to restore it. A failed save of the new identity right after a quarantine is an error (the daemon would otherwise create another key at every start); for a plain missing file it stays a warning. `deriveMeshIPForService` in `service.go` reads `<state dir>/wg0.json` with its own struct and does not check the private key, so `wgmesh service` can still derive from a file the daemon calls corrupt (BACKLOG row at T11). The quarantined file keeps the private key (mode forced to 0600) and nothing deletes it. `RefuseCorruptState` is a plain package variable: an operator cannot flip it without a release (the ADR says so).

## Stop Condition

Stop and ask if ADR-0002 T5 is not merged (the `stateFile` field is missing), or if `quarantineState`'s rename can cross filesystems on a supported layout.

## Out of Scope

- Setting `RefuseCorruptState = true` (deferred: docs/adr/BACKLOG.md).
- Validating that the public key matches the private key (permanent: boundary: a damaged file that still parses with both keys is not what #870 reports; a mismatch would show at the first handshake).

## Verification Log
- 2026-11-16 · 25cdb4c* · exit 1 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:2464 · test-lock-sha256:5e6386018e50d53b10390c5bf568a281084332a5b5276991578ef36204652076 · test-lock-file:docs/adr/locks/5e6386018e50d53b10390c5bf568a281084332a5b5276991578ef36204652076.lock
  ```
  --- last 10 line(s) of stderr (of 30 after folding 30 raw)
  === RUN   TestInitLocalNodeMissingGeneratesKey
  2026/11/16 08:41:31 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
  --- PASS: TestInitLocalNodeMissingGeneratesKey (0.00s)
  === RUN   TestInitLocalNodeUnreadableIsNotAbsent
  2026/11/16 08:41:31 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
      identity_state_test.go:228: initLocalNode succeeded on a file it cannot read
  --- FAIL: TestInitLocalNodeUnreadableIsNotAbsent (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.012s
  FAIL
  ```
- 2026-11-16 · 25cdb4c* · exit 0 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:18834
- 2026-11-16 · 25cdb4c* · exit 0 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:21262
- 2026-11-16 · 25cdb4c* · exit 0 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:21244
- 2026-11-16 · 25cdb4c* · exit 0 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:18637
- 2026-11-16 · 25cdb4c* · exit 0 · `set -o pipefail …` · acceptance-sha256:be180327d1e085de5dbc8eb69b397d93b483dc19f343390982ffff314d1b65fc · ms:16801
- 2026-11-16 · 195438a* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:0 · test-lock-sha256:16b272f806a5b02c3ee3fc9cf933ea01ae507280f2c49f0a7ba6d091a2ddd92c · test-lock-file:docs/adr/locks/16b272f806a5b02c3ee3fc9cf933ea01ae507280f2c49f0a7ba6d091a2ddd92c.lock · test-lock-kind:replace
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:19411
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:21563
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:18891
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:15463
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:16967
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:14016
- 2026-11-16 · e4568fb* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:0 · test-lock-sha256:3265c7b0f900fa138c6f8441971ae12fec47348f2da354e881d90e66f86fb06e · test-lock-file:docs/adr/locks/3265c7b0f900fa138c6f8441971ae12fec47348f2da354e881d90e66f86fb06e.lock · test-lock-kind:replace
- 2026-11-16 · e4568fb* · exit 0 · `set -o pipefail …` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:15763
- 2026-11-16 · eaeea0d* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:515eebc9c823c8c27303fae86d0b8b42fdc2b41125bd076866335f085ac57fc3 · ms:1 · test-lock-sha256:5a2d9af875086533c009f331919cafda316b438f8f39c7969bd277fd83058784 · test-lock-file:docs/adr/locks/5a2d9af875086533c009f331919cafda316b438f8f39c7969bd277fd83058784.lock · test-lock-kind:replace
