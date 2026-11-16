package wireguard

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DumpPeer is one peer line of `wg show <iface> dump`.
type DumpPeer struct {
	PublicKey       string
	PresharedKey    string
	Endpoint        string
	AllowedIPs      []string
	LatestHandshake int64 // unix seconds, 0 = never
	RxBytes         uint64
	TxBytes         uint64
	Keepalive       int // seconds, 0 = off
}

// DumpPeers is the peers of one dump, by public key.
type DumpPeers map[string]DumpPeer

// ParseDump parses the output of `wg show <iface> dump`: tab separated, the
// first line is the interface (private key, public key, listen port, fwmark)
// and is skipped, every other line is a peer (public key, preshared key,
// endpoint, allowed ips, latest handshake, rx, tx, keepalive). "(none)" is an
// empty field and keepalive "off" is 0. An empty output or a line that does
// not fit is an error: it must never read as a kernel without peers.
func ParseDump(output string) (DumpPeers, error) {
	output = strings.TrimRight(output, "\r\n")
	if output == "" {
		return nil, errors.New("wg dump: empty output")
	}
	lines := strings.Split(output, "\n")
	if n := len(strings.Split(lines[0], "\t")); n < 4 {
		return nil, fmt.Errorf("wg dump: interface line has %d fields, want 4", n)
	}

	peers := make(DumpPeers, len(lines)-1)
	for i, line := range lines[1:] {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 8 {
			return nil, fmt.Errorf("wg dump: peer line %d has %d fields, want 8", i+2, len(f))
		}
		p := DumpPeer{
			PublicKey:    f[0],
			PresharedKey: noneToEmpty(f[1]),
			Endpoint:     noneToEmpty(f[2]),
		}
		if ips := noneToEmpty(f[3]); ips != "" {
			p.AllowedIPs = strings.Split(ips, ",")
		}
		var err error
		if p.LatestHandshake, err = strconv.ParseInt(f[4], 10, 64); err != nil {
			return nil, fmt.Errorf("wg dump: peer line %d: latest handshake %q: %w", i+2, f[4], err)
		}
		if p.RxBytes, err = strconv.ParseUint(f[5], 10, 64); err != nil {
			return nil, fmt.Errorf("wg dump: peer line %d: rx %q: %w", i+2, f[5], err)
		}
		if p.TxBytes, err = strconv.ParseUint(f[6], 10, 64); err != nil {
			return nil, fmt.Errorf("wg dump: peer line %d: tx %q: %w", i+2, f[6], err)
		}
		if f[7] != "off" {
			if p.Keepalive, err = strconv.Atoi(f[7]); err != nil {
				return nil, fmt.Errorf("wg dump: peer line %d: keepalive %q: %w", i+2, f[7], err)
			}
		}
		peers[p.PublicKey] = p
	}
	return peers, nil
}

// Dump reads the kernel's view of the interface with one `wg show <iface> dump`.
func Dump(iface string) (DumpPeers, error) {
	out, err := exec.Command(wgPath, "show", iface, "dump").Output()
	if err != nil {
		return nil, fmt.Errorf("wg show dump failed: %w", err)
	}
	return ParseDump(string(out))
}
