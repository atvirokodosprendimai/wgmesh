package mesh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	keyAlpha = "YWxwaGEta2V5LTEwMDAwMDAwMDAwMDAwMDAwMDAwMDA="
	keyBravo = "YnJhdm8ta2V5LTIwMDAwMDAwMDAwMDAwMDAwMDAwMDA="
)

// validMesh is a two-node state that deployed before: no finding.
func validMesh() *Mesh {
	return &Mesh{
		InterfaceName: "wg0", Network: "10.99.0.0/16", ListenPort: 51820,
		Nodes: map[string]*Node{
			"alpha": {
				Hostname: "alpha", MeshIP: net.ParseIP("10.99.0.1"), PublicKey: keyAlpha,
				SSHHost: "alpha.example.net", SSHPort: 22, ListenPort: 51820,
				PublicEndpoint:   "alpha.example.net:51820",
				RoutableNetworks: []string{"192.168.10.0/24"},
			},
			"bravo": {
				Hostname: "bravo", MeshIP: net.ParseIP("10.99.0.2"), PublicKey: keyBravo,
				SSHHost: "10.0.0.7", SSHPort: 22, ListenPort: 51820,
			},
		},
	}
}

func loadFixture(t *testing.T, name string) *Mesh {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "central", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var m Mesh
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return &m
}

// selectFindings returns the findings for one node and field.
func selectFindings(fs []Finding, node, field string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Node == node && f.Field == field {
			out = append(out, f)
		}
	}
	return out
}

func classOf(fs []Finding, c Class) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Class == c {
			out = append(out, f)
		}
	}
	return out
}

// fieldSetters sets one text field of validMesh to a value; node and field
// name the finding it must produce.
// node of the finding it must produce.
var fieldSetters = []struct {
	node, field string
	set         func(m *Mesh, v string)
}{
	{"", "interface_name", func(m *Mesh, v string) { m.InterfaceName = v }},
	{"", "network", func(m *Mesh, v string) { m.Network = v }},
	{"alpha", "public_key", func(m *Mesh, v string) { m.Nodes["alpha"].PublicKey = v }},
	{"alpha", "public_endpoint", func(m *Mesh, v string) { m.Nodes["alpha"].PublicEndpoint = v }},
	{"alpha", "routable_networks", func(m *Mesh, v string) { m.Nodes["alpha"].RoutableNetworks = []string{"192.168.10.0/24", v} }},
	{"alpha", "ssh_host", func(m *Mesh, v string) { m.Nodes["alpha"].SSHHost = v }},
}

func TestValidateRefusesShellMetacharacters(t *testing.T) {
	// Every value maps to the rule that must fire first. Each shell
	// metacharacter has a value without a space, so the space rule cannot
	// hide a character missing from the refused set.
	values := []struct{ value, rule string }{
		{";id", "unsafe-character"},
		{"a;b", "unsafe-character"},
		{"a&b", "unsafe-character"},
		{"a|b", "unsafe-character"},
		{"a>b", "unsafe-character"},
		{"a<b", "unsafe-character"},
		{"a$b", "unsafe-character"},
		{"$(id)", "unsafe-character"},
		{"a(b", "unsafe-character"},
		{"a)b", "unsafe-character"},
		{"a*b", "unsafe-character"},
		{"a?b", "unsafe-character"},
		{"a\\b", "unsafe-character"},
		{"a\"b", "unsafe-character"},
		{"a'b", "unsafe-character"},
		{"a`b", "unsafe-character"},
		{"`id`", "unsafe-character"},
		{"a!b", "unsafe-character"},
		{"a#b", "unsafe-character"},
		{"a~b", "unsafe-character"},
		{"a{b", "unsafe-character"},
		{"a}b", "unsafe-character"},
		{"aéb", "unsafe-character"},
		{"; id", "space"},
		{"a b", "space"},
		{"a\nb", "control-character"},
		{"a\tb", "control-character"},
		{"a\x00b", "control-character"},
		{"a\x7fb", "control-character"},
		{"-oProxyCommand=id", "leading-dash"},
		{"-", "leading-dash"},
	}
	for _, fs := range fieldSetters {
		for _, tc := range values {
			t.Run(fmt.Sprintf("%s/%q", fs.field, tc.value), func(t *testing.T) {
				m := validMesh()
				fs.set(m, tc.value)
				got := selectFindings(m.Validate(), fs.node, fs.field)
				if len(got) == 0 {
					t.Fatalf("no finding for %q in %s of node %q", tc.value, fs.field, fs.node)
				}
				if got[0].Class != ClassA || got[0].Rule != tc.rule || got[0].Value != tc.value {
					t.Errorf("want class A rule %s carrying the value, got %+v", tc.rule, got[0])
				}
			})
		}
	}
}

func TestValidateRefusesMissingFields(t *testing.T) {
	m := validMesh()
	m.InterfaceName = ""
	m.Nodes["bravo"].MeshIP = nil
	fs := m.Validate()

	if got := classOf(selectFindings(fs, "", "interface_name"), ClassA); len(got) != 1 || got[0].Rule == "" {
		t.Errorf("empty interface name: want one class A finding with a rule, got %+v", fs)
	}
	if got := classOf(selectFindings(fs, "bravo", "mesh_ip"), ClassA); len(got) != 1 || got[0].Rule == "" {
		t.Errorf("node without mesh_ip: want one class A finding with a rule, got %+v", fs)
	}
	if len(classOf(fs, ClassA)) != 2 {
		t.Errorf("want exactly two class A findings, got %+v", fs)
	}

	// A null node is a finding, not a panic.
	t.Run("null node", func(t *testing.T) {
		m := validMesh()
		m.Nodes["bravo"] = nil
		got := selectFindings(m.Validate(), "bravo", "node")
		if len(got) != 1 || got[0].Class != ClassA || got[0].Rule != "null-node" {
			t.Errorf("want one class A null-node finding, got %+v", m.Validate())
		}
	})
}

// formatDeviations are values that parse wrongly but hold only safe-set
// characters: every one is one class B finding on the named field.
var formatDeviations = []struct {
	name        string
	node, field string
	set         func(m *Mesh)
}{
	{"bare IP route", "alpha", "routable_networks", func(m *Mesh) { m.Nodes["alpha"].RoutableNetworks = []string{"192.168.10.5"} }},
	{"/33 route", "alpha", "routable_networks", func(m *Mesh) { m.Nodes["alpha"].RoutableNetworks = []string{"192.168.10.0/33"} }},
	{"dot in interface name", "", "interface_name", func(m *Mesh) { m.InterfaceName = "wg.0" }},
	{"17 character interface name", "", "interface_name", func(m *Mesh) { m.InterfaceName = "abcdefghijklmnopq" }},
	{"endpoint port 0", "alpha", "public_endpoint", func(m *Mesh) { m.Nodes["alpha"].PublicEndpoint = "alpha.example.net:0" }},
	{"endpoint without port", "alpha", "public_endpoint", func(m *Mesh) { m.Nodes["alpha"].PublicEndpoint = "alpha.example.net" }},
	{"short key", "alpha", "public_key", func(m *Mesh) { m.Nodes["alpha"].PublicKey = "short" }},
	{"mesh network not a CIDR", "", "network", func(m *Mesh) { m.Network = "10.99.0.0" }},
	{"node listen port 70000", "alpha", "listen_port", func(m *Mesh) { m.Nodes["alpha"].ListenPort = 70000 }},
	{"ssh port 0", "alpha", "ssh_port", func(m *Mesh) { m.Nodes["alpha"].SSHPort = 0 }},
	{"mesh IP outside the network", "alpha", "mesh_ip", func(m *Mesh) { m.Nodes["alpha"].MeshIP = net.ParseIP("172.16.0.1") }},
	{"duplicate mesh IP", "bravo", "mesh_ip", func(m *Mesh) { m.Nodes["bravo"].MeshIP = net.ParseIP("10.99.0.1") }},
	{"duplicate public key", "bravo", "public_key", func(m *Mesh) { m.Nodes["bravo"].PublicKey = keyAlpha }},
	{"endpoint port with a plus sign", "alpha", "public_endpoint", func(m *Mesh) { m.Nodes["alpha"].PublicEndpoint = "alpha.example.net:+80" }},
	{"non-canonical trailing bits in key", "alpha", "public_key", func(m *Mesh) { m.Nodes["alpha"].PublicKey = keyAlpha[:42] + "B=" }},
	{"docs placeholder key", "alpha", "public_key", func(m *Mesh) { m.Nodes["alpha"].PublicKey = "base64-encoded-public-key" }},
	{"mesh listen port 0", "", "listen_port", func(m *Mesh) { m.ListenPort = 0 }},
	{"empty public key", "alpha", "public_key", func(m *Mesh) { m.Nodes["alpha"].PublicKey = "" }},
}

func TestValidateWarnsOnFormatDeviationAtN(t *testing.T) {
	if StrictState {
		t.Fatal("StrictState must be false at release N")
	}
	for _, d := range formatDeviations {
		t.Run(d.name, func(t *testing.T) {
			m := validMesh()
			d.set(m)
			fs := m.Validate()
			if len(fs) != 1 || fs[0].Class != ClassB || fs[0].Node != d.node || fs[0].Field != d.field || fs[0].Rule == "" {
				t.Fatalf("want exactly one class B finding on node %q field %q, got %+v", d.node, d.field, fs)
			}
			var out bytes.Buffer
			if err := m.validateForDeploy(&out); err != nil {
				t.Fatalf("class B must not refuse at N: %v", err)
			}
			if !strings.Contains(out.String(), "Warning:") || !strings.Contains(out.String(), d.field) {
				t.Errorf("want a Warning line naming %s, got %q", d.field, out.String())
			}
		})
	}
}

func TestValidateRefusesFormatDeviationWhenStrict(t *testing.T) {
	old := StrictState
	StrictState = true
	t.Cleanup(func() { StrictState = old })

	for _, d := range formatDeviations {
		t.Run(d.name, func(t *testing.T) {
			m := validMesh()
			d.set(m)
			var out bytes.Buffer
			err := m.validateForDeploy(&out)
			if err == nil {
				t.Fatalf("StrictState must refuse class B; output %q", out.String())
			}
			if !strings.Contains(err.Error(), d.field) {
				t.Errorf("error must list the finding's field %s: %v", d.field, err)
			}
		})
	}

	// A valid state is still accepted in strict mode.
	if err := validMesh().validateForDeploy(&bytes.Buffer{}); err != nil {
		t.Errorf("valid state refused in strict mode: %v", err)
	}
}

func TestValidateAcceptsSavedStates(t *testing.T) {
	for _, name := range []string{"state-valid.json", "state-docs-example.json"} {
		t.Run(name, func(t *testing.T) {
			m := loadFixture(t, name)
			if fs := m.Validate(); len(fs) != 0 {
				t.Errorf("a state that deployed before gives findings: %+v", fs)
			}
			var out bytes.Buffer
			if err := m.validateForDeploy(&out); err != nil || out.Len() != 0 {
				t.Errorf("validateForDeploy: err %v, output %q", err, out.String())
			}
		})
	}

	// An IPv6 literal endpoint and the default route are safe-set values.
	t.Run("ipv6 endpoint", func(t *testing.T) {
		m := validMesh()
		m.Nodes["alpha"].PublicEndpoint = "[fd00::1]:51820"
		m.Nodes["alpha"].RoutableNetworks = []string{"0.0.0.0/0", "::/0", "192.168.10.0/24"}
		if fs := m.Validate(); len(fs) != 0 {
			t.Errorf("findings: %+v", fs)
		}
	})

	// Fields that may be empty give no finding: no network (Deploy then uses
	// a /16) and no ssh_host.
	t.Run("empty network", func(t *testing.T) {
		m := validMesh()
		m.Network = ""
		if fs := m.Validate(); len(fs) != 0 {
			t.Errorf("findings: %+v", fs)
		}
	})
	t.Run("empty ssh_host", func(t *testing.T) {
		m := validMesh()
		m.Nodes["alpha"].SSHHost = ""
		if fs := m.Validate(); len(fs) != 0 {
			t.Errorf("findings: %+v", fs)
		}
	})
}

// Deploy must refuse a class A state before it opens a connection: after it
// returns, the listener must have nothing to accept.
func TestDeployValidatesBeforeDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	addr := ln.Addr().(*net.TCPAddr)

	m := validMesh()
	m.InterfaceName = "wg0; id"
	for _, n := range m.Nodes {
		n.SSHHost, n.SSHPort = "127.0.0.1", addr.Port
	}

	err = m.Deploy()
	if err == nil {
		t.Fatal("Deploy accepted a class A state")
	}
	if !strings.Contains(err.Error(), "interface_name") {
		t.Errorf("error must list the finding: %v", err)
	}

	// A dial still sitting in the backlog is accepted here; a timeout means
	// no connection was made.
	if err := ln.(*net.TCPListener).SetDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	c, err := ln.Accept()
	if err == nil {
		c.Close()
		t.Fatal("Deploy opened a connection before validating")
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Errorf("want an accept timeout, got %v", err)
	}
}
