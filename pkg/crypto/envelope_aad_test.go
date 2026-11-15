package crypto

import (
	"encoding/json"
	"testing"
	"time"
)

var allMessageTypes = []string{
	MessageTypeHello,
	MessageTypeReply,
	MessageTypeAnnounce,
	MessageTypeGoodbye,
	MessageTypeRendezvousOffer,
	MessageTypeRendezvousStart,
}

func aadTestPayload() PeerAnnouncement {
	return PeerAnnouncement{
		Protocol:   ProtocolVersion,
		WGPubKey:   "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		MeshIP:     "10.42.0.7",
		WGEndpoint: "127.0.0.1:51820",
		Timestamp:  1800000000,
	}
}

// editEnvelope decodes the outer envelope JSON, lets edit change it, and
// encodes it again, which is all an on-path attacker without the key can do.
func editEnvelope(t *testing.T, data []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	edit(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
	return out
}

func TestEnvelopeTypeIsAuthenticated(t *testing.T) {
	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer setTestNow(time.Unix(1800000000, 0))()

	for _, sealedAs := range allMessageTypes {
		sealed, err := SealEnvelopeAAD(sealedAs, aadTestPayload(), keys.GossipKey)
		if err != nil {
			t.Fatalf("seal as %s: %v", sealedAs, err)
		}
		if _, _, err := OpenEnvelopeRaw(sealed, keys.GossipKey); err != nil {
			t.Fatalf("untouched %s envelope refused: %v", sealedAs, err)
		}
		for _, claimedAs := range allMessageTypes {
			if claimedAs == sealedAs {
				continue
			}
			retyped := editEnvelope(t, sealed, func(m map[string]any) { m["type"] = claimedAs })
			if _, _, err := OpenEnvelopeRaw(retyped, keys.GossipKey); err == nil {
				t.Errorf("envelope sealed as %s opened after its type was changed to %s", sealedAs, claimedAs)
			}
		}
	}
}

func TestLegacyEnvelopeOpensAndIsMarked(t *testing.T) {
	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer setTestNow(time.Unix(1800000000, 0))()

	legacy, err := SealEnvelope(MessageTypeAnnounce, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	env, _, err := OpenEnvelopeRaw(legacy, keys.GossipKey)
	if err != nil {
		t.Fatalf("legacy envelope refused: %v", err)
	}
	if !env.LegacyAAD {
		t.Error("legacy envelope not marked LegacyAAD")
	}

	bound, err := SealEnvelopeAAD(MessageTypeAnnounce, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	env, _, err = OpenEnvelopeRaw(bound, keys.GossipKey)
	if err != nil {
		t.Fatalf("AAD envelope refused: %v", err)
	}
	if env.LegacyAAD {
		t.Error("AAD envelope marked LegacyAAD")
	}
}

func TestAADEnvelopeCannotBeDowngraded(t *testing.T) {
	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer setTestNow(time.Unix(1800000000, 0))()

	sealed, err := SealEnvelopeAAD(MessageTypeGoodbye, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(map[string]any){
		"v removed":   func(m map[string]any) { delete(m, "v") },
		"v set to 0":  func(m map[string]any) { m["v"] = 0 },
		"v set to 1":  func(m map[string]any) { m["v"] = 1 },
		"v set to 3":  func(m map[string]any) { m["v"] = 3 },
		"v set to -2": func(m map[string]any) { m["v"] = -2 },
	}
	for name, edit := range cases {
		if _, _, err := OpenEnvelopeRaw(editEnvelope(t, sealed, edit), keys.GossipKey); err == nil {
			t.Errorf("%s: AAD-bound envelope opened", name)
		}
	}
}

// An unknown version on an envelope that was sealed WITHOUT additional data
// would open if the decoder fell back to nil additional data for it, so the
// refusal has to be explicit and not only a side effect of a failed tag.
func TestUnknownEnvelopeVersionIsRefusedNotOpenedAsLegacy(t *testing.T) {
	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer setTestNow(time.Unix(1800000000, 0))()

	legacy, err := SealEnvelope(MessageTypeAnnounce, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []int{1, 3, -2} {
		forged := editEnvelope(t, legacy, func(m map[string]any) { m["v"] = v })
		if _, _, err := OpenEnvelopeRaw(forged, keys.GossipKey); err == nil {
			t.Errorf("legacy-sealed envelope with v=%d opened", v)
		}
	}
}

// When legacy envelopes are no longer accepted the AAD-bound form still opens
// and the legacy form is refused (the switch release N+2 flips).
func TestLegacyEnvelopeRefusedWhenNotAccepted(t *testing.T) {
	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer setTestNow(time.Unix(1800000000, 0))()
	old := acceptLegacyAAD
	acceptLegacyAAD = false
	defer func() { acceptLegacyAAD = old }()

	legacy, err := SealEnvelope(MessageTypeAnnounce, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenEnvelopeRaw(legacy, keys.GossipKey); err == nil {
		t.Error("legacy envelope opened although acceptLegacyAAD is false")
	}
	bound, err := SealEnvelopeAAD(MessageTypeAnnounce, aadTestPayload(), keys.GossipKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenEnvelopeRaw(bound, keys.GossipKey); err != nil {
		t.Errorf("AAD envelope refused: %v", err)
	}
}
