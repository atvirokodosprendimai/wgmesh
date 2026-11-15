package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	// EnvKnownHosts names the known_hosts file used to verify node host keys.
	// Default: ~/.ssh/known_hosts.
	EnvKnownHosts = "WGMESH_SSH_KNOWN_HOSTS"
	// EnvInsecureHostKey set to "1" turns host key verification off. It exists
	// for lab setups; a deployment writes every node's WireGuard private key
	// over this connection, so an unverified host can read them.
	EnvInsecureHostKey = "WGMESH_SSH_INSECURE_HOST_KEY"
)

// defaultHostKeyCallback builds the verifier from the environment.
func defaultHostKeyCallback() (ssh.HostKeyCallback, error) {
	path := os.Getenv(EnvKnownHosts)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot locate known_hosts (no home directory: %v): set %s", err, EnvKnownHosts)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	return newHostKeyCallback(path, os.Getenv(EnvInsecureHostKey) == "1", os.Stderr)
}

// newHostKeyCallback verifies host keys against the known_hosts file at path.
// With insecure set it accepts every key and says so on warn.
func newHostKeyCallback(path string, insecure bool, warn io.Writer) (ssh.HostKeyCallback, error) {
	if insecure {
		fmt.Fprintf(warn, "WARNING: SSH host key verification is disabled (%s=1); a man in the middle can read every key this deploy sends\n", EnvInsecureHostKey)
		return ssh.InsecureIgnoreHostKey(), nil
	}

	verify, err := knownhosts.New(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("SSH host keys cannot be verified: %s does not exist. Add each node's key (check its fingerprint first): ssh-keyscan -H <host> >> %s. Point %s at another file, or set %s=1 to skip verification (not recommended)",
				path, path, EnvKnownHosts, EnvInsecureHostKey)
		}
		return nil, fmt.Errorf("reading known_hosts %s: %w", path, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		host, port, splitErr := net.SplitHostPort(hostname)
		if splitErr != nil {
			host, port = hostname, "22"
		}
		fingerprint := ssh.FingerprintSHA256(key)
		if len(keyErr.Want) == 0 {
			return fmt.Errorf("host %s is not in %s (it presented %s %s). Verify the fingerprint with the host's owner, then add it: ssh-keyscan -p %s %s >> %s",
				hostname, path, key.Type(), fingerprint, port, host, path)
		}
		return fmt.Errorf("HOST KEY FOR %s CHANGED: it presented %s %s, which does not match %s. Possible man-in-the-middle attack; if the host was reinstalled, remove its old line from the file and add the new key",
			hostname, key.Type(), fingerprint, path)
	}, nil
}
