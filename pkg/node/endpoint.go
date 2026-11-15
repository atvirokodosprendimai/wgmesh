package node

import (
	"net"
	"strconv"
)

// ResolveEndpoint builds the endpoint of a peer from what it advertised and
// the address its packet came from. Stub for the red commit (ADR-0002 T7): the
// old exchange rule, no guess flag, no preferSender.
func ResolveEndpoint(advertised string, sender net.IP, fallbackPort int, preferSender bool) (endpoint string, guessed bool) {
	if host, port, err := net.SplitHostPort(advertised); err == nil {
		if host == "" || host == "0.0.0.0" || host == "::" {
			if sender != nil {
				host = sender.String()
			}
		}
		if host != "" {
			return net.JoinHostPort(host, port), false
		}
	}
	if sender != nil {
		return net.JoinHostPort(sender.String(), strconv.Itoa(fallbackPort)), false
	}
	return "", false
}
