package crypto

import (
	"net"
	"testing"
)

func TestMeshIPInSubnet(t *testing.T) {
	t.Parallel()

	_, customNet, err := net.ParseCIDR("192.168.100.0/24")
	if err != nil {
		t.Fatalf("failed to parse custom subnet: %v", err)
	}

	tests := []struct {
		name         string
		meshIP       string
		meshSubnet   [2]byte
		customSubnet *net.IPNet
		want         bool
	}{
		{
			name:       "legacy subnet match",
			meshIP:     "10.42.7.33",
			meshSubnet: [2]byte{42, 0},
			want:       true,
		},
		{
			name:       "legacy subnet mismatch",
			meshIP:     "10.99.1.1",
			meshSubnet: [2]byte{42, 0},
			want:       false,
		},
		{
			name:         "custom subnet match",
			meshIP:       "192.168.100.55",
			customSubnet: customNet,
			want:         true,
		},
		{
			name:         "custom subnet mismatch",
			meshIP:       "10.42.7.33",
			customSubnet: customNet,
			want:         false,
		},
		{
			name:       "invalid IP",
			meshIP:     "not-an-ip",
			meshSubnet: [2]byte{42, 0},
			want:       false,
		},
		{
			name:       "empty IP",
			meshIP:     "",
			meshSubnet: [2]byte{42, 0},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MeshIPInSubnet(tt.meshIP, tt.meshSubnet, tt.customSubnet)
			if got != tt.want {
				t.Errorf("MeshIPInSubnet(%q) = %v, want %v", tt.meshIP, got, tt.want)
			}
		})
	}
}
