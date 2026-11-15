package wireguard

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const dumpWG0 = "PRIV\tPUB\t51820\toff\n" +
	"keyA\t(none)\t203.0.113.5:51820\t10.0.0.2/32\t1700000000\t111\t222\t25\n" +
	"keyB\t(none)\t(none)\t10.0.0.3/32\t0\t0\t0\toff\n"

// fakeWG installs a shell script as `wg`. It appends its arguments to
// $FAKEWG_DIR/calls, fails when $FAKEWG_DIR/fail exists, and otherwise prints
// $FAKEWG_DIR/dump.<second argument>.
func fakeWG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FAKEWG_DIR", dir)
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$FAKEWG_DIR/calls\"\n" +
		"if [ -e \"$FAKEWG_DIR/fail\" ]; then echo boom >&2; exit 1; fi\n" +
		"cat \"$FAKEWG_DIR/dump.$2\"\n"
	path := filepath.Join(dir, "wg")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(SetWGPath(path))
	return dir
}

func setDump(t *testing.T, dir, iface, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "dump."+iface), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func calls(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "calls"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestReadStateParsesDumpColumns(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)

	st, err := ReadState("wg0", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Peers) != 2 {
		t.Fatalf("peers = %d, want 2: %+v", len(st.Peers), st.Peers)
	}
	a, b := st.Peers[0], st.Peers[1]
	if a.PublicKey != "keyA" || a.Endpoint != "203.0.113.5:51820" || a.LatestHandshake != 1700000000 || a.RxBytes != 111 || a.TxBytes != 222 {
		t.Errorf("peer A = %+v", a)
	}
	if b.PublicKey != "keyB" || b.Endpoint != "" || b.LatestHandshake != 0 || b.RxBytes != 0 || b.TxBytes != 0 {
		t.Errorf("peer B = %+v", b)
	}
}

func TestReadStateReusesResultInsideMaxAge(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)

	if _, err := ReadState("wg0", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadState("wg0", time.Hour); err != nil {
		t.Fatal(err)
	}
	if n := len(calls(t, dir)); n != 1 {
		t.Fatalf("forks = %d, want 1", n)
	}
}

func TestReadStateZeroMaxAgeAlwaysForks(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)

	first, err := ReadState("wg0", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Peers) != 2 {
		t.Fatalf("first peers = %d, want 2", len(first.Peers))
	}
	setDump(t, dir, "wg0", "PRIV\tPUB\t51820\toff\n")
	second, err := ReadState("wg0", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Peers) != 0 {
		t.Fatalf("second peers = %d, want 0 (changed output)", len(second.Peers))
	}
	if n := len(calls(t, dir)); n != 2 {
		t.Fatalf("forks = %d, want 2", n)
	}
}

func TestReadStateDoesNotStoreErrors(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)
	if err := os.WriteFile(filepath.Join(dir, "fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadState("wg0", time.Hour); err == nil {
		t.Fatal("want an error from the failing wg")
	}
	if err := os.Remove(filepath.Join(dir, "fail")); err != nil {
		t.Fatal(err)
	}
	st, err := ReadState("wg0", time.Hour)
	if err != nil {
		t.Fatalf("second call returned the stored error: %v", err)
	}
	if len(st.Peers) != 2 {
		t.Fatalf("peers = %d, want 2", len(st.Peers))
	}
	if n := len(calls(t, dir)); n != 2 {
		t.Fatalf("forks = %d, want 2 (the error must not be cached)", n)
	}
}

func TestRunWGKillsChildAfterTimeout(t *testing.T) {
	dir := fakeWG(t)
	script := "#!/bin/sh\necho $$ > \"$FAKEWG_DIR/pid\"\nexec sleep 30\n"
	path := filepath.Join(dir, "wg-slow")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(SetWGPath(path))

	old := wgCommandTimeout
	wgCommandTimeout = 200 * time.Millisecond
	t.Cleanup(func() { wgCommandTimeout = old })

	start := time.Now()
	_, _, err := runWG(nil, "show", "wg0", "dump")
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("runWG took %s, want < 3s", d)
	}
	if err == nil || !strings.Contains(err.Error(), "timed out after 200ms") || !strings.Contains(err.Error(), "show") {
		t.Fatalf("error = %v, want one naming the sub-command and the timeout", err)
	}

	b, rerr := os.ReadFile(filepath.Join(dir, "pid"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	var pid int
	for _, c := range strings.TrimSpace(string(b)) {
		pid = pid*10 + int(c-'0')
	}
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("child %d is still alive", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestReadStateKeepsSlotPerInterface(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)
	setDump(t, dir, "wg1", "PRIV\tPUB\t51821\toff\nkeyZ\t(none)\t(none)\t10.1.0.2/32\t5\t6\t7\toff\n")

	s0, err := ReadState("wg0", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s1, err := ReadState("wg1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(s0.Peers) != 2 || len(s1.Peers) != 1 || s1.Peers[0].PublicKey != "keyZ" {
		t.Fatalf("wg0 = %+v, wg1 = %+v", s0.Peers, s1.Peers)
	}
	if n := len(calls(t, dir)); n != 2 {
		t.Fatalf("forks = %d, want 2", n)
	}
}

func TestLegacyReadersForwardToReadState(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)

	if _, err := ReadState("wg0", time.Hour); err != nil {
		t.Fatal(err)
	}
	hs, err := GetLatestHandshakes("wg0")
	if err != nil {
		t.Fatal(err)
	}
	peers, err := GetPeers("wg0")
	if err != nil {
		t.Fatal(err)
	}
	tr, err := GetPeerTransfers("wg0")
	if err != nil {
		t.Fatal(err)
	}

	if len(hs) != 2 || hs["keyA"] != 1700000000 || hs["keyB"] != 0 {
		t.Errorf("handshakes = %v", hs)
	}
	if len(peers) != 2 || peers[0].PublicKey != "keyA" || peers[1].PublicKey != "keyB" {
		t.Errorf("peers = %+v", peers)
	}
	if len(tr) != 2 || tr["keyA"] != (PeerTransfer{RxBytes: 111, TxBytes: 222}) || tr["keyB"] != (PeerTransfer{}) {
		t.Errorf("transfers = %v", tr)
	}
	// One fork for the ReadState above and one per legacy reader (they ask
	// maxAge 0); every fork is the single `show wg0 dump` sub-command.
	got := calls(t, dir)
	if len(got) != 4 {
		t.Fatalf("forks = %d, want 4: %q", len(got), got)
	}
	for _, c := range got {
		if c != "show wg0 dump" {
			t.Errorf("fork %q, want only `show wg0 dump`", c)
		}
	}
}

func TestReadStateMalformedColumnsCountAsZero(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", "PRIV\tPUB\t51820\toff\n"+
		"keyA\t(none)\t(none)\t10.0.0.2/32\tx\t7\t8\toff\n"+
		"keyB\t(none)\t(none)\t10.0.0.3/32\t1700000000\t5\t6\toff\n")

	hs, err := GetLatestHandshakes("wg0")
	if err != nil {
		t.Fatalf("malformed column failed the whole read: %v", err)
	}
	if len(hs) != 2 || hs["keyA"] != 0 || hs["keyB"] != 1700000000 {
		t.Errorf("handshakes = %v, want keyA 0 and keyB intact", hs)
	}
	tr, err := GetPeerTransfers("wg0")
	if err != nil {
		t.Fatal(err)
	}
	if tr["keyA"] != (PeerTransfer{RxBytes: 7, TxBytes: 8}) || tr["keyB"] != (PeerTransfer{RxBytes: 5, TxBytes: 6}) {
		t.Errorf("transfers = %v", tr)
	}
}
