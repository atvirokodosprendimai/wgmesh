package wireguard

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
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

// Dump reads the kernel's view of the interface with one `wg show <iface>
// dump`, strictly parsed. It runs through runWG (ADR-0007 Decision 6: a
// deadline) and stores the result as the interface's snapshot, so a ReadState
// that follows within its maxAge shares the fork instead of starting another.
func Dump(iface string) (DumpPeers, error) {
	s := slotFor(iface)
	s.mu.Lock()
	defer s.mu.Unlock()

	started := time.Now()
	stdout, stderr, err := runWG(nil, "show", iface, "dump")
	if err != nil {
		if msg := strings.TrimSpace(string(stderr)); msg != "" {
			return nil, fmt.Errorf("wg show %s dump failed: %s: %w", iface, msg, err)
		}
		return nil, fmt.Errorf("wg show %s dump failed: %w", iface, err)
	}
	peers, err := ParseDump(string(stdout))
	if err != nil {
		return nil, err
	}
	st := State{Peers: make([]StatePeer, 0, len(peers))}
	for _, p := range peers {
		st.Peers = append(st.Peers, StatePeer{PublicKey: p.PublicKey, Endpoint: p.Endpoint, LatestHandshake: p.LatestHandshake, RxBytes: p.RxBytes, TxBytes: p.TxBytes})
	}
	sort.Slice(st.Peers, func(i, j int) bool { return st.Peers[i].PublicKey < st.Peers[j].PublicKey })
	s.state, s.at, s.ok = st, started, true
	return peers, nil
}
