package ssh

import (
	"path/filepath"
	"strings"
	"testing"
)

// NewClient must build the host key verifier before it dials anything: a
// missing known_hosts file is refused with instructions, not skipped. If the
// verifier were not wired in, this would fail with a connection error instead.
func TestNewClientVerifiesHostKeysByDefault(t *testing.T) {
	t.Setenv(EnvKnownHosts, filepath.Join(t.TempDir(), "absent"))
	t.Setenv(EnvInsecureHostKey, "")

	_, err := NewClient("127.0.0.1", 1)
	if err == nil {
		t.Fatal("NewClient succeeded without a known_hosts file")
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("error %q does not come from the host key check", err)
	}
}
