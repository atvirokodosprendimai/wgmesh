package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lineCheck says that exactly one line of a file starts with head (the head
// of a bullet, a table row or a paragraph: a sentence the file must keep) and
// that this line holds every needle. A needle is checked in its own line, so a
// word that occurs elsewhere in the file cannot stand in for it.
type lineCheck struct {
	head    string
	needles []string
}

type fileChecks struct {
	file string
	// absent are strings the file must not contain (an editorial placeholder).
	absent []string
	checks []lineCheck
}

const (
	reconcileSpec = "eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md"
	supportSpec   = "eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md"
)

var specChecks = []fileChecks{
	{
		file: reconcileSpec,
		checks: []lineCheck{
			{"- Each cycle (`converge`)", []string{"observe the kernel", "compute one desired state", "plan the difference"}},
			{"- Observe reads the kernel once per target", []string{"wg show <iface> dump", "never read as empty"}},
			{"- The desired state is computed once per cycle", []string{"routes.Resolve"}},
			{"- Only the difference between the dump and the desired state is written", []string{"pushedEndpoint", "Shrinking peers are set before growing ones"}},
			{"- Apply writes one item at a time", []string{"converge lock", "never skips the others"}},
			{"- `wireguard.Dump`", []string{"wg show <iface> dump", "handshakes"}},
			{"- `converge.go`, `observe.go`, `desired.go`", []string{"observe, desired state, plan and apply"}},
			{"> [[pkg/daemon/converge.go]]", nil},
			{"> [[pkg/daemon/observe.go]]", nil},
			{"> [[pkg/daemon/desired.go]]", nil},
		},
	},
	{
		file: supportSpec,
		checks: []lineCheck{
			{"- Kernel routes of the mesh interface are converged", []string{"both families", "never skips the others"}},
			{"- **One owner per network:**", []string{"routes.Resolve", "HandshakeStaleAfter", "routes.ExcludeOwn"}},
			{"- **Both families:**", []string{"carrier's mesh IPv6", "falling back to the peer's own"}},
			{"- A default route (", []string{"replaces the host's own default route", "puts it back once"}},
			{"- A network equal to or inside one this host is attached to", []string{"EnforceRouteGuard", "attached-network"}},
			{"- Linux uses `ip`; macOS reads", []string{"not yet verified on a real Mac"}},
			{"- **Forwarding is converged once:**", []string{"EnforceForwardScope", "logged once"}},
			{"> [[pkg/daemon/converge.go]]", nil},
		},
	},
	{
		file:   "docs/kernel-state.md",
		absent: []string{"{>>"},
		checks: []lineCheck{
			{"4. **Result.**", []string{"A steady node with nothing to correct"}},
			{"A target that cannot be read is skipped", []string{"Collision resolution", "collision.go"}},
			{"1. **Observe.**", []string{"wg show <iface> dump"}},
			{"Four switches exist", []string{"EnforceRevertOnExit", "atomic.Bool", "only a new build"}},
			{"| `EnforceForwardScope`", []string{"forward-scope"}},
			{"| `EnforceRouteGuard`", []string{"attached-network"}},
			{"| `EnforceAddressRemoval`", []string{"stale-address"}},
			{"| `EnforceRevertOnExit`", []string{"revert-on-exit:", "runs no command", "`ip_forward` back to 0", "no counter"}},
			{"- `attached-network` on macOS", []string{"EnforceRouteGuard"}},
			{"- `ip route replace` of an attached network", []string{"kernel 6.17.0-41", "2026-11-16", "restores the connected route"}},
			{"- Addresses:", []string{"never flushes", "setInterfaceAddress", "collision.go", "resetInterface"}},
			{"- An IPv6 address is held once", []string{"ipv6: address already assigned", "stale-address"}},
			{"- `wgmesh_kernel_changes_total{target}`", []string{"`route`", "`address`"}},
			{"- `wgmesh_kernel_warnings_total{rule}`", []string{"foreign-route", "host-default", "stale-address"}},
			{"How to check one node:", []string{"curl -s <addr>/metrics | grep wgmesh_kernel", "--metrics"}},
			{"An advertised `0.0.0.0/0`", []string{"replaces", "host-default", "puts it back once", "restored the host's", "not kept across a restart"}},
			{"Caveat, not built and not run", []string{"fwmark"}},
			{"- An IPv6 route uses the carrier's mesh IPv6", []string{"relay's", "falling back to the peer's own"}},
			{"- macOS routes (", []string{"-inet6", "not verified on a real Mac"}},
			{"- From release N a Mac gets route writes", []string{"previous binary", "No release switch"}},
			{"- macOS deletes only routes", []string{"in-memory"}},
			{"- macOS rule ", []string{"foreign-route", "unrecognised route row"}},
			{"These texts are gone", []string{"Failed to configure peer", "Failed to remove obsolete peer", "Failed to sync peer routes", "Failed to install relay FORWARD rule"}},
			{"The new texts are", []string{"Failed to set peer", "Failed to replace route", "Failed to add address", "Failed to append forward", "shortKey"}},
			{"The once-only lines:", []string{"forward-scope:", "revert-on-exit:", "host-default:", "iptables not found"}},
			{"- A forced reconnect never touches a relayed peer", nil},
			{"- A peer removed by stale cleanup", []string{"added again"}},
			{"- When the desired peer set is empty", []string{"removes every kernel peer", "BACKLOG"}},
			{"- A graceful exit", []string{"only what this run did", "found in place is never touched", "SIGKILL", "gap"}},
			{"- The `wg` forks have no deadline", []string{"ADR-0007"}},
			{"- Real `netstat` output on a Mac", []string{"BACKLOG"}},
			{"All kernel state is runtime state", []string{"#872", "T8, built", "N+1"}},
		},
	},
	{
		file: "docs/adr/BACKLOG.md",
		checks: []lineCheck{
			{"- ADR-0003 follow-up N+1:", []string{"EnforceForwardScope", "EnforceRouteGuard", "EnforceRevertOnExit", "EnforceAddressRemoval", "T8, built"}},
			{"- ADR-0003 deferred: transport exemption", []string{"fwmark"}},
			{"- ADR-0003 deferred: remove the redundant `setInterfaceAddress` call", []string{"collision", "flushes"}},
			{"- ADR-0003 known limit: when the owner of a shared network is relayed", []string{"lowest mesh IP"}},
			{"- ADR-0003 known limit: when the desired peer set is empty", []string{"removes every kernel peer"}},
			{"- ADR-0003 deferred: a deadline on the `wg` forks", []string{"ADR-0007"}},
			{"- ADR-0003 deferred: record real `netstat -rn` output on a Mac", []string{"T6"}},
		},
	},
}

// specProblems is one text per failed check of text: a head that matches no
// line or several, a needle missing from its line, an absent string present.
func specProblems(f fileChecks, text string) []string {
	var out []string
	lines := strings.Split(text, "\n")
	for _, a := range f.absent {
		if strings.Contains(text, a) {
			out = append(out, fmt.Sprintf("%s: still contains %q", f.file, a))
		}
	}
	for _, c := range f.checks {
		var hit []string
		for _, l := range lines {
			if strings.HasPrefix(l, c.head) {
				hit = append(hit, l)
			}
		}
		if len(hit) != 1 {
			out = append(out, fmt.Sprintf("%s: line starting %q matched %d lines, want 1", f.file, c.head, len(hit)))
			continue
		}
		for _, n := range c.needles {
			if !strings.Contains(hit[0], n) {
				out = append(out, fmt.Sprintf("%s: line starting %q does not name %q", f.file, c.head, n))
			}
		}
	}
	return out
}

func readSpecFile(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", file))
	if err != nil {
		t.Fatalf("%s: cannot be read: %v", file, err)
	}
	return string(raw)
}

// TestSpecNamesConverge fails when the two eidos specs, the operator note or
// the ADR backlog stop naming the mechanisms of the converge step (ADR-0003):
// every sentence or row listed in specChecks must exist once, and carry its
// names. It checks names, not meaning: review owns the rest.
func TestSpecNamesConverge(t *testing.T) {
	for _, f := range specChecks {
		for _, p := range specProblems(f, readSpecFile(t, f.file)) {
			t.Error(p)
		}
	}
}

// TestSpecChecksCanFail proves each check can fail: on a copy of the file
// (never on the file) it deletes the line of a check, which must be reported
// for that check alone, and cuts each needle out of its line, which must be
// reported for that needle alone.
func TestSpecChecksCanFail(t *testing.T) {
	for _, f := range specChecks {
		text := readSpecFile(t, f.file)
		lines := strings.Split(text, "\n")
		for _, c := range f.checks {
			idx := -1
			for i, l := range lines {
				if strings.HasPrefix(l, c.head) {
					idx = i
				}
			}
			if idx < 0 {
				t.Fatalf("%s: no line starts %q", f.file, c.head)
			}
			with := func(line string, drop bool) string {
				out := append([]string(nil), lines...)
				if drop {
					out = append(out[:idx], out[idx+1:]...)
				} else {
					out[idx] = line
				}
				return strings.Join(out, "\n")
			}
			t.Run("delete: "+c.head, func(t *testing.T) {
				got := specProblems(f, with("", true))
				if len(got) != 1 || !strings.Contains(got[0], fmt.Sprintf("line starting %q matched 0 lines", c.head)) {
					t.Errorf("deleting the line gave %q, want exactly its own check red", got)
				}
			})
			for _, n := range c.needles {
				t.Run("cut: "+c.head+" / "+n, func(t *testing.T) {
					got := specProblems(f, with(strings.ReplaceAll(lines[idx], n, ""), false))
					if len(got) == 0 {
						t.Fatalf("cutting %q from its line stayed green", n)
					}
					for _, g := range got {
						if !strings.Contains(g, fmt.Sprintf("line starting %q does not name", c.head)) {
							t.Errorf("cutting %q reported another check: %q", n, g)
						}
					}
				})
			}
		}
		for _, a := range f.absent {
			t.Run("absent: "+a, func(t *testing.T) {
				if got := specProblems(f, text+"\n"+a+" to be completed"); len(got) != 1 {
					t.Errorf("a file with %q gave %q, want one problem", a, got)
				}
			})
		}
	}
}
