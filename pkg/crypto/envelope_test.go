package crypto

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
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

// One invalid KnownPeers entry must not hide the message: the sender is intact
// and only the bad entry is gone (#875 item 2).
func TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage(t *testing.T) {
	keys, err := DeriveKeys("test-secret-for-filter")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	goodKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	ann := &PeerAnnouncement{
		Protocol:  ProtocolVersion,
		WGPubKey:  goodKey,
		MeshIP:    "10.0.0.2",
		Timestamp: time.Now().Unix(),
		KnownPeers: []KnownPeer{
			{WGPubKey: "not-a-key", MeshIP: "10.0.0.3"},
			{WGPubKey: goodKey, MeshIP: "10.0.0.4", WGEndpoint: "203.0.113.4:51820"},
		},
	}
	sealed, err := SealEnvelope(MessageTypeAnnounce, ann, keys.GossipKey)
	if err != nil {
		t.Fatalf("SealEnvelope: %v", err)
	}
	_, got, err := OpenEnvelope(sealed, keys.GossipKey)
	if err != nil {
		t.Fatalf("OpenEnvelope rejected the whole message for one bad known peer: %v", err)
	}
	if got.WGPubKey != goodKey || got.MeshIP != "10.0.0.2" {
		t.Errorf("sender changed: %q %q", got.WGPubKey, got.MeshIP)
	}
	if len(got.KnownPeers) != 1 || got.KnownPeers[0].MeshIP != "10.0.0.4" {
		t.Errorf("KnownPeers = %+v, want only the valid entry", got.KnownPeers)
	}
}

// The sender's own fields stay strict: an invalid mesh IP or key fails to open.
func TestOpenEnvelopeRejectsInvalidSender(t *testing.T) {
	keys, err := DeriveKeys("test-secret-for-filter")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	goodKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	tests := []struct {
		name string
		ann  PeerAnnouncement
	}{
		{"bad mesh ip", PeerAnnouncement{WGPubKey: goodKey, MeshIP: "10.0.0.2/16"}},
		{"bad key", PeerAnnouncement{WGPubKey: "garbage", MeshIP: "10.0.0.2"}},
		{"too many known peers", PeerAnnouncement{WGPubKey: goodKey, MeshIP: "10.0.0.2", KnownPeers: make([]KnownPeer, MaxKnownPeers+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ann := tt.ann
			ann.Protocol = ProtocolVersion
			ann.Timestamp = time.Now().Unix()
			sealed, err := SealEnvelope(MessageTypeAnnounce, &ann, keys.GossipKey)
			if err != nil {
				t.Fatalf("SealEnvelope: %v", err)
			}
			if _, _, err := OpenEnvelope(sealed, keys.GossipKey); err == nil {
				t.Fatal("expected OpenEnvelope to reject the message")
			}
		})
	}
}

// A failed call must leave the caller's announcement as it was received.
func TestFilterAnnouncementErrorKeepsKnownPeers(t *testing.T) {
	goodKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	ann := &PeerAnnouncement{
		WGPubKey: goodKey,
		MeshIP:   "not-an-ip",
		KnownPeers: []KnownPeer{
			{WGPubKey: goodKey, MeshIP: "10.0.0.3"},
			{WGPubKey: goodKey, MeshIP: "10.0.0.4"},
		},
	}
	if err := FilterAnnouncement(ann); err == nil {
		t.Fatal("expected an error for the invalid mesh IP")
	}
	if len(ann.KnownPeers) != 2 {
		t.Errorf("len(KnownPeers) = %d after a failed call, want 2", len(ann.KnownPeers))
	}
}

// slogCapture collects the records of the default logger for one test.
type slogCapture struct{ records []slog.Record }

func (c *slogCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *slogCapture) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}
func (c *slogCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *slogCapture) WithGroup(string) slog.Handler      { return c }

// A message that lost entries says so once at Warn, with counts and no keys.
// Not parallel: it swaps the default logger.
func TestFilterAnnouncementWarnsOncePerMessage(t *testing.T) {
	capture := &slogCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(prev) })

	goodKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	ann := &PeerAnnouncement{
		WGPubKey: goodKey,
		MeshIP:   "10.0.0.2",
		KnownPeers: []KnownPeer{
			{WGPubKey: "bad-1", MeshIP: "10.0.0.3"},
			{WGPubKey: goodKey, MeshIP: "10.0.0.4"},
			{WGPubKey: "bad-2", MeshIP: "10.0.0.5"},
		},
	}
	if err := FilterAnnouncement(ann); err != nil {
		t.Fatalf("FilterAnnouncement: %v", err)
	}
	var warns []slog.Record
	for _, r := range capture.records {
		if r.Level == slog.LevelWarn {
			warns = append(warns, r)
		}
	}
	if len(warns) != 1 {
		t.Fatalf("%d Warn records, want exactly 1", len(warns))
	}
	if warns[0].Message != "[crypto] dropped invalid known peers" {
		t.Errorf("Warn message = %q", warns[0].Message)
	}
	got := map[string]int64{}
	warns[0].Attrs(func(a slog.Attr) bool {
		if a.Value.Kind() == slog.KindInt64 {
			got[a.Key] = a.Value.Int64()
		}
		return true
	})
	if got["dropped"] != 2 || got["kept"] != 1 {
		t.Errorf("Warn attrs = %v, want dropped=2 kept=1", got)
	}

	// A clean message produces no Warn.
	capture.records = nil
	clean := &PeerAnnouncement{WGPubKey: goodKey, MeshIP: "10.0.0.2", KnownPeers: []KnownPeer{{WGPubKey: goodKey, MeshIP: "10.0.0.4"}}}
	if err := FilterAnnouncement(clean); err != nil {
		t.Fatal(err)
	}
	for _, r := range capture.records {
		if r.Level == slog.LevelWarn {
			t.Errorf("Warn for a message that dropped nothing: %q", r.Message)
		}
	}
}
