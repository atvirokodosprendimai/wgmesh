package routes

import (
	"net"
	"testing"
)

// --- NormalizeNetwork ---

func TestNormalizeNetwork_IPv4HostRoute(t *testing.T) {
	t.Parallel()
	got := NormalizeNetwork("192.168.5.5")
	if got != "192.168.5.5/32" {
		t.Errorf("NormalizeNetwork(%q) = %q, want %q", "192.168.5.5", got, "192.168.5.5/32")
	}
}

func TestNormalizeNetwork_IPv6HostRoute(t *testing.T) {
	t.Parallel()
	got := NormalizeNetwork("fd00::1")
	if got != "fd00::1/128" {
		t.Errorf("NormalizeNetwork(%q) = %q, want %q", "fd00::1", got, "fd00::1/128")
	}
}

func TestNormalizeNetwork_AlreadyHasPrefix(t *testing.T) {
	t.Parallel()
	tests := []string{"10.0.0.0/8", "192.168.1.0/24", "fd00::/64"}
	for _, cidr := range tests {
		got := NormalizeNetwork(cidr)
		if got != cidr {
			t.Errorf("NormalizeNetwork(%q) = %q, want unchanged", cidr, got)
		}
	}
}

// --- MakeKey ---

func TestMakeKey_Format(t *testing.T) {
	t.Parallel()
	got := MakeKey("10.0.0.0/8", "192.168.1.1")
	want := "10.0.0.0/8|192.168.1.1"
	if got != want {
		t.Errorf("MakeKey() = %q, want %q", got, want)
	}
}

func TestMakeKey_EmptyGateway(t *testing.T) {
	t.Parallel()
	got := MakeKey("10.0.0.0/8", "")
	want := "10.0.0.0/8|"
	if got != want {
		t.Errorf("MakeKey() = %q, want %q", got, want)
	}
}

// --- CalculateDiff ---

func TestCalculateDiff_NoChanges(t *testing.T) {
	t.Parallel()
	routes := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},
	}
	add, remove := CalculateDiff(routes, routes)
	if len(add) != 0 || len(remove) != 0 {
		t.Errorf("expected no changes; add=%v remove=%v", add, remove)
	}
}

func TestCalculateDiff_AddNewRoute(t *testing.T) {
	t.Parallel()
	current := []Entry{}
	desired := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 1 || add[0] != desired[0] {
		t.Errorf("expected 1 add; got %v", add)
	}
	if len(remove) != 0 {
		t.Errorf("expected 0 removes; got %v", remove)
	}
}

func TestCalculateDiff_RemoveStaleRoute(t *testing.T) {
	t.Parallel()
	current := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	desired := []Entry{}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 0 {
		t.Errorf("expected 0 adds; got %v", add)
	}
	if len(remove) != 1 || remove[0] != current[0] {
		t.Errorf("expected 1 remove; got %v", remove)
	}
}

func TestCalculateDiff_GatewayChanged(t *testing.T) {
	t.Parallel()
	current := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	desired := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.2"}}
	add, remove := CalculateDiff(current, desired)

	if len(add) != 1 || add[0] != desired[0] {
		t.Errorf("expected add of new gateway route; got %v", add)
	}
	if len(remove) != 1 || remove[0] != current[0] {
		t.Errorf("expected remove of old gateway route; got %v", remove)
	}
}

func TestCalculateDiff_DirectlyConnectedNotRemoved(t *testing.T) {
	t.Parallel()
	// Routes with empty gateway (directly connected) must not be removed.
	current := []Entry{
		{Network: "10.99.0.0/16", Gateway: ""},      // directly connected
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"}, // managed
	}
	desired := []Entry{}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 0 {
		t.Errorf("expected 0 adds; got %v", add)
	}
	// Only the managed route should be removed.
	if len(remove) != 1 || remove[0].Network != "10.0.0.0/8" {
		t.Errorf("expected only managed route removed; got %v", remove)
	}
}

func TestCalculateDiff_MultipleRoutes(t *testing.T) {
	t.Parallel()
	current := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},
		{Network: "172.16.0.0/12", Gateway: "192.168.1.1"},
	}
	desired := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},     // unchanged
		{Network: "192.168.5.5/32", Gateway: "192.168.1.2"}, // new
	}
	add, remove := CalculateDiff(current, desired)

	if len(add) != 1 || add[0].Network != "192.168.5.5/32" {
		t.Errorf("expected 1 add (192.168.5.5/32); got %v", add)
	}
	if len(remove) != 1 || remove[0].Network != "172.16.0.0/12" {
		t.Errorf("expected 1 remove (172.16.0.0/12); got %v", remove)
	}
}

func TestCalculateDiff_Idempotent(t *testing.T) {
	t.Parallel()
	// Calling CalculateDiff twice with the same result applied should be a no-op.
	initial := []Entry{
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"},
	}
	desired := []Entry{
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"},
		{Network: "172.16.0.0/12", Gateway: "1.2.3.4"},
	}

	add1, remove1 := CalculateDiff(initial, desired)
	// "Apply" the changes to build a new current state.
	afterApply := make([]Entry, len(initial))
	copy(afterApply, initial)
	for _, r := range remove1 {
		for i, e := range afterApply {
			if e == r {
				afterApply = append(afterApply[:i], afterApply[i+1:]...)
				break
			}
		}
	}
	afterApply = append(afterApply, add1...)

	// Second diff should be a no-op.
	add2, remove2 := CalculateDiff(afterApply, desired)
	if len(add2) != 0 || len(remove2) != 0 {
		t.Errorf("second CalculateDiff should be no-op; add=%v remove=%v", add2, remove2)
	}
}

func TestExcludeOwn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		networks []string
		own      []string
		want     []string
	}{
		{name: "no own networks keeps all", networks: []string{"10.1.0.0/24"}, want: []string{"10.1.0.0/24"}},
		{name: "disjoint kept", networks: []string{"10.1.0.0/24"}, own: []string{"10.2.0.0/24"}, want: []string{"10.1.0.0/24"}},
		{name: "shared dropped", networks: []string{"10.1.0.0/24", "10.2.0.0/24"}, own: []string{"10.1.0.0/24"}, want: []string{"10.2.0.0/24"}},
		{name: "host bits and spaces canonicalised", networks: []string{" 10.1.0.5/24"}, own: []string{"10.1.0.0/24"}, want: []string{}},
		{name: "bare v4 host matches /32", networks: []string{"192.168.5.5/32"}, own: []string{"192.168.5.5"}, want: []string{}},
		{name: "bare v6 host matches /128", networks: []string{"fd00::1/128"}, own: []string{"fd00::1"}, want: []string{}},
		{name: "default route dropped when both advertise it", networks: []string{"0.0.0.0/0"}, own: []string{"0.0.0.0/0"}, want: []string{}},
		{name: "kernel default matches own 0.0.0.0/0", networks: []string{"default"}, own: []string{"0.0.0.0/0"}, want: []string{}},
		{name: "own wider network does not drop a narrower peer network", networks: []string{"10.1.0.0/24"}, own: []string{"10.1.0.0/16"}, want: []string{"10.1.0.0/24"}},
		{name: "own narrower network does not drop a wider peer network", networks: []string{"10.1.0.0/16"}, own: []string{"10.1.0.0/24"}, want: []string{"10.1.0.0/16"}},
		{name: "default route kept unless advertised by both", networks: []string{"0.0.0.0/0"}, own: []string{"10.1.0.0/24"}, want: []string{"0.0.0.0/0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExcludeOwn(tt.networks, tt.own)
			if len(got) != len(tt.want) {
				t.Fatalf("ExcludeOwn() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ExcludeOwn() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

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
			iface, src, ok := ParseLocalAddr(tt.output, mustCIDR(t, tt.network))
			if iface != tt.wantIface || src != tt.wantSrc || ok != tt.wantOK {
				t.Errorf("ParseLocalAddr() = %q, %q, %v; want %q, %q, %v",
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
		if got := ConnectedRouteCmd(mustCIDR(t, tt.network), tt.iface, tt.src); got != tt.want {
			t.Errorf("ConnectedRouteCmd() = %q, want %q", got, tt.want)
		}
	}
}
