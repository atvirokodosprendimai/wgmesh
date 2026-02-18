package wireguard

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GenerateKeyPair generates a new WireGuard private/public key pair using the
// `wg genkey` and `wg pubkey` commands.
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	privCmd := exec.Command(wgPath, "genkey")
	var privOut bytes.Buffer
	privCmd.Stdout = &privOut

	if err := privCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKey = strings.TrimSpace(privOut.String())

	pubCmd := exec.Command(wgPath, "pubkey")
	pubCmd.Stdin = strings.NewReader(privateKey)
	var pubOut bytes.Buffer
	pubCmd.Stdout = &pubOut

	if err := pubCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKey = strings.TrimSpace(pubOut.String())

	return privateKey, publicKey, nil
}
