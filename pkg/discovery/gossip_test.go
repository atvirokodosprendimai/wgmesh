package discovery

import (
	"bytes"
	"encoding/base64"
	"net"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

const testSecret = "wgmesh-test-secret-long-enough-for-key-derivation"

func newTestConfig(t *testing.T) *daemon.Config {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestHandleAnnouncementUpdatesPeerStore(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	localNode.SetEndpoint("127.0.0.1:51820")
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate receiving an announcement from another peer
	announcement := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-key-A",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "192.168.1.10:51820",
		Timestamp:  time.Now().Unix(),
		KnownPeers: []crypto.KnownPeer{
			{WGPubKey: "remote-key-B", MeshIP: "10.0.0.3", WGEndpoint: "192.168.1.20:51820"},
		},
	}

	gossip.HandleAnnounceFrom(announcement, nil)

	// Direct peer should be stored via "gossip"
	peer, ok := store.Get("remote-key-A")
	if !ok {
		t.Fatal("expected remote-key-A in peer store")
	}
	if peer.MeshIP != "10.0.0.2" {
		t.Errorf("expected MeshIP 10.0.0.2, got %s", peer.MeshIP)
	}

	// Transitive peer should be stored via "gossip-transitive"
	transitive, ok := store.Get("remote-key-B")
	if !ok {
		t.Fatal("expected remote-key-B in peer store (transitive)")
	}
	if transitive.MeshIP != "10.0.0.3" {
		t.Errorf("expected MeshIP 10.0.0.3, got %s", transitive.MeshIP)
	}
}

func TestHandleAnnouncementIgnoresOwnKey(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "my-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	// Announcement from ourselves should be ignored
	announcement := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "my-key",
		MeshIP:     "10.0.0.1",
		WGEndpoint: "127.0.0.1:51820",
		Timestamp:  time.Now().Unix(),
	}

	gossip.HandleAnnounceFrom(announcement, nil)

	if store.Count() != 0 {
		t.Errorf("expected 0 peers, got %d", store.Count())
	}
}

func TestHandleAnnouncementIgnoresNil(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic
	gossip.HandleAnnounceFrom(nil, nil)

	if store.Count() != 0 {
		t.Errorf("expected 0 peers, got %d", store.Count())
	}
}

func TestHandleAnnouncementSkipsOwnKeyInTransitivePeers(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "my-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	announcement := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-A",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "192.168.1.10:51820",
		Timestamp:  time.Now().Unix(),
		KnownPeers: []crypto.KnownPeer{
			{WGPubKey: "my-key", MeshIP: "10.0.0.1", WGEndpoint: "127.0.0.1:51820"},
			{WGPubKey: "remote-B", MeshIP: "10.0.0.3", WGEndpoint: "192.168.1.20:51820"},
		},
	}

	gossip.HandleAnnounceFrom(announcement, nil)

	// remote-A should be added, remote-B should be added, but our own key should not
	if store.Count() != 2 {
		t.Errorf("expected 2 peers (remote-A + remote-B), got %d", store.Count())
	}
	if _, ok := store.Get("my-key"); ok {
		t.Error("own key should not be added to peer store from transitive peers")
	}
}

func TestHandleAnnounceFromResolvesWildcardEndpoint(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	announcement := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-key",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "0.0.0.0:51820",
		Timestamp:  time.Now().Unix(),
	}

	remoteAddr := &net.UDPAddr{IP: net.ParseIP("192.168.1.44"), Port: 50000}
	gossip.HandleAnnounceFrom(announcement, remoteAddr)

	peer, ok := store.Get("remote-key")
	if !ok {
		t.Fatal("expected remote-key in peer store")
	}
	if peer.Endpoint != "192.168.1.44:51820" {
		t.Fatalf("expected resolved endpoint 192.168.1.44:51820, got %s", peer.Endpoint)
	}
}

func TestHandleAnnouncementDropsWildcardEndpointWithoutSender(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	announcement := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-key",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "0.0.0.0:51820",
		Timestamp:  time.Now().Unix(),
	}

	gossip.HandleAnnounceFrom(announcement, nil)

	peer, ok := store.Get("remote-key")
	if !ok {
		t.Fatal("expected remote-key in peer store")
	}
	if peer.Endpoint != "" {
		t.Fatalf("expected empty endpoint for wildcard without sender, got %s", peer.Endpoint)
	}
}

// A gossip announcement without an endpoint, heard from a sender, stores the
// sender IP with the default port, marked guessed.
func TestHandleAnnouncementEmptyEndpointStoresGuess(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	gossip, err := NewMeshGossip(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, store)
	if err != nil {
		t.Fatal(err)
	}

	gossip.HandleAnnounceFrom(&crypto.PeerAnnouncement{
		Protocol:  crypto.ProtocolVersion,
		WGPubKey:  "remote-key",
		MeshIP:    "10.0.0.2",
		Timestamp: time.Now().Unix(),
	}, &net.UDPAddr{IP: net.ParseIP("192.168.1.44"), Port: 50000})

	peer, ok := store.Get("remote-key")
	if !ok {
		t.Fatal("expected remote-key in peer store")
	}
	if peer.Endpoint != "192.168.1.44:51820" || !peer.EndpointGuessed {
		t.Fatalf("stored endpoint %q guessed=%v, want 192.168.1.44:51820 guessed=true", peer.Endpoint, peer.EndpointGuessed)
	}
}

func TestNewMeshGossipWithExchangeSetsExchange(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}

	exchange := NewPeerExchange(cfg, localNode, store)
	gossip, err := NewMeshGossipWithExchange(cfg, localNode, store, exchange)
	if err != nil {
		t.Fatal(err)
	}

	if gossip.exchange == nil {
		t.Error("expected exchange to be set on gossip instance")
	}
	if gossip.conn != nil {
		t.Error("expected conn to be nil when using exchange mode")
	}
}

func TestMeshGossipRestartAfterStop(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Keys.GossipPort = 0
	store := daemon.NewPeerStore()
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "127.0.0.1"}
	g, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	if err := g.Start(); err != nil {
		t.Fatal(err)
	}
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}

	select {
	case <-g.stopCh:
		t.Fatal("stopCh is closed after restart: loops exit at once")
	default:
	}

	remoteKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	// The restarted listener must still receive and apply a message.
	ann := crypto.CreateAnnouncement(remoteKey, "10.0.0.2", "192.168.1.10:51820", false, nil, nil, "remote", "", "")
	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, ann, cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(g.port)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write(data); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := store.Get(remoteKey); ok {
			stopTwice(t, g)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	g.Stop()
	t.Fatal("restarted gossip did not process a message")
}

func TestMeshGossipStopStartRaceOnPort(t *testing.T) {
	old := gossipInterval
	gossipInterval = 50 * time.Microsecond
	defer func() { gossipInterval = old }()

	cfg := newTestConfig(t)
	cfg.Keys.GossipPort = 0
	store := daemon.NewPeerStore()
	peerKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	store.Update(&daemon.PeerInfo{WGPubKey: peerKey, MeshIP: "127.0.0.1"}, "test")
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "127.0.0.1"}
	g, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1500; i++ {
		done := make(chan struct{})
		go func() {
			g.Stop()
			close(done)
		}()
		// Start is allowed as soon as Stop has flipped running; retry until then.
		for g.Start() != nil {
			time.Sleep(time.Millisecond)
		}
		<-done
	}
	g.Stop()
}

func TestMeshGossipConcurrentStopWaitsForLoops(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Keys.GossipPort = 0
	g, err := NewMeshGossip(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "127.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Start(); err != nil {
		t.Fatal(err)
	}
	// Stand in for a loop that has not exited yet: both Stops must wait for it.
	g.wg.Add(1)
	stopped := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			g.Stop()
			stopped <- struct{}{}
		}()
	}
	select {
	case <-stopped:
		t.Fatal("a Stop returned while a loop was still running")
	case <-time.After(200 * time.Millisecond):
	}
	g.wg.Done()
	for i := 0; i < 2; i++ {
		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
			t.Fatal("Stop did not return after the loop exited")
		}
	}
}

// stopTwice pins that a second Stop is a no-op returning nil.
func stopTwice(t *testing.T, s interface{ Stop() error }) {
	t.Helper()
	for i := 0; i < 2; i++ {
		if err := s.Stop(); err != nil {
			t.Fatalf("Stop #%d: %v", i+1, err)
		}
	}
}

func TestMeshGossipExchangeModeRestartAfterStop(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	g, err := NewMeshGossipWithExchange(cfg, localNode, store, NewPeerExchange(cfg, localNode, store))
	if err != nil {
		t.Fatal(err)
	}

	if err := g.Start(); err != nil {
		t.Fatal(err)
	}
	if err := g.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := g.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	select {
	case <-g.stopCh:
		t.Fatal("stopCh is closed after restart: gossipLoop exits at once")
	default:
	}
	stopTwice(t, g)
}

// TestGossipTransitiveOffersSecondHand verifies that a gossip KnownPeers
// mention cannot rewrite the named peer's mesh IP, while the peer's own
// announcement can.
func TestGossipTransitiveOffersSecondHand(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	localNode.SetEndpoint("127.0.0.1:51820")
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	announce := func(from, meshIP string, known ...crypto.KnownPeer) {
		gossip.HandleAnnounceFrom(&crypto.PeerAnnouncement{
			Protocol:   crypto.ProtocolVersion,
			WGPubKey:   from,
			MeshIP:     meshIP,
			WGEndpoint: "192.168.1.10:51820",
			Timestamp:  time.Now().Unix(),
			KnownPeers: known,
		}, nil)
	}
	meshIP := func(key string) string {
		t.Helper()
		peer, ok := store.Get(key)
		if !ok {
			t.Fatalf("expected %s in peer store", key)
		}
		return peer.MeshIP
	}

	announce("remote-B", "10.0.0.3")
	announce("remote-A", "10.0.0.2", crypto.KnownPeer{WGPubKey: "remote-B", MeshIP: "10.0.0.66", WGEndpoint: "192.168.1.20:51820"})
	if got := meshIP("remote-B"); got != "10.0.0.3" {
		t.Errorf("after a mention by remote-A, remote-B MeshIP = %q, want 10.0.0.3", got)
	}
	announce("remote-B", "10.0.0.4")
	if got := meshIP("remote-B"); got != "10.0.0.4" {
		t.Errorf("after its own announcement, remote-B MeshIP = %q, want 10.0.0.4", got)
	}
}
