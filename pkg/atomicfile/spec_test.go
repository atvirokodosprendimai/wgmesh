package atomicfile

// Words: the five specs that ADR-0005 amends, the release note
// docs/state-rollout.md and docs/adr/BACKLOG.md must each name the pieces the
// decision built, so a rename or a dropped paragraph fails here.
//
// Why: the documents are the only place an operator or the next author learns
// that state files are written atomically, that two locks exist, that a corrupt
// identity is moved aside, how a reply reaches several callers and that a DHT
// run gets a fresh context. A document that stops naming them has drifted.
//
// Limits: the test checks names, not meaning; a name in a wrong sentence passes.
//
// Invariant: files are read from the module root (`go list -m -f {{.Dir}}`),
// a missing file is a failure, and nothing is skipped.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecNamesAtomicState(t *testing.T) {
	root := moduleRoot(t)

	const (
		supportSpec   = "eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md"
		lifecycleSpec = "eidos/spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md"
		rpcSpec       = "eidos/spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md"
		exchangeSpec  = "eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md"
		dhtSpec       = "eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md"
		note          = "docs/state-rollout.md"
		backlog       = "docs/adr/BACKLOG.md"
	)

	required := []struct {
		file  string
		names []string
	}{
		{supportSpec, []string{"atomicfile.WriteFile"}},
		{lifecycleSpec, []string{"ErrStateCorrupt", "RefuseCorruptState", "flock", ".lock"}},
		{rpcSpec, []string{"flock", ".lock"}},
		{exchangeSpec, []string{"registerReply"}},
		{dhtSpec, []string{"parentCtx", "beginRun"}},
		{note, []string{"ErrStateCorrupt", "RefuseCorruptState", ".lock"}},
		{backlog, []string{"RefuseCorruptState"}},
	}

	for _, r := range required {
		body, err := os.ReadFile(filepath.Join(root, r.file))
		if err != nil {
			t.Errorf("%s: cannot be read: %v", r.file, err)
			continue
		}
		for _, name := range r.names {
			if !strings.Contains(string(body), name) {
				t.Errorf("%s: does not name %q", r.file, name)
			}
		}
	}
}
