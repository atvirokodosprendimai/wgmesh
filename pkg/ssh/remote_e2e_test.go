package ssh_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh/sshtest"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// goldenPath holds the commands the unchanged code sent for the valid fixture
// (recorded before ADR-0006 T2 touched a builder). WGMESH_UPDATE_GOLDEN=1
// rewrites it; a change that is meant to alter a valid command says so here.
const goldenPath = "../../testdata/central/commands-valid.golden"

// namedServer pairs a test server with the node it plays, for the transcript.
type namedServer struct {
	name string
	srv  *sshtest.Server
}

// transcript renders every command a server received, one `$ command` line
// each, followed by its standard input as `< ` lines.
func transcript(servers ...namedServer) string {
	var b strings.Builder
	for _, ns := range servers {
		fmt.Fprintf(&b, "-- %s --\n", ns.name)
		for _, e := range ns.srv.Execs() {
			fmt.Fprintf(&b, "$ %s\n", e.Command)
			if e.Stdin == "" {
				continue
			}
			body := strings.TrimSuffix(e.Stdin, "\n")
			for _, line := range strings.Split(body, "\n") {
				fmt.Fprintf(&b, "< %s\n", line)
			}
			if !strings.HasSuffix(e.Stdin, "\n") {
				b.WriteString("< \\ no newline at end\n")
			}
		}
	}
	return b.String()
}

// fixture is the state a scenario deploys. The valid fixture is interface
// wg0, two nodes (each node has one peer, so the order of peers is fixed) and
// one routable network.
type fixture struct {
	iface         string
	alphaKey      string
	alphaNets     []string
	bravoEndpoint string
}

var validFixture = fixture{
	iface:     "wg0",
	alphaKey:  "ALPHAPUBLICKEYALPHAPUBLICKEYALPHAPUBLICKEY0=",
	alphaNets: []string{"192.168.10.0/24"},
}

// nodeReplies answers what a deployed node would: freshly installed (no wg
// binary, no interface) or already configured with a stale peer and stale
// routes. publicIP is what the node reports as its public address.
func nodeReplies(fresh bool, publicIP string) map[string]sshtest.Reply {
	r := map[string]sshtest.Reply{
		"hostname":    {Out: "node.example.net\n"},
		"hostname -f": {Out: "node.example.net\n"},
		"curl -s -4":  {Out: publicIP + "\n"},
	}
	if fresh {
		r["which wg"] = sshtest.Reply{Exit: 1}
		return r
	}
	r["which wg"] = sshtest.Reply{Out: "/usr/bin/wg\n"}
	r["wg show"] = sshtest.Reply{Out: "NODEPRIVATEKEYNODEPRIVATEKEYNODEPRIVATEKEY0=\tNODEPUBLICKEY\t51820\toff\n" +
		"OLDPEERKEYOLDPEERKEYOLDPEERKEYOLDPEERKEY00=\t(none)\t(none)\t10.99.0.9/32\t0\t0\t0\toff\n"}
	r["ip route show dev"] = sshtest.Reply{Out: "10.50.0.0/24 via 10.99.0.9 proto static\n192.168.10.0/24 scope link\n"}
	r["ip -4 -o addr show to 192.168.10.0/24"] = sshtest.Reply{
		Out: "2: eth1    inet 192.168.10.7/24 brd 192.168.10.255 scope global eth1\n"}
	return r
}

// deploy runs Mesh.Deploy for a two-node mesh whose nodes are two test
// servers and returns both servers.
func deploy(t *testing.T, fx fixture, replies map[string]sshtest.Reply) []namedServer {
	t.Helper()
	alpha := sshtest.Start(t, replies)
	bravo := sshtest.Start(t, replies)
	m := &mesh.Mesh{
		InterfaceName: fx.iface,
		Network:       "10.99.0.0/16",
		ListenPort:    51820,
		LocalHostname: "controller",
		Nodes: map[string]*mesh.Node{
			"alpha": {
				Hostname: "alpha", MeshIP: net.ParseIP("10.99.0.1"),
				PublicKey: fx.alphaKey, PrivateKey: "ALPHAPRIVATEKEYALPHAPRIVATEKEYALPHAPRIVATE0=",
				SSHHost: alpha.Host, SSHPort: alpha.Port, ListenPort: 51820,
				RoutableNetworks: fx.alphaNets,
			},
			"bravo": {
				Hostname: "bravo", MeshIP: net.ParseIP("10.99.0.2"),
				PublicKey: "BRAVOPUBLICKEYBRAVOPUBLICKEYBRAVOPUBLICKEY0=", PrivateKey: "BRAVOPRIVATEKEYBRAVOPRIVATEKEYBRAVOPRIVATE0=",
				SSHHost: bravo.Host, SSHPort: bravo.Port, ListenPort: 51820,
				PublicEndpoint: fx.bravoEndpoint,
			},
		},
	}
	if err := m.Deploy(); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	return []namedServer{{"alpha", alpha}, {"bravo", bravo}}
}

// direct calls the reachable functions the full deploy does not hit with a
// valid state: a pre-shared key, a peer with and without endpoint and
// keepalive, a route without gateway, an IPv6 connected route, and the
// persistent update of an online diff.
func direct(t *testing.T) namedServer {
	t.Helper()
	srv := sshtest.Start(t, map[string]sshtest.Reply{
		"ip -6 -o addr show to": {Out: "3: eth0    inet6 fd00:1::5/64 scope global\n"},
	})
	client, err := ssh.NewClient(srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	diff := &wireguard.ConfigDiff{
		RemovedPeers: []string{"OLDPEERKEYOLDPEERKEYOLDPEERKEYOLDPEERKEY00="},
		AddedPeers: map[string]wireguard.Peer{
			"NEWPEERKEYNEWPEERKEYNEWPEERKEYNEWPEERKEY00=": {
				PublicKey:           "NEWPEERKEYNEWPEERKEYNEWPEERKEYNEWPEERKEY00=",
				PresharedKey:        "PSKPSKPSKPSKPSKPSKPSKPSKPSKPSKPSKPSKPSKPSK0=",
				Endpoint:            "198.51.100.7:51820",
				AllowedIPs:          []string{"10.99.0.7/32", "192.168.70.0/24"},
				PersistentKeepalive: 25,
			},
		},
		ModifiedPeers: map[string]wireguard.Peer{
			"MODPEERKEYMODPEERKEYMODPEERKEYMODPEERKEY00=": {
				PublicKey: "MODPEERKEYMODPEERKEYMODPEERKEYMODPEERKEY00=",
			},
		},
	}
	if err := wireguard.ApplyDiff(client, "wg0", diff); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}

	full := &wireguard.FullConfig{
		Interface: wireguard.WGInterface{PrivateKey: "DIRECTPRIVATEKEYDIRECTPRIVATEKEYDIRECTPRIV0=", Address: "10.99.0.5/16", ListenPort: 51820},
		Peers: []wireguard.WGPeer{
			{PublicKey: "PEERONEPEERONEPEERONEPEERONEPEERONEPEER00=", Endpoint: "198.51.100.1:51820", AllowedIPs: []string{"10.99.0.1/32", "192.168.10.0/24"}, PersistentKeepalive: 5},
			{PublicKey: "PEERTWOPEERTWOPEERTWOPEERTWOPEERTWOPEER00=", AllowedIPs: []string{"10.99.0.2/32"}},
		},
	}
	routes := []ssh.RouteEntry{{Network: "192.168.10.0/24", Gateway: "10.99.0.1"}, {Network: "192.168.20.0/24", Gateway: "10.99.0.2"}}
	if err := wireguard.UpdatePersistentConfig(client, "wg0", full, routes, diff); err != nil {
		t.Fatalf("UpdatePersistentConfig: %v", err)
	}
	if _, err := wireguard.GetCurrentConfig(client, "wg0"); err == nil {
		t.Fatalf("GetCurrentConfig: want an error for an empty dump")
	}

	toAdd := []ssh.RouteEntry{{Network: "10.7.0.0/24", Gateway: "10.99.0.1"}, {Network: "10.8.0.0/24"}}
	toRemove := []ssh.RouteEntry{{Network: "10.5.0.0/24", Gateway: "10.99.0.3"}, {Network: "10.6.0.0/24"}}
	if err := ssh.ApplyRouteDiff(client, "wg0", toAdd, toRemove); err != nil {
		t.Fatalf("ApplyRouteDiff: %v", err)
	}
	ssh.RestoreConnectedRoute(client, "fd00:1::/64")
	return namedServer{"direct", srv}
}

// TestRecordedCommandsMatchGolden runs the valid fixture through the test
// server and compares every command, and every byte sent on standard input,
// with the text the unchanged code produced.
func TestRecordedCommandsMatchGolden(t *testing.T) {
	var b strings.Builder
	b.WriteString("=== deploy, fresh nodes ===\n")
	b.WriteString(transcript(deploy(t, validFixture, nodeReplies(true, "127.0.0.1"))...))
	b.WriteString("=== deploy, configured nodes ===\n")
	b.WriteString(transcript(deploy(t, validFixture, nodeReplies(false, "127.0.0.1"))...))
	b.WriteString("=== deploy, configured nodes, route query fails ===\n")
	failing := nodeReplies(false, "127.0.0.1")
	failing["ip route show dev"] = sshtest.Reply{Exit: 1}
	b.WriteString(transcript(deploy(t, validFixture, failing)...))
	b.WriteString("=== direct calls ===\n")
	b.WriteString(transcript(direct(t)))
	got := b.String()

	if os.Getenv("WGMESH_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("commands for the valid fixture differ from %s.\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}

// TestSSHTestHostKeyIsVerified shows that the host key check of PR #876 runs
// against the test server: the server's key is trusted through the
// known_hosts file Start wrote, and a different key for the same address is
// refused.
func TestSSHTestHostKeyIsVerified(t *testing.T) {
	if sshtest.EnvKnownHosts != ssh.EnvKnownHosts || sshtest.EnvInsecureHostKey != ssh.EnvInsecureHostKey {
		t.Fatalf("sshtest names %s and %s, pkg/ssh names %s and %s",
			sshtest.EnvKnownHosts, sshtest.EnvInsecureHostKey, ssh.EnvKnownHosts, ssh.EnvInsecureHostKey)
	}
	srv := sshtest.Start(t, nil)

	client, err := ssh.NewClient(srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("NewClient with the server's key in known_hosts: %v", err)
	}
	client.Close()

	sshtest.Start(t, nil)
	path := os.Getenv(ssh.EnvKnownHosts)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Move the first server's address onto the second server's key.
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("known_hosts has %d lines, want 2:\n%s", len(lines), data)
	}
	otherKey := strings.SplitN(lines[1], " ", 2)[1]
	firstAddr := strings.SplitN(lines[0], " ", 2)[0]
	if err := os.WriteFile(path, []byte(firstAddr+" "+otherKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if client, err := ssh.NewClient(srv.Host, srv.Port); err == nil {
		client.Close()
		t.Fatalf("NewClient accepted a host key that is not in known_hosts")
	} else if !strings.Contains(err.Error(), "CHANGED") {
		t.Fatalf("NewClient error = %v, want a changed-host-key refusal", err)
	}
}

// TestIPv6EndpointIsQuotedOnWgSet records the wg set command for a peer whose
// endpoint is an IPv6 address with brackets. The brackets are not in the safe
// set of ssh.Quote, so the endpoint is sent as one single-quoted word, which
// sh reads back as the same text. (The command is not in commands-valid.golden:
// that file is recorded from the old code and stays as it was.)
func TestIPv6EndpointIsQuotedOnWgSet(t *testing.T) {
	const endpoint = "[2001:db8::1]:51820"
	const key = "V6PEERKEYV6PEERKEYV6PEERKEYV6PEERKEYV6PEER0="
	srv := sshtest.Start(t, nil)
	client, err := ssh.NewClient(srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	diff := &wireguard.ConfigDiff{AddedPeers: map[string]wireguard.Peer{
		key: {PublicKey: key, Endpoint: endpoint, AllowedIPs: []string{"10.99.0.8/32"}},
	}}
	if err := wireguard.ApplyDiff(client, "wg0", diff); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}

	var found bool
	for _, cmd := range srv.Commands() {
		if !strings.Contains(cmd, "peer "+key) {
			continue
		}
		found = true
		if !strings.Contains(cmd, " endpoint '"+endpoint+"' ") {
			t.Errorf("endpoint is not one single-quoted word in %q", cmd)
		}
		words, _, ok := splitWords(cmd)
		if !ok {
			t.Fatalf("unbalanced quote in %q", cmd)
		}
		var got string
		for i, w := range words {
			if w == "endpoint" && i+1 < len(words) {
				got = words[i+1]
			}
		}
		if got != endpoint {
			t.Errorf("sh reads the endpoint as %q, want %q (command %q)", got, endpoint, cmd)
		}
	}
	if !found {
		t.Fatalf("no wg set command for the peer; got %q", srv.Commands())
	}
}

// splitWords reads cmd the way sh does for the quoting this repository
// produces: single quotes, and a backslash outside quotes. It returns the
// words, the text that sh would read as syntax (everything outside quotes
// and not backslash-escaped) and false when a quote is left open.
func splitWords(cmd string) (words []string, syntax string, ok bool) {
	var word, syn strings.Builder
	inWord, inQuote := false, false
	flush := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inQuote:
			if c == '\'' {
				inQuote = false
			} else {
				word.WriteByte(c)
			}
		case c == '\'':
			inQuote, inWord = true, true
		case c == '\\' && i+1 < len(cmd):
			i++
			word.WriteByte(cmd[i])
			inWord = true
		case c == ' ' || c == '\t' || c == '\n':
			flush()
			syn.WriteByte(c)
		default:
			word.WriteByte(c)
			syn.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return words, syn.String(), !inQuote
}

// TestHostileStateCommandsAreOneWordEach drives the remote builders with an
// interface name, public key, routable network and endpoint that each end in
// `; touch /x` and checks every command the node received: no shell syntax (a
// `;`) and no word of its own for the injected text, and each hostile value
// present as part of one word. Quoting is the second layer; validation is the
// first, so the last step checks that Deploy refuses the same state with a
// class A message before it sends any command.
func TestHostileStateCommandsAreOneWordEach(t *testing.T) {
	const marker = "; touch /x"
	iface := "wg0" + marker
	key := "ALPHAKEY" + marker
	route := "10.0.0.0/8" + marker
	endpoint := "198.51.100.1:51820" + marker
	values := map[string]string{
		"interface name": iface,
		"public key":     key,
		"route":          route,
		"endpoint":       endpoint,
	}

	srv := sshtest.Start(t, map[string]sshtest.Reply{})
	client, err := ssh.NewClient(srv.Host, srv.Port)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	peer := wireguard.Peer{PublicKey: key, Endpoint: endpoint, AllowedIPs: []string{route}, PersistentKeepalive: 25}
	diff := &wireguard.ConfigDiff{
		RemovedPeers:  []string{key},
		AddedPeers:    map[string]wireguard.Peer{key: peer},
		ModifiedPeers: map[string]wireguard.Peer{key: peer},
	}
	full := &wireguard.FullConfig{
		Interface: wireguard.WGInterface{PrivateKey: "PRIVATEKEY", Address: "10.99.0.5/16", ListenPort: 51820},
		Peers:     []wireguard.WGPeer{{PublicKey: key, Endpoint: endpoint, AllowedIPs: []string{route}}},
	}
	routes := []ssh.RouteEntry{{Network: route, Gateway: "10.99.0.1"}, {Network: route}}
	// The builders may fail on a hostile value; only what they sent matters.
	_ = wireguard.ApplyDiff(client, iface, diff)
	_ = wireguard.UpdatePersistentConfig(client, iface, full, routes, diff)
	_, _ = wireguard.GetCurrentConfig(client, iface)
	_ = ssh.ApplyRouteDiff(client, iface, routes, routes)
	client.Close()

	cmds := srv.Commands()
	if len(cmds) == 0 {
		t.Fatal("the builders sent no command")
	}
	seen := map[string]bool{}
	for _, cmd := range cmds {
		words, syntax, ok := splitWords(cmd)
		if !ok {
			t.Errorf("unbalanced quote in %q", cmd)
			continue
		}
		if strings.Contains(syntax, ";") {
			t.Errorf("`;` outside quotes in %q", cmd)
		}
		for _, w := range words {
			if w == "touch" || w == "/x" {
				t.Errorf("injected word %q in %q", w, cmd)
			}
			for name, v := range values {
				if strings.Contains(w, v) {
					seen[name] = true
				}
			}
		}
	}
	for name := range values {
		if !seen[name] {
			t.Errorf("no command carries the hostile %s inside one word", name)
		}
	}

	// First layer: Deploy refuses the same state, class A, before any command.
	alpha := sshtest.Start(t, nodeReplies(true, "203.0.113.5"))
	bravo := sshtest.Start(t, nodeReplies(true, "203.0.113.5"))
	m := &mesh.Mesh{
		InterfaceName: iface, Network: "10.99.0.0/16", ListenPort: 51820, LocalHostname: "controller",
		Nodes: map[string]*mesh.Node{
			"alpha": {
				Hostname: "alpha", MeshIP: net.ParseIP("10.99.0.1"), PublicKey: key,
				PrivateKey: "ALPHAPRIVATEKEYALPHAPRIVATEKEYALPHAPRIVATE0=",
				SSHHost:    alpha.Host, SSHPort: alpha.Port, ListenPort: 51820,
				RoutableNetworks: []string{route},
			},
			"bravo": {
				Hostname: "bravo", MeshIP: net.ParseIP("10.99.0.2"),
				PublicKey:  "BRAVOPUBLICKEYBRAVOPUBLICKEYBRAVOPUBLICKEY0=",
				PrivateKey: "BRAVOPRIVATEKEYBRAVOPRIVATEKEYBRAVOPRIVATE0=",
				SSHHost:    bravo.Host, SSHPort: bravo.Port, ListenPort: 51820,
				PublicEndpoint: endpoint,
			},
		},
	}
	err = m.Deploy()
	if err == nil {
		t.Fatal("Deploy accepted a hostile state")
	}
	if !strings.Contains(err.Error(), "(class A)") {
		t.Errorf("want a class A refusal, got %v", err)
	}
	if n := len(alpha.Commands()) + len(bravo.Commands()); n != 0 {
		t.Errorf("Deploy sent %d command(s) before refusing", n)
	}
}
