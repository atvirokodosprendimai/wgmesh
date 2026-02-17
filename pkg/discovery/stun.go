package discovery

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// STUN constants per RFC 5389
const (
	stunBindingRequest  = 0x0001
	stunBindingResponse = 0x0101
	stunMagicCookie     = 0x2112A442
	stunHeaderSize      = 20

	stunAttrMappedAddress    = 0x0001
	stunAttrXORMappedAddress = 0x0020
)

var stunTracer = otel.Tracer("wgmesh.stun")

// Default STUN servers (public, free, reliable)
var DefaultSTUNServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

// buildBindingRequest creates a minimal STUN Binding Request (RFC 5389 Section 6).
// 20 bytes: type(2) + length(2) + magic cookie(4) + transaction ID(12)
func buildBindingRequest() []byte {
	req := make([]byte, stunHeaderSize)
	binary.BigEndian.PutUint16(req[0:2], stunBindingRequest)
	binary.BigEndian.PutUint16(req[2:4], 0) // no attributes
	binary.BigEndian.PutUint32(req[4:8], stunMagicCookie)
	// Random transaction ID
	rand.Read(req[8:20])
	return req
}

// parseBindingResponse extracts the external IP and port from a STUN Binding Response.
// Validates the transaction ID matches the request, then prefers XOR-MAPPED-ADDRESS,
// falls back to MAPPED-ADDRESS.
func parseBindingResponse(data []byte, txnID [12]byte) (net.IP, int, error) {
	if len(data) < stunHeaderSize {
		return nil, 0, fmt.Errorf("response too short: %d bytes", len(data))
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType != stunBindingResponse {
		return nil, 0, fmt.Errorf("unexpected message type: 0x%04x", msgType)
	}

	// Validate magic cookie (RFC 5389 Section 6)
	cookie := binary.BigEndian.Uint32(data[4:8])
	if cookie != stunMagicCookie {
		return nil, 0, fmt.Errorf("invalid magic cookie: 0x%08x", cookie)
	}

	// Validate transaction ID matches our request (M5: prevents spoofed responses)
	var respTxnID [12]byte
	copy(respTxnID[:], data[8:20])
	if respTxnID != txnID {
		return nil, 0, fmt.Errorf("transaction ID mismatch")
	}

	attrLen := binary.BigEndian.Uint16(data[2:4])
	if int(attrLen) > len(data)-stunHeaderSize {
		return nil, 0, fmt.Errorf("attribute length %d exceeds data", attrLen)
	}

	attrs := data[stunHeaderSize : stunHeaderSize+int(attrLen)]

	// Parse attributes, preferring XOR-MAPPED-ADDRESS
	var mappedIP net.IP
	var mappedPort int

	for len(attrs) >= 4 {
		attrType := binary.BigEndian.Uint16(attrs[0:2])
		valLen := binary.BigEndian.Uint16(attrs[2:4])

		// Pad to 4-byte boundary
		padLen := valLen
		if padLen%4 != 0 {
			padLen += 4 - padLen%4
		}

		if int(4+valLen) > len(attrs) {
			break
		}

		val := attrs[4 : 4+valLen]

		switch attrType {
		case stunAttrXORMappedAddress:
			ip, port, err := parseXORMappedAddress(val, txnID)
			if err == nil {
				return ip, port, nil // preferred, return immediately
			}
		case stunAttrMappedAddress:
			ip, port, err := parseMappedAddress(val)
			if err == nil {
				mappedIP = ip
				mappedPort = port
			}
		}

		attrs = attrs[4+padLen:]
	}

	if mappedIP != nil {
		return mappedIP, mappedPort, nil
	}

	return nil, 0, fmt.Errorf("no mapped address in response")
}

// parseXORMappedAddress decodes a XOR-MAPPED-ADDRESS attribute (RFC 5389 Section 15.2).
func parseXORMappedAddress(val []byte, txnID [12]byte) (net.IP, int, error) {
	if len(val) < 4 {
		return nil, 0, fmt.Errorf("XOR-MAPPED-ADDRESS too short")
	}

	family := val[1]
	xorPort := binary.BigEndian.Uint16(val[2:4])
	port := int(xorPort ^ uint16(stunMagicCookie>>16))

	switch family {
	case 0x01: // IPv4
		if len(val) < 8 {
			return nil, 0, fmt.Errorf("XOR-MAPPED-ADDRESS IPv4 too short")
		}
		var cookieBytes [4]byte
		binary.BigEndian.PutUint32(cookieBytes[:], stunMagicCookie)
		ip := make(net.IP, 4)
		for i := 0; i < 4; i++ {
			ip[i] = val[4+i] ^ cookieBytes[i]
		}
		return ip, port, nil

	case 0x02: // IPv6
		if len(val) < 20 {
			return nil, 0, fmt.Errorf("XOR-MAPPED-ADDRESS IPv6 too short")
		}
		var xorKey [16]byte
		binary.BigEndian.PutUint32(xorKey[0:4], stunMagicCookie)
		copy(xorKey[4:16], txnID[:])
		ip := make(net.IP, 16)
		for i := 0; i < 16; i++ {
			ip[i] = val[4+i] ^ xorKey[i]
		}
		return ip, port, nil

	default:
		return nil, 0, fmt.Errorf("unknown address family: 0x%02x", family)
	}
}

// parseMappedAddress decodes a MAPPED-ADDRESS attribute (RFC 5389 Section 15.1).
func parseMappedAddress(val []byte) (net.IP, int, error) {
	if len(val) < 4 {
		return nil, 0, fmt.Errorf("MAPPED-ADDRESS too short")
	}

	family := val[1]
	port := int(binary.BigEndian.Uint16(val[2:4]))

	switch family {
	case 0x01: // IPv4
		if len(val) < 8 {
			return nil, 0, fmt.Errorf("MAPPED-ADDRESS IPv4 too short")
		}
		ip := make(net.IP, 4)
		copy(ip, val[4:8])
		return ip, port, nil

	case 0x02: // IPv6
		if len(val) < 20 {
			return nil, 0, fmt.Errorf("MAPPED-ADDRESS IPv6 too short")
		}
		ip := make(net.IP, 16)
		copy(ip, val[4:20])
		return ip, port, nil

	default:
		return nil, 0, fmt.Errorf("unknown address family: 0x%02x", family)
	}
}

// STUNQuery sends a STUN Binding Request and returns the server-reflexive address.
// localPort: if non-zero, binds the UDP socket to this local port (for port-preserving NATs).
// timeoutMs: response timeout in milliseconds.
func STUNQuery(server string, localPort int, timeoutMs int) (net.IP, int, error) {
	// Resolve STUN server
	raddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve STUN server %q: %w", server, err)
	}

	// Bind local socket
	var laddr *net.UDPAddr
	if localPort > 0 {
		laddr = &net.UDPAddr{Port: localPort}
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, 0, fmt.Errorf("bind UDP: %w", err)
	}
	defer conn.Close()

	// Build and send request
	req := buildBindingRequest()
	var txnID [12]byte
	copy(txnID[:], req[8:20])

	if _, err := conn.WriteToUDP(req, raddr); err != nil {
		return nil, 0, fmt.Errorf("send STUN request: %w", err)
	}

	// Read response — validate sender matches the STUN server (M4: prevents spoofed responses)
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 512)
	n, sender, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, 0, fmt.Errorf("read STUN response: %w", err)
	}
	if sender == nil || !sender.IP.Equal(raddr.IP) {
		return nil, 0, fmt.Errorf("STUN response from unexpected sender %v (expected %v)", sender, raddr)
	}

	return parseBindingResponse(buf[:n], txnID)
}

// DiscoverExternalEndpoint tries multiple STUN servers and returns the first
// successful result. Returns the external IP and the mapped port.
func DiscoverExternalEndpoint(localPort int) (net.IP, int, error) {
	for _, server := range DefaultSTUNServers {
		ip, port, err := STUNQuery(server, localPort, 3000)
		if err == nil {
			return ip, port, nil
		}
	}
	return nil, 0, fmt.Errorf("all STUN servers failed")
}

// NATType classifies the NAT behavior observed via STUN.
type NATType string

const (
	// NATUnknown means only one STUN server responded — can't classify.
	NATUnknown NATType = "unknown"
	// NATCone means both STUN servers saw the same external IP:port
	// (endpoint-independent mapping). Hole-punching works reliably.
	NATCone NATType = "cone"
	// NATSymmetric means STUN servers saw different external mappings
	// (endpoint-dependent). Direct hole-punching is unreliable; relay needed.
	NATSymmetric NATType = "symmetric"
)

// DetectNATType queries two STUN servers from the same local socket and
// compares the reflected external addresses.
//
// Same IP:port from both → Cone (port-preserving, hole-punch friendly).
// Different IP or port   → Symmetric (per-destination mapping, needs relay).
// Only one responds      → Unknown (still returns the successful result).
//
// Returns the NAT type, the external IP from the first server, the external
// port from the first server, and any error. Returns error only if both fail.
func DetectNATType(server1, server2 string, localPort int, timeoutMs int) (NATType, net.IP, int, error) {
	_, span := stunTracer.Start(context.Background(), "stun.detect_nat_type")
	defer span.End()

	// Bind a single UDP socket — both queries must originate from the
	// same source port to compare NAT mappings.
	var laddr *net.UDPAddr
	if localPort > 0 {
		laddr = &net.UDPAddr{Port: localPort}
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return "", nil, 0, fmt.Errorf("bind UDP for NAT detection: %w", err)
	}
	defer conn.Close()

	ip1, port1, err1 := stunQueryConn(conn, server1, timeoutMs)
	ip2, port2, err2 := stunQueryConn(conn, server2, timeoutMs)

	if err1 != nil && err2 != nil {
		return "", nil, 0, fmt.Errorf("both STUN servers failed: %v; %v", err1, err2)
	}

	// Only one server responded — can't determine NAT type
	if err1 != nil {
		span.SetAttributes(attribute.String("nat.type", string(NATUnknown)), attribute.String("external.addr", fmt.Sprintf("%s:%d", ip2, port2)))
		return NATUnknown, ip2, port2, nil
	}
	if err2 != nil {
		span.SetAttributes(attribute.String("nat.type", string(NATUnknown)), attribute.String("external.addr", fmt.Sprintf("%s:%d", ip1, port1)))
		return NATUnknown, ip1, port1, nil
	}

	// Compare: same IP and port → Cone; different → Symmetric
	if ip1.Equal(ip2) && port1 == port2 {
		span.SetAttributes(attribute.String("nat.type", string(NATCone)), attribute.String("external.addr", fmt.Sprintf("%s:%d", ip1, port1)))
		return NATCone, ip1, port1, nil
	}
	span.SetAttributes(attribute.String("nat.type", string(NATSymmetric)), attribute.String("external.addr", fmt.Sprintf("%s:%d", ip1, port1)))
	return NATSymmetric, ip1, port1, nil
}

// stunQueryConn sends a STUN Binding Request on an existing UDP connection
// and returns the server-reflexive address. Unlike STUNQuery, this reuses
// a shared socket (required for NAT type detection).
func stunQueryConn(conn *net.UDPConn, server string, timeoutMs int) (net.IP, int, error) {
	raddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve %q: %w", server, err)
	}

	req := buildBindingRequest()
	var txnID [12]byte
	copy(txnID[:], req[8:20])

	if _, err := conn.WriteToUDP(req, raddr); err != nil {
		return nil, 0, fmt.Errorf("send to %s: %w", server, err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 512)
	n, sender, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, 0, fmt.Errorf("read from %s: %w", server, err)
	}
	if sender == nil || !sender.IP.Equal(raddr.IP) {
		return nil, 0, fmt.Errorf("STUN response from unexpected sender %v (expected %v)", sender, raddr)
	}

	return parseBindingResponse(buf[:n], txnID)
}
