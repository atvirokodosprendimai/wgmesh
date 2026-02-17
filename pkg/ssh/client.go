package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Client struct {
	conn *ssh.Client
}

func NewClient(host string, port int) (*Client, error) {
	var authMethods []ssh.AuthMethod

	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers))
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		keyPaths := []string{
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		}

		for _, keyPath := range keyPaths {
			if key, err := os.ReadFile(keyPath); err == nil {
				if signer, err := ssh.ParsePrivateKey(key); err == nil {
					authMethods = append(authMethods, ssh.PublicKeys(signer))
				}
			}
		}
	}

	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (c *Client) RunQuiet(cmd string) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	return session.Run(cmd)
}

func (c *Client) WriteFile(path string, content []byte, mode os.FileMode) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin: %w", err)
	}

	if err := session.Start(fmt.Sprintf("cat > %s && chmod %o %s", path, mode, path)); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	if _, err := io.Copy(stdin, strings.NewReader(string(content))); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to complete write: %w", err)
	}

	return nil
}

func (c *Client) RunWithStdin(cmd, stdinContent string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var outputBuf strings.Builder
	session.Stdout = &outputBuf
	session.Stderr = &outputBuf

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdin: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	if _, err := io.Copy(stdin, strings.NewReader(stdinContent)); err != nil {
		return "", fmt.Errorf("failed to write stdin: %w", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return outputBuf.String(), fmt.Errorf("command failed: %w", err)
	}

	return outputBuf.String(), nil
}
