package ssh

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
)

func TestLocalAddrWarning(t *testing.T) {
	t.Parallel()

	_, ipnet, _ := net.ParseCIDR("192.168.10.0/24")
	if got := localAddrWarning(ipnet, nil); !strings.Contains(got, "no local address in 192.168.10.0/24") {
		t.Errorf("no error: got %q", got)
	}
	got := localAddrWarning(ipnet, errors.New("ssh: connection lost"))
	if !strings.Contains(got, "ssh: connection lost") || strings.Contains(got, "no local address") {
		t.Errorf("run error: got %q, want the error and not 'no local address'", got)
	}
}

// The kernel's "default" (an exit node's stale `default dev wg0`) has no
// connected route to restore and must not print an invalid-network warning.
func TestRestoreConnectedRouteDefaultIsQuiet(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	RestoreConnectedRoute(nil, "default") // nil client: must return before using it
	os.Stdout = old
	w.Close()
	out, _ := io.ReadAll(r)
	if len(out) != 0 {
		t.Errorf("printed %q, want nothing", out)
	}
}
