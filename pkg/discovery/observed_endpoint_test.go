package discovery

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func observedNode(t *testing.T, ep string) (*PeerExchange, *daemon.LocalNode) {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-observed-endpoint-1"})
	if err != nil {
		t.Fatal(err)
	}
	ln := daemon.NewLocalNode("local-pubkey", "", "", "10.0.0.1", "")
	ln.SetEndpoint(ep)
	return NewPeerExchange(cfg, ln, daemon.NewPeerStore()), ln
}

func TestApplyObservedEndpoint_IgnoresUnspecified(t *testing.T) {
	const current = "203.0.113.10:51820"
	for _, observed := range []string{"0.0.0.0:40000", "[::]:40000"} {
		pe, ln := observedNode(t, current)
		pe.applyObservedEndpoint(observed)
		if got := ln.GetEndpoint(); got != current {
			t.Errorf("after reflected %s endpoint = %q, want unchanged %q", observed, got, current)
		}
	}
}

func TestApplyObservedEndpoint_PublicReflectionAppliesAtOnce(t *testing.T) {
	pe, ln := observedNode(t, "203.0.113.10:51820")
	pe.applyObservedEndpoint("198.51.100.20:40000")
	if got, want := ln.GetEndpoint(), "198.51.100.20:51820"; got != want {
		t.Fatalf("endpoint = %q, want %q (observed IP, own WG port)", got, want)
	}
}
