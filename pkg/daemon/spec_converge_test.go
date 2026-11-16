package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecNamesConverge fails when the two eidos specs, the operator note or
// the ADR backlog stop naming the mechanisms of the converge step (ADR-0003).
// It checks names, not meaning: review owns the rest.
func TestSpecNamesConverge(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		file  string
		names []string
	}{
		{
			"eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md",
			[]string{"observe", "desired state", "dump"},
		},
		{
			"eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md",
			[]string{"one owner", "forwarding"},
		},
		{
			"docs/kernel-state.md",
			[]string{
				"EnforceForwardScope", "EnforceRouteGuard", "EnforceAddressRemoval",
				"wgmesh_kernel_warnings_total", "fwmark",
				"stale-address", "host-default", "attached-network", "forward-scope",
			},
		},
		{
			"docs/adr/BACKLOG.md",
			[]string{"ADR-0003"},
		},
	}
	for _, c := range cases {
		raw, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("%s: cannot be read: %v", c.file, err)
			continue
		}
		text := string(raw)
		for _, name := range c.names {
			if !strings.Contains(text, name) {
				t.Errorf("%s does not name %q", c.file, name)
			}
		}
	}
}
