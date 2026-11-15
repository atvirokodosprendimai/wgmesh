package discovery

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

func TestReplayedAnnounceIsProcessedOnce(t *testing.T) {
	t.Parallel()
	pe, _, cfg := newAADExchange(t, "wgmesh-test-replay-announce-1", aadKey(9), "")
	var calls atomic.Int32
	pe.SetAnnounceHandler(func(*crypto.PeerAnnouncement, *net.UDPAddr) { calls.Add(1) })

	sealed, err := crypto.SealEnvelopeAAD(crypto.MessageTypeAnnounce, announcementFor(aadKey(2)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(sealed, aadRemote)
	pe.handleMessage(sealed, aadRemote)

	if got := calls.Load(); got != 1 {
		t.Errorf("the same ANNOUNCE datagram reached the handler %d times, want 1", got)
	}
}

// The HELLO retry loop in ExchangeWithPeer resends one sealed buffer on
// purpose, so a lost reply can be recovered; the cache must not drop it.
func TestHelloRetransmitIsNotDropped(t *testing.T) {
	t.Parallel()
	pe, _, cfg := newAADExchange(t, "wgmesh-test-replay-hello-1", aadKey(9), "")
	rx, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer rx.Close()
	remote := rx.LocalAddr().(*net.UDPAddr)

	hello, err := crypto.SealEnvelopeAAD(crypto.MessageTypeHello, announcementFor(aadKey(2)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(hello, remote)
	pe.handleMessage(hello, remote)

	if err := rx.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 65536)
	for i := 1; i <= 2; i++ {
		if _, _, err := rx.ReadFromUDP(buf); err != nil {
			t.Fatalf("reply %d of 2 never arrived: %v", i, err)
		}
	}
}
