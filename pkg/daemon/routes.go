package daemon

import (
	"log"
	"net"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// restoreConnectedRoute re-adds the kernel connected route for an own network
// whose `<network> via <peer> dev <mesh iface>` route was just removed: older
// daemons installed that route with `ip route replace`, overwriting it.
// Best effort; without a local address in network there is nothing to restore.
func restoreConnectedRoute(network string) {
	_, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		return
	}
	family := "-4"
	if ipnet.IP.To4() == nil {
		family = "-6"
	}
	out, err := cmdExecutor.Command("ip", family, "-o", "addr", "show", "to", ipnet.String()).Output()
	const hint = "if it was reachable other than via the mesh interface, restore that route manually"
	if err != nil {
		log.Printf("Warning: could not query local addresses in %s: %v; %s", ipnet, err, hint)
		return
	}
	iface, src, ok := routes.ParseLocalAddr(string(out), ipnet)
	if !ok {
		log.Printf("Warning: no local address in %s; %s", ipnet, hint)
		return
	}
	args := strings.Fields(routes.ConnectedRouteCmd(ipnet, iface, src))
	if out, err := cmdExecutor.Command(args[0], args[1:]...).CombinedOutput(); err != nil && !strings.Contains(string(out), "File exists") {
		log.Printf("Failed to restore connected route %s dev %s: %s: %v", ipnet, iface, strings.TrimSpace(string(out)), err)
		return
	}
	log.Printf("Restored connected route: %s dev %s", ipnet, iface)
}

func (d *Daemon) currentRelayRoutesSnapshot() map[string]string {
	d.relayMu.RLock()
	defer d.relayMu.RUnlock()
	out := make(map[string]string, len(d.relayRoutes))
	for k, v := range d.relayRoutes {
		out[k] = v
	}
	return out
}

func (d *Daemon) directStableCyclesSnapshot() map[string]int {
	d.relayMu.RLock()
	defer d.relayMu.RUnlock()
	out := make(map[string]int, len(d.directStableCycles))
	for k, v := range d.directStableCycles {
		out[k] = v
	}
	return out
}
