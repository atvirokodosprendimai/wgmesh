package discovery

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
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
// Prefers XOR-MAPPED-ADDRESS, falls back to MAPPED-ADDRESS.
func parseBindingResponse(data []byte, txnID [12]byte) (net.IP, int, error) {
	if len(data) < stunHeaderSize {
		return nil, 0, fmt.Errorf("response too short: %d bytes", len(data))
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType != stunBindingResponse {
		return nil, 0, fmt.Errorf("unexpected message type: 0x%04x", msgType)
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

	// Read response
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 512)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, 0, fmt.Errorf("read STUN response: %w", err)
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
