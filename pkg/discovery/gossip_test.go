package discovery

import (
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

	localNode := &LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1", WGEndpoint: "127.0.0.1:51820"}
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

	gossip.HandleAnnounce(announcement)

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

	localNode := &LocalNode{WGPubKey: "my-key", MeshIP: "10.0.0.1"}
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

	gossip.HandleAnnounce(announcement)

	if store.Count() != 0 {
		t.Errorf("expected 0 peers, got %d", store.Count())
	}
}

func TestHandleAnnouncementIgnoresNil(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	gossip, err := NewMeshGossip(cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic
	gossip.HandleAnnounce(nil)

	if store.Count() != 0 {
		t.Errorf("expected 0 peers, got %d", store.Count())
	}
}

func TestHandleAnnouncementSkipsOwnKeyInTransitivePeers(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()

	localNode := &LocalNode{WGPubKey: "my-key", MeshIP: "10.0.0.1"}
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

	gossip.HandleAnnounce(announcement)

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

	localNode := &LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
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

	localNode := &LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
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

	gossip.HandleAnnounce(announcement)

	peer, ok := store.Get("remote-key")
	if !ok {
		t.Fatal("expected remote-key in peer store")
	}
	if peer.Endpoint != "" {
		t.Fatalf("expected empty endpoint for wildcard without sender, got %s", peer.Endpoint)
	}
}

func TestNewMeshGossipWithExchangeSetsExchange(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	localNode := &LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}

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
