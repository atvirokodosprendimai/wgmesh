package wireguard

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func GenerateKeyPair() (privateKey, publicKey string, err error) {
	privCmd := exec.Command("wg", "genkey")
	var privOut bytes.Buffer
	privCmd.Stdout = &privOut

	if err := privCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKey = strings.TrimSpace(privOut.String())

	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(privateKey)
	var pubOut bytes.Buffer
	pubCmd.Stdout = &pubOut

	if err := pubCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKey = strings.TrimSpace(pubOut.String())

	return privateKey, publicKey, nil
}
