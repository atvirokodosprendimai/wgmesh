package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

// knownHostsPath returns the known_hosts file named by the environment.
func knownHostsPath() (string, error) {
	if path := os.Getenv(EnvKnownHosts); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate known_hosts (no home directory: %v): set %s", err, EnvKnownHosts)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

// defaultHostKeyCallback builds the verifier from the environment.
func defaultHostKeyCallback() (ssh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	return newHostKeyCallback(path, os.Getenv(EnvInsecureHostKey) == "1", os.Stderr)
}

// defaultHostKeyAlgorithms returns the host key algorithms to offer when
// dialing addr ("host:port"), or nil for the library default.
func defaultHostKeyAlgorithms(addr string) []string {
	if os.Getenv(EnvInsecureHostKey) == "1" {
		return nil
	}
	path, err := knownHostsPath()
	if err != nil {
		return nil
	}
	return knownHostKeyAlgorithms(path, addr)
}

// knownHostKeyAlgorithms lists the host key algorithms for the key types the
// known_hosts file at path holds for addr, followed by the other supported
// ones. Without it the library offers ECDSA first, and a host recorded only
// by its Ed25519 key (what "ssh once and accept" stores) looks changed.
// The fallback algorithms only matter when no known type is offered; the
// strict callback then reports the changed key instead of a bare
// "no common algorithm". It returns nil (library default) when the host is
// unknown or matched by an @cert-authority line.
func knownHostKeyAlgorithms(path, addr string) []string {
	verify, err := knownhosts.New(path)
	if err != nil {
		return nil
	}
	// A throwaway key never matches, so the error lists every known key.
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil
	}
	probe, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil
	}
	var keyErr *knownhosts.KeyError
	if !errors.As(verify(addr, &net.TCPAddr{IP: net.IPv4zero}, probe), &keyErr) || len(keyErr.Want) == 0 {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")

	var algos []string
	for _, known := range keyErr.Want {
		// Want also holds matching @cert-authority keys. Those vouch for
		// certificates of any type, so keep the library default.
		if known.Line >= 1 && known.Line <= len(lines) &&
			strings.HasPrefix(strings.TrimSpace(lines[known.Line-1]), "@cert-authority") {
			return nil
		}
		types := []string{known.Key.Type()}
		if types[0] == ssh.KeyAlgoRSA {
			types = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
		}
		for _, t := range types {
			if !slices.Contains(algos, t) {
				algos = append(algos, t)
			}
		}
	}
	for _, t := range ssh.SupportedAlgorithms().HostKeys {
		if !slices.Contains(algos, t) {
			algos = append(algos, t)
		}
	}
	return algos
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
