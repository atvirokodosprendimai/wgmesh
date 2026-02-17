//go:build integration

package discovery

import "testing"

// TestSTUNQueryIntegration tests against a real STUN server.
// Run with: go test -tags=integration -run TestSTUNQueryIntegration ./pkg/discovery/...
func TestSTUNQueryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ip, port, err := STUNQuery("stun.l.google.com:19302", 0, 3000)
	if err != nil {
		t.Fatalf("STUNQuery: %v", err)
	}

	if ip == nil {
		t.Fatal("got nil IP")
	}
	if port == 0 {
		t.Fatal("got port 0")
	}

	t.Logf("External endpoint: %s:%d", ip, port)

	// Sanity: expect a public global unicast IP
	if !ip.IsGlobalUnicast() {
		t.Errorf("got non-public IP: %v", ip)
	}
}
