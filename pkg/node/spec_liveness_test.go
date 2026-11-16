package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecNamesLiveness fails when a document stops naming what ADR-0004
// added: it checks names, not meaning.
func TestSpecNamesLiveness(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	cases := []struct {
		file  string
		names []string
	}{
		{"eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md", []string{"Assess", "ObserveDataPlane", "GetProbeable"}},
		{"eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md", []string{"ResumeGrace", "probeOnly"}},
		{"eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md", []string{"EndpointSTUNResume", "OnResume"}},
		{"docs/liveness-rollout.md", []string{"EnforceLiveness", "EnforceRelayProbe", "N+1", "N+2", "wgmesh_liveness_disagree_total", "wgmesh_relay_route_age_seconds"}},
		{"docs/adr/BACKLOG.md", []string{"ADR-0004"}},
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
