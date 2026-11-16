package discovery

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func aadKey(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }

var aadRemote = &net.UDPAddr{IP: net.ParseIP("198.51.100.7"), Port: 51821}

// newAADExchange builds a PeerExchange whose socket is a loopback UDP socket,
// with victimKey already in its PeerStore.
func newAADExchange(t *testing.T, secret string, localKey, victimKey string) (*PeerExchange, *daemon.PeerStore, *daemon.Config) {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	store := daemon.NewPeerStore()
	if victimKey != "" {
		store.Update(&daemon.PeerInfo{WGPubKey: victimKey, MeshIP: "10.0.0.5", Endpoint: "203.0.113.5:51820"}, "dht")
	}
	pe := NewPeerExchange(cfg, daemon.NewLocalNode(localKey, "", "", "10.0.0.1", ""), store)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	pe.conn = conn
	return pe, store, cfg
}

func goodbyeFor(key string) goodbyeMessage {
	return goodbyeMessage{Protocol: crypto.ProtocolVersion, Timestamp: time.Now().Unix(), WGPubKey: key}
}

func announcementFor(key string) crypto.PeerAnnouncement {
	return crypto.PeerAnnouncement{
		Protocol: crypto.ProtocolVersion, Timestamp: time.Now().Unix(),
		WGPubKey: key, MeshIP: "10.0.0.2", WGEndpoint: "198.51.100.7:51820",
	}
}

func retype(t *testing.T, sealed []byte, newType string) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(sealed, &m); err != nil {
		t.Fatal(err)
	}
	m["type"] = newType
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLegacyGoodbyeIsIgnored(t *testing.T) {
	t.Parallel()
	victim := aadKey(5)
	pe, store, cfg := newAADExchange(t, "wgmesh-test-aad-goodbye-legacy-1", aadKey(9), victim)

	legacy, err := crypto.SealEnvelope(crypto.MessageTypeGoodbye, goodbyeFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(legacy, aadRemote)

	if _, ok := store.Get(victim); !ok {
		t.Error("a GOODBYE sealed without type authentication removed the peer")
	}
}

func TestAADGoodbyeRemovesPeer(t *testing.T) {
	t.Parallel()
	victim := aadKey(5)
	pe, store, cfg := newAADExchange(t, "wgmesh-test-aad-goodbye-bound-1", aadKey(9), victim)

	bound, err := crypto.SealEnvelopeAAD(crypto.MessageTypeGoodbye, goodbyeFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(bound, aadRemote)

	if _, ok := store.Get(victim); ok {
		t.Error("a GOODBYE sealed with type authentication did not remove the peer")
	}
}

func TestGoodbyeLeavesTombstone(t *testing.T) {
	t.Parallel()
	victim := aadKey(5)
	pe, store, cfg := newAADExchange(t, "wgmesh-test-aad-goodbye-tombstone-1", aadKey(9), victim)

	bound, err := crypto.SealEnvelopeAAD(crypto.MessageTypeGoodbye, goodbyeFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(bound, aadRemote)
	if _, ok := store.Get(victim); ok {
		t.Fatal("the GOODBYE did not remove the peer")
	}

	pe.updateTransitivePeers([]crypto.KnownPeer{{WGPubKey: victim, MeshIP: "10.0.0.5", WGEndpoint: "203.0.113.5:51820", Introducer: true}})
	if got, ok := store.Get(victim); ok {
		t.Errorf("a KnownPeers mention brought the peer back after its GOODBYE: %+v", got)
	}
}

// Stop-condition probe, not a requirement: ADR-0001 exempts HELLO from the
// nonce cache, so a captured HELLO replayed inside the age window is a
// first-hand offer and clears the tombstone. The test records that the peer
// returns; a nonce-aware rule would turn it around.
// It pins a KNOWN LIMIT accepted by M 2026-11-15 (BACKLOG row 'ADR-0002 T3
// known limit'). When ADR-0001's per-HELLO sequence lands this assertion must
// flip and the BACKLOG row be closed.
func TestReplayedHelloAfterGoodbyeResurrectsPeer(t *testing.T) {
	t.Parallel()
	victim := aadKey(5)
	pe, store, cfg := newAADExchange(t, "wgmesh-test-aad-goodbye-replay-1", aadKey(9), victim)

	hello, err := crypto.SealEnvelopeAAD(crypto.MessageTypeHello, announcementFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	bye, err := crypto.SealEnvelopeAAD(crypto.MessageTypeGoodbye, goodbyeFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(hello, aadRemote)
	pe.handleMessage(bye, aadRemote)
	if _, ok := store.Get(victim); ok {
		t.Fatal("the GOODBYE did not remove the peer")
	}

	pe.handleMessage(hello, aadRemote)
	if _, ok := store.Get(victim); !ok {
		t.Error("the replayed HELLO was refused: the known limit in docs/adr/BACKLOG.md is closed, delete this test and its row")
	}
}

func TestSendGoodbyeIsAADBound(t *testing.T) {
	t.Parallel()
	const secret = "wgmesh-test-aad-send-goodbye-1"
	sender, _, _ := newAADExchange(t, secret, aadKey(4), "")
	receiver, store, cfg := newAADExchange(t, secret, aadKey(8), aadKey(4))

	rx, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer rx.Close()
	if err := sender.SendGoodbye(rx.LocalAddr().String()); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	if err := rx.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := rx.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}

	env, _, err := crypto.OpenEnvelopeRaw(buf[:n], cfg.Keys.GossipKey)
	if err != nil {
		t.Fatalf("goodbye does not open: %v", err)
	}
	if env.LegacyAAD {
		t.Error("SendGoodbye sealed without type authentication")
	}
	receiver.handleMessage(buf[:n], aadRemote)
	if _, ok := store.Get(aadKey(4)); ok {
		t.Error("the receiver kept a peer that said goodbye")
	}
}

func TestLegacyGoodbyeRetypedAsHelloIsRejected(t *testing.T) {
	t.Parallel()
	victim := aadKey(5)
	pe, store, cfg := newAADExchange(t, "wgmesh-test-aad-retype-hello-1", aadKey(9), "")

	legacyGoodbye, err := crypto.SealEnvelope(crypto.MessageTypeGoodbye, goodbyeFor(victim), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, as := range []string{crypto.MessageTypeHello, crypto.MessageTypeReply, crypto.MessageTypeAnnounce} {
		retyped := retype(t, legacyGoodbye, as)
		// The retype must really work at the envelope level, or the rejection
		// below would pass for the wrong reason.
		if env, _, err := crypto.OpenEnvelopeRaw(retyped, cfg.Keys.GossipKey); err != nil || env.MessageType != as {
			t.Fatalf("test setup: retyped envelope does not open as %s: %v", as, err)
		}
		pe.handleMessage(retyped, aadRemote)
		if _, ok := store.Get(victim); ok {
			t.Errorf("a legacy GOODBYE re-sent as %s reached the PeerStore", as)
		}
	}
}

func TestLegacyOpensAreCounted(t *testing.T) {
	t.Parallel()
	pe, _, cfg := newAADExchange(t, "wgmesh-test-aad-counter-1", aadKey(9), "")

	legacy, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcementFor(aadKey(2)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := crypto.SealEnvelopeAAD(crypto.MessageTypeAnnounce, announcementFor(aadKey(3)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	pe.handleMessage(legacy, aadRemote)
	pe.handleMessage(legacy, aadRemote)
	pe.handleMessage(bound, aadRemote)

	if got := pe.LegacyEnvelopeOpens(); got != 2 {
		t.Errorf("LegacyEnvelopeOpens() = %d, want 2 (two legacy packets, one bound)", got)
	}
}
