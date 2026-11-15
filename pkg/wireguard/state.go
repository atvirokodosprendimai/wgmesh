package wireguard

import (
	"io"
	"time"
)

// wgCommandTimeout bounds every `wg` child started through runWG.
var wgCommandTimeout = 10 * time.Second

// StatePeer is one peer line of `wg show <iface> dump`.
type StatePeer struct {
	PublicKey       string
	Endpoint        string // "" when wg prints "(none)"
	LatestHandshake int64  // Unix seconds, 0 = never
	RxBytes         uint64
	TxBytes         uint64
}

// State is one snapshot of a local WireGuard interface.
type State struct {
	Peers []StatePeer
}

// ReadState is a stub.
func ReadState(iface string, maxAge time.Duration) (*State, error) {
	return &State{}, nil
}

// SetWGPath is a stub.
func SetWGPath(path string) (restore func()) {
	old := wgPath
	wgPath = path
	return func() { wgPath = old }
}

func runWG(stdin io.Reader, args ...string) (stdout, stderr []byte, err error) {
	return nil, nil, nil
}
