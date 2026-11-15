package ssh

import (
	"net"
	"testing"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return ipnet
}

func TestParseLocalAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		network   string
		output    string
		wantIface string
		wantSrc   string
		wantOK    bool
	}{
		{
			name:      "match",
			network:   "192.168.77.0/24",
			output:    `4: lan0    inet 192.168.77.1/24 scope global lan0\       valid_lft forever preferred_lft forever`,
			wantIface: "lan0",
			wantSrc:   "192.168.77.1",
			wantOK:    true,
		},
		{name: "no match", network: "192.168.77.0/24", output: ""},
		{
			name:    "address with a different prefix",
			network: "192.168.77.0/24",
			output:  `4: lan0    inet 192.168.77.1/16 scope global lan0\       valid_lft forever preferred_lft forever`,
		},
		{
			name:    "multiple lines pick first sane",
			network: "192.168.77.0/24",
			output: "4: bad;name    inet 192.168.77.5/24 scope global lan0\\\n" +
				"5: lan1    inet 192.168.77.9/24 brd 192.168.77.255 scope global lan1\\       valid_lft forever\n" +
				"6: lan2    inet 192.168.77.10/24 scope global lan2\\       valid_lft forever\n",
			wantIface: "lan1",
			wantSrc:   "192.168.77.9",
			wantOK:    true,
		},
		{
			name:      "ipv6",
			network:   "2001:db8:1::/64",
			output:    `3: eth1@if7    inet6 2001:db8:1::1/64 scope global \       valid_lft forever preferred_lft forever`,
			wantIface: "eth1",
			wantSrc:   "2001:db8:1::1",
			wantOK:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iface, src, ok := parseLocalAddr(tt.output, mustCIDR(t, tt.network))
			if iface != tt.wantIface || src != tt.wantSrc || ok != tt.wantOK {
				t.Errorf("parseLocalAddr() = %q, %q, %v; want %q, %q, %v",
					iface, src, ok, tt.wantIface, tt.wantSrc, tt.wantOK)
			}
		})
	}
}

func TestConnectedRouteCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		network, iface, src, want string
	}{
		{"192.168.77.0/24", "lan0", "192.168.77.1",
			"ip route add 192.168.77.0/24 dev lan0 proto kernel scope link src 192.168.77.1"},
		{"2001:db8:1::/64", "eth1", "2001:db8:1::1",
			"ip -6 route add 2001:db8:1::/64 dev eth1 proto kernel metric 256"},
	}
	for _, tt := range tests {
		if got := connectedRouteCmd(mustCIDR(t, tt.network), tt.iface, tt.src); got != tt.want {
			t.Errorf("connectedRouteCmd() = %q, want %q", got, tt.want)
		}
	}
}
