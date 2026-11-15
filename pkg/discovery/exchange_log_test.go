package discovery

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
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
