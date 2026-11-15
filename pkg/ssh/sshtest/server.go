// Package sshtest is an in-process SSH server for tests of the centralized
// deploy. It accepts any client without authentication, records every `exec`
// request (command and standard input) and answers from a table.
//
// Start writes the server's host key into a temporary known_hosts file and
// points WGMESH_SSH_KNOWN_HOSTS at it with t.Setenv, so the client's host key
// verification (PR #876) runs for real. It also clears
// WGMESH_SSH_INSECURE_HOST_KEY so an operator's environment cannot switch the
// verification off under a test. Because of t.Setenv, a test that calls Start
// cannot be parallel.
//
// The package imports only golang.org/x/crypto, never pkg/ssh, so a test inside
// package ssh can use it. It is meant for _test.go files only: a non-test
// import would put a test server into the binary.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	// EnvKnownHosts is pkg/ssh.EnvKnownHosts, repeated here because this
	// package must not import pkg/ssh; a test in pkg/ssh pins the two equal.
	EnvKnownHosts = "WGMESH_SSH_KNOWN_HOSTS"
	// EnvInsecureHostKey is pkg/ssh.EnvInsecureHostKey, see above.
	EnvInsecureHostKey = "WGMESH_SSH_INSECURE_HOST_KEY"
)

// Reply is the answer to one command: its output and its exit status.
type Reply struct {
	Out  string
	Exit int
}

// Exec is one command the server received and what the client sent on the
// command's standard input.
type Exec struct {
	Command string
	Stdin   string
}

// Server is a running test server on 127.0.0.1.
type Server struct {
	// Host and Port are what a client dials.
	Host string
	Port int

	replies map[string]Reply

	mu    sync.Mutex
	execs []Exec
}

// knownHostsFiles maps a test name to the known_hosts file its servers share,
// so a test that starts two servers trusts both.
var (
	knownHostsMu    sync.Mutex
	knownHostsFiles = map[string]string{}
)

// Start runs a server for the test and stops it when the test ends. A command
// is answered by the entry of replies whose key is the longest prefix of the
// command (an exact command is a prefix of itself); a command with no such
// entry succeeds with empty output.
func Start(t testing.TB, replies map[string]Reply) *Server {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("sshtest: host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("sshtest: host key signer: %v", err)
	}
	hostPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("sshtest: host public key: %v", err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("sshtest: listen: %v", err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	s := &Server{Host: "127.0.0.1", Port: addr.Port, replies: replies}

	trustHostKey(t, net.JoinHostPort(s.Host, strconv.Itoa(s.Port)), hostPub)

	var (
		wg    sync.WaitGroup
		connM sync.Mutex
		conns []net.Conn
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			connM.Lock()
			conns = append(conns, nc)
			connM.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.serve(nc, cfg)
			}()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		connM.Lock()
		for _, c := range conns {
			c.Close()
		}
		connM.Unlock()
		wg.Wait()
	})
	return s
}

// Execs returns every command received so far, in arrival order.
func (s *Server) Execs() []Exec {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Exec(nil), s.execs...)
}

// Commands returns the text of every command received so far.
func (s *Server) Commands() []string {
	execs := s.Execs()
	cmds := make([]string, len(execs))
	for i, e := range execs {
		cmds[i] = e.Command
	}
	return cmds
}

func (s *Server) reply(cmd string) Reply {
	best, found := "", false
	for key := range s.replies {
		if strings.HasPrefix(cmd, key) && (!found || len(key) > len(best)) {
			best, found = key, true
		}
	}
	if !found {
		return Reply{}
	}
	return s.replies[best]
}

func (s *Server) serve(nc net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nc, cfg)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "sshtest accepts session channels only")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.session(ch, chReqs)
	}
}

// session answers the first exec request of a channel and closes it. The
// client closes its side of the channel after the command's standard input,
// so reading to EOF always ends.
func (s *Server) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			req.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			req.Reply(false, nil)
			return
		}
		req.Reply(true, nil)

		stdin, _ := io.ReadAll(ch)
		s.mu.Lock()
		s.execs = append(s.execs, Exec{Command: payload.Command, Stdin: string(stdin)})
		s.mu.Unlock()

		r := s.reply(payload.Command)
		io.WriteString(ch, r.Out)
		status := struct{ Status uint32 }{uint32(r.Exit)}
		ch.SendRequest("exit-status", false, ssh.Marshal(&status))
		return
	}
}

// trustHostKey appends the server's key to the known_hosts file of the test,
// creating the file and pointing the environment at it on the first call.
func trustHostKey(t testing.TB, addr string, key ssh.PublicKey) {
	t.Helper()

	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	path, ok := knownHostsFiles[t.Name()]
	if !ok {
		path = filepath.Join(t.TempDir(), "known_hosts")
		knownHostsFiles[t.Name()] = path
		name := t.Name()
		t.Cleanup(func() {
			knownHostsMu.Lock()
			delete(knownHostsFiles, name)
			knownHostsMu.Unlock()
		})
		t.Setenv(EnvKnownHosts, path)
		t.Setenv(EnvInsecureHostKey, "")
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("sshtest: known_hosts: %v", err)
	}
	defer f.Close()
	if _, err := io.WriteString(f, knownhosts.Line([]string{addr}, key)+"\n"); err != nil {
		t.Fatalf("sshtest: known_hosts: %v", err)
	}
}
