// Package routes provides shared route management types and helpers used by
// both the ssh and daemon packages.  Platform-specific operations (reading the
// kernel routing table, applying route commands) remain in their respective
// packages; only the pure logic is shared here.
package routes

import (
	"fmt"
	"strings"
)

// Entry represents a single kernel routing table entry.
type Entry struct {
	Network string // CIDR, e.g. "10.0.0.0/8" or "192.168.5.5/32"
	Gateway string // Next-hop IP, empty for directly-connected routes
}

// NormalizeNetwork normalizes a network string returned by the kernel.
// Linux's `ip route` displays host routes without a prefix length
// (e.g. "192.168.5.5" instead of "192.168.5.5/32").  This function adds
// the appropriate suffix so that kernel output can be compared to user-
// supplied CIDR strings.
func NormalizeNetwork(network string) string {
	if !strings.Contains(network, "/") {
		if strings.Count(network, ".") == 3 {
			// IPv4 host route
			return network + "/32"
		}
		if strings.Contains(network, ":") {
			// IPv6 host route
			return network + "/128"
		}
	}
	return network
}

// MakeKey returns a string that uniquely identifies a route by both its
// network destination and its gateway.
func MakeKey(network, gateway string) string {
	return fmt.Sprintf("%s|%s", network, gateway)
}

// CalculateDiff compares current and desired route sets and returns the
// minimal set of routes to add and remove.
//
// Rules:
//   - If a desired route already exists exactly (same network + gateway) it
//     is skipped.
//   - If a desired network exists with a *different* gateway, the old route is
//     queued for removal before the new one is added.
//   - If a current network is no longer in the desired set at all, and it has
//     a non-empty gateway (i.e. it is a managed mesh route), it is removed.
//   - Directly-connected routes (empty gateway) are never removed.
func CalculateDiff(current, desired []Entry) (toAdd, toRemove []Entry) {
	currentMap := make(map[string]Entry)       // "network|gateway" -> entry
	desiredMap := make(map[string]Entry)       // "network|gateway" -> entry
	currentByNetwork := make(map[string]Entry) // "network" -> entry
	desiredByNetwork := make(map[string]Entry) // "network" -> entry

	for _, r := range current {
		key := MakeKey(r.Network, r.Gateway)
		currentMap[key] = r
		currentByNetwork[r.Network] = r
	}

	for _, r := range desired {
		key := MakeKey(r.Network, r.Gateway)
		desiredMap[key] = r
		desiredByNetwork[r.Network] = r
	}

	// Determine routes to add (and any prerequisite removals for gateway changes).
	for key, route := range desiredMap {
		if _, exists := currentMap[key]; !exists {
			// Route with this exact network+gateway doesn't exist yet.
			if currentRoute, networkExists := currentByNetwork[route.Network]; networkExists {
				if currentRoute.Gateway != route.Gateway && currentRoute.Gateway != "" {
					// Gateway changed — must remove old route first.
					toRemove = append(toRemove, currentRoute)
				}
			}
			toAdd = append(toAdd, route)
		}
		// else: exact route already present — no action needed.
	}

	// Determine routes to remove (network no longer in desired state at all).
	for key, route := range currentMap {
		if _, exactMatch := desiredMap[key]; !exactMatch {
			if _, stillNeeded := desiredByNetwork[route.Network]; !stillNeeded {
				// Network completely absent from desired state.
				// Only remove managed (gateway-routed) routes.
				if route.Gateway != "" {
					toRemove = append(toRemove, route)
				}
			}
			// else: network still needed with a different gateway — already
			// handled in the loop above.
		}
	}

	return toAdd, toRemove
}
