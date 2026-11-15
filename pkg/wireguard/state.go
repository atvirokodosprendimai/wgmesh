package wireguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// wgCommandTimeout bounds every `wg` child started through runWG (ADR-0007
// Decision 6). A variable so a test can shorten it.
var wgCommandTimeout = 10 * time.Second

// wgWaitDelay bounds the wait for a killed child's pipes, so a grandchild that
// inherited stdout cannot keep runWG blocked past the deadline.
const wgWaitDelay = time.Second

// StatePeer is one peer line of `wg show <iface> dump`.
type StatePeer struct {
	PublicKey       string
	Endpoint        string // "" when wg prints "(none)"
	LatestHandshake int64  // Unix seconds, 0 = never
	RxBytes         uint64
	TxBytes         uint64
}

// State is one snapshot of a local WireGuard interface. It is a separate type
// from Config/Peer (the SSH path), parsed from the same dump line format:
// columns 1 to 7 of a peer line are key, preshared key, endpoint, allowed IPs,
// latest handshake, rx and tx bytes.
type State struct {
	Peers []StatePeer
}

// slot is the last good snapshot of one interface.
type slot struct {
	mu    sync.Mutex
	at    time.Time // when the fork that produced state started
	state State
	ok    bool
}

var (
	slotsMu sync.Mutex
	slots   = map[string]*slot{}
)

func slotFor(iface string) *slot {
	slotsMu.Lock()
	defer slotsMu.Unlock()
	s, ok := slots[iface]
	if !ok {
		s = &slot{}
		slots[iface] = s
	}
	return s
}

// ReadState returns a snapshot of the local interface from one
// `wg show <iface> dump`. A stored result younger than maxAge is returned
// without forking; maxAge <= 0 always forks and refreshes the store. A failed
// fork or parse is returned and never stored. The caller owns the result.
// The staleness each caller accepts is ADR-0007 Decision 5.
func ReadState(iface string, maxAge time.Duration) (*State, error) {
	s := slotFor(iface)
	// The lock spans the fork on purpose: concurrent readers of one interface share one fork; a hung wg costs each queued caller up to wgCommandTimeout.
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxAge > 0 && s.ok && time.Since(s.at) < maxAge {
		return s.state.clone(), nil
	}

	started := time.Now()
	stdout, stderr, err := runWG(nil, "show", iface, "dump")
	if err != nil {
		if msg := strings.TrimSpace(string(stderr)); msg != "" {
			return nil, fmt.Errorf("wg show %s dump failed: %s: %w", iface, msg, err)
		}
		return nil, fmt.Errorf("wg show %s dump failed: %w", iface, err)
	}
	st, err := parseState(string(stdout))
	if err != nil {
		return nil, fmt.Errorf("wg show %s dump: %w", iface, err)
	}
	s.state, s.at, s.ok = *st, started, true
	return st.clone(), nil
}

func (st *State) clone() *State {
	return &State{Peers: append([]StatePeer(nil), st.Peers...)}
}

// parseState reads dump output: the first line is the interface, each other
// line with at least 7 columns is a peer. A non-numeric handshake, rx or tx
// column counts as 0 (the old Sscanf readers ignored the error the same way).
func parseState(output string) (*State, error) {
	st := &State{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		hs, _ := strconv.ParseInt(f[4], 10, 64)
		rx, _ := strconv.ParseUint(f[5], 10, 64)
		tx, _ := strconv.ParseUint(f[6], 10, 64)
		st.Peers = append(st.Peers, StatePeer{
			PublicKey:       f[0],
			Endpoint:        noneToEmpty(f[2]),
			LatestHandshake: hs,
			RxBytes:         rx,
			TxBytes:         tx,
		})
	}
	return st, nil
}

// SetWGPath points the package at another `wg` binary and drops every stored
// snapshot, so a test can use a fake script. The returned func restores the
// previous path and drops the snapshots again. For tests only; not safe to
// call while other goroutines use the package.
func SetWGPath(path string) (restore func()) {
	old := wgPath
	wgPath = path
	resetSlots()
	return func() {
		wgPath = old
		resetSlots()
	}
}

func resetSlots() {
	slotsMu.Lock()
	defer slotsMu.Unlock()
	slots = map[string]*slot{}
}

// runWG runs `wg args...` with an optional stdin and kills the child after
// wgCommandTimeout. It returns stdout and stderr separately; on expiry the
// error names the sub-command and the timeout.
func runWG(stdin io.Reader, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), wgCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, wgPath, args...)
	cmd.Stdin = stdin
	cmd.WaitDelay = wgWaitDelay
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb

	err = cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("wg %s timed out after %s: %w", args[0], wgCommandTimeout, context.DeadlineExceeded)
	}
	return out.Bytes(), errb.Bytes(), err
}
