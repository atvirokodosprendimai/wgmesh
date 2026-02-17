package discovery

import (
	"net"
	"testing"
)

func TestResolveEndpointPrefersLANSourceIP(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.42"), Port: 51830}

	tests := []struct {
		name       string
		advertised string
		sender     *net.UDPAddr
		want       string
	}{
		{name: "public advertised on LAN", advertised: "203.0.113.9:51820", sender: sender, want: "192.168.1.42:51820"},
		{name: "wildcard advertised", advertised: "0.0.0.0:51820", sender: sender, want: "192.168.1.42:51820"},
		{name: "invalid advertised", advertised: "not-an-endpoint", sender: sender, want: "192.168.1.42:51820"},
		{name: "no sender keeps explicit", advertised: "203.0.113.9:51820", sender: nil, want: "203.0.113.9:51820"},
		{name: "no sender invalid", advertised: "not-an-endpoint", sender: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(tt.advertised, tt.sender)
			if got != tt.want {
				t.Fatalf("resolveEndpoint(%q, %v) = %q, want %q", tt.advertised, tt.sender, got, tt.want)
			}
		})
	}
}
