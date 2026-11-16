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

`pkg/node` and `pkg/daemon` define the local node once, in `pkg/node/types.go`: the daemon's struct, with the `ObserveEndpoint` and pending-state fields that ADR-0002 T9 put on it, replaces the unused `node.LocalNode`, and `daemon.LocalNode` is an alias, so this record's accessors (T8) are on the value that `initLocalNode`, discovery and the tests use.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/types.go` | edit | replace the unused `LocalNode` (`:32-60`) by the daemon's struct and methods as they stand after ADR-0002 T9: `GetEndpoint`, `SetEndpoint`, `ObserveEndpoint`, `EndpointSource` and its constants, the pending-STUN fields, all under the one `endpointMu` |
| `pkg/daemon/peerstore_compat.go` | edit | `type LocalNode = node.LocalNode`, beside the `PeerInfo` and `PeerStore` aliases; the `EndpointSource` constants are re-exported there if `dht.go` and `exchange.go` name them as `daemon.X` |
| `pkg/daemon/daemon.go` | edit | delete the struct, its methods and the `EndpointSource` declarations (`:99-128` plus what ADR-0002 T9 added); the doc comment moves to the alias |
| `pkg/daemon/localnode_alias_test.go` | add | the identity test |

Selection: `grep -rn "LocalNode{" --include=*.go . | grep -v _test.go` lists the construction sites (`daemon.go:422`); all keep compiling because the structs have the same fields except what ADR-0002 T9 adds (read on 2026-11-15: `pkg/node/types.go:34-46`, `pkg/daemon/daemon.go:102-114`).

Ordering note: runs **after** ADR-0002 T9 (M4; ADR-0002 decision 10: `ObserveEndpoint` lives on `daemon.LocalNode` and the type merge is this record's work). T9 lands its tests in `pkg/node/local_endpoint_test.go` against `daemon.LocalNode`; after this move they sit beside the type, and they must pass unchanged.

## Ordered Steps

1. [S1] Add `TestLocalNodeIsOneType` (`reflect.TypeOf(daemon.LocalNode{}) == reflect.TypeOf(node.LocalNode{})`) and run it red. Run `grep -rn "endpointMu\|wgEndpoint" --include=*.go . | grep -v "pkg/node/types.go\|pkg/daemon/daemon.go"` (2026-11-15: no output, so no test names the unexported fields, which package `daemon` tests could do today and could not after the alias).
2. [S2] Move the struct and its methods into `pkg/node/types.go` (the unused struct is deleted), add the alias and delete the duplicate; `go build ./... && go vet ./...`; the tests ADR-0002 T9 added must pass unchanged. [proof: acceptance]
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
| 4 — it is used | T8; ADR-0002 T9's `ObserveEndpoint` after the move |

## Mutation Log
- 2026-11-16 · b848ddf* · mutant killed · exit 1 · `pkg/daemon/peerstore_compat.go` · the alias is replaced by a second struct: TestLocalNodeIsOneType must go red · acceptance-sha256:ad61c9da5e52ff36801db753b2127f4bc844bdd26901757dc9c264e55f11251a

## Invariants

- Field names, the zero value and the behaviour of `GetEndpoint` and `SetEndpoint` do not change.
- `pkg/node` keeps importing nothing from `pkg/daemon`.

## Risks

- A test or tool that builds `LocalNode` with positional fields would break; S1 greps for it.
- Test doubles: none.

## Stop Condition

Stop and ask if ADR-0002 T9 is not merged (there is nothing to move yet), if the two structs differ in a field other than what T9 added, or if the moved code names a symbol that only `pkg/daemon` defines (`pkg/node` may not import it).

## Out of Scope

- Adding locks or accessors (T8).
- `ObserveEndpoint` (ADR-0002 T9; permanent: boundary: owned by ADR-0002).

## Verification Log
- 2026-11-16 · b848ddf* · exit 1 · `set -o pipefail …` · acceptance-sha256:ad61c9da5e52ff36801db753b2127f4bc844bdd26901757dc9c264e55f11251a · ms:1179 · test-lock-sha256:761002d792e8d15db32d25adad6f4f206ac03e23c17a3b4a8e6d33685f0f18ee · test-lock-file:docs/adr/locks/761002d792e8d15db32d25adad6f4f206ac03e23c17a3b4a8e6d33685f0f18ee.lock
  ```
  --- last 6 line(s) of stderr
  === RUN   TestLocalNodeIsOneType
      localnode_alias_test.go:16: daemon.LocalNode (daemon.LocalNode) and node.LocalNode (node.LocalNode) are different types
  --- FAIL: TestLocalNodeIsOneType (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.004s
  FAIL
  ```
- 2026-11-16 · b848ddf* · exit 0 · `set -o pipefail …` · acceptance-sha256:ad61c9da5e52ff36801db753b2127f4bc844bdd26901757dc9c264e55f11251a · ms:31307
- 2026-11-16 · b848ddf* · exit 0 · `set -o pipefail …` · acceptance-sha256:ad61c9da5e52ff36801db753b2127f4bc844bdd26901757dc9c264e55f11251a · ms:22996
