package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lastSeenAges returns "position what" for every age computed from a LastSeen
// selector in the non-test Go files under root, leaving out skipDir, .git and
// vendor. Stub: S5 fills it in.
func lastSeenAges(root, skipDir string) ([]string, error) {
	return nil, nil
}

// TestNoDirectLastSeenAge fails when a production file outside pkg/node
// computes the age of a PeerInfo.LastSeen itself, with time.Since(x.LastSeen),
// a.Sub(x.LastSeen) or x.LastSeen.Sub(a): a liveness reader asks
// PeerInfo.Alive or PeerInfo.LastAlive instead. pkg/node is not scanned.
//
// Limit: an age computed through a local variable (t := p.LastSeen; now.Sub(t))
// is not seen.
func TestNoDirectLastSeenAge(t *testing.T) {
	root := filepath.Join("..", "..")
	found, err := lastSeenAges(root, filepath.Join(root, "pkg", "node"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("direct LastSeen age, ask Alive or LastAlive: %s", f)
	}

	// The guard sees the forms it names, and not the ones it must leave alone.
	cases := []struct {
		name string
		expr string
		want bool
	}{
		{"since", `time.Since(p.LastSeen)`, true},
		{"since_bare", `time.Since(peer.LastSeen) < time.Minute`, true},
		{"sub_arg", `now.Sub(p.LastSeen)`, true},
		{"sub_receiver", `p.LastSeen.Sub(now)`, true},
		{"since_alive", `time.Since(p.LastAlive(now))`, false},
		{"sub_alive", `now.Sub(p.LastAlive(now))`, false},
		{"copy_for_display", `use(p.LastSeen)`, false},
		{"other_since", `time.Since(p.Started)`, false},
	}
	dir := t.TempDir()
	skip := filepath.Join(dir, "skipped")
	if err := os.Mkdir(skip, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		src := "package bait\n\nfunc f() {\n\t_ = " + c.expr + "\n}\n"
		if err := os.WriteFile(filepath.Join(dir, c.name+".go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skip, "bait.go"), []byte("package bait\n\nfunc f() {\n\t_ = time.Since(p.LastSeen)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bait_test.go"), []byte("package bait\n\nfunc g() {\n\t_ = time.Since(p.LastSeen)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baitFound, err := lastSeenAges(dir, skip)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		var got []string
		for _, f := range baitFound {
			if strings.Contains(f, c.name+".go:") {
				got = append(got, f)
			}
		}
		if c.want && len(got) != 1 {
			t.Errorf("%s: %s: got %v, want one finding", c.name, c.expr, got)
		}
		if !c.want && len(got) != 0 {
			t.Errorf("%s: %s was flagged: %v", c.name, c.expr, got)
		}
	}
	for _, f := range baitFound {
		if strings.Contains(f, "skipped") || strings.Contains(f, "bait_test.go") {
			t.Errorf("a skipped or test file was scanned: %s", f)
		}
	}
}
