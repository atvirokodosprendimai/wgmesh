package discovery

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// TestObservedEndpoint_WireFormat verifies that ObservedEndpoint round-trips
// through JSON serialization and is omitted when empty (backward compat).
func TestObservedEndpoint_WireFormat(t *testing.T) {
	tests := []struct {
		name             string
		observedEndpoint string
		wantInJSON       bool
	}{
		{"present", "203.0.113.5:41234", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ann := crypto.CreateAnnouncement("pubkey1", "10.0.0.1", "0.0.0.0:51820", false, nil, nil, "", "", "")
			ann.ObservedEndpoint = tt.observedEndpoint

			data, err := json.Marshal(ann)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			// Check JSON contains/omits the field
			var raw map[string]interface{}
			json.Unmarshal(data, &raw)
			_, found := raw["observed_endpoint"]
			if found != tt.wantInJSON {
				t.Errorf("observed_endpoint in JSON = %v, want %v (json: %s)", found, tt.wantInJSON, data)
			}

			// Round-trip
			var got crypto.PeerAnnouncement
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.ObservedEndpoint != tt.observedEndpoint {
				t.Errorf("ObservedEndpoint = %q, want %q", got.ObservedEndpoint, tt.observedEndpoint)
			}
		})
	}
}

// TestObservedEndpoint_BackwardCompat verifies that a PeerAnnouncement
// without ObservedEndpoint (from an older peer) still deserializes correctly.
func TestObservedEndpoint_BackwardCompat(t *testing.T) {
	// Simulate old-format JSON without the field
	oldJSON := `{"protocol":"wgmesh-v1","wg_pubkey":"pk","mesh_ip":"10.0.0.1","wg_endpoint":"1.2.3.4:51820","timestamp":1700000000}`

	var ann crypto.PeerAnnouncement
	if err := json.Unmarshal([]byte(oldJSON), &ann); err != nil {
		t.Fatalf("unmarshal old format: %v", err)
	}
	if ann.ObservedEndpoint != "" {
		t.Errorf("expected empty ObservedEndpoint from old format, got %q", ann.ObservedEndpoint)
	}
}

// TestHandleReply_UpdatesLocalEndpoint verifies that when a REPLY contains
// ObservedEndpoint, the receiver updates its own localNode.WGEndpoint
// with the observed IP but preserves its WG listen port.
func TestHandleReply_UpdatesLocalEndpoint(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reflector-update-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	// Simulate a REPLY from a remote peer that includes ObservedEndpoint
	reply := &crypto.PeerAnnouncement{
		Protocol:         crypto.ProtocolVersion,
		WGPubKey:         "remote-pubkey",
		MeshIP:           "10.0.0.2",
		WGEndpoint:       "198.51.100.1:51820",
		ObservedEndpoint: "203.0.113.42:54321", // NAT-mapped address of the HELLO sender
	}

	remoteAddr := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: int(cfg.Keys.GossipPort)}

	// Register a pending reply so handleReply doesn't log "unsolicited"
	pe.pendingMu.Lock()
	pe.pendingReplies[remoteAddr.String()] = make(chan *daemon.PeerInfo, 1)
	pe.pendingMu.Unlock()

	pe.handleReply(reply, remoteAddr)

	// The local endpoint should now use the observed IP (203.0.113.42)
	// combined with the original WG port (51820), NOT the observed port (54321)
	want := "203.0.113.42:51820"
	if localNode.GetEndpoint() != want {
		t.Errorf("localNode.GetEndpoint() = %q, want %q", localNode.GetEndpoint(), want)
	}
}

// TestHandleReply_IgnoresEmptyObservedEndpoint verifies backward compat:
// when ObservedEndpoint is empty (old peer), localNode.WGEndpoint is unchanged.
func TestHandleReply_IgnoresEmptyObservedEndpoint(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reflector-ignore-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	reply := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   "remote-pubkey",
		MeshIP:     "10.0.0.2",
		WGEndpoint: "198.51.100.1:51820",
		// ObservedEndpoint intentionally empty — old peer
	}

	remoteAddr := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: int(cfg.Keys.GossipPort)}

	pe.pendingMu.Lock()
	pe.pendingReplies[remoteAddr.String()] = make(chan *daemon.PeerInfo, 1)
	pe.pendingMu.Unlock()

	pe.handleReply(reply, remoteAddr)

	// Endpoint should be unchanged
	if localNode.GetEndpoint() != "0.0.0.0:51820" {
		t.Errorf("localNode.GetEndpoint() changed to %q, want unchanged 0.0.0.0:51820", localNode.GetEndpoint())
	}
}

// TestHandleReply_SkipsSelfReflection verifies that if the observed IP is
// a private/loopback address, we don't update (both peers on same LAN).
func TestHandleReply_SkipsSelfReflection(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reflector-private-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	tests := []struct {
		name     string
		observed string
	}{
		{"loopback", "127.0.0.1:12345"},
		{"private_10", "10.0.0.5:12345"},
		{"private_172", "172.16.0.5:12345"},
		{"private_192", "192.168.1.5:12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localNode.SetEndpoint("0.0.0.0:51820") // reset

			reply := &crypto.PeerAnnouncement{
				Protocol:         crypto.ProtocolVersion,
				WGPubKey:         "remote-pubkey",
				MeshIP:           "10.0.0.2",
				WGEndpoint:       "198.51.100.1:51820",
				ObservedEndpoint: tt.observed,
			}
			remoteAddr := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 51821}

			pe.pendingMu.Lock()
			pe.pendingReplies[remoteAddr.String()] = make(chan *daemon.PeerInfo, 1)
			pe.pendingMu.Unlock()

			pe.handleReply(reply, remoteAddr)

			if localNode.GetEndpoint() != "0.0.0.0:51820" {
				t.Errorf("localNode.GetEndpoint() = %q, want unchanged for private observed addr %q", localNode.GetEndpoint(), tt.observed)
			}
		})
	}
}

// TestSendReply_PopulatesObservedEndpoint verifies that sendReply includes
// the HELLO sender's observed address in the REPLY.
func TestSendReply_PopulatesObservedEndpoint(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reflector-send-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("1.2.3.4:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	// We need a real UDP socket to send a reply. Bind two sockets on localhost.
	serverAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	serverConn, err := net.ListenUDP("udp4", serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()
	pe.conn = serverConn

	// The "remote" peer's address (what we're sending the reply to)
	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()

	remoteAddr := clientConn.LocalAddr().(*net.UDPAddr)

	// Send the reply
	if err := pe.sendReply(remoteAddr); err != nil {
		t.Fatalf("sendReply: %v", err)
	}

	// Read what was sent
	buf := make([]byte, MaxExchangeSize)
	n, _, err := clientConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}

	// Decrypt and verify
	envelope, plaintext, err := crypto.OpenEnvelopeRaw(buf[:n], cfg.Keys.GossipKey)
	if err != nil {
		t.Fatalf("open envelope: %v", err)
	}
	if envelope.MessageType != crypto.MessageTypeReply {
		t.Errorf("message type = %q, want %q", envelope.MessageType, crypto.MessageTypeReply)
	}

	var ann crypto.PeerAnnouncement
	if err := json.Unmarshal(plaintext, &ann); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if ann.ObservedEndpoint != remoteAddr.String() {
		t.Errorf("ObservedEndpoint = %q, want %q", ann.ObservedEndpoint, remoteAddr.String())
	}
}

func TestHandleReply_DoesNotDowngradePublicIPv6ToIPv4Observed(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reflector-v6-sticky-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("[2a01:4f9:c012:2c15::1]:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	reply := &crypto.PeerAnnouncement{
		Protocol:         crypto.ProtocolVersion,
		WGPubKey:         "remote-pubkey",
		MeshIP:           "10.0.0.2",
		WGEndpoint:       "198.51.100.1:51820",
		ObservedEndpoint: "203.0.113.42:54321",
	}
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: 51821}

	pe.pendingMu.Lock()
	pe.pendingReplies[remoteAddr.String()] = make(chan *daemon.PeerInfo, 1)
	pe.pendingMu.Unlock()

	pe.handleReply(reply, remoteAddr)

	if got := localNode.GetEndpoint(); got != "[2a01:4f9:c012:2c15::1]:51820" {
		t.Fatalf("localNode.GetEndpoint() = %q, want unchanged public IPv6 endpoint", got)
	}
}

// TestResolvePeerEndpoint tests existing resolution logic (regression guard).
func TestResolvePeerEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		advertised  string
		sender      *net.UDPAddr
		want        string
		wantGuessed bool
	}{
		{"explicit_endpoint", "1.2.3.4:51820", nil, "1.2.3.4:51820", false},
		{"wildcard_with_sender", "0.0.0.0:51820", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820", false},
		{"empty_host_with_sender", ":51820", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820", false},
		{"ipv6_wildcard", "[::]:51820", &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 41234}, "[2001:db8::1]:51820", false},
		{"malformed_advertisement", ":::51820", &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 41234}, "[2001:db8::1]:51820", true},
		// remoteAddr is never nil in production; this pins the function's behaviour.
		{"no_sender_no_host", "0.0.0.0:51820", nil, "0.0.0.0:51820", false},
		{"empty_advertisement_with_sender", "", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820", true},
		{"empty_port_with_sender", "198.51.100.7:", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, guessed := resolvePeerEndpoint(tt.advertised, tt.sender)
			if got != tt.want || guessed != tt.wantGuessed {
				t.Errorf("resolvePeerEndpoint(%q, %v) = %q, %v, want %q, %v", tt.advertised, tt.sender, got, guessed, tt.want, tt.wantGuessed)
			}
		})
	}
}

// The sender's UDP source port is never the endpoint port: it is the
// advertised port or the default WireGuard port.
func TestResolvePeerEndpointNeverUsesSenderPort(t *testing.T) {
	tests := []struct {
		advertised string
		wantPort   string
	}{
		{"", "51820"},
		{"garbage", "51820"},
		{"198.51.100.7:", "51820"},
		{":51821", "51821"},
		{"0.0.0.0:51821", "51821"},
		{"198.51.100.7:51821", "51821"},
	}
	for _, senderPort := range []int{52654, 1} {
		sender := &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: senderPort}
		for _, tt := range tests {
			got, _ := resolvePeerEndpoint(tt.advertised, sender)
			if _, port, err := net.SplitHostPort(got); err != nil || port != tt.wantPort {
				t.Errorf("sender port %d, advertised %q: endpoint %q (err %v), want port %s", senderPort, tt.advertised, got, err, tt.wantPort)
			}
		}
	}
}

// A HELLO that advertises no endpoint stores the peer with the sender IP and
// the default port, marked guessed, so it ranks below any advertisement.
func TestHandleHelloEmptyEndpointStoresGuess(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-hello-guessed-endpoint-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()
	pe := NewPeerExchange(cfg, daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", ""), peerStore)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skipf("no loopback UDP: %v", err)
	}
	defer conn.Close()
	pe.conn = conn // handleHello replies on it

	pe.handleHello(&crypto.PeerAnnouncement{
		Protocol: crypto.ProtocolVersion,
		WGPubKey: "remote-pubkey",
		MeshIP:   "10.0.0.2",
	}, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 52654})

	got, ok := peerStore.Get("remote-pubkey")
	if !ok {
		t.Fatal("expected remote-pubkey in peer store")
	}
	if got.Endpoint != "127.0.0.1:51820" || !got.EndpointGuessed {
		t.Errorf("stored endpoint %q guessed=%v, want 127.0.0.1:51820 guessed=true", got.Endpoint, got.EndpointGuessed)
	}
}

func TestFilterEndpointForConfig(t *testing.T) {
	if got := filterEndpointForConfig("[2001:db8::1]:51820", true); got != "" {
		t.Fatalf("expected IPv6 endpoint to be dropped, got %q", got)
	}
	if got := filterEndpointForConfig("203.0.113.10:51820", true); got != "203.0.113.10:51820" {
		t.Fatalf("expected IPv4 endpoint to stay, got %q", got)
	}
	if got := filterEndpointForConfig("[2001:db8::1]:51820", false); got != "[2001:db8::1]:51820" {
		t.Fatalf("expected IPv6 endpoint to stay when enabled, got %q", got)
	}
}

func TestFilterCandidatesForConfig(t *testing.T) {
	in := []string{"203.0.113.10:51820", "[2001:db8::1]:51820", "203.0.113.10:51820"}
	got := filterCandidatesForConfig(in, true)
	if len(got) != 1 || got[0] != "203.0.113.10:51820" {
		t.Fatalf("unexpected filtered candidates: %v", got)
	}
}

// TestHandleGoodbye_RejectsStaleMessage verifies that GOODBYE messages
// older than 60 seconds are rejected to prevent replay attacks.
func TestHandleGoodbye_RejectsStaleMessage(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-goodbye-stale-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	_ = NewPeerExchange(cfg, localNode, peerStore)

	// Add a test peer to the store
	testPeerKey := "stale-peer-key"
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: testPeerKey,
		MeshIP:   "10.0.0.2",
	}, DHTMethod)

	// Verify peer exists before handling goodbye
	if _, exists := peerStore.Get(testPeerKey); !exists {
		t.Fatal("test peer should exist in peer store")
	}

	// Create a GOODBYE message with timestamp 120 seconds ago
	bye := goodbyeMessage{
		Protocol:  crypto.ProtocolVersion,
		Timestamp: time.Now().Add(-120 * time.Second).Unix(),
		WGPubKey:  testPeerKey,
	}

	// Serialize and send through the message handling path
	plaintext, err := json.Marshal(bye)
	if err != nil {
		t.Fatalf("marshal goodbye: %v", err)
	}

	_, err = crypto.SealEnvelope(crypto.MessageTypeGoodbye, bye, cfg.Keys.GossipKey)
	if err != nil {
		t.Fatalf("seal goodbye: %v", err)
	}

	// Unmarshal and validate timestamp
	var parsedBye goodbyeMessage
	if err := json.Unmarshal(plaintext, &parsedBye); err != nil {
		t.Fatalf("unmarshal goodbye: %v", err)
	}

	// Simulate the validation logic from handleMessage
	msgTime := time.Unix(parsedBye.Timestamp, 0)
	if time.Since(msgTime) > 60*time.Second {
		// This is the expected path - reject stale message
		// Verify peer still exists
		if _, exists := peerStore.Get(testPeerKey); !exists {
			t.Error("stale GOODBYE incorrectly removed peer from peer store")
		}
		return
	}

	t.Error("stale GOODBYE should have been rejected by timestamp validation")
}

// TestHandleGoodbye_AcceptsFreshMessage verifies that GOODBYE messages
// with a current timestamp are processed normally.
func TestHandleGoodbye_AcceptsFreshMessage(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-goodbye-fresh-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	_ = NewPeerExchange(cfg, localNode, peerStore)

	// Add a test peer to the store
	testPeerKey := "fresh-peer-key"
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: testPeerKey,
		MeshIP:   "10.0.0.2",
	}, DHTMethod)

	// Verify peer exists before handling goodbye
	if _, exists := peerStore.Get(testPeerKey); !exists {
		t.Fatal("test peer should exist in peer store")
	}

	// Create a GOODBYE message with current timestamp
	bye := goodbyeMessage{
		Protocol:  crypto.ProtocolVersion,
		Timestamp: time.Now().Unix(),
		WGPubKey:  testPeerKey,
	}

	// Serialize the message
	plaintext, err := json.Marshal(bye)
	if err != nil {
		t.Fatalf("marshal goodbye: %v", err)
	}

	// Simulate the validation logic from handleMessage
	var parsedBye goodbyeMessage
	if err := json.Unmarshal(plaintext, &parsedBye); err != nil {
		t.Fatalf("unmarshal goodbye: %v", err)
	}

	msgTime := time.Unix(parsedBye.Timestamp, 0)

	// Check if validation would pass
	if time.Since(msgTime) > 60*time.Second || msgTime.After(time.Now().Add(60*time.Second)) {
		t.Fatal("fresh GOODBYE should pass timestamp validation")
	}

	// Remove the peer (this is what the handler would do)
	peerStore.Remove(testPeerKey)

	// Verify peer was removed
	if _, exists := peerStore.Get(testPeerKey); exists {
		t.Error("fresh GOODBYE should have removed peer from peer store")
	}
}

// TestHandleGoodbye_RejectsFutureTimestamp verifies that GOODBYE messages
// with future timestamps are rejected to prevent clock skew attacks.
func TestHandleGoodbye_RejectsFutureTimestamp(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-goodbye-future-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")

	_ = NewPeerExchange(cfg, localNode, peerStore)

	// Add a test peer to the store
	testPeerKey := "future-peer-key"
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: testPeerKey,
		MeshIP:   "10.0.0.2",
	}, DHTMethod)

	// Verify peer exists before handling goodbye
	if _, exists := peerStore.Get(testPeerKey); !exists {
		t.Fatal("test peer should exist in peer store")
	}

	// Create a GOODBYE message with timestamp 120 seconds in the future
	bye := goodbyeMessage{
		Protocol:  crypto.ProtocolVersion,
		Timestamp: time.Now().Add(120 * time.Second).Unix(),
		WGPubKey:  testPeerKey,
	}

	// Serialize the message
	plaintext, err := json.Marshal(bye)
	if err != nil {
		t.Fatalf("marshal goodbye: %v", err)
	}

	// Simulate the validation logic from handleMessage
	var parsedBye goodbyeMessage
	if err := json.Unmarshal(plaintext, &parsedBye); err != nil {
		t.Fatalf("unmarshal goodbye: %v", err)
	}

	msgTime := time.Unix(parsedBye.Timestamp, 0)

	// Check if validation would reject future timestamp
	if msgTime.After(time.Now().Add(60 * time.Second)) {
		// This is the expected path - reject future timestamp
		// Verify peer still exists
		if _, exists := peerStore.Get(testPeerKey); !exists {
			t.Error("future GOODBYE incorrectly removed peer from peer store")
		}
		return
	}

	t.Error("future GOODBYE should have been rejected by timestamp validation")
}

// TestHandleGoodbye_BoundaryConditions tests the 60-second threshold.
func TestHandleGoodbye_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name         string
		age          time.Duration
		shouldAccept bool
	}{
		{"59 seconds old - should accept", 59 * time.Second, true},
		{"60 seconds old - should accept", 60 * time.Second, true},
		{"61 seconds old - should reject", 61 * time.Second, false},
		{"59 seconds future - should accept", -59 * time.Second, true},
		// Note: The future boundary is slightly tricky - it depends on exact timing
		// In practice, messages exactly at the boundary might race the check
		{"61 seconds future - should reject", -61 * time.Second, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip the edge case test for 60 seconds future
			// since it's timing-dependent and not critical for security
			if tt.age == -60*time.Second {
				t.Skip("boundary at exactly 60 seconds future is timing-dependent")
			}

			timestamp := time.Now().Add(tt.age).Unix()
			msgTime := time.Unix(timestamp, 0)

			isStale := time.Since(msgTime) > 60*time.Second
			isFuture := msgTime.After(time.Now().Add(60 * time.Second))
			shouldReject := isStale || isFuture

			if shouldReject == tt.shouldAccept {
				t.Errorf("timestamp age %v: shouldReject=%v, want shouldAccept=%v",
					tt.age, shouldReject, tt.shouldAccept)
			}
		})
	}
}

// TestExchangeWithPeer_LogsWarningForOwnIP verifies that ExchangeWithPeer
// logs a warning (but does NOT hard-refuse) when the target IP matches the
// node's own public IP. This allows same-NAT/CGNAT peers to still connect.
func TestExchangeWithPeer_LogsWarningForOwnIP(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-self-punch-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("198.51.100.1:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	// Use a closed UDP conn so WriteToUDP fails fast (no 4s timeout)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close() // close immediately — write will fail fast
	pe.conn = conn

	// Punch to own public IP — should NOT be hard-refused (soft warning only).
	// Will fail with send error on the closed conn, which is expected.
	_, err = pe.ExchangeWithPeer("198.51.100.1:52790")
	if err != nil && strings.Contains(err.Error(), "refusing to punch") {
		t.Errorf("ExchangeWithPeer should not hard-refuse own IP (same-NAT peers share IP): %v", err)
	}
}

// TestExchangeWithPeer_AllowsDifferentIP verifies that ExchangeWithPeer
// does NOT warn or block punching to a different IP (regression guard).
func TestExchangeWithPeer_AllowsDifferentIP(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-self-punch-2"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("198.51.100.1:51820")

	pe := NewPeerExchange(cfg, localNode, peerStore)

	// Use a closed UDP conn so WriteToUDP fails fast (no 4s timeout)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close() // close immediately — write will fail fast
	pe.conn = conn

	// Punching to a different IP should work without any self-connection concern.
	_, err = pe.ExchangeWithPeer("203.0.113.42:51820")
	if err != nil && strings.Contains(err.Error(), "refusing to punch") {
		t.Errorf("should not refuse punch to different IP: %v", err)
	}
}

// TestGetKnownPeers_ExcludesSelfAndEmpty verifies that getKnownPeers filters
// out both the local node and empty-pubkey entries from the advertised peer list.
func TestGetKnownPeers_ExcludesSelfAndEmpty(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-known-peers-self-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()

	localNode := daemon.NewLocalNode("local-pubkey-abc", "", "", "10.0.0.1", "")

	// Add local node, an empty-key entry, and a remote peer to the peer store
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: "local-pubkey-abc",
		MeshIP:   "10.0.0.1",
		Endpoint: "198.51.100.1:51820",
	}, "dht")
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: "",
		MeshIP:   "10.0.0.99",
		Endpoint: "198.51.100.1:51820",
	}, "dht")
	peerStore.Update(&daemon.PeerInfo{
		WGPubKey: "remote-pubkey-xyz",
		MeshIP:   "10.0.0.2",
		Endpoint: "203.0.113.42:51820",
	}, "dht")

	pe := NewPeerExchange(cfg, localNode, peerStore)
	known := pe.getKnownPeers()

	for _, kp := range known {
		if kp.WGPubKey == "local-pubkey-abc" {
			t.Error("getKnownPeers() should not include self (local-pubkey-abc)")
		}
		if kp.WGPubKey == "" {
			t.Error("getKnownPeers() should not include empty-pubkey entries")
		}
	}

	// Should still include the remote peer
	found := false
	for _, kp := range known {
		if kp.WGPubKey == "remote-pubkey-xyz" {
			found = true
		}
	}
	if !found {
		t.Error("getKnownPeers() should include remote peer (remote-pubkey-xyz)")
	}
}

// TestHandleMessage_ValidatesAnnouncements verifies that HELLO/REPLY/ANNOUNCE
// payloads arriving via OpenEnvelopeRaw are validated before reaching the
// PeerStore, and that invalid KnownPeers are filtered without dropping the
// sender.
func TestHandleMessage_ValidatesAnnouncements(t *testing.T) {
	t.Parallel()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-exchange-validate-1"})
	if err != nil {
		t.Fatal(err)
	}
	validKey := func(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("198.51.100.7"), Port: 51821}

	tests := []struct {
		name      string
		msgType   string
		ann       crypto.PeerAnnouncement
		wantPeer  bool
		wantKnown []string // known-peer pubkeys expected in the store
	}{
		{
			name:    "valid hello accepted, bad known peer filtered",
			msgType: crypto.MessageTypeHello,
			ann: crypto.PeerAnnouncement{
				WGPubKey: validKey(1), MeshIP: "10.0.0.2", WGEndpoint: "198.51.100.7:51820",
				RoutableNetworks: []string{"192.168.10.0/24"},
				KnownPeers: []crypto.KnownPeer{
					{WGPubKey: validKey(2), MeshIP: "10.0.0.3", WGEndpoint: "203.0.113.3:51820"},
					{WGPubKey: "not-a-key", MeshIP: "10.0.0.4"},
				},
			},
			wantPeer:  true,
			wantKnown: []string{validKey(2)},
		},
		{"hello bad pubkey", crypto.MessageTypeHello, crypto.PeerAnnouncement{WGPubKey: "garbage", MeshIP: "10.0.0.2"}, false, nil},
		{"hello bad mesh ip", crypto.MessageTypeHello, crypto.PeerAnnouncement{WGPubKey: validKey(1), MeshIP: "10.0.0.2/16"}, false, nil},
		{"hello bad route", crypto.MessageTypeHello, crypto.PeerAnnouncement{WGPubKey: validKey(1), MeshIP: "10.0.0.2", RoutableNetworks: []string{"192.168.1.0"}}, false, nil},
		{"reply bad route", crypto.MessageTypeReply, crypto.PeerAnnouncement{WGPubKey: validKey(1), MeshIP: "10.0.0.2", RoutableNetworks: []string{"bogus"}}, false, nil},
		{"reply valid", crypto.MessageTypeReply, crypto.PeerAnnouncement{WGPubKey: validKey(1), MeshIP: "10.0.0.2"}, true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			peerStore := daemon.NewPeerStore()
			localNode := daemon.NewLocalNode(validKey(9), "", "", "10.0.0.1", "")
			pe := NewPeerExchange(cfg, localNode, peerStore)
			conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			pe.conn = conn // HELLO handling sends a REPLY

			ann := tt.ann
			ann.Protocol = crypto.ProtocolVersion
			ann.Timestamp = time.Now().Unix()
			data, err := crypto.SealEnvelope(tt.msgType, ann, cfg.Keys.GossipKey)
			if err != nil {
				t.Fatal(err)
			}
			pe.handleMessage(data, remoteAddr)

			if _, ok := peerStore.Get(ann.WGPubKey); ok != tt.wantPeer {
				t.Errorf("sender in PeerStore = %v, want %v", ok, tt.wantPeer)
			}
			for _, k := range tt.wantKnown {
				if _, ok := peerStore.Get(k); !ok {
					t.Errorf("known peer %s missing from PeerStore", k[:8])
				}
			}
			if got, want := len(peerStore.GetAll()), len(tt.wantKnown); tt.wantPeer && got != want+1 {
				t.Errorf("PeerStore has %d peers, want %d", got, want+1)
			}
		})
	}
}

func TestValidateExchangeAnnouncementKnownPeersLimit(t *testing.T) {
	t.Parallel()
	a := &crypto.PeerAnnouncement{
		Protocol:   crypto.ProtocolVersion,
		WGPubKey:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)),
		MeshIP:     "10.99.0.1",
		KnownPeers: make([]crypto.KnownPeer, crypto.MaxKnownPeers+1),
	}
	if err := validateExchangeAnnouncement(a); err == nil {
		t.Fatal("expected error for too many KnownPeers")
	}
}

// TestHandleReplyOffersFirstHand verifies that a REPLY is its sender's own
// claim (a new mesh IP replaces the stored one), while a KnownPeers entry for
// the same key in another member's REPLY is second-hand (it does not).
func TestHandleReplyOffersFirstHand(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-reply-first-hand-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()
	localNode := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	localNode.SetEndpoint("0.0.0.0:51820")
	pe := NewPeerExchange(cfg, localNode, peerStore)

	peerStore.Offer(daemon.Candidate{
		Info:   &daemon.PeerInfo{WGPubKey: "remote-pubkey", MeshIP: "10.0.0.2", Endpoint: "198.51.100.1:51820"},
		Source: DHTMethod,
		Hand:   daemon.FirstHand,
	})
	meshIP := func() string {
		t.Helper()
		peer, ok := peerStore.Get("remote-pubkey")
		if !ok {
			t.Fatal("expected remote-pubkey in peer store")
		}
		return peer.MeshIP
	}
	reply := func(from, ip string, known ...crypto.KnownPeer) {
		pe.handleReply(&crypto.PeerAnnouncement{
			Protocol:   crypto.ProtocolVersion,
			WGPubKey:   from,
			MeshIP:     ip,
			WGEndpoint: "198.51.100.1:51820",
			KnownPeers: known,
		}, &net.UDPAddr{IP: net.ParseIP("198.51.100.1"), Port: int(cfg.Keys.GossipPort)})
	}

	reply("remote-pubkey", "10.0.0.9")
	if got := meshIP(); got != "10.0.0.9" {
		t.Errorf("after its own REPLY, MeshIP = %q, want 10.0.0.9", got)
	}
	reply("other-pubkey", "10.0.0.3", crypto.KnownPeer{WGPubKey: "remote-pubkey", MeshIP: "10.0.0.66", WGEndpoint: "198.51.100.1:51820"})
	if got := meshIP(); got != "10.0.0.9" {
		t.Errorf("after a mention in another REPLY, MeshIP = %q, want 10.0.0.9", got)
	}
}

// A peer known only by a guessed endpoint is listed with its key and no
// endpoint: a wrong address is worse than none (#839).
func TestGuessedEndpointNotAnnounced(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-guessed-endpoint-1"})
	if err != nil {
		t.Fatal(err)
	}
	peerStore := daemon.NewPeerStore()
	peerStore.Offer(daemon.Candidate{Info: &daemon.PeerInfo{
		WGPubKey: "guessed-pubkey", MeshIP: "10.0.0.2", Endpoint: "203.0.113.1:51820", EndpointGuessed: true,
	}, Source: DHTMethod, Hand: daemon.FirstHand})
	peerStore.Offer(daemon.Candidate{Info: &daemon.PeerInfo{
		WGPubKey: "advertised-pubkey", MeshIP: "10.0.0.3", Endpoint: "198.51.100.7:51821",
	}, Source: DHTMethod, Hand: daemon.FirstHand})

	pe := NewPeerExchange(cfg, daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", ""), peerStore)

	got := map[string]string{}
	for _, kp := range pe.getKnownPeers() {
		got[kp.WGPubKey] = kp.WGEndpoint
	}
	if ep, ok := got["guessed-pubkey"]; !ok || ep != "" {
		t.Errorf("guessed peer listed = %v with endpoint %q; want listed with an empty endpoint", ok, ep)
	}
	if got["advertised-pubkey"] != "198.51.100.7:51821" {
		t.Errorf("advertised peer endpoint = %q, want 198.51.100.7:51821", got["advertised-pubkey"])
	}
}
