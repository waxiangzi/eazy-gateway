package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// GenerateEd25519KeyPair generates an Ed25519 key pair.
// Returns the private key as an OpenSSH PEM string and the public key
// in authorized_keys format (e.g. "ssh-ed25519 AAAA...").
func GenerateEd25519KeyPair() (privatePEM string, publicKey string, err error) {
	// Generate Ed25519 private key
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Marshal private key to OpenSSH PEM format
	privBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := string(pem.EncodeToMemory(privBlock))

	// Create signer to get the public key
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("create signer: %w", err)
	}

	// Marshal public key to authorized_keys format
	pubBytes := ssh.MarshalAuthorizedKey(signer.PublicKey())
	pubKey := string(pubBytes)

	return privPEM, pubKey, nil
}
