# Task ADR-0006-T3: Validate the loaded state before the first connection (#868)

**Depends-on:** T1, T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `Mesh.Validate`, `mesh.Finding`, `mesh.StrictState`, `TestCentralizedPackagesDoNotImportPeerStore`
**Consumes:** `ifname.ValidateLinux` (T1); `pkg/ssh/sshtest` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`Deploy` refuses, before it opens any connection, a state whose values a shell or a wg-quick file would give a meaning to (class A), warns about values that merely parse wrongly (class B) until `StrictState` is set, and a test fails if the centralized packages import the PeerStore side.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/mesh/validate.go` | add | `Finding{Node, Field, Value, Rule, Class}`, `Validate() []Finding` (pure, holds `m.mu.RLock`), `StrictState` variable (`false`), the class A safe-set check and the class B rules of the ADR's Decision item 2 |
| `pkg/mesh/deploy.go` | edit | `Deploy` calls `m.Validate()` first, before `ValidateGroups` and before `detectEndpoints`: class A findings (and class B when `StrictState`) return an error listing every finding; class B otherwise prints `Warning:` lines and goes on |
| `pkg/mesh/validate_test.go` | add | tests below |
| `pkg/mesh/boundary_test.go` | add | `TestCentralizedPackagesDoNotImportPeerStore` |
| `testdata/central/state-valid.json`, `testdata/central/state-docs-example.json` | add | saved states: a realistic one with a hostname endpoint, and the example of `docs/centralized-mode.md` with well-formed keys |

Selection: `Deploy` calls `Validate`; deleting that call turns `TestDeployValidatesBeforeDial` red (the fake server sees a connection).

## Ordered Steps

1. [S1] Add the tests and a stub `Validate` that returns no findings; run red (the hostile-state tests fail on assertions; the saved-state test passes by construction and is the guard against over-refusing, not a red test).
2. [S2] Implement the class A checks per field and per node: characters outside the safe set, control characters and spaces, empty `InterfaceName`, a value that starts with `-` (it would reach `wg` or `ip` as an option; review 2026-11-15), nil `MeshIP` (`<nil>` today), in `InterfaceName`, `PublicKey`, `PublicEndpoint`, `Network`, `RoutableNetworks`, `SSHHost`.
3. [S3] Implement the class B checks (ADR Decision item 2) with `StrictState` selecting warn or refuse.
4. [S4] Wire it into `Deploy`; make `TestDeployValidatesBeforeDial` pass (zero connections to the fake server for a class A state).
5. [S5] Add `TestCentralizedPackagesDoNotImportPeerStore`: it parses the non-test imports of `pkg/mesh`, `pkg/ssh`, `pkg/wireguard`; it is green at authoring because the imports are clean today, so its proof is the mutant. [proof: mutation]
6. [S6] Run the mutants with `adr-verify --mutant`: `Validate` returns nil; the safe-set check admits `;`; the class B check is applied as class A; `Deploy` stops calling `Validate`; `pkg/mesh` imports `pkg/node`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/mesh -run '^(TestValidateRefusesShellMetacharacters|TestValidateRefusesMissingFields|TestValidateWarnsOnFormatDeviationAtN|TestValidateRefusesFormatDeviationWhenStrict|TestValidateAcceptsSavedStates|TestDeployValidatesBeforeDial|TestCentralizedPackagesDoNotImportPeerStore)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestValidateRefusesShellMetacharacters` | `pkg/mesh/validate_test.go` | a class A finding for `; id` in each of the six fields, and the finding names node, field and rule | — | S1, S2 |
| `TestValidateRefusesMissingFields` | `pkg/mesh/validate_test.go` | an empty interface name and a node without `mesh_ip` are class A findings | — | S1, S2 |
| `TestValidateWarnsOnFormatDeviationAtN` | `pkg/mesh/validate_test.go` | a bare-IP route, a `/33` CIDR, an interface name with a dot, a 17-character name and a port 0 are class B findings and `Deploy`'s pre-check returns no error with `StrictState` false | — | S1, S3 |
| `TestValidateRefusesFormatDeviationWhenStrict` | `pkg/mesh/validate_test.go` | the same findings are an error with `StrictState` true (the variable is reset with `t.Cleanup`) | — | S1, S3 |
| `TestValidateAcceptsSavedStates` | `pkg/mesh/validate_test.go` | the two saved states produce no finding at all (a hostname endpoint, an IPv6 literal endpoint `[fd00::1]:51820`, `0.0.0.0/0`, a route list, a documented placeholder-free key) | — | S1, S3 |
| `TestDeployValidatesBeforeDial` | `pkg/mesh/validate_test.go` | for a class A state `Deploy` returns an error and the fake server saw zero connections | — | S1, S4 |
| `TestCentralizedPackagesDoNotImportPeerStore` | `pkg/mesh/boundary_test.go` | no non-test file in the three packages imports `pkg/node`, `pkg/daemon` or `pkg/discovery` | — | S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `Deploy` calls `Validate`; the stop-calling mutant turns `TestDeployValidatesBeforeDial` red |
| 3 — the caller can discover it | the error text lists each finding; T6 documents the rules and the flip |
| 4 — it is used | every `wgmesh -deploy` |

## Mutation Log

## Invariants

- `Load`, `Save`, `AddNode`, `RemoveNode`, `List` and `ListSimple` are unchanged: a state that fails validation can still be listed, edited with `-remove` and saved (M's decision 7, recommended answer).
- A state that deployed before and holds only characters from the safe set is never refused at N.
- `Validate` reads the mesh under `m.mu.RLock` and is not called with the write lock held (`Deploy` calls it before it takes the lock).
- No import of `pkg/node`, `pkg/daemon` or `pkg/discovery`.

## Risks

- The class B list is a judgement about what `ip` and `wg` tolerate; it is measured against no node (not run), which is why it only warns at N.
- Test doubles: the fake server of T2; the saved states are files, not constructed in code.

## Stop Condition

Stop and ask if either saved state produces a class A finding (the safe set refuses something real), or if `Deploy` has a caller that holds `m.mu` when it would call `Validate`.

## Out of Scope

- Validation in `Load` (permanent: boundary: M's decision 7 puts validation in `Deploy`, so `-list` and `-remove` work on a state that fails it).
- Cross-checking `Groups` and `AccessPolicies` (permanent: boundary: `ValidateGroups` and `ValidatePolicies` already do it and run after this).
- Setting `StrictState` (deferred: docs/adr/BACKLOG.md, the N+1 row written by T6).

## Verification Log
