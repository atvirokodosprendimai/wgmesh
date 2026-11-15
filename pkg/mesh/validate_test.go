package mesh

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
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
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return &m
}

// select returns the findings for one node and field.
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

// mutations sets one field of validMesh to a value and names the field and
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
	values := map[string]string{
		"semicolon":          "; id",
		"command subst":      "$(id)",
		"backtick":           "`id`",
		"pipe":               "a|b",
		"quote":              "a'b",
		"space":              "a b",
		"newline":            "a\nb",
		"tab":                "a\tb",
		"nul":                "a\x00b",
		"del":                "a\x7fb",
		"non-ASCII":          "aéb",
		"leading dash":       "-oProxyCommand=id",
		"leading dash alone": "-",
	}
	for _, fs := range fieldSetters {
		for vname, v := range values {
			t.Run(fs.field+"/"+vname, func(t *testing.T) {
				m := validMesh()
				fs.set(m, v)
				got := selectFindings(m.Validate(), fs.node, fs.field)
				a := classOf(got, ClassA)
				if len(a) == 0 {
					t.Fatalf("no class A finding for %q in %s of node %q; all findings: %+v", v, fs.field, fs.node, m.Validate())
				}
				if a[0].Rule == "" || a[0].Value != v {
					t.Errorf("finding must name a rule and carry the value: %+v", a[0])
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

	// The placeholders of docs/centralized-mode.md are not 44 base64
	// characters: only class B findings on public_key, nothing refused.
	t.Run("docs placeholders", func(t *testing.T) {
		m := loadFixture(t, "state-docs-example.json")
		m.Nodes["node1"].PublicKey = "base64-encoded-public-key"
		fs := m.Validate()
		if len(fs) != 1 || fs[0].Class != ClassB || fs[0].Field != "public_key" || fs[0].Node != "node1" {
			t.Fatalf("want one class B public_key finding, got %+v", fs)
		}
		if err := m.validateForDeploy(&bytes.Buffer{}); err != nil {
			t.Errorf("docs placeholders refused at N: %v", err)
		}
	})

	// An IPv6 literal endpoint and the default route are safe-set values.
	t.Run("ipv6 endpoint", func(t *testing.T) {
		m := validMesh()
		m.Nodes["alpha"].PublicEndpoint = "[fd00::1]:51820"
		m.Nodes["alpha"].RoutableNetworks = []string{"0.0.0.0/0", "::/0", "192.168.10.0/24"}
		if fs := m.Validate(); len(fs) != 0 {
			t.Errorf("findings: %+v", fs)
		}
	})
}

// Deploy must refuse a class A state before it opens a connection: the
// listener counts every accepted connection.
func TestDeployValidatesBeforeDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var conns atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns.Add(1)
			c.Close()
		}
	}()
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
	if n := conns.Load(); n != 0 {
		t.Errorf("Deploy opened %d connection(s) before validating", n)
	}
}
