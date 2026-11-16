# Task ADR-0003-T6: Install peers' advertised networks on macOS (#862)

**Depends-on:** T5
**Blocked-on:** the sign-off of step S6, `checked by: a maintainer with a Mac running the Homebrew build`
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `planRoutesDarwin`, `applyRouteDiffDarwin`, `observeRoutesDarwin`, `parseNetstatDarwin`
**Consumes:** `routeTarget`, `EnforceRouteGuard` (T5); `routes.Canonical` (T1)
**Data dependency:** needs recorded `netstat -rn -f inet` and `netstat -rn -f inet6` output from a Mac with a `utun` WireGuard interface, recorded by S1 into `pkg/daemon/testdata/darwin/`
**Proof map:** v1

## Goal

On macOS the route target reads the kernel table with `netstat`, adds `route -n add -net <cidr> -interface <utun>` and deletes with `route -n delete`, so a peer's advertised network is routed into the tunnel, and a network equal to or inside an attached subnet is skipped from the first release (macOS has no old behaviour to keep).

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/darwin_routes.go` | add | `observeRoutesDarwin`, `parseNetstatDarwin` and the pure `planRoutesDarwin` (macOS routes carry no gateway, so `routes.CalculateDiff` is not reused; the mesh subnet and mesh IPv6 prefix are kept); the apply `applyRouteDiffDarwin` is in `converge.go`, so the guard test can name it (abbreviated destinations such as `192.168.5/24` and `10.99/16` are expanded; `default` by the family queried); not named `*_darwin.go`, because that suffix is a Go build constraint and the file would not compile or test on Linux |
| `pkg/daemon/routes.go` | edit | none: `syncPeerRoutes` no longer exists (T5 moved it); the non-Linux early returns were in `convergeRoutes`, `observeRoutes` and `attachedSubnet` in `converge.go`, `observe.go` and `desired.go`, now one switch on `osName()` (the `Daemon` field `goos`) each |
| `pkg/daemon/converge.go` | edit | on Darwin the forwarding and address targets are not wanted (out of scope), the attached-network rule is enforced regardless of `EnforceRouteGuard` |
| `pkg/daemon/darwin_routes_test.go`, `pkg/daemon/testdata/darwin/netstat-inet.handwritten.txt`, `netstat-inet6.handwritten.txt` (hand-written from the man page, NOT recorded: S1 replaces them with `netstat-inet.txt` and `netstat-inet6.txt` from a Mac and renames the references in the test and in the Acceptance check) | add | the six tests below |

Selection: `syncPeerRoutes` picks the adapter by `goos`; deleting that branch turns `TestSyncRoutesRunsOnDarwin` red.

## Ordered Steps

1. [S2] Add the six tests below with compiling stubs (an adapter that returns nothing); the fixtures start as hand-written files in the shape of the netstat man page. Run red.
2. [S1] On a Mac with the Homebrew build joined to a mesh with two peers, one advertising `192.168.50.0/24` and one advertising `0.0.0.0/0`, record `netstat -rn -f inet` and `netstat -rn -f inet6` into the two fixture files, replacing the hand-written ones. [proof: human: real command output from a Mac; the parse formats are from the netstat man page and are not run here; record the macOS version and date in the commit message]
3. [S3] Implement `parseNetstatDarwin` and the observe; implement add and delete with `-interface <utun>` and `-inet6` for IPv6; treat `File exists` on add as `route -n change`, as `setInterfaceAddress` already does (`helpers.go:226-237`).
4. [S4] Remove the early return and select the adapter by `goos`.
5. [S5] Enforce the attached-network rule on Darwin without the switch.
6. [S6] On the Mac, build the branch and verify: `route -n get 192.168.50.1` names the `utun` interface; a second tick prints no route change; removing the advertising peer removes the route within one tick plus the peer's removal time; a peer advertising the Mac's own LAN prefix is skipped with one log line. [proof: human: needs a Mac; record the macOS version, the build sha and the four observations in the Verification Log]
7. [S7] Run the mutants with `adr-verify --mutant`: the adapter uses `-net` without `-interface`; the abbreviated destination is not expanded; the early return is restored; `File exists` is treated as a failure. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestParseNetstatDarwin|TestDarwinRouteAddUsesInterface|TestDarwinRouteDeleteUsesInterface|TestDarwinAttachedNetworkSkippedFromFirstRelease|TestDarwinV6RouteUsesInet6|TestSyncRoutesRunsOnDarwin)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 6 ] && [ -s pkg/daemon/testdata/darwin/netstat-inet.handwritten.txt ] && go test -race ./pkg/daemon -count=1
```

The fence is hermetic apart from the two recorded files; the real behaviour is S6's sign-off.

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestParseNetstatDarwin` | `pkg/daemon/darwin_routes_test.go` | the recorded fixtures parse to canonical networks on the `utun` interface; `default`, `10.99/16` and link-level rows are handled | — | S2, S3 |
| `TestDarwinRouteAddUsesInterface` | `pkg/daemon/darwin_routes_test.go` | the add command is `route -n add -net <cidr> -interface <utun>` | — | S2, S3 |
| `TestDarwinRouteDeleteUsesInterface` | `pkg/daemon/darwin_routes_test.go` | the delete command is `route -n delete -net <cidr> -interface <utun>` | — | S2, S3 |
| `TestDarwinAttachedNetworkSkippedFromFirstRelease` | `pkg/daemon/darwin_routes_test.go` | a peer network equal to the host's LAN prefix is not installed with `EnforceRouteGuard` false | — | S2, S5 |
| `TestDarwinV6RouteUsesInet6` | `pkg/daemon/darwin_routes_test.go` | an IPv6 network uses `-inet6` | — | S2, S3 |
| `TestSyncRoutesRunsOnDarwin` | `pkg/daemon/darwin_routes_test.go` | with `goos` set to `darwin` the route target issues commands where it returned `nil` before | — | S2, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `syncPeerRoutes` selects the adapter by `goos`; the early-return mutant turns `TestSyncRoutesRunsOnDarwin` red |
| 3 — the caller can discover it | the daemon support spec no longer says "no-op on other platforms" (T9) |
| 4 — it is used | S6 on a real Mac |

## Mutation Log
- 2026-11-16 · 69e2b4d* · mutant killed · exit 1 · `pkg/daemon/converge.go` · no -interface: TestDarwinRouteAddUsesInterface must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · 69e2b4d* · mutant killed · exit 1 · `pkg/daemon/darwin_routes.go` · abbreviation not expanded: TestParseNetstatDarwin must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · 69e2b4d* · mutant killed · exit 1 · `pkg/daemon/converge.go` · early return restored: TestSyncRoutesRunsOnDarwin must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · 69e2b4d* · mutant killed · exit 1 · `pkg/daemon/converge.go` · File exists not turned into change: TestDarwinFileExistsBecomesChange must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · fe6f879* · mutant killed · exit 1 · `pkg/daemon/converge.go` · no -interface: TestDarwinRouteAddUsesInterface must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · fe6f879* · mutant killed · exit 1 · `pkg/daemon/darwin_routes.go` · abbreviation not expanded: TestParseNetstatDarwin must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · fe6f879* · mutant killed · exit 1 · `pkg/daemon/converge.go` · early return restored: TestSyncRoutesRunsOnDarwin must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a
- 2026-11-16 · fe6f879* · mutant killed · exit 1 · `pkg/daemon/converge.go` · File exists not turned into change: TestDarwinFileExistsBecomesChange must go red · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a

## Invariants

- Linux behaviour is unchanged by this task.
- The mesh subnet route that `setInterfaceAddress` installs on macOS (`helpers.go:223-263`) stays.
- No gateway is used on macOS: WireGuard picks the peer by AllowedIPs.

## Risks

- `netstat` abbreviates destinations and differs between macOS versions; the fixtures are real output and S6 verifies the result; the task is not done without the sign-off.
- A peer network that overlaps the Mac's own LAN would break its LAN; the attached-network rule is on from the first release for that reason.
- Test doubles: `MockCommandExecutor` and the `goos` field.

## Stop Condition

Stop and ask if `route -n add -net <cidr> -interface <utun>` does not route into the tunnel on the Mac (then a gateway is needed and the adapter's shape changes), or if no Mac is available for S1 and S6.

## Out of Scope

- Forwarding, `pf` rules and NAT on macOS (permanent: boundary: wgmesh does not manage firewalls on macOS).
- Windows (permanent: boundary: the daemon has no Windows interface code, `helpers.go:166-168`).

## Verification Log
- 2026-11-16 · 69e2b4d* · exit 1 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:2614 · test-lock-sha256:82dbada4497c9083e1ba7b4f7fcaf003b89b52bd9cd56cf9a0727d4e50b17130 · test-lock-file:docs/adr/locks/82dbada4497c9083e1ba7b4f7fcaf003b89b52bd9cd56cf9a0727d4e50b17130.lock
  ```
  --- last 10 line(s) of stderr (of 48 after folding 48 raw)
  === RUN   TestSyncRoutesRunsOnDarwin/darwin_reads_and_writes_the_table
      darwin_routes_test.go:418: the table was not read once per family: []
      darwin_routes_test.go:421: the peer's network is not routed; commands []
  === RUN   TestSyncRoutesRunsOnDarwin/another_OS_touches_no_route
  --- FAIL: TestSyncRoutesRunsOnDarwin (0.00s)
      --- FAIL: TestSyncRoutesRunsOnDarwin/darwin_reads_and_writes_the_table (0.00s)
      --- PASS: TestSyncRoutesRunsOnDarwin/another_OS_touches_no_route (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.006s
  FAIL
  ```
- 2026-11-16 · 69e2b4d* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10879
- 2026-11-16 · 69e2b4d* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10624
- 2026-11-16 · 69e2b4d* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10390
- 2026-11-16 · 69e2b4d* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:11106
- 2026-11-16 · 69e2b4d* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:6853
- 2026-11-16 · fe6f879* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:0 · test-lock-sha256:030032fb0dfecd8cedcc293237cde2df97d6e57fe2bb1f8f1fc30aa6e4235e55 · test-lock-file:docs/adr/locks/030032fb0dfecd8cedcc293237cde2df97d6e57fe2bb1f8f1fc30aa6e4235e55.lock · test-lock-kind:replace
- 2026-11-16 · fe6f879* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10732
- 2026-11-16 · fe6f879* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10303
- 2026-11-16 · fe6f879* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:11613
- 2026-11-16 · fe6f879* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:10343
- 2026-11-16 · fe6f879* · exit 0 · `set -o pipefail …` · acceptance-sha256:c2b2a9a9842ce42cb5d47a46e26148a410e4a0d3525921f20ebff28390d7db6a · ms:7086
