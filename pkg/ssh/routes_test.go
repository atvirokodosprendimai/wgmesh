package ssh

import (
	"errors"
	"net"
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
