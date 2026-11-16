package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Invariants (the rule text lives in helpers.go and ADR-0005): a file that is
// MISSING gets a new identity; one that is CORRUPT is moved aside, never
// deleted or overwritten, and replaced (or refused with RefuseCorruptState);
// any other read error is neither and is returned. No log or error prints key
// material.
//
// These tests point localNodeStateDir at a temp dir, set RefuseCorruptState
// and capture the log, so none of them runs in parallel.

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

// corruptSamples are the inputs every corrupt-state test shares: loadLocalNode
// must call each one ErrStateCorrupt and initLocalNode must move each aside.
var corruptSamples = []struct{ name, content string }{
	{"garbage", "not json at all"},
	{"truncated json", `{"wg_pubkey": "abc`},
	{"zero bytes", ""},
	{"empty object", "{}"},
	{"no private key", `{"wg_pubkey":"pub"}`},
	{"no public key", `{"wg_private_key":"priv"}`},
}

func TestLoadLocalNodeDistinguishesMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	type tcase struct {
		name    string
		content *string // nil: no file
		want    error   // nil: loads
	}
	cases := []tcase{
		{"no file", nil, ErrStateMissing},
		{"good file", strPtr(`{"wg_pubkey":"pub","wg_private_key":"priv"}`), nil},
	}
	for _, s := range corruptSamples {
		cases = append(cases, tcase{s.name, strPtr(s.content), ErrStateCorrupt})
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

// captureLog returns what log.Printf writes until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return &buf
}

// The corrupt file may hold a private key: no error may carry the text of the
// JSON decoder, which can quote an input byte.
func TestCorruptStateErrorsCarryNoFileContent(t *testing.T) {
	const bad = `{"wg_pubkey":"AAAA","wg_private_key":"QUJD"DEF"}`
	check := func(what string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: no error", what)
		}
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			t.Errorf("%s: error wraps the decoder's error: %v", what, err)
		}
		for _, leak := range []string{"QUJD", "DEF", "invalid character"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("%s: error %q contains %q", what, err, leak)
			}
		}
	}
	d, path := identityTestDaemon(t)
	if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := loadLocalNode(path)
	check("loadLocalNode", err)
	if !errors.Is(err, ErrStateCorrupt) {
		t.Errorf("loadLocalNode: %v is not ErrStateCorrupt", err)
	}

	setRefuseCorruptState(t, true)
	check("initLocalNode", d.initLocalNode())
}

func TestInitLocalNodeCorruptMovesAside(t *testing.T) {
	setRefuseCorruptState(t, false)
	for _, s := range corruptSamples {
		bad := s.content
		d, path := identityTestDaemon(t)
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		logged := captureLog(t)
		if err := d.initLocalNode(); err != nil {
			t.Fatalf("%q: initLocalNode: %v", bad, err)
		}
		moved := corruptCopies(t, path)
		if len(moved) != 1 {
			t.Fatalf("%q: corrupt copies = %v, want exactly one", bad, moved)
		}
		if !strings.Contains(logged.String(), path) || !strings.Contains(logged.String(), moved[0]) {
			t.Errorf("%q: log %q names neither the original path %s nor the moved file %s", bad, logged, path, moved[0])
		}
		got, err := os.ReadFile(moved[0])
		if err != nil || !bytes.Equal(got, []byte(bad)) {
			t.Errorf("%q: moved file holds %q (%v), want the original bytes", bad, got, err)
		}
		fresh, err := loadLocalNode(path)
		if err != nil {
			t.Fatalf("%q: new identity at %s does not load: %v", bad, path, err)
		}
		if fresh.WGPubKey == "" || fresh.WGPrivateKey == "" || fresh.GetMeshIP() == "" {
			t.Errorf("%q: new identity incomplete: %+v", bad, fresh)
		}
		if d.localNode == nil || d.localNode.WGPubKey != fresh.WGPubKey {
			t.Errorf("%q: daemon identity differs from the saved one", bad)
		}
	}
}

// sameSecond runs fn with the unix second quarantineState will use for its
// base name and reports whether the clock stayed in that second while fn ran;
// a test that pre-creates names for a second retries when it did not.
func sameSecond(fn func(sec int64)) bool {
	sec := time.Now().Unix()
	fn(sec)
	return time.Now().Unix() == sec
}

func TestQuarantineStateNeverTakesAnExistingName(t *testing.T) {
	for attempt := 0; attempt < 5; attempt++ {
		var taken, got string
		var gotErr error
		var path string
		ok := sameSecond(func(sec int64) {
			_, path = identityTestDaemon(t)
			taken = fmt.Sprintf("%s.corrupt-%d", path, sec)
			if err := os.WriteFile(taken, []byte("older quarantine"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("garbage"), 0644); err != nil {
				t.Fatal(err)
			}
			got, gotErr = quarantineState(path)
		})
		if !ok {
			continue
		}
		if gotErr != nil {
			t.Fatalf("quarantineState: %v", gotErr)
		}
		if got == taken {
			t.Fatalf("quarantine reused the taken name %s", taken)
		}
		if b, _ := os.ReadFile(taken); string(b) != "older quarantine" {
			t.Errorf("the pre-existing %s now holds %q", taken, b)
		}
		if b, _ := os.ReadFile(got); string(b) != "garbage" {
			t.Errorf("%s holds %q, want the quarantined bytes", got, b)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the old name %s still exists (%v)", path, err)
		}
		if fi, err := os.Stat(got); err != nil || fi.Mode().Perm() != 0600 {
			t.Errorf("%s mode = %v (%v), want 0600: it may hold a private key", got, fi, err)
		}
		return
	}
	t.Fatal("the clock crossed a second boundary in 5 attempts")
}

func TestQuarantineStateGivesUpWithoutTouchingAnything(t *testing.T) {
	for attempt := 0; attempt < 5; attempt++ {
		var path string
		var before []string
		var gotErr error
		ok := sameSecond(func(sec int64) {
			_, path = identityTestDaemon(t)
			base := fmt.Sprintf("%s.corrupt-%d", path, sec)
			names := []string{base}
			for i := 1; i <= 1000; i++ {
				names = append(names, fmt.Sprintf("%s-%d", base, i))
			}
			for _, n := range names {
				if err := os.WriteFile(n, []byte("taken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, []byte("garbage"), 0600); err != nil {
				t.Fatal(err)
			}
			before = corruptCopies(t, path)
			_, gotErr = quarantineState(path)
		})
		if !ok {
			continue
		}
		if gotErr == nil {
			t.Fatal("quarantineState succeeded with every name taken")
		}
		if b, _ := os.ReadFile(path); string(b) != "garbage" {
			t.Errorf("the state file now holds %q, want it untouched", b)
		}
		if after := corruptCopies(t, path); len(after) != len(before) || len(before) != 1001 {
			t.Errorf("corrupt copies before=%d after=%d, want 1001 unchanged", len(before), len(after))
		}
		return
	}
	t.Fatal("the clock crossed a second boundary in 5 attempts")
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

// A directory at the state path is a read error that is neither "missing" nor
// "corrupt" (ReadFile returns EISDIR), and it works the same for root and not.
func TestInitLocalNodeUnreadableIsNotAbsent(t *testing.T) {
	setRefuseCorruptState(t, false)
	d, path := identityTestDaemon(t)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep")
	if err := os.WriteFile(inner, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	err := d.initLocalNode()
	if err == nil {
		t.Fatal("initLocalNode succeeded on a state path it cannot read")
	}
	if errors.Is(err, ErrStateMissing) || errors.Is(err, ErrStateCorrupt) {
		t.Errorf("a read error was classified as %v", err)
	}
	if d.localNode != nil {
		t.Errorf("a key was generated: %+v", d.localNode)
	}
	if m := corruptCopies(t, path); len(m) != 0 {
		t.Errorf("an unreadable path was moved: %v", m)
	}
	if fi, serr := os.Stat(path); serr != nil || !fi.IsDir() {
		t.Errorf("the state path after the error: %v, %v; want the directory untouched", fi, serr)
	}
	if got, _ := os.ReadFile(inner); string(got) != "x" {
		t.Errorf("the directory's content changed: %q", got)
	}
}
