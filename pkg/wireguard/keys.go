package wireguard

import (
	"fmt"
	"strings"
)

// GenerateKeyPair generates a new WireGuard private/public key pair using the
// `wg genkey` and `wg pubkey` commands.
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	privOut, _, err := runWG(nil, "genkey")
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKey = strings.TrimSpace(string(privOut))

	pubOut, _, err := runWG(strings.NewReader(privateKey), "pubkey")
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKey = strings.TrimSpace(string(pubOut))

	return privateKey, publicKey, nil
}
