package daemon

import "github.com/atvirokodosprendimai/wgmesh/pkg/ifname"

// ValidateInterfaceName delegates to ifname.Validate.
// Kept here for backward compatibility within the daemon package.
func ValidateInterfaceName(name string) error {
	return ifname.Validate(name)
}
