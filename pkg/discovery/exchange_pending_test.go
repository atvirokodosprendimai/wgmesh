package discovery

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// Words: ExchangeWithPeer registers one reply channel per call; two calls to
// one address each keep their own channel, a REPLY reaches every registered
// caller, and a caller removes only its own registration (#855, ADR-0005 M5).
// Why: with one slot per address the second call overwrote the first's
// channel and the first's deferred clear deleted the second's.
// Invariants: the map entry of an address goes with its last registration;
// delivery is non-blocking; unregister is idempotent and goroutine-safe.

func newPendingTestExchange(t *testing.T) (*PeerExchange, *net.UDPAddr) {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reply-registry-1"})
	if err != nil {
		t.Fatal(err)
	}
	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	pe := NewPeerExchange(cfg, localNode, daemon.NewPeerStore())
	return pe, &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 51821}
}

func pendingTestReply() *crypto.PeerAnnouncement {
	return &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-pubkey",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "198.51.100.1:51820",
	}
}

func pendingTestEntries(pe *PeerExchange, remote string) (entry bool, total int) {
	pe.pendingMu.Lock()
	defer pe.pendingMu.Unlock()
	_, entry = pe.pendingReplies[remote]
	return entry, len(pe.pendingReplies)
}

func receivedWithin(ch chan *daemon.PeerInfo, d time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(d):
		return false
	}
}

func TestRegisterReplyKeepsBothCallers(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	ch1, un1 := pe.registerReply(addr.String())
	ch2, un2 := pe.registerReply(addr.String())
	defer un1()
	defer un2()

	if ch1 == nil || ch2 == nil {
		t.Fatal("registerReply returned a nil channel")
	}
	if ch1 == ch2 {
		t.Fatal("two registrations for one address share a channel")
	}
	if cap(ch1) != 1 || cap(ch2) != 1 {
		t.Fatalf("channel capacity = %d and %d, want 1", cap(ch1), cap(ch2))
	}

	pe.handleReply(pendingTestReply(), addr)
	if !receivedWithin(ch1, time.Second) {
		t.Error("first caller's channel got no reply after the second registered")
	}
	if !receivedWithin(ch2, time.Second) {
		t.Error("second caller's channel got no reply")
	}
}

func TestHandleReplyReachesEveryRegisteredCaller(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	const callers = 5
	chans := make([]chan *daemon.PeerInfo, callers)
	for i := range chans {
		ch, un := pe.registerReply(addr.String())
		defer un()
		chans[i] = ch
	}
	other, unOther := pe.registerReply("203.0.113.9:51821")
	defer unOther()

	pe.handleReply(pendingTestReply(), addr)

	for i, ch := range chans {
		select {
		case info := <-ch:
			if info == nil || info.WGPubKey != "remote-pubkey" {
				t.Errorf("caller %d got %+v, want the reply's peer", i, info)
			}
		case <-time.After(time.Second):
			t.Errorf("caller %d got no reply", i)
		}
	}
	select {
	case <-other:
		t.Error("a reply from one address reached a registration for another")
	default:
	}

	// A full channel is skipped, never waited on.
	chans[0] <- &daemon.PeerInfo{}
	done := make(chan struct{})
	go func() {
		pe.handleReply(pendingTestReply(), addr)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleReply blocked on a full channel")
	}
	if !receivedWithin(chans[1], time.Second) {
		t.Error("a full channel of one caller stopped delivery to the others")
	}
}

func TestUnregisterRemovesOnlyOwnRegistration(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	_, un1 := pe.registerReply(addr.String())
	ch2, un2 := pe.registerReply(addr.String())
	defer un2()

	un1()

	pe.handleReply(pendingTestReply(), addr)
	if !receivedWithin(ch2, time.Second) {
		t.Fatal("caller 2 lost its registration when caller 1 unregistered")
	}
}

func TestUnregisterLastRemovesAddressEntry(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	_, un1 := pe.registerReply(addr.String())
	_, un2 := pe.registerReply(addr.String())

	un1()
	if entry, _ := pendingTestEntries(pe, addr.String()); !entry {
		t.Fatal("address entry removed while a registration is left")
	}
	un2()
	if entry, total := pendingTestEntries(pe, addr.String()); entry || total != 0 {
		t.Fatalf("after the last unregister: entry=%v, map size=%d, want none", entry, total)
	}
}

func TestUnregisterIsIdempotentAndConcurrent(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	ch2, un2 := pe.registerReply(addr.String())
	defer un2()
	_, un1 := pe.registerReply(addr.String())

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			un1()
			un1()
		}()
	}
	wg.Wait()

	pe.handleReply(pendingTestReply(), addr)
	if !receivedWithin(ch2, time.Second) {
		t.Fatal("repeated unregister of one registration removed another")
	}
	un2()
	if entry, total := pendingTestEntries(pe, addr.String()); entry || total != 0 {
		t.Fatalf("entry=%v, map size=%d after all unregistered, want none", entry, total)
	}
}

// Run with -race: registrations, replies and unregisters at once.
func TestRegisterReplyConcurrentCallersUnderRace(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	const callers = 32
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unregister := pe.registerReply(addr.String())
			defer unregister()
			pe.handleReply(pendingTestReply(), addr)
			if !receivedWithin(ch, 5*time.Second) {
				t.Error("a registered caller got no reply to a reply sent while it was registered")
			}
		}()
	}
	wg.Wait()

	if entry, total := pendingTestEntries(pe, addr.String()); entry || total != 0 {
		t.Fatalf("entry=%v, map size=%d after all callers returned, want none", entry, total)
	}
}

// Words: handleReply gives every registered caller its own copy of the
// received PeerInfo, slices and pointer fields included.
// Why: one shared *PeerInfo sent to several goroutines turns any later write
// by one caller (Endpoint, RoutableNetworks) into a race with the others.
// Invariant: no two receivers, and not the receiver and the PeerStore, share
// a PeerInfo or the backing array of one of its slices.
func TestHandleReplyGivesEveryCallerItsOwnCopy(t *testing.T) {
	pe, addr := newPendingTestExchange(t)
	chA, unA := pe.registerReply(addr.String())
	defer unA()
	chB, unB := pe.registerReply(addr.String())
	defer unB()

	reply := pendingTestReply()
	reply.RoutableNetworks = []string{"192.168.50.0/24", "192.168.51.0/24"}
	pe.handleReply(reply, addr)

	a, b := <-chA, <-chB
	if a == b {
		t.Fatal("two callers received the same *PeerInfo")
	}
	if &a.RoutableNetworks[0] == &b.RoutableNetworks[0] {
		t.Fatal("two callers share the RoutableNetworks backing array")
	}

	// Run with -race: A writes its copy while B reads its own.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			a.Endpoint = "203.0.113.1:1"
			a.RoutableNetworks[0] = "10.9.9.0/24"
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = b.Endpoint
			_ = b.RoutableNetworks[0]
		}
	}()
	wg.Wait()
	if b.RoutableNetworks[0] != "192.168.50.0/24" {
		t.Fatalf("caller B's RoutableNetworks changed to %q by caller A's write", b.RoutableNetworks[0])
	}
}

// Words: two ExchangeWithPeer calls to one address at the same time both
// return the peer's announcement when the peer sends ONE reply.
// Why: with one reply slot per address (#855) the second call replaced the
// first's channel, so only one caller got the reply and the other timed out
// after ExchangeTimeout (4 s). The test fails on that code by reasoning from
// it, not by a run: the old registerReply stored one channel per address and
// the second call's registration (and its deferred clear) dropped the first's.
// Invariant: one REPLY from the address reaches every call in flight to it.
func TestConcurrentExchangeWithPeerBothGetTheReply(t *testing.T) {
	pe, _, cfg := newAADExchange(t, "wgmesh-test-concurrent-exchange-1", aadKey(9), "")
	done := make(chan struct{})
	go func() {
		defer close(done)
		pe.listenLoop()
	}()
	t.Cleanup(func() {
		close(pe.stopCh)
		pe.conn.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("listenLoop did not stop")
		}
	})

	// The loopback socket that plays the remote peer.
	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peerAddr := peer.LocalAddr().(*net.UDPAddr)

	type result struct {
		info *daemon.PeerInfo
		err  error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			info, err := pe.ExchangeWithPeer(peerAddr.String())
			results <- result{info, err}
		}()
	}

	// Both calls register before they send, so once two registrations exist
	// the peer's single reply meets both of them.
	deadline := time.Now().Add(5 * time.Second)
	for {
		pe.pendingMu.Lock()
		n := len(pe.pendingReplies[peerAddr.String()])
		pe.pendingMu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d registrations for the peer's address after 5 s, want 2", n)
		}
		time.Sleep(time.Millisecond)
	}

	buf := make([]byte, MaxExchangeSize)
	peer.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, from, err := peer.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("the peer got no HELLO: %v", err)
	}
	ann := announcementFor(aadKey(7))
	reply, err := crypto.SealEnvelopeAAD(crypto.MessageTypeReply, ann, cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.WriteToUDP(reply, from); err != nil {
		t.Fatal(err)
	}

	// The peer sends no second reply: each caller must have the first one.
	// Bounded below ExchangeTimeout so a caller left waiting fails the test.
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if r.err != nil || r.info == nil || r.info.WGPubKey != aadKey(7) {
				t.Fatalf("call %d: info=%+v err=%v, want the peer's announcement", i, r.info, r.err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of 2 concurrent calls got the reply", i)
		}
	}
}
