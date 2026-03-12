// Package ifname provides validation for network interface names.
// It is intentionally dependency-free to avoid import cycles between
// packages like daemon and wireguard that both need validation.
package ifname

import (
	"fmt"
	"regexp"
	"runtime"
)

const (
	// MaxLen is the Linux kernel IFNAMSIZ (16) minus the null terminator.
	MaxLen = 15
)

var (
	// linuxRegex allows names starting with a letter, followed by alphanumeric, underscore, or hyphen.
	linuxRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

	// darwinRegex requires the utun<N> pattern used by wireguard-go on macOS.
	darwinRegex = regexp.MustCompile(`^utun[0-9]+$`)
)

// Validate checks that name is a safe, OS-appropriate WireGuard interface
// name. An empty string is valid (means "use platform default").
//
// On Linux: must match ^[a-zA-Z][a-zA-Z0-9_-]*$ and be at most 15 characters.
// On macOS: must match ^utun[0-9]+$ (wireguard-go requirement).
//
// The function also rejects path-traversal sequences and shell metacharacters
// as defense-in-depth, since interface names appear in file paths and — in the
// centralized deploy path — inside SSH shell commands.
func Validate(name string) error {
	if name == "" {
		return nil // empty means "use default"
	}

	// Reject null bytes (would truncate C strings / file paths).
	for i := range name {
		if name[i] == 0 {
			return fmt.Errorf("interface name must not contain null bytes")
		}
	}

	// Reject path separators and traversal (name is used in file paths).
	for _, ch := range name {
		if ch == '/' || ch == '\\' {
			return fmt.Errorf("interface name must not contain path separators ('/' or '\\')")
		}
	}
	if name == "." || name == ".." {
		return fmt.Errorf("interface name must not be '.' or '..'")
	}

	// OS-specific validation.
	switch runtime.GOOS {
	case "darwin":
		if !darwinRegex.MatchString(name) {
			return fmt.Errorf(
				"on macOS, interface name must follow the utun<N> pattern (e.g. utun20); got %q",
				name,
			)
		}
	default: // Linux and other Unix
		if len(name) > MaxLen {
			return fmt.Errorf(
				"interface name %q is %d characters; maximum is %d (kernel IFNAMSIZ limit)",
				name, len(name), MaxLen,
			)
		}
		if !linuxRegex.MatchString(name) {
			return fmt.Errorf(
				"interface name %q is invalid; must start with a letter and contain only "+
					"letters, digits, underscores, or hyphens (e.g. wg0, cloudroof0, mesh-1)",
				name,
			)
		}
	}

	return nil
}
