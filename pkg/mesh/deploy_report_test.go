package mesh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh/sshtest"
)

// tnode is one node of a test mesh: where it is dialled and whether it is the
// local one.
type tnode struct {
	name  string
	host  string
	port  int
	local bool
}

// testKey is a 44 character key made of one repeated letter, so every node's
// key is distinct and recognisable in a rendered config.
func testKey(c byte) string {
	return strings.Repeat(string(rune(c)), 43) + "="
}

// newTestMesh builds a mesh whose i-th node has public key testKey('A'+i) and
// private key testKey('a'+i).
func newTestMesh(nodes ...tnode) *Mesh {
	m := &Mesh{
		InterfaceName: "wg0", Network: "10.99.0.0/16", ListenPort: 51820,
		LocalHostname: "controller",
		Nodes:         map[string]*Node{},
	}
	for i, n := range nodes {
		m.Nodes[n.name] = &Node{
			Hostname: n.name, MeshIP: net.ParseIP(fmt.Sprintf("10.99.0.%d", i+1)),
			PublicKey: testKey(byte('A' + i)), PrivateKey: testKey(byte('a' + i)),
			SSHHost: n.host, SSHPort: n.port, ListenPort: 51820, IsLocal: n.local,
		}
	}
	return m
}

// nodeReplies answers a node on which wireguard-tools is installed and no
// interface exists yet, so the deploy takes the fresh-configuration path.
func nodeReplies() map[string]sshtest.Reply {
	return map[string]sshtest.Reply{
		"hostname":    {Out: "node.example.net\n"},
		"hostname -f": {Out: "node.example.net\n"},
		"curl -s -4":  {Out: "203.0.113.9\n"},
		"which wg":    {Out: "/usr/bin/wg\n"},
	}
}

// closedPort returns a local port nothing listens on: a dial is refused at once.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// confWrite is the first command of a server that writes the wg-quick file.
const confWrite = "cat > /etc/wireguard/wg0.conf"

// writtenConfig returns what a server received as the content of the wg-quick
// file, and whether it received a write at all.
func writtenConfig(s *sshtest.Server) (string, bool) {
	for _, e := range s.Execs() {
		if strings.HasPrefix(e.Command, confWrite) {
			return e.Stdin, true
		}
	}
	return "", false
}

// deployCapturing runs m.Deploy with standard output captured, bounded at 60 s.
func deployCapturing(t *testing.T, m *Mesh) (error, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	outC := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		outC <- string(b)
	}()
	errC := make(chan error, 1)
	go func() { errC <- m.Deploy() }()

	var deployErr error
	select {
	case deployErr = <-errC:
	case <-time.After(60 * time.Second):
		os.Stdout = old
		t.Fatal("Deploy did not return within 60s")
	}
	w.Close()
	os.Stdout = old
	return deployErr, <-outC
}

// failureLines returns the lines of the error text that report host.
func failureLines(err error, host string) []string {
	if err == nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(l, "  "+host+": ") {
			lines = append(lines, l)
		}
	}
	return lines
}

// wantFailure asserts that the error is a *DeployError that reports host once,
// with the phase.
func wantFailure(t *testing.T, err error, host string, phase Phase) {
	t.Helper()
	var de *DeployError
	if !errors.As(err, &de) {
		t.Fatalf("Deploy returned %T (%v), want *DeployError", err, err)
	}
	lines := failureLines(err, host)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "  "+host+": "+string(phase)+": ") {
		t.Errorf("want exactly one line %q, got %q in:\n%v", "  "+host+": "+string(phase)+": ...", lines, err)
	}
}

func TestDeployContinuesPastUnreachableNode(t *testing.T) {
	b := sshtest.Start(t, nodeReplies())
	c := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		tnode{name: "a", host: "127.0.0.1", port: closedPort(t)},
		tnode{name: "b", host: b.Host, port: b.Port},
		tnode{name: "c", host: c.Host, port: c.Port},
	)

	err, _ := deployCapturing(t, m)

	wantFailure(t, err, "a", PhaseDetect)
	for name, srv := range map[string]*sshtest.Server{"b": b, "c": c} {
		conf, ok := writtenConfig(srv)
		if !ok {
			t.Errorf("node %s received no config: the run stopped at the failed node", name)
			continue
		}
		// The failed node stays part of the others' desired state.
		if !strings.Contains(conf, "PublicKey = "+testKey('A')) {
			t.Errorf("config of %s lacks the peer a:\n%s", name, conf)
		}
	}
}

func TestDeployReportsEveryFailureWithPhase(t *testing.T) {
	bReplies := nodeReplies()
	bReplies["systemctl restart"] = sshtest.Reply{Exit: 1}
	b := sshtest.Start(t, bReplies)
	c := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		// A local node is not probed, so its closed port fails at connect.
		tnode{name: "a", host: "127.0.0.1", port: closedPort(t), local: true},
		tnode{name: "b", host: b.Host, port: b.Port},
		tnode{name: "c", host: c.Host, port: c.Port},
	)

	err, _ := deployCapturing(t, m)

	wantFailure(t, err, "a", PhaseConnect)
	wantFailure(t, err, "b", PhaseApply)
	if err != nil && !strings.Contains(err.Error(), "configured: c") {
		t.Errorf("error does not list the configured node c:\n%v", err)
	}
	if _, ok := writtenConfig(c); !ok {
		t.Error("node c received no config")
	}
}

func TestDeployAllReachableReturnsNil(t *testing.T) {
	var servers []*sshtest.Server
	var nodes []tnode
	for _, name := range []string{"a", "b", "c"} {
		s := sshtest.Start(t, nodeReplies())
		servers = append(servers, s)
		nodes = append(nodes, tnode{name: name, host: s.Host, port: s.Port})
	}

	err, out := deployCapturing(t, newTestMesh(nodes...))

	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if n := strings.Count(out, "✓ Deployed successfully"); n != 3 {
		t.Errorf("success lines = %d, want 3:\n%s", n, out)
	}
	for i, s := range servers {
		if _, ok := writtenConfig(s); !ok {
			t.Errorf("node %d received no config", i)
		}
	}
}

func TestDeployVisitsNodesInSortedOrder(t *testing.T) {
	names := []string{"delta", "alpha", "charlie", "bravo"}
	want := []string{"alpha", "bravo", "charlie", "delta"}
	for run := 0; run < 20; run++ {
		var nodes []tnode
		for _, name := range names {
			s := sshtest.Start(t, nodeReplies())
			nodes = append(nodes, tnode{name: name, host: s.Host, port: s.Port})
		}

		err, out := deployCapturing(t, newTestMesh(nodes...))
		if err != nil {
			t.Fatalf("run %d: Deploy: %v", run, err)
		}

		// The detect step and the apply loop each visit the nodes once.
		var detected, deployed []string
		for _, line := range strings.Split(out, "\n") {
			if f := strings.Fields(line); len(f) > 2 && f[0] == "Detected" {
				detected = append(detected, f[1])
			}
			if name, ok := strings.CutPrefix(line, "Deploying to "); ok {
				deployed = append(deployed, strings.TrimSuffix(name, "..."))
			}
		}
		if strings.Join(detected, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d: detect order %v, want %v", run, detected, want)
		}
		if strings.Join(deployed, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d: apply order %v, want %v", run, deployed, want)
		}
	}
	if !sort.StringsAreSorted(want) {
		t.Fatal("test table: want is not sorted")
	}
}

func TestDeployDoesNotRedialAFailedDetect(t *testing.T) {
	// The node accepts a connection and closes it, so the dial fails in the
	// handshake and every dial of the node is counted.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var accepts atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			c.Close()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		wg.Wait()
	})
	b := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		tnode{name: "a", host: "127.0.0.1", port: ln.Addr().(*net.TCPAddr).Port},
		tnode{name: "b", host: b.Host, port: b.Port},
	)

	deployErr, _ := deployCapturing(t, m)

	wantFailure(t, deployErr, "a", PhaseDetect)
	if _, ok := writtenConfig(b); !ok {
		t.Error("node b received no config")
	}
	// A dial cannot return before the node closed its connection, so the
	// count read after Deploy returns sees every dial.
	if n := accepts.Load(); n != 1 {
		t.Errorf("node a was dialled %d times, want 1", n)
	}
}

// persistReplies answers a node that is configured and in sync (so the deploy
// takes the update path and ends with the config file write) and refuses the
// write of the file.
func persistReplies() map[string]sshtest.Reply {
	r := nodeReplies()
	r["wg show"] = sshtest.Reply{Out: testKey('x') + "\tNODEPUBLICKEY\t51820\toff\n"}
	r[confWrite] = sshtest.Reply{Exit: 1}
	return r
}

func writeAttempts(s *sshtest.Server) int {
	n := 0
	for _, c := range s.Commands() {
		if strings.HasPrefix(c, confWrite) {
			n++
		}
	}
	return n
}

func TestPersistWriteFailureIsAWarningAtN(t *testing.T) {
	a := sshtest.Start(t, persistReplies())
	m := newTestMesh(tnode{name: "a", host: a.Host, port: a.Port})

	err, out := deployCapturing(t, m)

	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if n := writeAttempts(a); n != 1 {
		t.Fatalf("config file writes attempted = %d, want 1", n)
	}
	if n := strings.Count(out, "Warning: failed to update config file"); n != 1 {
		t.Errorf("warnings printed = %d, want 1:\n%s", n, out)
	}
}

func TestPersistWriteFailureFailsWhenFatal(t *testing.T) {
	PersistFailureFatal = true
	t.Cleanup(func() { PersistFailureFatal = false })
	a := sshtest.Start(t, persistReplies())
	m := newTestMesh(tnode{name: "a", host: a.Host, port: a.Port})

	err, _ := deployCapturing(t, m)

	wantFailure(t, err, "a", PhasePersist)
}

func TestDeployRenderErrorIsAPersistFailureAndLeavesTheNodeUntouched(t *testing.T) {
	a := sshtest.Start(t, nodeReplies())
	b := sshtest.Start(t, nodeReplies())
	m := newTestMesh(
		tnode{name: "a", host: a.Host, port: a.Port},
		tnode{name: "b", host: b.Host, port: b.Port},
	)
	secret := "SECRETPRIVATEKEY"
	m.Nodes["a"].PrivateKey = secret + "\nPostUp = id"

	err, out := deployCapturing(t, m)

	wantFailure(t, err, "a", PhasePersist)
	if err != nil && strings.Contains(err.Error(), secret) || strings.Contains(out, secret) {
		t.Errorf("a private key reached the error or the output:\n%v\n%s", err, out)
	}
	// Only the read-only probes of the detect step reached node a.
	for _, c := range a.Commands() {
		if !strings.HasPrefix(c, "hostname") && !strings.HasPrefix(c, "curl") {
			t.Errorf("node a received %q after a render refusal", c)
		}
	}
	if _, ok := writtenConfig(b); !ok {
		t.Error("node b received no config")
	}
}
