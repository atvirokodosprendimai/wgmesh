# Task ADR-0005-T7: One `LocalNode` type, so a method added to it reaches the node the daemon runs

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** `daemon.LocalNode` as an alias of `node.LocalNode`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`pkg/node` and `pkg/daemon` define the local node once; a method or field added to `node.LocalNode` (ADR-0002 T9's `ObserveEndpoint`, this record's accessors) is on the value that `initLocalNode`, discovery and the tests use.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/peerstore_compat.go` | edit | `type LocalNode = node.LocalNode`, beside the `PeerInfo` and `PeerStore` aliases |
| `pkg/daemon/daemon.go` | edit | delete the struct and its `GetEndpoint` and `SetEndpoint` methods (`:99-128`); the doc comment moves to the alias |
| `pkg/daemon/localnode_alias_test.go` | add | the identity test |

Selection: `grep -rn "LocalNode{" --include=*.go . | grep -v _test.go` lists the construction sites (`daemon.go:422`); all keep compiling because both structs have the same fields (read on 2026-11-15: `pkg/node/types.go:34-46`, `pkg/daemon/daemon.go:102-114`).

Ordering note: runs **before** ADR-0002 T9, which adds `ObserveEndpoint` to `pkg/node/types.go` (Open Question 4); if T9 has already landed, run it now and amend ADR-0002 T9's note that its method was on an unused type.

## Ordered Steps

1. [S1] Add `TestLocalNodeIsOneType` (`reflect.TypeOf(daemon.LocalNode{}) == reflect.TypeOf(node.LocalNode{})`) and run it red. Run `grep -rn "endpointMu\|wgEndpoint" --include=*.go . | grep -v "pkg/node/types.go\|pkg/daemon/daemon.go"` (2026-11-15: no output, so no test names the unexported fields, which package `daemon` tests could do today and could not after the alias).
2. [S2] Add the alias and delete the duplicate; `go build ./... && go vet ./...`.
3. [S3] Run the mutants with `adr-verify --mutant`: the alias line removed and the struct restored. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^TestLocalNodeIsOneType$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && go build ./... && go vet ./... && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLocalNodeIsOneType` | `pkg/daemon/localnode_alias_test.go` | the two names are one type | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test |
| 2 — something selects it | the alias is the type every caller names |
| 3 — the caller can discover it | the alias comment in `peerstore_compat.go` |
| 4 — it is used | T8, ADR-0002 T9 |

## Mutation Log

## Invariants

- Field names, the zero value and the behaviour of `GetEndpoint` and `SetEndpoint` do not change.
- `pkg/node` keeps importing nothing from `pkg/daemon`.

## Risks

- A test or tool that builds `LocalNode` with positional fields would break; S1 greps for it.
- Test doubles: none.

## Stop Condition

Stop and ask if the two structs have diverged (a field present in one only) when the task starts.

## Out of Scope

- Adding locks or accessors (T8).
- `ObserveEndpoint` (ADR-0002 T9; permanent: boundary: owned by ADR-0002).

## Verification Log
