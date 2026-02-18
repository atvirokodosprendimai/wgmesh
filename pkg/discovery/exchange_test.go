package discovery

import (
	"encoding/json"
	"net"
	"testing"

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

	localNode := &daemon.LocalNode{
		WGPubKey: "local-pubkey",
		MeshIP:   "10.0.0.1",
	}
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

	localNode := &daemon.LocalNode{
		WGPubKey: "local-pubkey",
		MeshIP:   "10.0.0.1",
	}
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

	localNode := &daemon.LocalNode{
		WGPubKey: "local-pubkey",
		MeshIP:   "10.0.0.1",
	}
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

	localNode := &daemon.LocalNode{
		WGPubKey: "local-pubkey",
		MeshIP:   "10.0.0.1",
	}
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

	localNode := &daemon.LocalNode{
		WGPubKey: "local-pubkey",
		MeshIP:   "10.0.0.1",
	}
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
		name       string
		advertised string
		sender     *net.UDPAddr
		want       string
	}{
		{"explicit_endpoint", "1.2.3.4:51820", nil, "1.2.3.4:51820"},
		{"wildcard_with_sender", "0.0.0.0:51820", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820"},
		{"empty_host_with_sender", ":51820", &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 41234}, "203.0.113.1:51820"},
		{"ipv6_wildcard", ":::51820", &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 41234}, "[2001:db8::1]:51820"},
		{"no_sender_no_host", "0.0.0.0:51820", nil, "0.0.0.0:51820"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePeerEndpoint(tt.advertised, tt.sender)
			if got != tt.want {
				t.Errorf("resolvePeerEndpoint(%q, %v) = %q, want %q", tt.advertised, tt.sender, got, tt.want)
			}
		})
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
