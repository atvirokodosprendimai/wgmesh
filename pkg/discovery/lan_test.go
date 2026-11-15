package discovery

import (
	"net"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func TestResolveEndpointPrefersLANSourceIP(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.42"), Port: 51830}

	tests := []struct {
		name       string
		advertised string
		sender     *net.UDPAddr
		want       string
	}{
		{name: "public advertised on LAN", advertised: "203.0.113.9:51820", sender: sender, want: "192.168.1.42:51820"},
		{name: "wildcard advertised", advertised: "0.0.0.0:51820", sender: sender, want: "192.168.1.42:51820"},
		{name: "invalid advertised", advertised: "not-an-endpoint", sender: sender, want: "192.168.1.42:51820"},
		{name: "no sender keeps explicit", advertised: "203.0.113.9:51820", sender: nil, want: "203.0.113.9:51820"},
		{name: "no sender invalid", advertised: "not-an-endpoint", sender: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(tt.advertised, tt.sender)
			if got != tt.want {
				t.Fatalf("resolveEndpoint(%q, %v) = %q, want %q", tt.advertised, tt.sender, got, tt.want)
			}
		})
	}
}

func TestLANDiscoveryRestartAfterStop(t *testing.T) {
	cfg := newTestConfig(t)
	l, err := NewLANDiscovery(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Start(); err != nil {
		t.Skipf("multicast not available: %v", err)
	}
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := l.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}

	select {
	case <-l.stopCh:
		t.Fatal("stopCh is closed after restart: loops exit at once")
	case <-time.After(300 * time.Millisecond):
	}

	stopTwice(t, l)
}

func TestLANDiscoveryConcurrentStopWaitsForLoops(t *testing.T) {
	l, err := NewLANDiscovery(newTestConfig(t), &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Start(); err != nil {
		t.Skipf("multicast join failed: %v", err)
	}
	// Stand in for a loop that has not exited yet: both Stops must wait for it.
	l.wg.Add(1)
	stopped := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			l.Stop()
			stopped <- struct{}{}
		}()
	}
	select {
	case <-stopped:
		t.Fatal("a Stop returned while a loop was still running")
	case <-time.After(200 * time.Millisecond):
	}
	l.wg.Done()
	for i := 0; i < 2; i++ {
		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
			t.Fatal("Stop did not return after the loop exited")
		}
	}
}
