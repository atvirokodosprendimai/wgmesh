package crypto

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPeerAnnouncementValidate(t *testing.T) {
	// Valid WireGuard public key (32 bytes base64-encoded = 44 chars)
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))

	validAnnouncement := func() *PeerAnnouncement {
		return &PeerAnnouncement{
			Protocol:   ProtocolVersion,
			WGPubKey:   validKey,
			MeshIP:     "10.0.0.1",
			WGEndpoint: "1.2.3.4:51820",
			KnownPeers: nil,
		}
	}

	tests := []struct {
		name        string
		modify      func(pa *PeerAnnouncement)
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid announcement",
			modify: func(pa *PeerAnnouncement) {},
		},
		{
			name: "valid with routable networks",
			modify: func(pa *PeerAnnouncement) {
				pa.RoutableNetworks = []string{"192.168.1.0/24", "10.1.0.0/16"}
			},
		},
		{
			name: "valid with hostname",
			modify: func(pa *PeerAnnouncement) {
				pa.Hostname = "node-1.example.com"
			},
		},
		{
			name: "valid with known peers",
			modify: func(pa *PeerAnnouncement) {
				pa.KnownPeers = []KnownPeer{
					{WGPubKey: validKey, MeshIP: "10.0.0.2", WGEndpoint: "5.6.7.8:51820"},
				}
			},
		},
		// WGPubKey validation
		{
			name:        "empty WGPubKey",
			modify:      func(pa *PeerAnnouncement) { pa.WGPubKey = "" },
			wantErr:     true,
			errContains: "WGPubKey",
		},
		{
			name:        "invalid base64 WGPubKey",
			modify:      func(pa *PeerAnnouncement) { pa.WGPubKey = "not-valid-base64!!!" },
			wantErr:     true,
			errContains: "WGPubKey",
		},
		{
			name: "wrong length WGPubKey (16 bytes instead of 32)",
			modify: func(pa *PeerAnnouncement) {
				pa.WGPubKey = base64.StdEncoding.EncodeToString(make([]byte, 16))
			},
			wantErr:     true,
			errContains: "WGPubKey",
		},
		// MeshIP validation
		{
			name:        "empty MeshIP",
			modify:      func(pa *PeerAnnouncement) { pa.MeshIP = "" },
			wantErr:     true,
			errContains: "MeshIP",
		},
		{
			name:        "invalid MeshIP",
			modify:      func(pa *PeerAnnouncement) { pa.MeshIP = "not-an-ip" },
			wantErr:     true,
			errContains: "MeshIP",
		},
		{
			name:        "MeshIP with CIDR notation",
			modify:      func(pa *PeerAnnouncement) { pa.MeshIP = "10.0.0.1/24" },
			wantErr:     true,
			errContains: "MeshIP",
		},
		{
			name:   "valid IPv6 MeshIP",
			modify: func(pa *PeerAnnouncement) { pa.MeshIP = "2001:db8::1" },
		},
		// WGEndpoint validation
		{
			name:   "empty WGEndpoint is allowed",
			modify: func(pa *PeerAnnouncement) { pa.WGEndpoint = "" },
		},
		{
			name:        "invalid WGEndpoint no port",
			modify:      func(pa *PeerAnnouncement) { pa.WGEndpoint = "1.2.3.4" },
			wantErr:     true,
			errContains: "WGEndpoint",
		},
		{
			name:        "invalid WGEndpoint port zero",
			modify:      func(pa *PeerAnnouncement) { pa.WGEndpoint = "1.2.3.4:0" },
			wantErr:     true,
			errContains: "WGEndpoint",
		},
		{
			name:        "invalid WGEndpoint port too high",
			modify:      func(pa *PeerAnnouncement) { pa.WGEndpoint = "1.2.3.4:70000" },
			wantErr:     true,
			errContains: "WGEndpoint",
		},
		{
			name: "valid IPv6 endpoint",
			modify: func(pa *PeerAnnouncement) {
				pa.WGEndpoint = "[::1]:51820"
			},
		},
		{
			name:   "valid max port 65535",
			modify: func(pa *PeerAnnouncement) { pa.WGEndpoint = "1.2.3.4:65535" },
		},
		{
			name:        "invalid port 65536",
			modify:      func(pa *PeerAnnouncement) { pa.WGEndpoint = "1.2.3.4:65536" },
			wantErr:     true,
			errContains: "WGEndpoint",
		},
		// RoutableNetworks validation
		{
			name: "invalid CIDR in routable networks",
			modify: func(pa *PeerAnnouncement) {
				pa.RoutableNetworks = []string{"192.168.1.0/24", "not-a-cidr"}
			},
			wantErr:     true,
			errContains: "RoutableNetworks",
		},
		{
			name: "IP without mask in routable networks",
			modify: func(pa *PeerAnnouncement) {
				pa.RoutableNetworks = []string{"192.168.1.1"}
			},
			wantErr:     true,
			errContains: "RoutableNetworks",
		},
		{
			name: "too many routable networks",
			modify: func(pa *PeerAnnouncement) {
				pa.RoutableNetworks = make([]string, MaxRoutableNetworks+1)
				for i := range pa.RoutableNetworks {
					pa.RoutableNetworks[i] = fmt.Sprintf("10.%d.%d.0/24", i/256, i%256)
				}
			},
			wantErr:     true,
			errContains: "RoutableNetworks",
		},
		{
			name: "routable networks at max count",
			modify: func(pa *PeerAnnouncement) {
				pa.RoutableNetworks = make([]string, MaxRoutableNetworks)
				for i := range pa.RoutableNetworks {
					pa.RoutableNetworks[i] = fmt.Sprintf("10.%d.%d.0/24", i/256, i%256)
				}
			},
		},
		// Hostname validation (issue #102)
		{
			name: "hostname too long",
			modify: func(pa *PeerAnnouncement) {
				pa.Hostname = strings.Repeat("a", 254)
			},
			wantErr:     true,
			errContains: "hostname",
		},
		{
			name:   "hostname at max length (253)",
			modify: func(pa *PeerAnnouncement) { pa.Hostname = strings.Repeat("a", 253) },
		},
		{
			name: "hostname with control characters",
			modify: func(pa *PeerAnnouncement) {
				pa.Hostname = "node\x00evil"
			},
			wantErr:     true,
			errContains: "hostname",
		},
		{
			name: "hostname with non-printable byte",
			modify: func(pa *PeerAnnouncement) {
				pa.Hostname = "node\x01bad"
			},
			wantErr:     true,
			errContains: "hostname",
		},
		// KnownPeer validation
		{
			name: "known peer with invalid pubkey",
			modify: func(pa *PeerAnnouncement) {
				pa.KnownPeers = []KnownPeer{
					{WGPubKey: "bad", MeshIP: "10.0.0.2", WGEndpoint: "5.6.7.8:51820"},
				}
			},
			wantErr:     true,
			errContains: "KnownPeers[0]",
		},
		{
			name: "known peer with invalid MeshIP",
			modify: func(pa *PeerAnnouncement) {
				pa.KnownPeers = []KnownPeer{
					{WGPubKey: validKey, MeshIP: "bogus", WGEndpoint: "5.6.7.8:51820"},
				}
			},
			wantErr:     true,
			errContains: "KnownPeers[0]",
		},
		{
			name: "known peer with invalid hostname",
			modify: func(pa *PeerAnnouncement) {
				pa.KnownPeers = []KnownPeer{
					{WGPubKey: validKey, MeshIP: "10.0.0.2", Hostname: "bad\x00host"},
				}
			},
			wantErr:     true,
			errContains: "KnownPeers[0]",
		},
		{
			name: "too many known peers",
			modify: func(pa *PeerAnnouncement) {
				pa.KnownPeers = make([]KnownPeer, MaxKnownPeers+1)
				for i := range pa.KnownPeers {
					pa.KnownPeers[i] = KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2"}
				}
			},
			wantErr:     true,
			errContains: "KnownPeers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := validAnnouncement()
			tt.modify(pa)
			err := pa.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestKnownPeerValidate(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))

	tests := []struct {
		name        string
		peer        KnownPeer
		wantErr     bool
		errContains string
	}{
		{
			name: "valid known peer",
			peer: KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2", WGEndpoint: "1.2.3.4:51820"},
		},
		{
			name: "valid known peer without endpoint",
			peer: KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2"},
		},
		{
			name:        "empty pubkey",
			peer:        KnownPeer{WGPubKey: "", MeshIP: "10.0.0.2"},
			wantErr:     true,
			errContains: "WGPubKey",
		},
		{
			name:        "invalid mesh IP",
			peer:        KnownPeer{WGPubKey: validKey, MeshIP: "not-ip"},
			wantErr:     true,
			errContains: "MeshIP",
		},
		{
			name: "valid hostname",
			peer: KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2", Hostname: "node-1.example.com"},
		},
		{
			name:        "hostname too long",
			peer:        KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2", Hostname: strings.Repeat("x", 254)},
			wantErr:     true,
			errContains: "Hostname",
		},
		{
			name:        "hostname with control characters",
			peer:        KnownPeer{WGPubKey: validKey, MeshIP: "10.0.0.2", Hostname: "bad\x00host"},
			wantErr:     true,
			errContains: "Hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.peer.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestOpenEnvelopeValidation(t *testing.T) {
	// Verify that OpenEnvelope calls Validate() and rejects invalid announcements
	keys, err := DeriveKeys("test-secret-for-validation")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	// Create announcement with invalid MeshIP but valid timestamp
	badAnnouncement := &PeerAnnouncement{
		Protocol:   ProtocolVersion,
		WGPubKey:   base64.StdEncoding.EncodeToString(make([]byte, 32)),
		MeshIP:     "not-an-ip",
		WGEndpoint: "1.2.3.4:51820",
		Timestamp:  time.Now().Unix(),
	}

	sealed, err := SealEnvelope(MessageTypeAnnounce, badAnnouncement, keys.GossipKey)
	if err != nil {
		t.Fatalf("SealEnvelope: %v", err)
	}

	_, _, err = OpenEnvelope(sealed, keys.GossipKey)
	if err == nil {
		t.Fatal("expected OpenEnvelope to reject invalid announcement, got nil error")
	}
	if !strings.Contains(err.Error(), "MeshIP") {
		t.Errorf("error %q should mention MeshIP", err.Error())
	}
}
