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
	pe := NewPeerExchange(cfg, &daemon.LocalNode{WGPubKey: localKey, MeshIP: "10.0.0.1"}, store)
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
	_ = rx.SetReadDeadline(time.Now().Add(2 * time.Second))
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
		pe.handleMessage(retype(t, legacyGoodbye, as), aadRemote)
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
