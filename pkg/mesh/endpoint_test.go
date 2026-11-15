package mesh

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh/sshtest"
)

// lanHost makes a server on 127.0.0.1 stand for a private LAN address: the
// real classifier never calls loopback private (TestPrivateHostClassification).
// Each extra host is classed private as well.
func lanHost(t *testing.T, extra ...string) {
	t.Helper()
	old := hostIsPrivate
	hostIsPrivate = func(h string) bool { return h == "127.0.0.1" || slices.Contains(extra, h) || old(h) }
	t.Cleanup(func() { hostIsPrivate = old })
}

// trustAlias makes the server's host key valid under another name for its
// address, so a test can dial it as a DNS name.
func trustAlias(t *testing.T, s *sshtest.Server, name string) {
	t.Helper()
	path := os.Getenv(sshtest.EnvKnownHosts)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	from := fmt.Sprintf("[%s]:%d", s.Host, s.Port)
	to := fmt.Sprintf("[%s]:%d", name, s.Port)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, from+" ") {
			if _, err := fmt.Fprintln(f, strings.Replace(line, from, to, 1)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func countCurl(s *sshtest.Server) int {
	n := 0
	for _, c := range s.Commands() {
		if strings.Contains(c, "curl") {
			n++
		}
	}
	return n
}

func TestResolveEndpointTable(t *testing.T) {
	const operator = "vpn.example.com:443"
	rows := []struct {
		name       string
		host       string
		set        string
		allPrivate bool
		publicIP   string
		probed     bool
		want       string
		wantNAT    bool
	}{
		{"operator endpoint wins when public IP equals the SSH host", "203.0.113.9", operator, false, "203.0.113.9", true, operator, false},
		{"operator endpoint wins when public IP differs", "203.0.113.9", operator, false, "198.51.100.7", true, operator, false},
		{"operator endpoint wins in an all-private mesh", "192.168.1.10", operator, true, "", false, operator, false},
		{"all-private local node gets host:port", "192.168.1.10", "", true, "", false, "192.168.1.10:51820", false},
		{"all-private remote node gets host:port", "10.0.0.2", "", true, "", false, "10.0.0.2:51820", false},
		{"IPv6 literal is bracketed", "fd00::1", "", true, "", false, "[fd00::1]:51820", false},
		{"mixed mesh: private host gets none", "192.168.1.10", "", false, "", false, "", true},
		{"public IP equal to SSH host", "203.0.113.9", "", false, "203.0.113.9", true, "203.0.113.9:51820", false},
		{"public IPv6 equal to SSH host is bracketed", "2001:db8::1", "", false, "2001:db8::1", true, "[2001:db8::1]:51820", false},
		{"DNS name equal to the probed value", "node.example.com", "", false, "node.example.com", true, "node.example.com:51820", false},
		{"public IP differs: behind NAT", "203.0.113.9", "", false, "198.51.100.7", true, "", true},
		{"probe failed: behind NAT", "203.0.113.9", "", false, "", true, "", true},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			node := &Node{SSHHost: r.host, ListenPort: 51820, PublicEndpoint: r.set}
			got := resolveEndpoint(node, r.allPrivate, r.publicIP, r.probed)
			if got.Endpoint != r.want || got.BehindNAT != r.wantNAT {
				t.Errorf("got endpoint %q behindNAT %v, want %q %v", got.Endpoint, got.BehindNAT, r.want, r.wantNAT)
			}
			if got.Reason == "" || got.Reason == "stub" {
				t.Errorf("reason %q does not tell the operator why", got.Reason)
			}
		})
	}
}

func TestResolveEndpointIsPure(t *testing.T) {
	node := &Node{SSHHost: "10.0.0.2", ListenPort: 51820}
	first := resolveEndpoint(node, true, "", false)
	if second := resolveEndpoint(node, true, "", false); second != first {
		t.Errorf("two calls differ: %+v vs %+v", first, second)
	}
	if node.PublicEndpoint != "" || node.BehindNAT {
		t.Errorf("the node was written: %+v", node)
	}
}

func TestPrivateHostClassification(t *testing.T) {
	for host, want := range map[string]bool{
		"10.0.0.1": true, "172.16.0.1": true, "192.168.0.1": true, "fd00::1": true,
		"8.8.8.8": false, "100.64.0.1": false, "127.0.0.1": false, "169.254.1.1": false,
		"::1": false, "node.example.com": false, "": false,
	} {
		if got := isPrivateHost(host); got != want {
			t.Errorf("isPrivateHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestDetectEndpointsSkipsProbeForPrivateHost(t *testing.T) {
	t.Run("private host: no curl", func(t *testing.T) {
		lanHost(t)
		s := sshtest.Start(t, nodeReplies())
		m := newTestMesh(tnode{name: "a", host: s.Host, port: s.Port})

		if err, out := deployCapturing(t, m); err != nil {
			t.Fatalf("Deploy: %v\n%s", err, out)
		}

		if n := countCurl(s); n != 0 {
			t.Errorf("a private host received %d curl commands: %q", n, s.Commands())
		}
	})
	t.Run("public host: one curl", func(t *testing.T) {
		s := sshtest.Start(t, nodeReplies())
		m := newTestMesh(tnode{name: "a", host: s.Host, port: s.Port})

		if err, out := deployCapturing(t, m); err != nil {
			t.Fatalf("Deploy: %v\n%s", err, out)
		}

		if n := countCurl(s); n != 1 {
			t.Errorf("a public host received %d curl commands, want 1: %q", n, s.Commands())
		}
	})
}

func TestAllLANMeshRendersEndpoints(t *testing.T) {
	lanHost(t)
	a := sshtest.Start(t, nodeReplies())
	b := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port},
		tnode{name: "b", host: b.Host, port: b.Port},
	)

	err, out := deployCapturing(t, m)

	if err != nil {
		t.Fatalf("Deploy: %v\n%s", err, out)
	}
	for name, srv := range map[string]*sshtest.Server{"a": a, "b": b} {
		conf, ok := writtenConfig(srv)
		if !ok {
			t.Fatalf("node %s received no config", name)
		}
		if !strings.Contains(conf, "Endpoint = 127.0.0.1:51820") {
			t.Errorf("config of %s has no endpoint for its peer:\n%s", name, conf)
		}
	}
	if !strings.Contains(out, "Detected a ") || !strings.Contains(out, "all SSH hosts are private") {
		t.Errorf("the deploy does not print why a has an endpoint:\n%s", out)
	}
}

func TestMixedMeshKeepsTodaysRule(t *testing.T) {
	lanHost(t)
	// Node a is private (127.0.0.1); node b is a DNS name whose probed public
	// IP equals its SSH host, so it keeps the endpoint of b8b83f3.
	aReplies := nodeReplies()
	bReplies := nodeReplies()
	bReplies["curl -s -4"] = sshtest.Reply{Out: "localhost\n"}
	a := sshtest.Start(t, aReplies)
	b := sshtest.Start(t, bReplies)
	trustAlias(t, b, "localhost")
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port},
		tnode{name: "b", host: "localhost", port: b.Port},
	)

	err, out := deployCapturing(t, m)

	if err != nil {
		t.Fatalf("Deploy: %v\n%s", err, out)
	}
	aConf, _ := writtenConfig(a)
	bConf, _ := writtenConfig(b)
	if !strings.Contains(aConf, "Endpoint = localhost:51820") {
		t.Errorf("the public node lost its endpoint:\n%s", aConf)
	}
	if strings.Contains(bConf, "Endpoint") {
		t.Errorf("the private node got an endpoint in a mixed mesh:\n%s", bConf)
	}
	if n := countCurl(a); n != 0 {
		t.Errorf("the private host was probed: %q", a.Commands())
	}
}

// A node that cannot be dialled still counts in the classification: it needs no
// connection, so an unreachable public host keeps the mesh mixed.
func TestDetectEndpointsClassifiesUnreachableNodes(t *testing.T) {
	build := func(t *testing.T) *Mesh {
		a := sshtest.Start(t, nodeReplies())
		b := sshtest.Start(t, nodeReplies())
		return newTestMesh(
			tnode{name: "a", host: a.Host, port: a.Port},
			tnode{name: "b", host: b.Host, port: b.Port},
			tnode{name: "c", host: "127.0.0.2", port: closedPort(t)},
		)
	}
	t.Run("unreachable public node: peers get no endpoint", func(t *testing.T) {
		lanHost(t)
		m := build(t)

		failures := m.detectEndpoints()

		if failures["c"] == nil {
			t.Fatalf("c was dialled successfully, the test needs it refused: %v", failures)
		}
		for _, name := range []string{"a", "b"} {
			if n := m.Nodes[name]; n.PublicEndpoint != "" || !n.BehindNAT {
				t.Errorf("%s: endpoint %q behindNAT %v, want none and true", name, n.PublicEndpoint, n.BehindNAT)
			}
		}
	})
	t.Run("unreachable private node: peers get endpoints", func(t *testing.T) {
		lanHost(t, "127.0.0.2")
		m := build(t)

		failures := m.detectEndpoints()

		if failures["c"] == nil {
			t.Fatalf("c was dialled successfully, the test needs it refused: %v", failures)
		}
		for _, name := range []string{"a", "b"} {
			if n := m.Nodes[name]; n.PublicEndpoint != "127.0.0.1:51820" || n.BehindNAT {
				t.Errorf("%s: endpoint %q behindNAT %v, want 127.0.0.1:51820 and false", name, n.PublicEndpoint, n.BehindNAT)
			}
		}
	})
}
