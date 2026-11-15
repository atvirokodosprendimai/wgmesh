package mesh

import (
	"fmt"
	"net"
	"strconv"
)

// endpointDecision is what the deploy records for one node: the endpoint its
// peers dial (empty for none), whether the node is behind NAT, and the reason
// the operator reads in the deploy output.
type endpointDecision struct {
	Endpoint  string
	BehindNAT bool
	Reason    string
}

// hostIsPrivate is the classifier detectEndpoints uses; a test replaces it so a
// server on 127.0.0.1 can stand for a LAN host.
var hostIsPrivate = isPrivateHost

// isPrivateHost reports whether host is a private IP address (RFC 1918 or
// RFC 4193). A DNS name is never private: the resolver is not consulted.
// Loopback, link-local and carrier-grade NAT addresses are not private either.
func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

// resolveEndpoint decides a node's endpoint (ADR-0006, decision 4). It is pure:
// no I/O, no lock, and it does not write the node.
//
// allPrivate is true when every node's SSH host (this node's included) is a
// private IP address. publicIP is what the node reported as its public IP and
// probed says whether a probe was made; a private host is never probed, so a
// probe with an empty publicIP is a probe that failed.
//
// Order: (a) an endpoint already in the state wins; (b) an all-private mesh
// uses the SSH host; (c) a public IP equal to the SSH host uses the SSH host;
// (d) otherwise there is no endpoint and the node is behind NAT.
func resolveEndpoint(node *Node, allPrivate bool, publicIP string, probed bool) endpointDecision {
	hostPort := net.JoinHostPort(node.SSHHost, strconv.Itoa(node.ListenPort))
	switch {
	case node.PublicEndpoint != "":
		return endpointDecision{Endpoint: node.PublicEndpoint, Reason: "public_endpoint is set in the state"}
	case allPrivate:
		return endpointDecision{Endpoint: hostPort, Reason: "all SSH hosts are private IP addresses"}
	case probed && publicIP != "" && publicIP == node.SSHHost:
		return endpointDecision{Endpoint: hostPort, Reason: "public IP equals the SSH host"}
	case !probed:
		return endpointDecision{BehindNAT: true, Reason: "private SSH host in a mesh with other hosts; set public_endpoint to give it an endpoint"}
	case publicIP == "":
		return endpointDecision{BehindNAT: true, Reason: "public IP could not be detected; set public_endpoint to give it an endpoint"}
	default:
		return endpointDecision{BehindNAT: true, Reason: fmt.Sprintf("behind NAT (public IP %s differs from the SSH host)", publicIP)}
	}
}
