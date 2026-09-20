package api

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/eazy-gateway/eazy-gateway/internal/crypto"
	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// ErrLocked is returned when private key material is needed while the key store
// is locked, which happens when the server process has no admin password in
// memory (a restart where the initial generated password is no longer in
// effect). An admin login unlocks the store.
var ErrLocked = errors.New("key store is locked: log in to unlock SSH private keys")

// ErrKeyUndecryptable is returned when a private key file exists but cannot be
// decrypted with the current admin password. That happens after the admin
// password was reset while the key store was locked (CLI recovery), because the
// password needed to re-encrypt the keys was unknown.
var ErrKeyUndecryptable = errors.New("private key cannot be decrypted with the current admin password")

// pemHeader opens every unencrypted private key file. Encrypted files hold
// base64, whose alphabet has no hyphen, so a stored file is encrypted exactly
// when its contents do not start with this marker.
const pemHeader = "-----BEGIN"

// currentCredential holds the admin password this process can use to decrypt
// and re-encrypt private keys. It is empty while the store is locked. Handlers
// update it on login, password change, and CLI password reset.
var currentCredential atomic.Value

// SetCredential records the admin password that private-key files are encrypted
// with (empty string locks the store).
func SetCredential(password string) { currentCredential.Store(password) }

// Credential returns the admin password in effect, or "" while locked.
func Credential() string {
	if pw, ok := currentCredential.Load().(string); ok {
		return pw
	}
	return ""
}

// KeyManager centralizes private-key file access. Private key material is
// stored AES-GCM encrypted at rest, with the key derived from the admin
// password, so a leaked data directory does not expose the PEM.
//
// On-disk layout of data/keys/<name>:
//
//	<b64(salt || nonce || ciphertext)>  encrypted under the admin password
//	-----BEGIN ...                      legacy plaintext PEM
//
// The encrypted form is exactly the format produced by internal/crypto. The
// store is locked while no password is available in memory: reads fail with
// ErrLocked, and an admin login unlocks the process.
type KeyManager struct {
	dataDir string
}

// NewKeyManager creates a KeyManager rooted at the given data directory.
func NewKeyManager(dataDir string) *KeyManager {
	return &KeyManager{dataDir: dataDir}
}

// KeysDir returns the directory holding private key files.
func (km *KeyManager) KeysDir() string {
	return filepath.Join(km.dataDir, "keys")
}

// keyErrorStatus maps a private-key access failure to the status code and
// client-facing message for an API response. A locked or undecryptable store is
// a condition the caller can act on, so both are reported as 503 with an
// explanation; any other failure is internal and its details stay in the log.
func keyErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, ErrLocked):
		return http.StatusServiceUnavailable, ErrLocked.Error()
	case errors.Is(err, ErrKeyUndecryptable):
		return http.StatusServiceUnavailable, ErrKeyUndecryptable.Error()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

// keyRewrite is a file whose contents are to be replaced by an atomic write.
type keyRewrite struct {
	path string
	data []byte
	perm os.FileMode
}

// writeKeyRewrites replaces every listed file with new contents, staging all of
// them as temporary siblings first and renaming them into place only after the
// whole set has been written and synced. A failure while staging removes the
// temporary files and leaves every original untouched, so callers can treat a
// rotated key store as either fully written or unchanged.
func writeKeyRewrites(rewrites []keyRewrite) error {
	staged := make([]string, 0, len(rewrites))
	discard := func() {
		for _, tmp := range staged {
			os.Remove(tmp)
		}
	}

	for _, rw := range rewrites {
		f, err := os.CreateTemp(filepath.Dir(rw.path), ".keytmp-*")
		if err != nil {
			discard()
			return fmt.Errorf("stage key file %q: %w", rw.path, err)
		}
		staged = append(staged, f.Name())

		_, err = f.Write(rw.data)
		if err == nil {
			err = f.Chmod(rw.perm)
		}
		if err == nil {
			err = f.Sync()
		}
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			discard()
			return fmt.Errorf("stage key file %q: %w", rw.path, err)
		}
	}

	for i, rw := range rewrites {
		if err := os.Rename(staged[i], rw.path); err != nil {
			discard()
			return fmt.Errorf("replace key file %q: %w", rw.path, err)
		}
		staged[i] = "" // renamed into place; nothing left to discard
	}
	return nil
}

// writeFileAtomic writes data to path via a temporary sibling file and a
// rename, so a crash or a full disk never exposes a truncated private key.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeKeyRewrites([]keyRewrite{{path: path, data: data, perm: perm}})
}

// readStoredKey returns the raw stored contents of a key file and whether it is
// encrypted rather than legacy plaintext.
func readStoredKey(path string) (raw []byte, encrypted bool, err error) {
	raw, err = os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	raw = bytes.TrimRight(raw, "\n")
	return raw, !bytes.HasPrefix(bytes.TrimSpace(raw), []byte(pemHeader)), nil
}

// decryptStored decrypts an encrypted key file's contents with an explicit
// password, as used by admin login (current password) and password rotation.
func decryptStored(raw []byte, password string) ([]byte, error) {
	plaintext, err := crypto.DecryptKey(raw, password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyUndecryptable, err)
	}
	return plaintext, nil
}

// encryptFile wraps plaintext into the on-disk encrypted format.
func encryptFile(plaintext []byte, password string) ([]byte, error) {
	ct, err := crypto.EncryptKey(plaintext, password)
	if err != nil {
		return nil, err
	}
	return append(ct, '\n'), nil
}

// decryptedFile returns the PEM for a stored file, decrypting it with an
// explicit password. Legacy plaintext files are returned as-is.
func decryptedFile(path string, password string) ([]byte, error) {
	raw, encrypted, err := readStoredKey(path)
	if err != nil {
		return nil, err
	}
	if !encrypted {
		return raw, nil
	}
	return decryptStored(raw, password)
}

// LoadKeyPEM returns the decrypted private key PEM for a stored key. Encrypted
// files require the admin password to be available, so a locked store fails
// with ErrLocked; legacy plaintext files are returned as-is until the migration
// on unlock rewrites them.
func (km *KeyManager) LoadKeyPEM(key *db.Key) ([]byte, error) {
	raw, encrypted, err := readStoredKey(key.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}
	if !encrypted {
		return raw, nil
	}
	password := Credential()
	if password == "" {
		return nil, ErrLocked
	}
	return decryptStored(raw, password)
}

// SaveKeyPEM encrypts plaintextPEM with the current admin password and writes it
// to keys/<name> (0600), plus the public key to keys/<name>.pub (0644) so it can
// be copied straight into a server's authorized_keys. Returns the stored key
// with its path filled in.
func (km *KeyManager) SaveKeyPEM(name string, plaintextPEM []byte, publicKey string) (*db.Key, error) {
	password := Credential()
	if password == "" {
		return nil, fmt.Errorf("admin password unavailable; cannot encrypt private key")
	}
	if err := os.MkdirAll(km.KeysDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create keys directory: %w", err)
	}
	path := filepath.Join(km.KeysDir(), filepath.Base(name))
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("key file %q already exists", path)
	}
	enc, err := encryptFile(plaintextPEM, password)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}
	if err := writeKeyRewrites([]keyRewrite{
		{path: path, data: enc, perm: 0o600},
		{path: path + ".pub", data: []byte(publicKey), perm: 0o644},
	}); err != nil {
		return nil, fmt.Errorf("write key pair: %w", err)
	}
	return &db.Key{Name: name, PublicKey: publicKey, PrivateKeyPath: path}, nil
}

// EnsureDefaultKey generates the default Ed25519 key pair when no key is stored
// yet, writing it through SaveKeyPEM so the private key is encrypted at rest. A
// fresh deployment has the generated admin password available and creates the
// key immediately; a locked store defers generation until an admin logs in.
func (km *KeyManager) EnsureDefaultKey(d *db.DB) error {
	keys, err := d.ListKeys()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return nil // already have keys
	}
	if Credential() == "" {
		log.Printf("WARN: key store locked; default SSH key will be generated after the next login")
		return nil
	}

	privPEM, pubKey, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		return fmt.Errorf("generate default key: %w", err)
	}

	key, err := km.SaveKeyPEM("default", []byte(privPEM), pubKey)
	if err != nil {
		return fmt.Errorf("save default key: %w", err)
	}
	if _, err := d.CreateKey(key); err != nil {
		return fmt.Errorf("save default key: %w", err)
	}

	log.Printf("INFO: SSH keys initialized at %s/", km.KeysDir())
	return nil
}

// UpgradeLegacyFiles rewrites every plaintext key file to the encrypted format
// using the current admin password. It is a no-op while the store is locked and
// safe to call repeatedly. A file that cannot be read or encrypted is logged
// and left as-is, so one bad key does not block startup.
func (km *KeyManager) UpgradeLegacyFiles(d *db.DB) error {
	password := Credential()
	if password == "" {
		return nil
	}
	keys, err := d.ListKeys()
	if err != nil {
		return err
	}

	var rewrites []keyRewrite
	for _, k := range keys {
		if k.PrivateKeyPath == "" {
			continue
		}
		raw, encrypted, err := readStoredKey(k.PrivateKeyPath)
		if err != nil {
			log.Printf("WARN: read key file %q for migration: %v", k.PrivateKeyPath, err)
			continue
		}
		if encrypted {
			continue
		}
		enc, err := encryptFile(raw, password)
		if err != nil {
			log.Printf("WARN: encrypt key file %q during migration: %v", k.PrivateKeyPath, err)
			continue
		}
		rewrites = append(rewrites, keyRewrite{path: k.PrivateKeyPath, data: enc, perm: 0o600})
	}
	if err := writeKeyRewrites(rewrites); err != nil {
		return fmt.Errorf("migrate legacy key files: %w", err)
	}
	return nil
}

// ReencryptAll re-encrypts every stored private-key file from the old password
// to the new one. Every key is decrypted and every replacement is staged before
// any file is swapped, so a failure leaves the originals in place and the caller
// must keep the old password.
//
// Call it before the admin record is updated, so decryption still uses the
// validated old password. Legacy plaintext files are picked up by the rotation
// and become encrypted under the new password.
func (km *KeyManager) ReencryptAll(d *db.DB, oldPassword, newPassword string) error {
	keys, err := d.ListKeys()
	if err != nil {
		return fmt.Errorf("list keys: %w", err)
	}

	var rewrites []keyRewrite
	for _, k := range keys {
		path := k.PrivateKeyPath
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("WARN: private key file %q missing; skipping re-encryption", path)
			continue
		}
		plaintext, err := decryptedFile(path, oldPassword)
		if err != nil {
			return fmt.Errorf("decrypt key file %q with old password: %w", path, err)
		}
		enc, err := encryptFile(plaintext, newPassword)
		if err != nil {
			return fmt.Errorf("encrypt key file %q with new password: %w", path, err)
		}
		rewrites = append(rewrites, keyRewrite{path: path, data: enc, perm: 0o600})
	}
	if err := writeKeyRewrites(rewrites); err != nil {
		return fmt.Errorf("re-encrypt key files: %w", err)
	}
	return nil
}
