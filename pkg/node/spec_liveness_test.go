package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecNamesLiveness fails when a document stops naming what ADR-0004
// added: it checks that names and phrases are present in the documents (and
// that a retired wording is gone), not that the text describes the code.
func TestSpecNamesLiveness(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(file string) (string, bool) {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Errorf("%s: %v", file, err)
			return "", false
		}
		return string(body), true
	}
	const (
		backlog   = "docs/adr/BACKLOG.md"
		admitNote = "docs/admit-rollout.md"
	)
	cases := []struct {
		file  string
		names []string
	}{
		{"eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md", []string{"Assess(Evidence, now)", "ObserveDataPlane", "GetProbeable"}},
		{"eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md", []string{"ResumeGrace", "probeOnly", "not already in the store"}},
		{"eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md", []string{"`daemon.EndpointSTUNResume`", "`DHTDiscovery.OnResume`"}},
		{"docs/liveness-rollout.md", []string{
			"`node.EnforceLiveness` true", "`daemon.EnforceRelayProbe` true",
			"## Release note for N+1", "## Release note for N+2",
			"wgmesh_liveness_disagree_total", "wgmesh_relay_route_age_seconds",
			"grep wgmesh_liveness_disagree_total", "MeshProbeFailLimit", "after a rollback", "## Closed", "`SetPeer`",
		}},
		{backlog, []string{
			"ADR-0004 follow-up N+1", "ADR-0004 follow-up N+2",
			"ADR-0004 deferred: OS power events", "ADR-0004 deferred: rebind the transport socket",
			"ADR-0004 known limit: the wake time",
		}},
		{admitNote, []string{"liveness release N"}},
	}
	for _, c := range cases {
		body, ok := read(c.file)
		if !ok {
			continue
		}
		for _, name := range c.names {
			if !strings.Contains(body, name) {
				t.Errorf("%s does not name %q", c.file, name)
			}
		}
	}

	// ADR-0004 builds the stun-resume source: no document may still promise it.
	forbidden := []string{"ADR-0004 M5 will add", "ADR-0004 M5 is planned to add", "will add a `stun-resume`"}
	for _, file := range []string{admitNote, backlog, "docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md"} {
		body, ok := read(file)
		if !ok {
			continue
		}
		for _, bad := range forbidden {
			if strings.Contains(body, bad) {
				t.Errorf("%s still says %q", file, bad)
			}
		}
	}
}
