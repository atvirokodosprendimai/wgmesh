package crypto

import "net"

// MeshIPInSubnet reports whether meshIP falls within the mesh subnet implied
// by meshSubnet/customSubnet. When customSubnet is non-nil it defines the
// address space; otherwise the legacy derivation space 10.<meshSubnet[0]>.0.0/16
// applies. Used to decide whether a persisted mesh IP is still valid after an
// operator changed --mesh-subnet.
func MeshIPInSubnet(meshIP string, meshSubnet [2]byte, customSubnet *net.IPNet) bool {
	ip := net.ParseIP(meshIP)
	if ip == nil {
		return false
	}
	if customSubnet != nil {
		return customSubnet.Contains(ip)
	}
	// Legacy derivation: 10.<meshSubnet[0]>.x.y — check the /16 prefix only.
	subnet := &net.IPNet{
		IP:   net.IP{10, meshSubnet[0], 0, 0},
		Mask: net.CIDRMask(16, 32),
	}
	return subnet.Contains(ip)
}
