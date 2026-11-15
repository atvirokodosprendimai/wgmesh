package discovery

import (
	"bytes"
	"encoding/base64"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
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
			got, _ := resolveEndpoint(tt.advertised, tt.sender)
			if got != tt.want {
				t.Fatalf("resolveEndpoint(%q, %v) = %q, want %q", tt.advertised, tt.sender, got, tt.want)
			}
		})
	}
}

func TestLANDiscoveryRestartAfterStop(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	l, err := NewLANDiscovery(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, store)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Start(); err != nil {
		t.Skipf("multicast join failed: %v", err)
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
	default:
	}

	// The restarted listener must still receive and apply a message.
	remoteKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	ann := crypto.CreateAnnouncement(remoteKey, "10.0.0.2", "192.168.1.10:51820", false, nil, nil, "remote", "", "")
	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, ann, cfg.Keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp4", nil, l.multicastAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// The announce loop also sends; repeat ours until the store has it.
		conn.Write(data)
		if _, ok := store.Get(remoteKey); ok {
			stopTwice(t, l)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	l.Stop()
	t.Fatal("restarted LAN discovery did not process a multicast message")
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

func TestLANPortClashWarning(t *testing.T) {
	tests := []struct {
		name string
		port int
		warn bool
	}{
		{name: "default WireGuard port", port: 51820, warn: false},
		{name: "LAN discovery port", port: LANMulticastPort, warn: true},
		{name: "unset", port: 0, warn: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lanPortClashWarning(tt.port)
			if (got != "") != tt.warn {
				t.Fatalf("lanPortClashWarning(%d) = %q, warn want %v", tt.port, got, tt.warn)
			}
			if tt.warn && !strings.Contains(got, "--listen-port") {
				t.Fatalf("warning must tell the operator to choose another --listen-port: %q", got)
			}
			if tt.warn && !strings.Contains(got, strconv.Itoa(LANMulticastPort)) {
				t.Fatalf("warning must name port %d: %q", LANMulticastPort, got)
			}
		})
	}
}

func TestLANDiscoveryStartLogsPortClash(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.WGListenPort = LANMulticastPort
	l, err := NewLANDiscovery(cfg, &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	// The warning precedes the bind, so it is logged even when Start fails.
	if err := l.Start(); err == nil {
		if err := l.Stop(); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Fatalf("Start did not log the port clash warning: %q", buf.String())
	}
}
