package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func newKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func writeKnownHosts(t *testing.T, host string, key ssh.PublicKey) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{host}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var remote = &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}

func TestHostKeyCallbackAcceptsKnownKey(t *testing.T) {
	t.Parallel()
	key := newKey(t)
	cb, err := newHostKeyCallback(writeKnownHosts(t, "node1.example.com", key), false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("node1.example.com:22", remote, key); err != nil {
		t.Errorf("known key refused: %v", err)
	}
}

func TestHostKeyCallbackRefusesUnknownHost(t *testing.T) {
	t.Parallel()
	cb, err := newHostKeyCallback(writeKnownHosts(t, "other.example.com", newKey(t)), false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	err = cb("node1.example.com:22", remote, newKey(t))
	if err == nil {
		t.Fatal("unknown host accepted")
	}
	for _, want := range []string{"not in", "ssh-keyscan", "SHA256:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestHostKeyCallbackRefusesChangedKey(t *testing.T) {
	t.Parallel()
	cb, err := newHostKeyCallback(writeKnownHosts(t, "node1.example.com", newKey(t)), false, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	err = cb("node1.example.com:22", remote, newKey(t))
	if err == nil {
		t.Fatal("changed host key accepted")
	}
	if !strings.Contains(err.Error(), "CHANGED") {
		t.Errorf("error %q does not say the key changed", err)
	}
}

func TestHostKeyCallbackMissingFileIsAnError(t *testing.T) {
	t.Parallel()
	_, err := newHostKeyCallback(filepath.Join(t.TempDir(), "absent"), false, &bytes.Buffer{})
	if err == nil {
		t.Fatal("missing known_hosts accepted: the default must fail closed")
	}
	for _, want := range []string{"ssh-keyscan", EnvKnownHosts, EnvInsecureHostKey} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestHostKeyCallbackInsecureOptOutWarns(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	cb, err := newHostKeyCallback(filepath.Join(t.TempDir(), "absent"), true, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("anything:22", remote, newKey(t)); err != nil {
		t.Errorf("insecure mode refused a key: %v", err)
	}
	if !strings.Contains(warn.String(), "WARNING") {
		t.Errorf("opt-out was silent: %q", warn.String())
	}
}
