package lighthouse

import (
	"net"
	"strings"
)

// Resolver abstracts DNS lookups so tests can inject a mock.
type Resolver interface {
	LookupCNAME(host string) (string, error)
	LookupHost(host string) ([]string, error)
}

// NetResolver is the production Resolver using the standard library.
type NetResolver struct{}

func (NetResolver) LookupCNAME(host string) (string, error) {
	return net.LookupCNAME(host)
}

func (NetResolver) LookupHost(host string) ([]string, error) {
	return net.LookupHost(host)
}

// VerifyDNS checks whether domain's DNS points to expectedCNAME (suffix match)
// or resolves to one of expectedIPs. Returns true when DNS is correctly configured.
func VerifyDNS(r Resolver, domain, expectedCNAME string, expectedIPs []string) (bool, error) {
	// Check CNAME first
	cname, err := r.LookupCNAME(domain)
	if err == nil {
		// net.LookupCNAME always returns a fully-qualified name ending with "."
		cname = strings.TrimSuffix(cname, ".")
		if strings.EqualFold(cname, strings.TrimSuffix(expectedCNAME, ".")) {
			return true, nil
		}
	}

	if len(expectedIPs) == 0 {
		return false, nil
	}

	// Check A/AAAA records
	addrs, err := r.LookupHost(domain)
	if err != nil {
		return false, err
	}
	for _, addr := range addrs {
		for _, expected := range expectedIPs {
			if addr == expected {
				return true, nil
			}
		}
	}
	return false, nil
}
