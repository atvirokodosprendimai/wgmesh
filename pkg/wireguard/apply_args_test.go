package wireguard

import (
	"reflect"
	"testing"
)

func TestSetPeerArgsClearsAllowedIPs(t *testing.T) {
	got := SetPeerArgs("wg0", "PUBKEY", false, "203.0.113.5:51820", "-")
	want := []string{"set", "wg0", "peer", "PUBKEY", "endpoint", "203.0.113.5:51820", "allowed-ips", "", "persistent-keepalive", "25"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetPeerArgs with \"-\" = %q, want %q (allowed-ips followed by an empty argument)", got, want)
	}
}

func TestSetPeerArgsUnchangedForExistingCallers(t *testing.T) {
	tests := []struct {
		name       string
		hasPSK     bool
		endpoint   string
		allowedIPs string
		want       []string
	}{
		{
			name: "endpoint and list", endpoint: "1.2.3.4:51820", allowedIPs: "10.0.0.2/32,10.0.1.0/24",
			want: []string{"set", "wg0", "peer", "PUBKEY", "endpoint", "1.2.3.4:51820", "allowed-ips", "10.0.0.2/32,10.0.1.0/24", "persistent-keepalive", "25"},
		},
		{
			name: "empty list omits the flag", endpoint: "1.2.3.4:51820", allowedIPs: "",
			want: []string{"set", "wg0", "peer", "PUBKEY", "endpoint", "1.2.3.4:51820", "persistent-keepalive", "25"},
		},
		{
			name: "no endpoint", allowedIPs: "10.0.0.2/32",
			want: []string{"set", "wg0", "peer", "PUBKEY", "allowed-ips", "10.0.0.2/32", "persistent-keepalive", "25"},
		},
		{
			name: "preshared key", hasPSK: true, endpoint: "1.2.3.4:51820", allowedIPs: "10.0.0.2/32",
			want: []string{"set", "wg0", "peer", "PUBKEY", "preshared-key", "/dev/stdin", "endpoint", "1.2.3.4:51820", "allowed-ips", "10.0.0.2/32", "persistent-keepalive", "25"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SetPeerArgs("wg0", "PUBKEY", tt.hasPSK, tt.endpoint, tt.allowedIPs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SetPeerArgs = %q, want %q", got, tt.want)
			}
			if last := got[len(got)-2:]; len(got) >= 2 && !reflect.DeepEqual(last, []string{"persistent-keepalive", "25"}) {
				t.Fatalf("keepalive is not last: %q", got)
			}
		})
	}
}
