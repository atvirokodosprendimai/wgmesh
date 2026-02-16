package crypto

import (
	"testing"
)

func TestCreateAnnouncementIncludesHostname(t *testing.T) {
	ann := CreateAnnouncement("pubkey", "10.0.0.1", "1.2.3.4:51820", "node-alpha", nil, nil)
	if ann.Hostname != "node-alpha" {
		t.Errorf("expected hostname node-alpha, got %q", ann.Hostname)
	}
	if ann.Protocol != ProtocolVersion {
		t.Errorf("expected protocol %s, got %s", ProtocolVersion, ann.Protocol)
	}
}

func TestCreateAnnouncementEmptyHostname(t *testing.T) {
	ann := CreateAnnouncement("pubkey", "10.0.0.1", "1.2.3.4:51820", "", nil, nil)
	if ann.Hostname != "" {
		t.Errorf("expected empty hostname, got %q", ann.Hostname)
	}
}

func TestEnvelopeRoundTripPreservesHostname(t *testing.T) {
	// Derive a test key
	var gossipKey [32]byte
	copy(gossipKey[:], "test-key-must-be-32-bytes-long!!")

	ann := CreateAnnouncement("pubkey", "10.0.0.1", "1.2.3.4:51820", "node-bravo",
		[]string{"192.168.0.0/24"},
		[]KnownPeer{{WGPubKey: "peer2", MeshIP: "10.0.0.2", WGEndpoint: "5.6.7.8:51820", Hostname: "node-charlie"}},
	)

	// Seal
	data, err := SealEnvelope(MessageTypeHello, ann, gossipKey)
	if err != nil {
		t.Fatalf("SealEnvelope failed: %v", err)
	}

	// Open
	_, got, err := OpenEnvelope(data, gossipKey)
	if err != nil {
		t.Fatalf("OpenEnvelope failed: %v", err)
	}

	if got.Hostname != "node-bravo" {
		t.Errorf("hostname not preserved: expected node-bravo, got %q", got.Hostname)
	}
	if got.WGPubKey != "pubkey" {
		t.Errorf("pubkey not preserved: expected pubkey, got %q", got.WGPubKey)
	}
	if len(got.KnownPeers) != 1 {
		t.Fatalf("expected 1 known peer, got %d", len(got.KnownPeers))
	}
	if got.KnownPeers[0].Hostname != "node-charlie" {
		t.Errorf("known peer hostname not preserved: expected node-charlie, got %q", got.KnownPeers[0].Hostname)
	}
}

func TestEnvelopeRoundTripWithoutHostname(t *testing.T) {
	// Verify backward compatibility — announcements without hostname still work
	var gossipKey [32]byte
	copy(gossipKey[:], "test-key-must-be-32-bytes-long!!")

	ann := CreateAnnouncement("pubkey", "10.0.0.1", "1.2.3.4:51820", "", nil, nil)

	data, err := SealEnvelope(MessageTypeAnnounce, ann, gossipKey)
	if err != nil {
		t.Fatalf("SealEnvelope failed: %v", err)
	}

	_, got, err := OpenEnvelope(data, gossipKey)
	if err != nil {
		t.Fatalf("OpenEnvelope failed: %v", err)
	}

	if got.Hostname != "" {
		t.Errorf("expected empty hostname, got %q", got.Hostname)
	}
}
