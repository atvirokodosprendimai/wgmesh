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
	defer l.Stop()

	select {
	case <-l.stopCh:
		t.Fatal("stopCh is closed after restart: loops exit at once")
	case <-time.After(300 * time.Millisecond):
	}
}
