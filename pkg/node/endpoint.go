package node

import (
	"net"
	"strconv"
)

// ResolveEndpoint builds the WireGuard endpoint of a peer from what it
// advertised and the IP its packet came from. It is the one place for the
// rules (ADR-0002 R6, #839):
//
//   - an advertised real host keeps its advertised host and port;
//   - an unspecified or empty advertised host takes the sender IP and keeps
//     the advertised port;
//   - with no usable advertisement the endpoint is the sender IP with
//     fallbackPort and guessed is true: a guess ranks below every
//     advertisement in the store and is never re-announced;
//   - preferSender (LAN) takes the sender IP over an advertised real host.
//
// The sender's UDP source port is never an input: it is not the WireGuard
// port. fallbackPort is passed in (the daemon's DefaultWGPort) because this
// package does not import pkg/daemon. A nil sender with nothing usable
// advertised gives an empty endpoint.
func ResolveEndpoint(advertised string, sender net.IP, fallbackPort int, preferSender bool) (endpoint string, guessed bool) {
	host, port, err := net.SplitHostPort(advertised)
	advertisedOK := err == nil

	if preferSender {
		if sender != nil {
			if advertisedOK && port != "" {
				return net.JoinHostPort(sender.String(), port), false
			}
			return net.JoinHostPort(sender.String(), strconv.Itoa(fallbackPort)), true
		}
		if advertisedOK {
			return advertised, false
		}
		return "", false
	}

	if advertisedOK {
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
		return net.JoinHostPort(sender.String(), strconv.Itoa(fallbackPort)), true
	}
	return "", false
}
