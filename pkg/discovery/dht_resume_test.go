package discovery

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

const (
	resumeOldIP = "203.0.113.10"
	resumeNewIP = "198.51.100.20"
)

// resumeProbe counts what a layer under test did instead of touching the network.
type resumeProbe struct {
	stun     atomic.Int32
	announce atomic.Int32
	mu       sync.Mutex
	sentTo   []*net.UDPAddr
}

func (p *resumeProbe) sends() []*net.UDPAddr {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*net.UDPAddr(nil), p.sentTo...)
}

func resumeEndpoint(d *DHTDiscovery, ip string) string {
	return net.JoinHostPort(ip, strconv.Itoa(d.config.WGListenPort))
}

// resumeDHT builds a layer that counts as started for OnResume's purpose, with
// every network hook replaced by a counter. The STUN stub answers resumeNewIP.
func resumeDHT(t *testing.T) (*DHTDiscovery, *resumeProbe) {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-resume-1"})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	cfg.DisableIPv6 = true
	local := &daemon.LocalNode{WGPubKey: "local", MeshIP: "10.250.0.1"}
	d, err := NewDHTDiscovery(context.Background(), cfg, local, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery: %v", err)
	}
	t.Cleanup(d.cancel)
	local.SetEndpoint(resumeEndpoint(d, resumeOldIP))

	p := &resumeProbe{}
	d.stunFn = func() (NATType, net.IP, error) {
		p.stun.Add(1)
		return NATCone, net.ParseIP(resumeNewIP), nil
	}
	d.announceFn = func() { p.announce.Add(1) }
	d.sendAnnounceFn = func(a *net.UDPAddr) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.sentTo = append(p.sentTo, a)
		return nil
	}
	d.mu.Lock()
	d.running = true
	d.resumable = true
	d.mu.Unlock()
	return d, p
}

func addActivePeers(t *testing.T, d *DHTDiscovery, ips ...string) {
	t.Helper()
	for i, ip := range ips {
		d.peerStore.Update(&daemon.PeerInfo{
			WGPubKey: "peer-" + strconv.Itoa(i),
			MeshIP:   ip,
			LastSeen: time.Now(),
		}, "lan")
	}
	if got := len(d.peerStore.GetActive()); got != len(ips) {
		t.Fatalf("GetActive = %d peers, want %d", got, len(ips))
	}
}

func TestOnResumeRefreshesSTUNAtOnce(t *testing.T) {
	d, p := resumeDHT(t)
	old := resumeEndpoint(d, resumeOldIP)
	want := resumeEndpoint(d, resumeNewIP)

	// the ticker path keeps rule R7: a changed STUN answer waits for three
	d.refreshSTUNOnce(daemon.EndpointSTUN)
	if got := d.localNode.GetEndpoint(); got != old {
		t.Fatalf("after one ticker pass endpoint = %s, want %s (pending)", got, old)
	}

	d.OnResume()
	d.resumeWG.Wait()
	if got := d.localNode.GetEndpoint(); got != want {
		t.Fatalf("after OnResume endpoint = %s, want %s", got, want)
	}
	if got := d.localNode.GetNATType(); got != string(NATCone) {
		t.Fatalf("NATType = %q, want %q", got, NATCone)
	}
	if got := p.stun.Load(); got != 2 {
		t.Fatalf("STUN queries = %d, want 2 (one ticker pass, one resume)", got)
	}
}

func TestOnResumeAnnouncesNow(t *testing.T) {
	tests := []struct {
		name      string
		conn      bool
		gossip    bool
		wantSends int
	}{
		{"conn and gossip: one send per active peer", true, true, 2},
		{"exchange without a socket: infohash announce only", false, true, 0},
		{"gossip off: infohash announce only", true, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, p := resumeDHT(t)
			d.config.Gossip = tt.gossip
			if tt.gossip {
				g, err := NewMeshGossipWithExchange(d.config, d.localNode, d.peerStore, d.exchange)
				if err != nil {
					t.Fatalf("NewMeshGossipWithExchange: %v", err)
				}
				d.gossip = g
			}
			if tt.conn {
				conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { conn.Close() })
				d.exchange.mu.Lock()
				d.exchange.conn = conn
				d.exchange.mu.Unlock()
			}
			addActivePeers(t, d, "10.250.0.2", "10.250.0.3")

			d.OnResume()
			d.resumeWG.Wait()

			if got := p.announce.Load(); got != 1 {
				t.Fatalf("announce() calls = %d, want 1", got)
			}
			sent := p.sends()
			if len(sent) != tt.wantSends {
				t.Fatalf("SendAnnounce calls = %d, want %d", len(sent), tt.wantSends)
			}
			seen := map[string]bool{}
			for _, a := range sent {
				if a.Port != int(d.config.Keys.GossipPort) {
					t.Errorf("sent to port %d, want the gossip port %d", a.Port, d.config.Keys.GossipPort)
				}
				seen[a.IP.String()] = true
			}
			if tt.wantSends == 2 && (!seen["10.250.0.2"] || !seen["10.250.0.3"]) {
				t.Errorf("sent to %v, want both peers' mesh IPs", seen)
			}
		})
	}
}

func TestOnResumeResetsRendezvousBackoff(t *testing.T) {
	d, _ := resumeDHT(t)
	d.mu.Lock()
	d.rendezvousBackoff["peer-x"] = backoffEntry{nextAttempt: time.Now().Add(time.Hour), duration: time.Hour}
	d.mu.Unlock()
	if d.canAttemptRendezvous("peer-x") {
		t.Fatal("peer in back-off may attempt before the resume: setup is wrong")
	}

	d.OnResume()
	d.resumeWG.Wait()

	if !d.canAttemptRendezvous("peer-x") {
		t.Fatal("peer still in back-off after OnResume")
	}
}

func TestDHTImplementsResumer(t *testing.T) {
	d, _ := resumeDHT(t)
	var layer daemon.DiscoveryLayer = d
	if _, ok := layer.(node.Resumer); !ok {
		t.Fatal("the daemon's DiscoveryLayer value does not satisfy node.Resumer")
	}
}

func TestOnResumeBeforeStartAndAfterStopIsSafe(t *testing.T) {
	assertIdle := func(t *testing.T, d *DHTDiscovery, p *resumeProbe) {
		t.Helper()
		d.OnResume()
		d.resumeWG.Wait()
		if p.stun.Load() != 0 || p.announce.Load() != 0 || len(p.sends()) != 0 {
			t.Fatalf("stun=%d announce=%d sends=%d, want none", p.stun.Load(), p.announce.Load(), len(p.sends()))
		}
		if got := d.localNode.GetEndpoint(); got != resumeEndpoint(d, resumeOldIP) {
			t.Fatalf("endpoint changed to %s", got)
		}
	}

	t.Run("never started", func(t *testing.T) {
		d, p := resumeDHT(t)
		d.mu.Lock()
		d.running, d.resumable = false, false
		d.mu.Unlock()
		assertIdle(t, d, p)
	})
	t.Run("stopped", func(t *testing.T) {
		d, p := resumeDHT(t)
		if err := d.Stop(); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		assertIdle(t, d, p)
	})
}

// With no ticker running, each OnResume is exactly one STUN pass, applied at
// once (the ticker source would hold a changed answer back).
func TestOnResumeRunsOneSTUNPassPerCall(t *testing.T) {
	d, p := resumeDHT(t)
	d.stunInterval = time.Hour
	const resumes = 10
	for i := 0; i < resumes; i++ {
		d.OnResume()
		d.resumeWG.Wait()
		if i == 0 {
			if got := d.localNode.GetEndpoint(); got != resumeEndpoint(d, resumeNewIP) {
				t.Fatalf("after the first OnResume endpoint = %s, want the new one applied at once", got)
			}
		}
	}
	if got := p.stun.Load(); got != resumes {
		t.Fatalf("STUN passes = %d, want %d (one per OnResume)", got, resumes)
	}
}

// The ticker and OnResume share refreshSTUNOnce: stunMu must keep two passes
// from being inside the STUN query at the same time.
func TestOnResumeSerialisedWithStunRefreshLoop(t *testing.T) {
	d, p := resumeDHT(t)
	d.stunInterval = 2 * time.Millisecond
	var inFlight, maxInFlight atomic.Int32
	d.stunFn = func() (NATType, net.IP, error) {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		inFlight.Add(-1)
		nat := NATCone
		if p.stun.Add(1)%2 == 0 {
			nat = NATSymmetric
		}
		return nat, net.ParseIP(resumeNewIP), nil
	}

	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		d.stunRefreshLoop()
	}()

	// the ticker path must be running before the resumes start
	deadline := time.Now().Add(5 * time.Second)
	for p.stun.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("stunRefreshLoop never refreshed on its ticker")
		}
		time.Sleep(time.Millisecond)
	}

	var readers sync.WaitGroup
	stopReaders := make(chan struct{})
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stopReaders:
				return
			default:
				_ = d.localNode.GetNATType()
				_ = d.localNode.GetEndpoint()
			}
		}
	}()

	for i := 0; i < 30; i++ {
		d.OnResume()
		d.resumeWG.Wait()
	}
	close(stopReaders)
	readers.Wait()

	d.cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("stunRefreshLoop did not stop on cancel")
	}
	if got := maxInFlight.Load(); got != 1 {
		t.Fatalf("max STUN passes in flight = %d, want 1 (stunMu serialises the ticker and OnResume)", got)
	}
}

// A layer cancelled while the STUN query runs must not apply its answer or
// announce: the resume goroutine may outlive Stop.
func TestOnResumeDropsAnswerAfterCancelDuringSTUN(t *testing.T) {
	d, p := resumeDHT(t)
	d.stunFn = func() (NATType, net.IP, error) {
		p.stun.Add(1)
		d.cancel()
		return NATCone, net.ParseIP(resumeNewIP), nil
	}

	d.OnResume()
	d.resumeWG.Wait()

	if got := p.stun.Load(); got != 1 {
		t.Fatalf("STUN queries = %d, want 1", got)
	}
	if got := d.localNode.GetEndpoint(); got != resumeEndpoint(d, resumeOldIP) {
		t.Fatalf("endpoint = %s, want it unchanged after cancel", got)
	}
	if got := d.localNode.GetNATType(); got != "" {
		t.Fatalf("NATType = %q, want it unchanged after cancel", got)
	}
	if got := p.announce.Load(); got != 0 {
		t.Fatalf("announce() calls = %d, want 0 after cancel", got)
	}
}

func TestOnResumeStopWhileSTUNBlockedKeepsEndpoint(t *testing.T) {
	d, p := resumeDHT(t)
	started := make(chan struct{})
	release := make(chan struct{})
	d.stunFn = func() (NATType, net.IP, error) {
		p.stun.Add(1)
		close(started)
		<-release
		return NATCone, net.ParseIP(resumeNewIP), nil
	}

	d.OnResume()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the resume goroutine never reached the STUN query")
	}
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	close(release)
	d.resumeWG.Wait()

	if got := d.localNode.GetEndpoint(); got != resumeEndpoint(d, resumeOldIP) {
		t.Fatalf("endpoint = %s, want it unchanged after Stop", got)
	}
	if got := p.announce.Load(); got != 0 {
		t.Fatalf("announce() calls = %d, want 0 after Stop", got)
	}
}

// Wake events can come in bursts: while one STUN+announce pass is running a
// further OnResume only clears the back-off; it starts no second pass.
func TestOnResumeCoalescesWhileAPassRuns(t *testing.T) {
	d, p := resumeDHT(t)
	started := make(chan struct{}, 32)
	release := make(chan struct{})
	d.stunFn = func() (NATType, net.IP, error) {
		p.stun.Add(1)
		started <- struct{}{}
		<-release
		return NATCone, net.ParseIP(resumeNewIP), nil
	}

	d.OnResume()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first pass never reached the STUN query")
	}
	d.mu.Lock()
	d.rendezvousBackoff["peer-x"] = backoffEntry{nextAttempt: time.Now().Add(time.Hour), duration: time.Hour}
	d.mu.Unlock()
	for i := 0; i < 20; i++ {
		d.OnResume()
	}
	if !d.canAttemptRendezvous("peer-x") {
		t.Fatal("a coalesced OnResume did not clear the rendezvous back-off")
	}
	close(release)
	d.resumeWG.Wait()

	if got := p.stun.Load(); got != 1 {
		t.Fatalf("STUN passes = %d, want 1 for 21 OnResume calls during one pass", got)
	}
	if got := p.announce.Load(); got != 1 {
		t.Fatalf("announce() calls = %d, want 1", got)
	}

	d.OnResume()
	d.resumeWG.Wait()
	if got := p.stun.Load(); got != 2 {
		t.Fatalf("STUN passes after the pass ended = %d, want 2 (a new OnResume runs again)", got)
	}
	if got := p.announce.Load(); got != 2 {
		t.Fatalf("announce() calls after the pass ended = %d, want 2", got)
	}
}
