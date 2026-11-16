package daemon

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Words: the identity file (<iface>.json) holds the node's WireGuard key and
// mesh IP. initLocalNode generates a key only when that file is MISSING. A file
// that exists but cannot be used (not JSON, empty, `{}`, a key missing) is
// CORRUPT: at release N it is renamed to <file>.corrupt-<unix> and a new
// identity is created; with RefuseCorruptState (N+1) the daemon refuses to
// start. Any other read error (permission) is neither: it is returned.
// Why: an unreadable file was treated as absent and a new key and mesh IP were
// written over it (#870).
// Invariants: the corrupt bytes are never deleted or overwritten; the new
// identity goes to the original path; no log or error prints key material.
//
// These tests point localNodeStateDir at a temp dir and set RefuseCorruptState,
// so none of them runs in parallel.

// identityTestDaemon returns a daemon whose state file will live in a fresh
// temp dir, and that file's path.
func identityTestDaemon(t *testing.T) (*Daemon, string) {
	t.Helper()
	cfg, err := NewConfig(DaemonOpts{Secret: "identity-state-test-secret-000000"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	old := localNodeStateDir
	localNodeStateDir = dir
	t.Cleanup(func() { localNodeStateDir = old })
	return &Daemon{config: cfg}, filepath.Join(dir, cfg.InterfaceName+".json")
}

func setRefuseCorruptState(t *testing.T, v bool) {
	t.Helper()
	old := RefuseCorruptState
	RefuseCorruptState = v
	t.Cleanup(func() { RefuseCorruptState = old })
}

func corruptCopies(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLoadLocalNodeDistinguishesMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content *string // nil: no file
		want    error   // nil: loads
	}{
		{"no file", nil, ErrStateMissing},
		{"garbage", strPtr("not json at all"), ErrStateCorrupt},
		{"truncated json", strPtr(`{"wg_pubkey": "abc`), ErrStateCorrupt},
		{"zero bytes", strPtr(""), ErrStateCorrupt},
		{"empty object", strPtr("{}"), ErrStateCorrupt},
		{"no private key", strPtr(`{"wg_pubkey":"pub"}`), ErrStateCorrupt},
		{"no public key", strPtr(`{"wg_private_key":"priv"}`), ErrStateCorrupt},
		{"good file", strPtr(`{"wg_pubkey":"pub","wg_private_key":"priv"}`), nil},
	}
	for i, tc := range cases {
		path := filepath.Join(dir, string(rune('a'+i))+".json")
		if tc.content != nil {
			if err := os.WriteFile(path, []byte(*tc.content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		node, err := loadLocalNode(path)
		if tc.want == nil {
			if err != nil || node == nil || node.WGPubKey != "pub" || node.WGPrivateKey != "priv" {
				t.Errorf("%s: got %+v, %v; want the node", tc.name, node, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want errors.Is %v", tc.name, err, tc.want)
		}
		other := ErrStateMissing
		if tc.want == ErrStateMissing {
			other = ErrStateCorrupt
		}
		if errors.Is(err, other) {
			t.Errorf("%s: err = %v is also %v", tc.name, err, other)
		}
		if tc.want == ErrStateMissing && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: ErrStateMissing must wrap os.ErrNotExist, got %v", tc.name, err)
		}
		if node != nil {
			t.Errorf("%s: node = %+v, want nil", tc.name, node)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestInitLocalNodeCorruptMovesAside(t *testing.T) {
	setRefuseCorruptState(t, false)
	for _, bad := range []string{"not json at all", "", "{}", `{"wg_pubkey":"pub"}`} {
		d, path := identityTestDaemon(t)
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := d.initLocalNode(); err != nil {
			t.Fatalf("%q: initLocalNode: %v", bad, err)
		}
		moved := corruptCopies(t, path)
		if len(moved) != 1 {
			t.Fatalf("%q: corrupt copies = %v, want exactly one", bad, moved)
		}
		got, err := os.ReadFile(moved[0])
		if err != nil || !bytes.Equal(got, []byte(bad)) {
			t.Errorf("%q: moved file holds %q (%v), want the original bytes", bad, got, err)
		}
		fresh, err := loadLocalNode(path)
		if err != nil {
			t.Fatalf("%q: new identity at %s does not load: %v", bad, path, err)
		}
		if fresh.WGPubKey == "" || fresh.WGPrivateKey == "" || fresh.MeshIP == "" {
			t.Errorf("%q: new identity incomplete: %+v", bad, fresh)
		}
		if d.localNode == nil || d.localNode.WGPubKey != fresh.WGPubKey {
			t.Errorf("%q: daemon identity differs from the saved one", bad)
		}
	}
}

func TestQuarantineStateNeverTakesAnExistingName(t *testing.T) {
	_, path := identityTestDaemon(t)
	if err := os.WriteFile(path, []byte("garbage"), 0600); err != nil {
		t.Fatal(err)
	}
	// Two quarantines in the same second must not share a name; each keeps its
	// own bytes.
	first, err := quarantineState(path)
	if err != nil {
		t.Fatalf("quarantineState: %v", err)
	}
	if err := os.WriteFile(path, []byte("second garbage"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := quarantineState(path)
	if err != nil {
		t.Fatalf("quarantineState again: %v", err)
	}
	if first == second {
		t.Fatalf("both quarantines used %s", first)
	}
	a, _ := os.ReadFile(first)
	b, _ := os.ReadFile(second)
	if string(a) != "garbage" || string(b) != "second garbage" {
		t.Errorf("quarantined bytes = %q, %q", a, b)
	}
}

func TestInitLocalNodeRefusesWhenEnforced(t *testing.T) {
	setRefuseCorruptState(t, true)
	d, path := identityTestDaemon(t)
	bad := []byte(`{"wg_pubkey":"pub"}`)
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	err := d.initLocalNode()
	if err == nil {
		t.Fatal("initLocalNode succeeded on a corrupt file with RefuseCorruptState")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the file %s", err, path)
	}
	if !errors.Is(err, ErrStateCorrupt) {
		t.Errorf("error %v is not ErrStateCorrupt", err)
	}
	got, rerr := os.ReadFile(path)
	if rerr != nil || !bytes.Equal(got, bad) {
		t.Errorf("file after refusal = %q (%v), want it byte-identical", got, rerr)
	}
	if m := corruptCopies(t, path); len(m) != 0 {
		t.Errorf("refusal moved the file: %v", m)
	}
	if d.localNode != nil {
		t.Errorf("a key was generated: %+v", d.localNode)
	}
}

func TestInitLocalNodeMissingGeneratesKey(t *testing.T) {
	setRefuseCorruptState(t, true) // missing is not corrupt: the flag must not matter
	d, path := identityTestDaemon(t)
	if err := d.initLocalNode(); err != nil {
		t.Fatalf("initLocalNode: %v", err)
	}
	fresh, err := loadLocalNode(path)
	if err != nil || fresh.WGPubKey == "" || fresh.WGPrivateKey == "" {
		t.Fatalf("no identity written at %s: %+v, %v", path, fresh, err)
	}
	if m := corruptCopies(t, path); len(m) != 0 {
		t.Errorf("a missing file produced corrupt copies: %v", m)
	}
}

func TestInitLocalNodeUnreadableIsNotAbsent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not stop root from reading, so an unreadable file cannot be made")
	}
	setRefuseCorruptState(t, false)
	d, path := identityTestDaemon(t)
	orig := []byte(`{"wg_pubkey":"pub","wg_private_key":"priv"}`)
	if err := os.WriteFile(path, orig, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })

	err := d.initLocalNode()
	if err == nil {
		t.Fatal("initLocalNode succeeded on a file it cannot read")
	}
	if errors.Is(err, ErrStateMissing) || errors.Is(err, ErrStateCorrupt) {
		t.Errorf("a permission error was classified as %v", err)
	}
	if d.localNode != nil {
		t.Errorf("a key was generated: %+v", d.localNode)
	}
	if m := corruptCopies(t, path); len(m) != 0 {
		t.Errorf("an unreadable file was moved: %v", m)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, orig) {
		t.Errorf("file after the error = %q, want it untouched", got)
	}
}
