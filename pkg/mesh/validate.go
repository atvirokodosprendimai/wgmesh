package mesh

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ifname"
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

// The class B rules: each returns the rule name a value breaks, or "". The
// grammars are permissive supersets of what wg, ip and systemctl accept, and
// are measured against no node, which is why they only warn at release N.

func interfaceNameRule(v string) string {
	if v != "" && ifname.ValidateLinux(v) != nil {
		return "interface-name-grammar"
	}
	return ""
}

func cidrRule(v string) string {
	if _, _, err := net.ParseCIDR(v); err != nil {
		return "not-a-cidr"
	}
	return ""
}

func meshNetworkRule(v string) string {
	if v == "" {
		return ""
	}
	return cidrRule(v)
}

// validPort reports whether p is a TCP/UDP port number Deploy can use.
func validPort(p int) bool {
	return p >= 1 && p <= 65535
}

func endpointRule(v string) string {
	if v == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(v)
	if err != nil || host == "" {
		return "not-host-port"
	}
	p, err := strconv.Atoi(port)
	if err != nil || strings.Trim(port, "0123456789") != "" || !validPort(p) {
		return "port-out-of-range"
	}
	return ""
}

func publicKeyRule(v string) string {
	if raw, err := base64.StdEncoding.Strict().DecodeString(v); err != nil || len(v) != 44 || len(raw) != 32 {
		return "not-44-base64"
	}
	return ""
}

// Validate checks the loaded state and returns every finding, the mesh fields
// first and then the nodes in name order. It is pure and holds the read lock.
// A value with a class A finding gets no class B finding on top.
func (m *Mesh) Validate() []Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var fs []Finding
	text := func(node, field, v string, ruleB func(string) string) {
		if rule := unsafeRule(v); rule != "" {
			fs = append(fs, Finding{Node: node, Field: field, Value: v, Rule: rule, Class: ClassA})
		} else if rule := ruleB(v); rule != "" {
			fs = append(fs, Finding{Node: node, Field: field, Value: v, Rule: rule, Class: ClassB})
		}
	}
	port := func(node, field string, p int) {
		if !validPort(p) {
			fs = append(fs, Finding{Node: node, Field: field, Value: strconv.Itoa(p), Rule: "port-out-of-range", Class: ClassB})
		}
	}

	if m.InterfaceName == "" {
		fs = append(fs, Finding{Field: "interface_name", Rule: "empty", Class: ClassA})
	}
	text("", "interface_name", m.InterfaceName, interfaceNameRule)
	text("", "network", m.Network, meshNetworkRule)
	port("", "listen_port", m.ListenPort)

	var meshNet *net.IPNet
	if m.Network != "" {
		_, meshNet, _ = net.ParseCIDR(m.Network)
	}

	names := make([]string, 0, len(m.Nodes))
	for name := range m.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	seenIP := map[string]bool{}
	seenKey := map[string]bool{}
	for _, name := range names {
		n := m.Nodes[name]
		if n == nil {
			fs = append(fs, Finding{Node: name, Field: "node", Rule: "null-node", Class: ClassA})
			continue
		}
		text(name, "public_key", n.PublicKey, publicKeyRule)
		text(name, "public_endpoint", n.PublicEndpoint, endpointRule)
		text(name, "ssh_host", n.SSHHost, func(string) string { return "" })
		for _, r := range n.RoutableNetworks {
			text(name, "routable_networks", r, cidrRule)
		}
		port(name, "listen_port", n.ListenPort)
		port(name, "ssh_port", n.SSHPort)

		switch {
		case len(n.MeshIP) == 0:
			fs = append(fs, Finding{Node: name, Field: "mesh_ip", Rule: "missing", Class: ClassA})
		case net.IP(n.MeshIP).To16() == nil:
			fs = append(fs, Finding{Node: name, Field: "mesh_ip", Value: n.MeshIP.String(), Rule: "unparsable", Class: ClassA})
		default:
			ip := n.MeshIP.String()
			if meshNet != nil && !meshNet.Contains(n.MeshIP) {
				fs = append(fs, Finding{Node: name, Field: "mesh_ip", Value: ip, Rule: "outside-network", Class: ClassB})
			}
			if seenIP[ip] {
				fs = append(fs, Finding{Node: name, Field: "mesh_ip", Value: ip, Rule: "duplicate-mesh-ip", Class: ClassB})
			}
			seenIP[ip] = true
		}
		if n.PublicKey != "" {
			if seenKey[n.PublicKey] {
				fs = append(fs, Finding{Node: name, Field: "public_key", Value: n.PublicKey, Rule: "duplicate-public-key", Class: ClassB})
			}
			seenKey[n.PublicKey] = true
		}
	}
	return fs
}

// String is the finding as one line, for a warning or the refusal list.
func (f Finding) String() string {
	where := f.Field
	if f.Node != "" {
		where = "node " + f.Node + ", " + f.Field
	}
	return fmt.Sprintf("%s: rule %s (class %s), value %q", where, f.Rule, f.Class, f.Value)
}

// validateForDeploy prints the class B warnings to out and returns an error
// listing every finding it refuses: class A always, class B when StrictState
// is set. Deploy calls it before the first connection.
func (m *Mesh) validateForDeploy(out io.Writer) error {
	var refused []string
	for _, f := range m.Validate() {
		if f.Class == ClassA || StrictState {
			refused = append(refused, "  "+f.String())
			continue
		}
		fmt.Fprintf(out, "Warning: state: %s (will be refused once StrictState is set in a later release)\n", f)
	}
	if len(refused) > 0 {
		return fmt.Errorf("state validation failed, %d finding(s):\n%s\n"+
			"fix the named fields in the mesh state file (see docs/centralized-mode.md); nothing was deployed",
			len(refused), strings.Join(refused, "\n"))
	}
	return nil
}
