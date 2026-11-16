package daemon

import "testing"

const (
	epOld  = "203.0.113.10:51820"
	epNew  = "198.51.100.20:51820"
	epV6   = "[2001:db8::1]:51820"
	epNone = "0.0.0.0:51820"
)

func nodeWith(ep string) *LocalNode {
	n := &LocalNode{}
	n.SetEndpoint(ep)
	return n
}

func TestEndpointDoesNotFlapOnAlternatingObservations(t *testing.T) {
	n := nodeWith(epOld)
	changes := 0
	for i := 0; i < 10; i++ {
		if n.ObserveEndpoint(EndpointSTUN, epNew) {
			changes++
		}
		if n.ObserveEndpoint(EndpointReflection, epOld) {
			changes++
		}
	}
	if changes != 0 {
		t.Fatalf("endpoint changed %d times in 20 alternating observations, want 0", changes)
	}
}

func TestStunNeedsThreeConsecutiveObservations(t *testing.T) {
	n := nodeWith(epOld)
	for i := 1; i <= 2; i++ {
		if n.ObserveEndpoint(EndpointSTUN, epNew) {
			t.Fatalf("observation %d applied, want it pending", i)
		}
		if got := n.GetEndpoint(); got != epOld {
			t.Fatalf("after observation %d endpoint = %s, want %s", i, got, epOld)
		}
	}
	if !n.ObserveEndpoint(EndpointSTUN, epNew) {
		t.Fatal("third identical observation did not apply")
	}
	if got := n.GetEndpoint(); got != epNew {
		t.Fatalf("endpoint = %s, want %s", got, epNew)
	}

	// consecutive: B, B, C, B must not apply; then B, B, B does
	m := nodeWith(epOld)
	for i, ep := range []string{epNew, epNew, epV6, epNew} {
		if m.ObserveEndpoint(EndpointSTUN, ep) {
			t.Fatalf("interleaved observation %d (%s) applied, want it pending", i+1, ep)
		}
	}
	if got := m.GetEndpoint(); got != epOld {
		t.Fatalf("endpoint = %s, want %s", got, epOld)
	}
	m.ObserveEndpoint(EndpointSTUN, epNew)
	if !m.ObserveEndpoint(EndpointSTUN, epNew) {
		t.Fatal("three identical observations in a row did not apply")
	}
}

func TestStunEqualToCurrentDiscardsPending(t *testing.T) {
	n := nodeWith(epOld)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	n.ObserveEndpoint(EndpointSTUN, epOld)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	if n.ObserveEndpoint(EndpointSTUN, epNew) {
		t.Fatal("pending STUN count survived a STUN equal to the current endpoint")
	}
}

func TestInterfaceDiscardsPendingStun(t *testing.T) {
	n := nodeWith(epOld)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	if !n.ObserveEndpoint(EndpointInterface, epV6) {
		t.Fatal("interface observation did not apply at once")
	}
	n.ObserveEndpoint(EndpointSTUN, epNew)
	if n.ObserveEndpoint(EndpointSTUN, epNew) {
		t.Fatal("pending STUN count survived an interface observation")
	}
	if got := n.GetEndpoint(); got != epV6 {
		t.Fatalf("endpoint = %s, want %s", got, epV6)
	}
}

func TestReflectionAppliesAtOnce(t *testing.T) {
	n := nodeWith(epOld)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	n.ObserveEndpoint(EndpointSTUN, epNew)
	if !n.ObserveEndpoint(EndpointReflection, "192.0.2.30:51820") {
		t.Fatal("reflection did not apply at once")
	}
	if got := n.GetEndpoint(); got != "192.0.2.30:51820" {
		t.Fatalf("endpoint = %s", got)
	}
	// the two pending STUN observations were discarded: one more is not a third
	if n.ObserveEndpoint(EndpointSTUN, epNew) {
		t.Fatal("pending STUN count survived a reflection")
	}
}

func TestInterfaceSourceAppliesAtOnce(t *testing.T) {
	n := nodeWith(epOld)
	if !n.ObserveEndpoint(EndpointInterface, epV6) {
		t.Fatal("interface observation did not apply at once")
	}
	if got := n.GetEndpoint(); got != epV6 {
		t.Fatalf("endpoint = %s, want %s", got, epV6)
	}
}

func TestFirstStunAppliesOverUnspecified(t *testing.T) {
	for _, start := range []string{epNone, "", "[::]:51820"} {
		n := nodeWith(start)
		if !n.ObserveEndpoint(EndpointSTUN, epNew) {
			t.Fatalf("first STUN over %q did not apply", start)
		}
		if got := n.GetEndpoint(); got != epNew {
			t.Fatalf("endpoint = %s, want %s", got, epNew)
		}
	}
}
