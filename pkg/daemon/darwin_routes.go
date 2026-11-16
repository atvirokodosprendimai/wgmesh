package daemon

import (
	"net/netip"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// Stubs of ADR-0003 T6: they compile and do nothing, so the tests fail on
// their assertions.

// parseNetstatDarwin is a stub.
func parseNetstatDarwin(out []byte, iface string, ipv6 bool) ([]routes.Entry, error) {
	return nil, nil
}

// planRoutesDarwin is a stub.
func planRoutesDarwin(want []routes.Entry, k kernelObserved, keep []netip.Prefix) routePlan {
	return routePlan{}
}
