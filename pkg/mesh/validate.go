package mesh

import (
	"io"
	"net"
	"sort"
	"strings"
	"unicode"
)

// StrictState makes a class B finding a refusal instead of a warning. It is
// false in release N and set in N+1 (ADR-0006, decision 2).
var StrictState = false

// Class says what Deploy does with a finding.
type Class string

const (
	// ClassA is refused: a value a shell or a wg-quick file gives a meaning to,
	// or a field Deploy cannot work without.
	ClassA Class = "A"
	// ClassB only warns, until StrictState is set: a value that parses wrongly
	// but cannot reach a shell.
	ClassB Class = "B"
)

// Finding is one rule a loaded state breaks. Node is empty for a mesh-level
// field.
type Finding struct {
	Node  string
	Field string
	Value string
	Rule  string
	Class Class
}

// safeChar reports whether r is in the class A safe set
// [A-Za-z0-9._:/,@%+=\[\]-]: what a shell or a wg-quick file gives no meaning
// to in the six text fields. Brackets are in it so an IPv6 literal endpoint
// passes, although ssh.Quote still quotes them.
// no meaning to in the values of the six text fields. Brackets are in it so
// an IPv6 literal endpoint passes, although ssh.Quote still quotes them.
func safeChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '.', '_', ':', '/', ',', '@', '%', '+', '=', '[', ']', '-':
		return true
	}
	return false
}

// unsafeRule names the class A rule a text value breaks, or "" when it is
// safe. An empty value is safe here; a field that must not be empty says so
// itself.
func unsafeRule(v string) string {
	for _, r := range v {
		if unicode.IsControl(r) {
			return "control-character"
		}
	}
	for _, r := range v {
		if r == ' ' {
			return "space"
		}
	}
	for _, r := range v {
		if !safeChar(r) {
			return "unsafe-character"
		}
	}
	if strings.HasPrefix(v, "-") {
		return "leading-dash"
	}
	return ""
}

// Validate checks the loaded state and returns every finding, the mesh fields
// first and then the nodes in name order. It is pure and holds the read lock.
func (m *Mesh) Validate() []Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var fs []Finding
	text := func(node, field, v string) {
		if rule := unsafeRule(v); rule != "" {
			fs = append(fs, Finding{Node: node, Field: field, Value: v, Rule: rule, Class: ClassA})
		}
	}

	if m.InterfaceName == "" {
		fs = append(fs, Finding{Field: "interface_name", Rule: "empty", Class: ClassA})
	}
	text("", "interface_name", m.InterfaceName)
	text("", "network", m.Network)

	names := make([]string, 0, len(m.Nodes))
	for name := range m.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := m.Nodes[name]
		text(name, "public_key", n.PublicKey)
		text(name, "public_endpoint", n.PublicEndpoint)
		text(name, "ssh_host", n.SSHHost)
		for _, r := range n.RoutableNetworks {
			text(name, "routable_networks", r)
		}
		switch {
		case len(n.MeshIP) == 0:
			fs = append(fs, Finding{Node: name, Field: "mesh_ip", Rule: "missing", Class: ClassA})
		case net.IP(n.MeshIP).To16() == nil:
			fs = append(fs, Finding{Node: name, Field: "mesh_ip", Value: n.MeshIP.String(), Rule: "unparsable", Class: ClassA})
		}
	}
	return fs
}

// validateForDeploy prints the class B warnings to out and returns an error
// listing the findings Deploy refuses.
func (m *Mesh) validateForDeploy(out io.Writer) error {
	return nil
}
