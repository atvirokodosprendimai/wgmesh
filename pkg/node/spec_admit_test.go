package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecNamesAdmit fails when a document stops naming what ADR-0002 added:
// it checks names, not meaning.
func TestSpecNamesAdmit(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		file  string
		names []string
	}{
		{"eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md", []string{"Offer", "Withdraw", "tombstone"}},
		{"eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md", []string{"FilterAnnouncement"}},
		{"eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md", []string{"ResolveClaims"}},
		{"eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md", []string{"ObserveEndpoint"}},
		{"docs/admit-rollout.md", []string{"EnforceIPBinding", "EnforceRouteBinding", "N+1", "N+2", "accept-routes", "wgmesh_admit_warnings_total"}},
		{"docs/adr/BACKLOG.md", []string{"ADR-0002 follow-up N+1", "ADR-0002 follow-up N+2", "ADR-0002 known limit"}},
	}
	for _, c := range cases {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		for _, name := range c.names {
			if !strings.Contains(string(body), name) {
				t.Errorf("%s does not name %q", c.file, name)
			}
		}
	}
}
