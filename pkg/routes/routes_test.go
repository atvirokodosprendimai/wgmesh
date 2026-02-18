package routes

import (
	"testing"
)

// --- NormalizeNetwork ---

func TestNormalizeNetwork_IPv4HostRoute(t *testing.T) {
	t.Parallel()
	got := NormalizeNetwork("192.168.5.5")
	if got != "192.168.5.5/32" {
		t.Errorf("NormalizeNetwork(%q) = %q, want %q", "192.168.5.5", got, "192.168.5.5/32")
	}
}

func TestNormalizeNetwork_IPv6HostRoute(t *testing.T) {
	t.Parallel()
	got := NormalizeNetwork("fd00::1")
	if got != "fd00::1/128" {
		t.Errorf("NormalizeNetwork(%q) = %q, want %q", "fd00::1", got, "fd00::1/128")
	}
}

func TestNormalizeNetwork_AlreadyHasPrefix(t *testing.T) {
	t.Parallel()
	tests := []string{"10.0.0.0/8", "192.168.1.0/24", "fd00::/64"}
	for _, cidr := range tests {
		got := NormalizeNetwork(cidr)
		if got != cidr {
			t.Errorf("NormalizeNetwork(%q) = %q, want unchanged", cidr, got)
		}
	}
}

// --- MakeKey ---

func TestMakeKey_Format(t *testing.T) {
	t.Parallel()
	got := MakeKey("10.0.0.0/8", "192.168.1.1")
	want := "10.0.0.0/8|192.168.1.1"
	if got != want {
		t.Errorf("MakeKey() = %q, want %q", got, want)
	}
}

func TestMakeKey_EmptyGateway(t *testing.T) {
	t.Parallel()
	got := MakeKey("10.0.0.0/8", "")
	want := "10.0.0.0/8|"
	if got != want {
		t.Errorf("MakeKey() = %q, want %q", got, want)
	}
}

// --- CalculateDiff ---

func TestCalculateDiff_NoChanges(t *testing.T) {
	t.Parallel()
	routes := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},
	}
	add, remove := CalculateDiff(routes, routes)
	if len(add) != 0 || len(remove) != 0 {
		t.Errorf("expected no changes; add=%v remove=%v", add, remove)
	}
}

func TestCalculateDiff_AddNewRoute(t *testing.T) {
	t.Parallel()
	current := []Entry{}
	desired := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 1 || add[0] != desired[0] {
		t.Errorf("expected 1 add; got %v", add)
	}
	if len(remove) != 0 {
		t.Errorf("expected 0 removes; got %v", remove)
	}
}

func TestCalculateDiff_RemoveStaleRoute(t *testing.T) {
	t.Parallel()
	current := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	desired := []Entry{}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 0 {
		t.Errorf("expected 0 adds; got %v", add)
	}
	if len(remove) != 1 || remove[0] != current[0] {
		t.Errorf("expected 1 remove; got %v", remove)
	}
}

func TestCalculateDiff_GatewayChanged(t *testing.T) {
	t.Parallel()
	current := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.1"}}
	desired := []Entry{{Network: "10.0.0.0/8", Gateway: "192.168.1.2"}}
	add, remove := CalculateDiff(current, desired)

	if len(add) != 1 || add[0] != desired[0] {
		t.Errorf("expected add of new gateway route; got %v", add)
	}
	if len(remove) != 1 || remove[0] != current[0] {
		t.Errorf("expected remove of old gateway route; got %v", remove)
	}
}

func TestCalculateDiff_DirectlyConnectedNotRemoved(t *testing.T) {
	t.Parallel()
	// Routes with empty gateway (directly connected) must not be removed.
	current := []Entry{
		{Network: "10.99.0.0/16", Gateway: ""},      // directly connected
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"}, // managed
	}
	desired := []Entry{}
	add, remove := CalculateDiff(current, desired)
	if len(add) != 0 {
		t.Errorf("expected 0 adds; got %v", add)
	}
	// Only the managed route should be removed.
	if len(remove) != 1 || remove[0].Network != "10.0.0.0/8" {
		t.Errorf("expected only managed route removed; got %v", remove)
	}
}

func TestCalculateDiff_MultipleRoutes(t *testing.T) {
	t.Parallel()
	current := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},
		{Network: "172.16.0.0/12", Gateway: "192.168.1.1"},
	}
	desired := []Entry{
		{Network: "10.0.0.0/8", Gateway: "192.168.1.1"},     // unchanged
		{Network: "192.168.5.5/32", Gateway: "192.168.1.2"}, // new
	}
	add, remove := CalculateDiff(current, desired)

	if len(add) != 1 || add[0].Network != "192.168.5.5/32" {
		t.Errorf("expected 1 add (192.168.5.5/32); got %v", add)
	}
	if len(remove) != 1 || remove[0].Network != "172.16.0.0/12" {
		t.Errorf("expected 1 remove (172.16.0.0/12); got %v", remove)
	}
}

func TestCalculateDiff_Idempotent(t *testing.T) {
	t.Parallel()
	// Calling CalculateDiff twice with the same result applied should be a no-op.
	initial := []Entry{
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"},
	}
	desired := []Entry{
		{Network: "10.0.0.0/8", Gateway: "1.2.3.4"},
		{Network: "172.16.0.0/12", Gateway: "1.2.3.4"},
	}

	add1, remove1 := CalculateDiff(initial, desired)
	// "Apply" the changes to build a new current state.
	afterApply := make([]Entry, len(initial))
	copy(afterApply, initial)
	for _, r := range remove1 {
		for i, e := range afterApply {
			if e == r {
				afterApply = append(afterApply[:i], afterApply[i+1:]...)
				break
			}
		}
	}
	afterApply = append(afterApply, add1...)

	// Second diff should be a no-op.
	add2, remove2 := CalculateDiff(afterApply, desired)
	if len(add2) != 0 || len(remove2) != 0 {
		t.Errorf("second CalculateDiff should be no-op; add=%v remove=%v", add2, remove2)
	}
}
