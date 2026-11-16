package node

import (
	"net"
	"testing"
)

func TestResolveEndpointKeepsAdvertisedPort(t *testing.T) {
	t.Parallel()

	sender := net.ParseIP("203.0.113.1")
	tests := []struct {
		name         string
		advertised   string
		preferSender bool
		want         string
	}{
		{"real host keeps host and port", "198.51.100.7:51821", false, "198.51.100.7:51821"},
		{"unspecified host takes sender IP", "0.0.0.0:51821", false, "203.0.113.1:51821"},
		{"empty host takes sender IP", ":51821", false, "203.0.113.1:51821"},
		{"ipv6 unspecified takes sender IP", "[::]:51821", false, "203.0.113.1:51821"},
		{"preferSender keeps sender IP over a real host", "198.51.100.7:51821", true, "203.0.113.1:51821"},
	}
	for _, tt := range tests {
		got, guessed := ResolveEndpoint(tt.advertised, sender, 51820, tt.preferSender)
		if got != tt.want || guessed {
			t.Errorf("%s: ResolveEndpoint(%q) = %q, %v; want %q, false", tt.name, tt.advertised, got, guessed, tt.want)
		}
	}
}

// With no sender an advertised unspecified host is kept as it is. In
// production the remote address is never nil, so this pins today's behaviour
// of the function, not a rule a caller relies on.
func TestResolveEndpointNilSenderKeepsAdvertised(t *testing.T) {
	t.Parallel()

	if got, guessed := ResolveEndpoint("0.0.0.0:51821", nil, 51820, false); got != "0.0.0.0:51821" || guessed {
		t.Errorf("ResolveEndpoint(0.0.0.0:51821, nil) = %q, %v; want 0.0.0.0:51821, false", got, guessed)
	}
}

func TestResolveEndpointMarksGuess(t *testing.T) {
	t.Parallel()

	sender := net.ParseIP("203.0.113.1")
	for _, preferSender := range []bool{false, true} {
		got, guessed := ResolveEndpoint("", sender, 51820, preferSender)
		if got != "203.0.113.1:51820" || !guessed {
			t.Errorf("preferSender=%v: ResolveEndpoint(\"\") = %q, %v; want 203.0.113.1:51820, true", preferSender, got, guessed)
		}
	}
	if got, guessed := ResolveEndpoint("", nil, 51820, false); got != "" || guessed {
		t.Errorf("no advertisement and no sender = %q, %v; want empty, false", got, guessed)
	}
}

// The port of an endpoint is the advertised one or the fallback, whatever the
// sender's UDP source port was (ResolveEndpoint takes no sender port; the
// wrappers in pkg/discovery are tested with real sender ports).
func TestNoEndpointFromSenderPort(t *testing.T) {
	t.Parallel()

	sender := net.ParseIP("203.0.113.1")
	tests := []struct {
		advertised string
		wantPort   string
	}{
		{"", "51820"},
		{"garbage", "51820"},
		{"198.51.100.7:", "51820"},
		{":51821", "51821"},
		{"0.0.0.0:51821", "51821"},
		{"198.51.100.7:51821", "51821"},
	}
	for _, tt := range tests {
		for _, preferSender := range []bool{false, true} {
			got, _ := ResolveEndpoint(tt.advertised, sender, 51820, preferSender)
			_, port, err := net.SplitHostPort(got)
			if err != nil {
				t.Fatalf("%q: endpoint %q: %v", tt.advertised, got, err)
			}
			if port != tt.wantPort {
				t.Errorf("advertised %q, preferSender %v: endpoint %q has port %s, want %s", tt.advertised, preferSender, got, port, tt.wantPort)
			}
		}
	}
}

// An advertised endpoint with no port is unusable in both modes: the sender IP
// with the fallback port is a guess, and with no sender there is no endpoint.
func TestResolveEndpointEmptyPortIsUnusable(t *testing.T) {
	t.Parallel()

	sender := net.ParseIP("203.0.113.1")
	tests := []struct {
		name         string
		advertised   string
		sender       net.IP
		preferSender bool
		want         string
		wantGuessed  bool
	}{
		{"host without port", "198.51.100.7:", sender, false, "203.0.113.1:51820", true},
		{"colon only", ":", sender, false, "203.0.113.1:51820", true},
		{"host without port, LAN", "198.51.100.7:", sender, true, "203.0.113.1:51820", true},
		{"colon only, LAN", ":", sender, true, "203.0.113.1:51820", true},
		{"host without port, no sender", "198.51.100.7:", nil, false, "", false},
		{"colon only, no sender", ":", nil, false, "", false},
		{"host without port, no sender, LAN", "198.51.100.7:", nil, true, "", false},
	}
	for _, tt := range tests {
		got, guessed := ResolveEndpoint(tt.advertised, tt.sender, 51820, tt.preferSender)
		if got != tt.want || guessed != tt.wantGuessed {
			t.Errorf("%s: ResolveEndpoint(%q) = %q, %v; want %q, %v", tt.name, tt.advertised, got, guessed, tt.want, tt.wantGuessed)
		}
	}
}
