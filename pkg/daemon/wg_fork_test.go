package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const forkIface = "wgfork0"

// installFakeWG points pkg/wireguard at a shell script that appends its
// arguments to $FAKEWG_DIR/calls and, for `show`, prints
// $FAKEWG_DIR/dump.<iface>. It returns the directory.
func installFakeWG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FAKEWG_DIR", dir)
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$FAKEWG_DIR/calls\"\n" +
		"[ \"$1\" = show ] && cat \"$FAKEWG_DIR/dump.$2\"\n" +
		"exit 0\n"
	path := filepath.Join(dir, "wg")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wireguard.SetWGPath(path))
	return dir
}

// writeDump renders a `wg show <iface> dump` with one line per key=handshake.
func writeDump(t *testing.T, dir string, handshakes map[string]int64) {
	t.Helper()
	var b strings.Builder
	b.WriteString("privkey\tpubkey\t51820\toff\n")
	for k, hs := range handshakes {
		fmt.Fprintf(&b, "%s\t(none)\t1.2.3.4:51820\t10.0.0.9/32\t%d\t10\t20\toff\n", k, hs)
	}
	tmp := filepath.Join(dir, "dump.tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "dump."+forkIface)); err != nil {
		t.Fatal(err)
	}
}

func fakeCalls(t *testing.T, dir string) []string {
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

func showForks(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	for _, c := range fakeCalls(t, dir) {
		if strings.HasPrefix(c, "show ") {
			n++
		}
	}
	return n
}

func forkTestDaemon() *Daemon {
	d := &Daemon{
		config:                &Config{InterfaceName: forkIface},
		localNode:             NewLocalNode("local-key", "", "", "10.0.0.1", ""),
		peerStore:             NewPeerStore(),
		directStableCycles:    map[string]int{},
		relayRoutes:           map[string]string{},
		peerHealthFailures:    map[string]int{},
		lastPeerTransferTotal: map[string]uint64{},
		probeSessions:         map[string]*peerProbeSession{},
		probeFailures:         map[string]int{},
		temporaryOffline:      map[string]time.Time{},
		localSubnetsFn:        func() []*net.IPNet { return nil },
	}
	d.peerStore.Update(&PeerInfo{WGPubKey: "peer-a", MeshIP: "10.0.0.2", LastSeen: time.Now()}, "test")
	return d
}

// withNoopExecutor keeps reconcile's `ip` calls away from the host.
func withNoopExecutor(t *testing.T, fn func()) {
	t.Helper()
	withMockExecutor(t, &MockCommandExecutor{}, fn)
}

func TestOneShowForkPerReconcileTick(t *testing.T) {
	dir := installFakeWG(t)
	writeDump(t, dir, map[string]int64{"peer-a": time.Now().Unix()})
	d := forkTestDaemon()

	withNoopExecutor(t, func() {
		d.reconcile()
		d.probePeersOverMesh()
		d.checkPeerHealth()
	})

	if got := showForks(t, dir); got != 1 {
		t.Fatalf("show forks for one reconcile + probe + health = %d, want 1; calls: %q", got, fakeCalls(t, dir))
	}
}

func TestProbeAndHealthReuseSnapshotInsideReconcileInterval(t *testing.T) {
	dir := installFakeWG(t)
	writeDump(t, dir, map[string]int64{"peer-a": time.Now().Unix()})
	d := forkTestDaemon()

	if _, err := wireguard.ReadState(forkIface, 0); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		d.probePeersOverMesh()
	}
	for i := 0; i < 10; i++ {
		d.checkPeerHealth()
	}

	if got := showForks(t, dir); got != 1 {
		t.Fatalf("show forks after the priming read = %d, want 1 (no fork from 100 probes and 10 health checks)", got)
	}
}

// This test sleeps ReconcileInterval + 200 ms (about 5.2 s of wall time): the
// snapshot clock is the real one.
func TestProbeRefreshesSnapshotOlderThanReconcileInterval(t *testing.T) {
	dir := installFakeWG(t)
	writeDump(t, dir, map[string]int64{"peer-a": 0})
	d := forkTestDaemon()
	d.markTemporarilyOffline("peer-a")

	if _, err := wireguard.ReadState(forkIface, 0); err != nil {
		t.Fatal(err)
	}
	writeDump(t, dir, map[string]int64{"peer-a": time.Now().Unix()})
	time.Sleep(ReconcileInterval + 200*time.Millisecond)

	d.probePeersOverMesh()

	if got := showForks(t, dir); got != 2 {
		t.Fatalf("show forks = %d, want 2 (priming read + one refresh)", got)
	}
	if d.isTemporarilyOffline("peer-a") {
		t.Fatal("the probe did not see the new handshake: peer-a is still temporarily offline")
	}
	// The refresh is a maxAge ReconcileInterval read, not a fork per probe: the
	// next probe inside the interval reuses it.
	d.probePeersOverMesh()
	if got := showForks(t, dir); got != 2 {
		t.Fatalf("show forks after a second probe = %d, want still 2 (the refreshed snapshot is reused)", got)
	}
}

func TestReconcileAlwaysReadsFresh(t *testing.T) {
	dir := installFakeWG(t)
	writeDump(t, dir, map[string]int64{"peer-a": time.Now().Unix()})
	d := forkTestDaemon()

	if _, err := wireguard.ReadState(forkIface, 0); err != nil {
		t.Fatal(err)
	}
	// obsolete-peer appears after the priming read; only a fresh read sees it.
	writeDump(t, dir, map[string]int64{"peer-a": time.Now().Unix(), "obsolete-peer": 0})

	withNoopExecutor(t, d.reconcile)

	if got := showForks(t, dir); got != 2 {
		t.Fatalf("show forks = %d, want 2 (priming read + the reconcile read)", got)
	}
	want := "set " + forkIface + " peer obsolete-peer remove"
	for _, c := range fakeCalls(t, dir) {
		if c == want {
			return
		}
	}
	t.Fatalf("reconcile did not remove the peer only the fresh dump lists; calls: %q", fakeCalls(t, dir))
}

// installFailingWG is installFakeWG for a wg that logs its arguments and
// exits 1 on every call, `show` included.
func installFailingWG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FAKEWG_DIR", dir)
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$FAKEWG_DIR/calls\"\n" +
		"exit 1\n"
	path := filepath.Join(dir, "wg")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wireguard.SetWGPath(path))
	return dir
}

// TestFailedShowDoesNotPanicOrRemovePeers: when `wg show` fails, reconcile
// has no snapshot, so it must not remove any peer; probe and health run on
// an empty handshake map.
func TestFailedShowDoesNotPanicOrRemovePeers(t *testing.T) {
	dir := installFailingWG(t)
	d := forkTestDaemon()

	withNoopExecutor(t, func() {
		d.reconcile()
		d.probePeersOverMesh()
		d.checkPeerHealth()
	})

	if got := showForks(t, dir); got == 0 {
		t.Fatalf("the failing wg was never asked to show; calls: %q", fakeCalls(t, dir))
	}
	for _, c := range fakeCalls(t, dir) {
		if strings.HasSuffix(c, " remove") {
			t.Errorf("a failed read still removed a peer: %q", c)
		}
	}
}
