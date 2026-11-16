package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

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
func writeDump(t *testing.T, dir, iface string, handshakes map[string]int64) {
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
	if err := os.Rename(tmp, filepath.Join(dir, "dump."+iface)); err != nil {
		t.Fatal(err)
	}
}

func showForks(t *testing.T, dir string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "calls"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.HasPrefix(c, "show ") {
			n++
		}
	}
	return n
}

// TestStaleCheckReadsOneSnapshotForAllPeers: five peers inside their
// rendezvous phase, each with a fresh handshake (so tryRendezvousForPeer
// returns after the stale check), cost at most one show fork.
func TestStaleCheckReadsOneSnapshotForAllPeers(t *testing.T) {
	dir := installFakeWG(t)
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-fork-stale-check-1"})
	if err != nil {
		t.Fatal(err)
	}
	local := &daemon.LocalNode{WGPubKey: aadKey(1), MeshIP: "10.0.0.1"}
	store := daemon.NewPeerStore()
	d, err := NewDHTDiscovery(context.Background(), cfg, local, store)
	if err != nil {
		t.Fatal(err)
	}

	// Pick keys whose rendezvous phase started at most 1 s ago, so all five
	// stay inside the 4 s phase for the length of this test.
	now := time.Now()
	fresh := map[string]int64{}
	for i := 2; len(fresh) < 5 && i < 256; i++ {
		key := aadKey(byte(i))
		if !d.shouldAttemptRendezvous(key, now) || !d.shouldAttemptRendezvous(key, now.Add(-time.Second)) {
			continue
		}
		fresh[key] = now.Add(-10 * time.Second).Unix()
		store.Update(&daemon.PeerInfo{WGPubKey: key, MeshIP: fmt.Sprintf("10.0.1.%d", i), Endpoint: "203.0.113.5:51820", LastSeen: now}, "test")
	}
	if len(fresh) < 5 {
		t.Fatalf("found only %d keys inside the rendezvous phase", len(fresh))
	}
	writeDump(t, dir, cfg.InterfaceName, fresh)

	d.checkStaleHandshakes()

	if got := showForks(t, dir); got != 1 {
		t.Fatalf("show forks for the stale check over 5 peers = %d, want 1", got)
	}
}

// TestPunchWaitForksEveryPoll: the wait for a punch handshake forks on each
// 250 ms poll and returns the handshake that appears mid-wait.
func TestPunchWaitForksEveryPoll(t *testing.T) {
	dir := installFakeWG(t)
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-fork-punch-wait-1"})
	if err != nil {
		t.Fatal(err)
	}
	peer := aadKey(2)
	writeDump(t, dir, cfg.InterfaceName, map[string]int64{peer: 0})
	pe := NewPeerExchange(cfg, &daemon.LocalNode{WGPubKey: aadKey(1), MeshIP: "10.0.0.1"}, daemon.NewPeerStore())

	go func() {
		time.Sleep(300 * time.Millisecond)
		writeDump(t, dir, cfg.InterfaceName, map[string]int64{peer: 100})
	}()
	got := pe.waitForHandshake(peer, 0, time.Second)

	if got != 100 {
		t.Fatalf("waitForHandshake = %d, want 100 (the handshake that appeared at 300 ms)", got)
	}
	if forks := showForks(t, dir); forks < 3 {
		t.Fatalf("show forks during the wait = %d, want at least 3 (one per 250 ms poll)", forks)
	}
}
