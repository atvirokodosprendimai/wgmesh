package discovery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// wgReadRecorder is the readWGState stub: it records the maxAge of every
// read and answers with a one-peer State whose handshake hsAt(call) picks
// (call counts from 1).
type wgReadRecorder struct {
	mu   sync.Mutex
	ages []time.Duration
}

func (r *wgReadRecorder) all() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.ages...)
}

func installReadWGState(t *testing.T, key string, hsAt func(call int) int64) *wgReadRecorder {
	t.Helper()
	rec := &wgReadRecorder{}
	stub := func(_ string, maxAge time.Duration) (*wireguard.State, error) {
		rec.mu.Lock()
		rec.ages = append(rec.ages, maxAge)
		n := len(rec.ages)
		rec.mu.Unlock()
		return &wireguard.State{Peers: []wireguard.StatePeer{{PublicKey: key, LatestHandshake: hsAt(n)}}}, nil
	}
	old := readWGState.Load()
	readWGState.Store(&stub)
	t.Cleanup(func() { readWGState.Store(old) })
	return rec
}

// TestPunchWaitReadsFreshEveryPoll pins the punch wait to maxAge 0: each poll
// and the final read after the timeout must fork, never reuse a snapshot.
func TestPunchWaitReadsFreshEveryPoll(t *testing.T) {
	key := aadKey(2)
	rec := installReadWGState(t, key, func(call int) int64 {
		if call < 3 {
			return 0
		}
		return 100
	})
	if got := waitForPeerHandshake("wgfork-punch0", key, 0, 2*time.Second); got != 100 {
		t.Fatalf("waitForPeerHandshake = %d, want 100", got)
	}
	if ages := rec.all(); len(ages) < 3 {
		t.Fatalf("reads = %d, want at least 3 polls", len(ages))
	}
	for i, age := range rec.all() {
		if age != 0 {
			t.Errorf("poll read %d used maxAge %v, want 0", i+1, age)
		}
	}

	// No handshake ever: the loop times out and the final read must be fresh too.
	rec = installReadWGState(t, key, func(int) int64 { return 0 })
	if got := waitForPeerHandshake("wgfork-punch0", key, 0, 100*time.Millisecond); got != 0 {
		t.Fatalf("waitForPeerHandshake without handshake = %d, want 0", got)
	}
	ages := rec.all()
	if len(ages) < 2 {
		t.Fatalf("reads = %d, want the polls plus the final read", len(ages))
	}
	for i, age := range ages {
		if age != 0 {
			t.Errorf("timeout-path read %d used maxAge %v, want 0", i+1, age)
		}
	}
}

// TestGetPeerHandshakeTSPassesMaxAge: the caller's staleness reaches the read.
func TestGetPeerHandshakeTSPassesMaxAge(t *testing.T) {
	key := aadKey(2)
	rec := installReadWGState(t, key, func(int) int64 { return 42 })
	for _, want := range []time.Duration{0, 7 * time.Second} {
		if got := getPeerHandshakeTS("wgfork-ts0", key, want); got != 42 {
			t.Fatalf("getPeerHandshakeTS = %d, want 42", got)
		}
	}
	ages := rec.all()
	if len(ages) != 2 || ages[0] != 0 || ages[1] != 7*time.Second {
		t.Fatalf("maxAges = %v, want [0 7s]", ages)
	}
}

// TestStaleHandshakeCheckReadsAtReconcileInterval: the skip check of
// tryRendezvousForPeer accepts a snapshot up to ReconcileInterval old.
func TestStaleHandshakeCheckReadsAtReconcileInterval(t *testing.T) {
	key := aadKey(2)
	rec := installReadWGState(t, key, func(int) int64 { return time.Now().Unix() })
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-stale-maxage-1"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDHTDiscovery(context.Background(), cfg, &daemon.LocalNode{WGPubKey: aadKey(1), MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}

	d.tryRendezvousForPeer(&daemon.PeerInfo{WGPubKey: key, MeshIP: "10.0.0.2", Endpoint: "203.0.113.5:51820"})

	ages := rec.all()
	if len(ages) != 1 || ages[0] != daemon.ReconcileInterval {
		t.Fatalf("maxAges = %v, want [%v] (one skip-check read at ReconcileInterval)", ages, daemon.ReconcileInterval)
	}
}

// TestIntroducerRankingReadsSnapshotWithinReconcileInterval: selectRendezvousIntroducers
// over several candidates forks wg show at most once, and a second call
// inside ReconcileInterval reuses that snapshot (a read with maxAge 0 would
// fork again).
func TestIntroducerRankingReadsSnapshotWithinReconcileInterval(t *testing.T) {
	dir := installFakeWG(t)
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-fork-introducers-1"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDHTDiscovery(context.Background(), cfg, &daemon.LocalNode{WGPubKey: aadKey(1), MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}
	var peers []*daemon.PeerInfo
	hs := map[string]int64{}
	for i := 3; i < 8; i++ {
		key := aadKey(byte(i))
		hs[key] = time.Now().Unix()
		peers = append(peers, &daemon.PeerInfo{
			WGPubKey: key, MeshIP: fmt.Sprintf("10.0.1.%d", i), Endpoint: "203.0.113.5:51820",
			DiscoveredVia: []string{DHTMethod}, LastSeen: time.Now(),
		})
	}
	writeDump(t, dir, cfg.InterfaceName, hs)

	d.selectRendezvousIntroducers(aadKey(2), peers, 3)
	if got := showForks(t, dir); got != 1 {
		t.Fatalf("show forks for one ranking over %d candidates = %d, want 1", len(peers), got)
	}
	d.selectRendezvousIntroducers(aadKey(2), peers, 3)
	if got := showForks(t, dir); got != 1 {
		t.Fatalf("show forks after a second ranking inside ReconcileInterval = %d, want still 1", got)
	}
}
