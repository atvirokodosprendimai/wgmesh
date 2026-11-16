package discovery

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func TestAllowLogIsThrottledAndBounded(t *testing.T) {
	t.Parallel()
	pe, _, _ := newAADExchange(t, "wgmesh-test-allowlog-1", aadKey(9), "")

	addr := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 1000}
	if !pe.allowLog("ANNOUNCE", addr) {
		t.Fatal("first log line for a kind and source was refused")
	}
	if pe.allowLog("ANNOUNCE", addr) {
		t.Error("second log line inside the cooldown was allowed")
	}
	if !pe.allowLog("HELLO", addr) {
		t.Error("another kind from the same source shares the throttle")
	}

	for i := 0; i < 3*maxLogThrottleEntries; i++ {
		pe.allowLog("legacy ANNOUNCE", &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 1 + i%60000, Zone: fmt.Sprint(i)})
	}
	pe.logMu.Lock()
	n := len(pe.lastPacketLog)
	pe.logMu.Unlock()
	if n > maxLogThrottleEntries {
		t.Errorf("throttle map holds %d entries, want at most %d", n, maxLogThrottleEntries)
	}
}

func TestLegacyOpenCounterIsRaceFree(t *testing.T) {
	t.Parallel()
	pe, _, cfg := newAADExchange(t, "wgmesh-test-legacy-race-1", aadKey(9), "")

	legacy, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcementFor(aadKey(2)), cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	const workers, each = 8, 50
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				pe.handleMessage(legacy, aadRemote)
			}
		}()
	}
	wg.Wait()
	if got := pe.LegacyEnvelopeOpens(); got != workers*each {
		t.Errorf("LegacyEnvelopeOpens() = %d, want %d", got, workers*each)
	}
}

// lockedBuffer is a log sink that is safe to write from any goroutine.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The "Received valid" line is written after validation: an invalid HELLO,
// REPLY or ANNOUNCE produces none, a valid one exactly one. Not parallel: it
// swaps the global logger.
func TestValidLogFollowsValidation(t *testing.T) {
	cfg := newTestConfig(t)
	validKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	out := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })

	send := func(msgType string, addr *net.UDPAddr, meshIP string) {
		pe := NewPeerExchange(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		pe.conn = conn
		ann := crypto.CreateAnnouncement(validKey, meshIP, "198.51.100.7:51820", false, nil, nil, "", "", "")
		data, err := crypto.SealEnvelope(msgType, ann, cfg.Keys.GossipKey)
		if err != nil {
			t.Fatal(err)
		}
		pe.handleMessage(data, addr)
	}
	for i, msgType := range []string{crypto.MessageTypeHello, crypto.MessageTypeReply, crypto.MessageTypeAnnounce} {
		bad := &net.UDPAddr{IP: net.ParseIP("198.51.100.8"), Port: 51821 + i}
		good := &net.UDPAddr{IP: net.ParseIP("198.51.100.9"), Port: 51821 + i}
		send(msgType, bad, "10.0.0.2/16")
		send(msgType, good, "10.0.0.2")

		got := out.String()
		if n := strings.Count(got, "Received valid "+msgType+" from "+bad.String()); n != 0 {
			t.Errorf("invalid %s logged as valid %d times: %q", msgType, n, got)
		}
		if n := strings.Count(got, "Received valid "+msgType+" from "+good.String()); n != 1 {
			t.Errorf("valid %s logged %d times, want 1: %q", msgType, n, got)
		}
	}
}
