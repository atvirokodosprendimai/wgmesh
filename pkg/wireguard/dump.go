package wireguard

import (
	"errors"
	"fmt"
	"os/exec"
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

// ParseDump parses the output of `wg show <iface> dump`.
func ParseDump(output string) (DumpPeers, error) {
	return nil, errors.New("ParseDump: not implemented")
}

// Dump reads the kernel's view of the interface with one `wg show <iface> dump`.
func Dump(iface string) (DumpPeers, error) {
	out, err := exec.Command(wgPath, "show", iface, "dump").Output()
	if err != nil {
		return nil, fmt.Errorf("wg show dump failed: %w", err)
	}
	return ParseDump(string(out))
}
