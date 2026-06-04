package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltLen    = 16
	nonceLen   = 12
	keyLen     = 32
	pbkdf2Iter = 100000
)

// EncryptKey encrypts plaintext PEM bytes using AES-GCM with a key derived
// from the password via PBKDF2. Returns base64(salt || nonce || ciphertext).
func EncryptKey(plaintextPEM []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive 32-byte key using PBKDF2 with SHA-256
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iter, keyLen, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate random nonce
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt (Seal appends ciphertext to nonce, so we pass nonce as dst)
	ciphertext := gcm.Seal(nil, nonce, plaintextPEM, nil)

	// Concatenate salt || nonce || ciphertext and base64-encode
	out := make([]byte, 0, saltLen+nonceLen+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(out)))
	base64.StdEncoding.Encode(encoded, out)

	return encoded, nil
}

// DecryptKey decrypts a base64-encoded ciphertext produced by EncryptKey.
// Returns the original plaintext PEM bytes.
func DecryptKey(ciphertext []byte, password string) ([]byte, error) {
	// Decode base64
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(ciphertext)))
	n, err := base64.StdEncoding.Decode(decoded, ciphertext)
	if err != nil {
		return nil, err
	}
	decoded = decoded[:n]

	// Must at least contain salt + nonce
	if len(decoded) < saltLen+nonceLen {
		return nil, errors.New("ciphertext too short")
	}

	salt := decoded[:saltLen]
	nonce := decoded[saltLen : saltLen+nonceLen]
	ct := decoded[saltLen+nonceLen:]

	// Derive key using the same PBKDF2 parameters
	key := pbkdf2.Key([]byte(password), salt, pbkdf2Iter, keyLen, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Decrypt (Open verifies authentication tag and returns plaintext)
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
