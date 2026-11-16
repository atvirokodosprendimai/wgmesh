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
	const (
		operator   = "vpn.example.com:443"
		inState    = "public_endpoint is set in the state"
		lan        = "all SSH hosts are private IP addresses"
		mixed      = "private SSH host in a mesh with other hosts; set public_endpoint to give it an endpoint"
		equal      = "public IP equals the SSH host"
		differs    = "behind NAT (public IP 198.51.100.7 differs from the SSH host)"
		undetected = "public IP could not be detected; set public_endpoint to give it an endpoint"
	)
	rows := []struct {
		name       string
		host       string
		set        string
		allPrivate bool
		publicIP   string
		probed     bool
		want       string
		wantNAT    bool
		wantReason string
	}{
		{"operator endpoint wins when public IP equals the SSH host", "203.0.113.9", operator, false, "203.0.113.9", true, operator, false, inState},
		{"operator endpoint wins when public IP differs", "203.0.113.9", operator, false, "198.51.100.7", true, operator, false, inState},
		{"operator endpoint wins in an all-private mesh", "192.168.1.10", operator, true, "", false, operator, false, inState},
		{"all-private local node gets host:port", "192.168.1.10", "", true, "", false, "192.168.1.10:51820", false, lan},
		{"all-private remote node gets host:port", "10.0.0.2", "", true, "", false, "10.0.0.2:51820", false, lan},
		{"IPv6 literal is bracketed", "fd00::1", "", true, "", false, "[fd00::1]:51820", false, lan},
		{"mixed mesh: private host gets none", "192.168.1.10", "", false, "", false, "", true, mixed},
		{"public IP equal to SSH host", "203.0.113.9", "", false, "203.0.113.9", true, "203.0.113.9:51820", false, equal},
		{"public IPv6 equal to SSH host is bracketed", "2001:db8::1", "", false, "2001:db8::1", true, "[2001:db8::1]:51820", false, equal},
		{"DNS name equal to the probed value", "node.example.com", "", false, "node.example.com", true, "node.example.com:51820", false, equal},
		{"public IP differs: behind NAT", "203.0.113.9", "", false, "198.51.100.7", true, "", true, differs},
		{"probe failed: behind NAT", "203.0.113.9", "", false, "", true, "", true, undetected},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			node := &Node{SSHHost: r.host, ListenPort: 51820, PublicEndpoint: r.set}
			got := resolveEndpoint(node, r.allPrivate, r.publicIP, r.probed)
			if got.Endpoint != r.want || got.BehindNAT != r.wantNAT {
				t.Errorf("got endpoint %q behindNAT %v, want %q %v", got.Endpoint, got.BehindNAT, r.want, r.wantNAT)
			}
			if got.Reason != r.wantReason {
				t.Errorf("reason %q, want %q", got.Reason, r.wantReason)
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
	t.Run("not private host: one curl", func(t *testing.T) {
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
	lanHost(t, "localhost")
	// Node a is the local node at 127.0.0.1; node b is dialled as localhost, so
	// the two peers carry different endpoints.
	a := sshtest.Start(t, nodeReplies())
	b := sshtest.Start(t, nodeReplies())
	trustAlias(t, b, "localhost")
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port, local: true},
		tnode{name: "b", host: "localhost", port: b.Port},
	)

	err, out := deployCapturing(t, m)

	if err != nil {
		t.Fatalf("Deploy: %v\n%s", err, out)
	}
	aConf, ok := writtenConfig(a)
	if !ok {
		t.Fatal("node a received no config")
	}
	bConf, ok := writtenConfig(b)
	if !ok {
		t.Fatal("node b received no config")
	}
	if n := strings.Count(aConf, "Endpoint ="); n != 1 || !strings.Contains(aConf, "Endpoint = localhost:51820\n") {
		t.Errorf("the config of a has %d Endpoint lines, want one for b (localhost:51820):\n%s", n, aConf)
	}
	if n := strings.Count(bConf, "Endpoint ="); n != 1 || !strings.Contains(bConf, "Endpoint = 127.0.0.1:51820\n") {
		t.Errorf("the config of b has %d Endpoint lines, want one for a (127.0.0.1:51820):\n%s", n, bConf)
	}
	if strings.Contains(aConf, "127.0.0.1:51820") || strings.Contains(bConf, "localhost:51820") {
		t.Errorf("a node's own endpoint is in its own config:\n%s\n%s", aConf, bConf)
	}
	if n := m.Nodes["a"]; n.PublicEndpoint != "127.0.0.1:51820" || n.BehindNAT {
		t.Errorf("the local node a has endpoint %q behindNAT %v, want 127.0.0.1:51820 and false", n.PublicEndpoint, n.BehindNAT)
	}
	why := resolveEndpoint(&Node{SSHHost: "127.0.0.1", ListenPort: 51820}, true, "", false).Reason
	if !strings.Contains(out, why) {
		t.Errorf("the deploy does not print why a has an endpoint (%q):\n%s", why, out)
	}
}

func TestMixedMeshKeepsTodaysRule(t *testing.T) {
	lanHost(t)
	// Node a is private (127.0.0.1) and local; node b is a DNS name whose probed
	// public IP equals its SSH host, so it keeps the endpoint of b8b83f3.
	aReplies := nodeReplies()
	bReplies := nodeReplies()
	bReplies["curl -s -4"] = sshtest.Reply{Out: "localhost\n"}
	a := sshtest.Start(t, aReplies)
	b := sshtest.Start(t, bReplies)
	trustAlias(t, b, "localhost")
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port, local: true},
		tnode{name: "b", host: "localhost", port: b.Port},
	)

	err, out := deployCapturing(t, m)

	if err != nil {
		t.Fatalf("Deploy: %v\n%s", err, out)
	}
	aConf, ok := writtenConfig(a)
	if !ok {
		t.Fatal("node a received no config")
	}
	bConf, ok := writtenConfig(b)
	if !ok {
		t.Fatal("node b received no config")
	}
	if !strings.Contains(aConf, "Endpoint = localhost:51820") {
		t.Errorf("the public node lost its endpoint:\n%s", aConf)
	}
	if strings.Contains(bConf, "Endpoint") {
		t.Errorf("the config of b carries an endpoint for the private peer a in a mixed mesh:\n%s", bConf)
	}
	if n := countCurl(a); n != 0 {
		t.Errorf("the private host was probed: %q", a.Commands())
	}
	if n := m.Nodes["a"]; n.PublicEndpoint != "" || n.BehindNAT {
		t.Errorf("the local node a was touched in a mixed mesh: endpoint %q behindNAT %v", n.PublicEndpoint, n.BehindNAT)
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

// An endpoint detectEndpoints wrote is not the operator's: a second call in the
// same process recomputes it, while an operator-set endpoint stays.
func TestDetectEndpointsTwiceRecomputesItsOwnEndpoints(t *testing.T) {
	lanHost(t)
	const operator = "vpn.example.com:443"
	a := sshtest.Start(t, nodeReplies())
	o := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port},
		tnode{name: "o", host: o.Host, port: o.Port},
	)
	m.Nodes["o"].PublicEndpoint = operator

	m.detectEndpoints()

	if got := m.Nodes["a"].PublicEndpoint; got != "127.0.0.1:51820" {
		t.Fatalf("first call: a has endpoint %q, want 127.0.0.1:51820", got)
	}
	m.Nodes["c"] = &Node{Hostname: "c", SSHHost: "127.0.0.2", SSHPort: closedPort(t), ListenPort: 51820}

	m.detectEndpoints()

	if n := m.Nodes["a"]; n.PublicEndpoint != "" || !n.BehindNAT {
		t.Errorf("second call: a has endpoint %q behindNAT %v, want none and true", n.PublicEndpoint, n.BehindNAT)
	}
	if got := m.Nodes["o"].PublicEndpoint; got != operator {
		t.Errorf("the operator-set endpoint is %q after two calls, want %q", got, operator)
	}
}
