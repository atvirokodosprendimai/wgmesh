package wireguard

import "testing"

func TestDerivePublicKey(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Skipf("wg binary not available: %v", err)
	}
	derived, err := DerivePublicKey(priv)
	if err != nil {
		t.Fatalf("DerivePublicKey() error: %v", err)
	}
	if derived != pub {
		t.Errorf("DerivePublicKey() = %q, want %q", derived, pub)
	}
}
