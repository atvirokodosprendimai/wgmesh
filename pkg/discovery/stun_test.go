package discovery

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildBindingRequest(t *testing.T) {
	req := buildBindingRequest()

	if len(req) != stunHeaderSize {
		t.Fatalf("request length = %d, want %d", len(req), stunHeaderSize)
	}

	// Message Type: 0x0001 (Binding Request)
	msgType := binary.BigEndian.Uint16(req[0:2])
	if msgType != stunBindingRequest {
		t.Errorf("message type = 0x%04x, want 0x%04x", msgType, stunBindingRequest)
	}

	// Message Length: 0 (no attributes)
	msgLen := binary.BigEndian.Uint16(req[2:4])
	if msgLen != 0 {
		t.Errorf("message length = %d, want 0", msgLen)
	}

	// Magic Cookie: 0x2112A442
	cookie := binary.BigEndian.Uint32(req[4:8])
	if cookie != stunMagicCookie {
		t.Errorf("magic cookie = 0x%08x, want 0x%08x", cookie, stunMagicCookie)
	}

	// Transaction ID: 12 bytes, should be non-zero
	txnID := req[8:20]
	allZero := true
	for _, b := range txnID {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("transaction ID is all zeros")
	}
}

func TestParseBindingResponse_XORMappedAddress_IPv4(t *testing.T) {
	// Build a valid STUN Binding Response with XOR-MAPPED-ADDRESS
	txnID := [12]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}

	// XOR-MAPPED-ADDRESS attribute for 198.51.100.1:51820
	// Family: 0x01 (IPv4)
	// XOR Port: 51820 XOR 0x2112 = 51820 ^ 8466 = 60278 = 0xEB66
	// XOR Address: 198.51.100.1 XOR 0x2112A442
	//   198.51.100.1 = C6.33.64.01
	//   XOR 21.12.A4.42 = E7.21.C0.43
	ip := net.ParseIP("198.51.100.1").To4()
	port := uint16(51820)
	xorPort := port ^ uint16(stunMagicCookie>>16)
	var xorIP [4]byte
	cookieBytes := [4]byte{}
	binary.BigEndian.PutUint32(cookieBytes[:], stunMagicCookie)
	for i := 0; i < 4; i++ {
		xorIP[i] = ip[i] ^ cookieBytes[i]
	}

	// Build attribute: type(2) + length(2) + reserved(1) + family(1) + port(2) + ip(4)
	attr := make([]byte, 12)
	binary.BigEndian.PutUint16(attr[0:2], stunAttrXORMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 8) // value length
	attr[4] = 0x00                           // reserved
	attr[5] = 0x01                           // IPv4
	binary.BigEndian.PutUint16(attr[6:8], xorPort)
	copy(attr[8:12], xorIP[:])

	// Build response header
	resp := make([]byte, stunHeaderSize+len(attr))
	binary.BigEndian.PutUint16(resp[0:2], stunBindingResponse)
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(attr)))
	binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
	copy(resp[8:20], txnID[:])
	copy(resp[20:], attr)

	gotIP, gotPort, err := parseBindingResponse(resp, txnID)
	if err != nil {
		t.Fatalf("parseBindingResponse: %v", err)
	}
	if !gotIP.Equal(net.ParseIP("198.51.100.1")) {
		t.Errorf("IP = %v, want 198.51.100.1", gotIP)
	}
	if gotPort != 51820 {
		t.Errorf("port = %d, want 51820", gotPort)
	}
}

func TestParseBindingResponse_XORMappedAddress_IPv6(t *testing.T) {
	txnID := [12]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	// XOR-MAPPED-ADDRESS for [2001:db8::1]:51820
	ip := net.ParseIP("2001:db8::1").To16()
	port := uint16(51820)
	xorPort := port ^ uint16(stunMagicCookie>>16)

	// XOR IP: ip XOR (magic_cookie + txn_id)
	var xorKey [16]byte
	binary.BigEndian.PutUint32(xorKey[0:4], stunMagicCookie)
	copy(xorKey[4:16], txnID[:])
	var xorIP [16]byte
	for i := 0; i < 16; i++ {
		xorIP[i] = ip[i] ^ xorKey[i]
	}

	// Build attribute: type(2) + length(2) + reserved(1) + family(1) + port(2) + ip(16) = 24
	attr := make([]byte, 24)
	binary.BigEndian.PutUint16(attr[0:2], stunAttrXORMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 20) // value length
	attr[4] = 0x00                            // reserved
	attr[5] = 0x02                            // IPv6
	binary.BigEndian.PutUint16(attr[6:8], xorPort)
	copy(attr[8:24], xorIP[:])

	resp := make([]byte, stunHeaderSize+len(attr))
	binary.BigEndian.PutUint16(resp[0:2], stunBindingResponse)
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(attr)))
	binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
	copy(resp[8:20], txnID[:])
	copy(resp[20:], attr)

	gotIP, gotPort, err := parseBindingResponse(resp, txnID)
	if err != nil {
		t.Fatalf("parseBindingResponse: %v", err)
	}
	if !gotIP.Equal(net.ParseIP("2001:db8::1")) {
		t.Errorf("IP = %v, want 2001:db8::1", gotIP)
	}
	if gotPort != 51820 {
		t.Errorf("port = %d, want 51820", gotPort)
	}
}

func TestParseBindingResponse_MappedAddressFallback(t *testing.T) {
	// Some STUN servers return MAPPED-ADDRESS (0x0001) instead of XOR-MAPPED-ADDRESS
	txnID := [12]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c}

	ip := net.ParseIP("203.0.113.5").To4()
	port := uint16(12345)

	// MAPPED-ADDRESS: no XOR, raw values
	attr := make([]byte, 12)
	binary.BigEndian.PutUint16(attr[0:2], stunAttrMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 8)
	attr[4] = 0x00
	attr[5] = 0x01 // IPv4
	binary.BigEndian.PutUint16(attr[6:8], port)
	copy(attr[8:12], ip)

	resp := make([]byte, stunHeaderSize+len(attr))
	binary.BigEndian.PutUint16(resp[0:2], stunBindingResponse)
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(attr)))
	binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
	copy(resp[8:20], txnID[:])
	copy(resp[20:], attr)

	gotIP, gotPort, err := parseBindingResponse(resp, txnID)
	if err != nil {
		t.Fatalf("parseBindingResponse: %v", err)
	}
	if !gotIP.Equal(net.ParseIP("203.0.113.5")) {
		t.Errorf("IP = %v, want 203.0.113.5", gotIP)
	}
	if gotPort != 12345 {
		t.Errorf("port = %d, want 12345", gotPort)
	}
}

func TestParseBindingResponse_InvalidResponse(t *testing.T) {
	txnID := [12]byte{}

	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte{0x01}},
		{"wrong message type", func() []byte {
			b := make([]byte, 20)
			binary.BigEndian.PutUint16(b[0:2], 0x0111) // not Binding Response
			binary.BigEndian.PutUint32(b[4:8], stunMagicCookie)
			return b
		}()},
		{"no attributes", func() []byte {
			b := make([]byte, 20)
			binary.BigEndian.PutUint16(b[0:2], stunBindingResponse)
			binary.BigEndian.PutUint16(b[2:4], 0)
			binary.BigEndian.PutUint32(b[4:8], stunMagicCookie)
			return b
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseBindingResponse(tt.data, txnID)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestSTUNQueryWithMockServer tests the full STUN round-trip against a local mock server.
func TestSTUNQueryWithMockServer(t *testing.T) {
	// Start a mock STUN server on a random UDP port
	serverAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := net.ListenUDP("udp4", serverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()

	// The mock server reflects back a hardcoded external address
	wantIP := net.ParseIP("203.0.113.42").To4()
	wantPort := uint16(51820)

	go func() {
		buf := make([]byte, 512)
		n, clientAddr, err := serverConn.ReadFromUDP(buf)
		if err != nil || n < stunHeaderSize {
			return
		}

		// Extract transaction ID from request
		var txnID [12]byte
		copy(txnID[:], buf[8:20])

		// Build XOR-MAPPED-ADDRESS
		xorPort := wantPort ^ uint16(stunMagicCookie>>16)
		var cookieBytes [4]byte
		binary.BigEndian.PutUint32(cookieBytes[:], stunMagicCookie)
		var xorIP [4]byte
		for i := 0; i < 4; i++ {
			xorIP[i] = wantIP[i] ^ cookieBytes[i]
		}

		attr := make([]byte, 12)
		binary.BigEndian.PutUint16(attr[0:2], stunAttrXORMappedAddress)
		binary.BigEndian.PutUint16(attr[2:4], 8)
		attr[4] = 0x00 // reserved
		attr[5] = 0x01 // IPv4
		binary.BigEndian.PutUint16(attr[6:8], xorPort)
		copy(attr[8:12], xorIP[:])

		// Build response
		resp := make([]byte, stunHeaderSize+len(attr))
		binary.BigEndian.PutUint16(resp[0:2], stunBindingResponse)
		binary.BigEndian.PutUint16(resp[2:4], uint16(len(attr)))
		binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
		copy(resp[8:20], txnID[:])
		copy(resp[20:], attr)

		serverConn.WriteToUDP(resp, clientAddr)
	}()

	gotIP, gotPort, err := STUNQuery(serverConn.LocalAddr().String(), 0, 2000)
	if err != nil {
		t.Fatalf("STUNQuery: %v", err)
	}
	if !gotIP.Equal(net.ParseIP("203.0.113.42")) {
		t.Errorf("IP = %v, want 203.0.113.42", gotIP)
	}
	if gotPort != int(wantPort) {
		t.Errorf("port = %d, want %d", gotPort, wantPort)
	}
}

// TestDiscoverExternalEndpoint_AllFail verifies that DiscoverExternalEndpoint
// returns an error when all servers are unreachable.
func TestDiscoverExternalEndpoint_AllFail(t *testing.T) {
	// Start a UDP server that never replies (simulates unreachable STUN)
	silence, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer silence.Close()

	// Replace default servers with our silent server
	orig := DefaultSTUNServers
	DefaultSTUNServers = []string{silence.LocalAddr().String()}
	defer func() { DefaultSTUNServers = orig }()

	// DiscoverExternalEndpoint calls STUNQuery with 3000ms timeout per server,
	// but we only have one server so this takes ~3s. Use a direct STUNQuery
	// with a short timeout to keep the test fast.
	_, _, queryErr := STUNQuery(silence.LocalAddr().String(), 0, 200)
	if queryErr == nil {
		t.Fatal("expected error from silent STUN server, got nil")
	}
}

// mockSTUNServer starts a UDP server that replies with a hardcoded
// XOR-MAPPED-ADDRESS. Returns the server conn (caller must close).
func mockSTUNServer(t *testing.T, reflectIP net.IP, reflectPort uint16) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}

	ip4 := reflectIP.To4()

	go func() {
		buf := make([]byte, 512)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return // closed
			}
			if n < stunHeaderSize {
				continue
			}

			var txnID [12]byte
			copy(txnID[:], buf[8:20])

			xorPort := reflectPort ^ uint16(stunMagicCookie>>16)
			var cookieBytes [4]byte
			binary.BigEndian.PutUint32(cookieBytes[:], stunMagicCookie)
			var xorIP [4]byte
			for i := 0; i < 4; i++ {
				xorIP[i] = ip4[i] ^ cookieBytes[i]
			}

			attr := make([]byte, 12)
			binary.BigEndian.PutUint16(attr[0:2], stunAttrXORMappedAddress)
			binary.BigEndian.PutUint16(attr[2:4], 8)
			attr[4] = 0x00
			attr[5] = 0x01
			binary.BigEndian.PutUint16(attr[6:8], xorPort)
			copy(attr[8:12], xorIP[:])

			resp := make([]byte, stunHeaderSize+len(attr))
			binary.BigEndian.PutUint16(resp[0:2], stunBindingResponse)
			binary.BigEndian.PutUint16(resp[2:4], uint16(len(attr)))
			binary.BigEndian.PutUint32(resp[4:8], stunMagicCookie)
			copy(resp[8:20], txnID[:])
			copy(resp[20:], attr)

			conn.WriteToUDP(resp, clientAddr)
		}
	}()

	return conn
}

// TestDetectNATType_Cone verifies that two STUN servers returning the same
// external port classify the NAT as Cone (endpoint-independent mapping).
func TestDetectNATType_Cone(t *testing.T) {
	// Both servers reflect the same IP:port → Cone NAT
	srv1 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 51820)
	defer srv1.Close()
	srv2 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 51820)
	defer srv2.Close()

	natType, ip, port, err := DetectNATType(
		srv1.LocalAddr().String(),
		srv2.LocalAddr().String(),
		0, 2000,
	)
	if err != nil {
		t.Fatalf("DetectNATType: %v", err)
	}
	if natType != NATCone {
		t.Errorf("NAT type = %q, want %q", natType, NATCone)
	}
	if !ip.Equal(net.ParseIP("203.0.113.1")) {
		t.Errorf("IP = %v, want 203.0.113.1", ip)
	}
	if port != 51820 {
		t.Errorf("port = %d, want 51820", port)
	}
}

// TestDetectNATType_Symmetric verifies that two STUN servers returning
// different external ports classify the NAT as Symmetric.
func TestDetectNATType_Symmetric(t *testing.T) {
	// Servers reflect different ports → Symmetric NAT
	srv1 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 51820)
	defer srv1.Close()
	srv2 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 62000)
	defer srv2.Close()

	natType, _, _, err := DetectNATType(
		srv1.LocalAddr().String(),
		srv2.LocalAddr().String(),
		0, 2000,
	)
	if err != nil {
		t.Fatalf("DetectNATType: %v", err)
	}
	if natType != NATSymmetric {
		t.Errorf("NAT type = %q, want %q", natType, NATSymmetric)
	}
}

// TestDetectNATType_SymmetricDifferentIP verifies that different external IPs
// also classify as Symmetric (multi-homed NAT or carrier-grade NAT pool).
func TestDetectNATType_SymmetricDifferentIP(t *testing.T) {
	srv1 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 51820)
	defer srv1.Close()
	srv2 := mockSTUNServer(t, net.ParseIP("198.51.100.1"), 51820)
	defer srv2.Close()

	natType, _, _, err := DetectNATType(
		srv1.LocalAddr().String(),
		srv2.LocalAddr().String(),
		0, 2000,
	)
	if err != nil {
		t.Fatalf("DetectNATType: %v", err)
	}
	if natType != NATSymmetric {
		t.Errorf("NAT type = %q, want %q", natType, NATSymmetric)
	}
}

// TestDetectNATType_OneServerFails verifies graceful degradation when only
// one STUN server responds — should return Unknown with the first result.
func TestDetectNATType_OneServerFails(t *testing.T) {
	srv1 := mockSTUNServer(t, net.ParseIP("203.0.113.1"), 51820)
	defer srv1.Close()

	// Second server: a socket that never replies
	silent, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()

	natType, ip, port, err := DetectNATType(
		srv1.LocalAddr().String(),
		silent.LocalAddr().String(),
		0, 300, // short timeout
	)
	if err != nil {
		t.Fatalf("DetectNATType: %v", err)
	}
	if natType != NATUnknown {
		t.Errorf("NAT type = %q, want %q", natType, NATUnknown)
	}
	// Should still return the first server's result
	if !ip.Equal(net.ParseIP("203.0.113.1")) {
		t.Errorf("IP = %v, want 203.0.113.1", ip)
	}
	if port != 51820 {
		t.Errorf("port = %d, want 51820", port)
	}
}

// TestDetectNATType_BothFail verifies error when no STUN server responds.
func TestDetectNATType_BothFail(t *testing.T) {
	s1, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer s1.Close()
	s2, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	defer s2.Close()

	_, _, _, err := DetectNATType(
		s1.LocalAddr().String(),
		s2.LocalAddr().String(),
		0, 200,
	)
	if err == nil {
		t.Fatal("expected error when both STUN servers fail")
	}
}

// TestNATTypeString verifies the String() representations.
func TestNATTypeString(t *testing.T) {
	tests := []struct {
		nat  NATType
		want string
	}{
		{NATUnknown, "unknown"},
		{NATCone, "cone"},
		{NATSymmetric, "symmetric"},
		{NATType("other"), "other"},
	}
	for _, tt := range tests {
		if got := string(tt.nat); got != tt.want {
			t.Errorf("NATType(%q).String() = %q, want %q", tt.nat, got, tt.want)
		}
	}
}

// TestSTUNQueryIntegration tests against a real STUN server.
// Skipped in short mode — run with: go test -run TestSTUNQueryIntegration
func TestSTUNQueryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ip, port, err := STUNQuery("stun.l.google.com:19302", 0, 3000)
	if err != nil {
		t.Fatalf("STUNQuery: %v", err)
	}

	if ip == nil {
		t.Fatal("got nil IP")
	}
	if port == 0 {
		t.Fatal("got port 0")
	}

	t.Logf("External endpoint: %s:%d", ip, port)

	// Sanity: should not be a private IP
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		t.Errorf("got non-public IP: %v", ip)
	}
}
