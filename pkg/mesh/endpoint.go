package mesh

import "fmt"

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

// isPrivateHost is a stub (ADR-0006 T5, step S1).
func isPrivateHost(host string) bool { return false }

// resolveEndpoint is a stub that reproduces the rule of b8b83f3 (ADR-0006 T5,
// step S1).
func resolveEndpoint(node *Node, allPrivate bool, publicIP string, probed bool) endpointDecision {
	if probed && publicIP != "" && publicIP == node.SSHHost {
		return endpointDecision{Endpoint: fmt.Sprintf("%s:%d", node.SSHHost, node.ListenPort), Reason: "stub"}
	}
	return endpointDecision{BehindNAT: true, Reason: "stub"}
}
