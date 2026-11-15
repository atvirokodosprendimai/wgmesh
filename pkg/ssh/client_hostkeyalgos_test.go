package ssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// startHostKeyServer runs an in-process SSH server that holds an ECDSA and
// an Ed25519 host key and accepts any client. It returns the listen address
// and the two public keys.
func startHostKeyServer(t *testing.T) (addr string, ecdsaKey, ed25519Key ssh.PublicKey) {
	t.Helper()
	ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	for _, priv := range []any{ecPriv, edPriv} {
		signer, err := ssh.NewSignerFromKey(priv)
		if err != nil {
			t.Fatal(err)
		}
		config.AddHostKey(signer)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					ch.Reject(ssh.Prohibited, "test server")
				}
				sconn.Close()
			}()
		}
	}()

	ecPub, err := ssh.NewPublicKey(&ecPriv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	edPub, err := ssh.NewPublicKey(edPriv.Public())
	if err != nil {
		t.Fatal(err)
	}
	return ln.Addr().String(), ecPub, edPub
}

func newRSAKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func authorizedKey(key ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

// NewClient must prefer the key types known_hosts holds for the host. The
// server holds ECDSA and Ed25519 keys; the library default offers ECDSA first.
func TestNewClientHostKeyAlgorithms(t *testing.T) {
	addr, ecKey, edKey := startHostKeyServer(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	entry := knownhosts.Normalize(addr) // "[127.0.0.1]:<port>"

	tests := []struct {
		name       string
		knownHosts string
		wantErr    string // "" means the connection must succeed
	}{
		{"only ed25519", knownhosts.Line([]string{addr}, edKey), ""},
		{"only ecdsa", knownhosts.Line([]string{addr}, ecKey), ""},
		{"both", knownhosts.Line([]string{addr}, edKey) + "\n" + knownhosts.Line([]string{addr}, ecKey), ""},
		{"only rsa", knownhosts.Line([]string{addr}, newRSAKey(t)), "CHANGED"},
		{"host unknown", knownhosts.Line([]string{"192.0.2.9:22"}, edKey), "is not in"},
		{"hashed ed25519", knownhosts.HashHostname(entry) + " " + authorizedKey(edKey), ""},
		{
			// A port-22 entry for the same IP with another key type must not
			// steer the choice for the non-default port.
			"non-default port ed25519",
			host + " " + authorizedKey(ecKey) + "\n[" + host + "]:" + port + " " + authorizedKey(edKey),
			"",
		},
		{
			"revoked line plus ed25519",
			"@revoked * " + authorizedKey(newKey(t)) + "\n" + knownhosts.Line([]string{addr}, edKey),
			"",
		},
		{
			// An Ed25519 CA for the host must not make the client ask for the
			// plain Ed25519 key, which is not trusted; the default order picks
			// the trusted ECDSA key.
			"cert-authority plus ecdsa",
			"@cert-authority " + entry + " " + authorizedKey(newKey(t)) + "\n" + knownhosts.Line([]string{addr}, ecKey),
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "known_hosts")
			if err := os.WriteFile(path, []byte(tt.knownHosts+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(EnvKnownHosts, path)
			t.Setenv(EnvInsecureHostKey, "")
			t.Setenv("HOME", t.TempDir())
			t.Setenv("SSH_AUTH_SOCK", "")

			var portNum int
			if _, err := fmt.Sscan(port, &portNum); err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(host, portNum)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NewClient: %v", err)
				}
				client.Close()
				return
			}
			if err == nil {
				client.Close()
				t.Fatalf("NewClient succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
