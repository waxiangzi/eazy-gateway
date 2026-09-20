package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/eazy-gateway/eazy-gateway/internal/crypto"
	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// newKeyTestEnv builds a database and key manager over a fresh temporary data
// directory with an unlocked key store, mirroring how runServer wires them up.
func newKeyTestEnv(t *testing.T, password string) (*db.DB, *KeyManager) {
	t.Helper()

	dataDir := t.TempDir()
	d, err := db.Open(filepath.Join(dataDir, "eazy-gateway.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	SetCredential(password)
	t.Cleanup(func() { SetCredential("") })

	return d, NewKeyManager(dataDir)
}

// onlyKey returns the single stored key, failing the test otherwise.
func onlyKey(t *testing.T, d *db.DB) *db.Key {
	t.Helper()

	keys, err := d.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want exactly 1 stored key, got %d", len(keys))
	}
	return keys[0]
}

// addKey generates and stores an additional named keypair through the same path
// the server uses, returning the key record.
func addKey(t *testing.T, d *db.DB, km *KeyManager, name string) *db.Key {
	t.Helper()

	priv, pub, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	key, err := km.SaveKeyPEM(name, []byte(priv), pub)
	if err != nil {
		t.Fatalf("save key %q: %v", name, err)
	}
	if _, err := d.CreateKey(key); err != nil {
		t.Fatalf("store key %q: %v", name, err)
	}
	return key
}

// addKeyByName returns the stored key with the given name, failing the test otherwise.
func addKeyByName(t *testing.T, d *db.DB, name string) *db.Key {
	t.Helper()

	keys, err := d.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	for _, k := range keys {
		if k.Name == name {
			return k
		}
	}
	t.Fatalf("key %q not found", name)
	return nil
}

// TestDeleteKeyRemovesKeyFiles asserts that deleting a key removes its private
// and public files, not just the database record. Leaving the files behind made
// the next startup fail with "key file already exists".
func TestDeleteKeyRemovesKeyFiles(t *testing.T) {
	d, km := newKeyTestEnv(t, "hunter2hunter2")

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key: %v", err)
	}
	extra := addKey(t, d, km, "extra")

	handler := NewKeysHandler(d, km)
	req := httptest.NewRequest(http.MethodDelete, "/api/keys/"+extra.ID, nil)
	req.SetPathValue("id", extra.ID)
	rec := httptest.NewRecorder()
	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete key: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gone, _ := d.GetKey(extra.ID); gone != nil {
		t.Fatalf("key record %q still present after delete", extra.ID)
	}
	for _, path := range []string{extra.PrivateKeyPath, extra.PrivateKeyPath + ".pub"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("key file %q still on disk after delete (err=%v)", path, err)
		}
	}

	// The untouched default key must survive.
	keys, err := d.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "default" {
		t.Fatalf("want only the default key left, got %+v", keys)
	}
}

// TestDeleteLastKeyRegeneratesDefault covers recovery after the store was left
// undecryptable (CLI reset-password on a locked store): once the stale key is
// deletable, the system must not be left without any key at all.
func TestDeleteLastKeyRegeneratesDefault(t *testing.T) {
	d, km := newKeyTestEnv(t, "hunter2hunter2")

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key: %v", err)
	}
	key := onlyKey(t, d)

	handler := NewKeysHandler(d, km)
	req := httptest.NewRequest(http.MethodDelete, "/api/keys/"+key.ID, nil)
	req.SetPathValue("id", key.ID)
	rec := httptest.NewRecorder()
	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete key: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gone, _ := d.GetKey(key.ID); gone != nil {
		t.Fatalf("deleted key record %q still present", key.ID)
	}

	regenerated := onlyKey(t, d)
	if regenerated.ID == key.ID {
		t.Fatal("want a freshly generated key, got the deleted one")
	}
	pem, err := km.LoadKeyPEM(regenerated)
	if err != nil {
		t.Fatalf("load regenerated key: %v", err)
	}
	if !strings.HasPrefix(string(pem), pemHeader) {
		t.Fatalf("regenerated key is not a PEM private key: %q", pem[:min(len(pem), 32)])
	}

	// The startup call must be a no-op now instead of colliding with a file.
	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key after regeneration: %v", err)
	}
}

// TestEnsureDefaultKeyReplacesOrphanedFile covers a data directory where a key
// file survives without a database record (for example an interrupted delete).
// Startup must heal that state rather than refuse to boot.
func TestEnsureDefaultKeyReplacesOrphanedFile(t *testing.T) {
	d, km := newKeyTestEnv(t, "hunter2hunter2")

	if err := os.MkdirAll(km.KeysDir(), 0o700); err != nil {
		t.Fatalf("create keys dir: %v", err)
	}
	orphan := filepath.Join(km.KeysDir(), "default")
	if err := os.WriteFile(orphan, []byte("not a real key"), 0o600); err != nil {
		t.Fatalf("write orphan key: %v", err)
	}

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key over orphan: %v", err)
	}
	key := onlyKey(t, d)
	pem, err := km.LoadKeyPEM(key)
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	if !strings.HasPrefix(string(pem), pemHeader) {
		t.Fatalf("default key was not replaced with an encrypted PEM: %q", pem[:min(len(pem), 32)])
	}
}

// registerStoredKey inserts a key record whose private key file lives at path,
// simulating a legacy import that points outside the app's key store.
func registerStoredKey(t *testing.T, d *db.DB, name, path string) *db.Key {
	t.Helper()

	key := &db.Key{Name: name, PublicKey: "ssh-ed25519 AAAA", PrivateKeyPath: path}
	if _, err := d.CreateKey(key); err != nil {
		t.Fatalf("store key %q: %v", name, err)
	}
	return key
}

// plaintextKey is a legacy PEM that readStoredKey classifies as unencrypted.
var plaintextKey = []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nUSER-OWNED-KEY\n-----END OPENSSH PRIVATE KEY-----\n")

// TestReencryptAllLeavesKeysOutsideTheStoreUntouched asserts a password change
// never rewrites a file that is not ours. A database record pointing outside the
// key store (e.g. an operator's ~/.ssh/id_ed25519) must survive rotation
// verbatim instead of being replaced with our ciphertext.
func TestReencryptAllLeavesKeysOutsideTheStoreUntouched(t *testing.T) {
	d, km := newKeyTestEnv(t, "oldpassword1")

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key: %v", err)
	}
	inStore := onlyKey(t, d)

	outside := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(outside, plaintextKey, 0o600); err != nil {
		t.Fatalf("write external key: %v", err)
	}
	registerStoredKey(t, d, "system", outside)

	if err := km.ReencryptAll(d, "oldpassword1", "newpassword2"); err != nil {
		t.Fatalf("re-encrypt keys: %v", err)
	}

	if got, err := os.ReadFile(outside); err != nil || !bytes.Equal(got, plaintextKey) {
		t.Fatalf("key outside the store was modified: got %q, err=%v", got, err)
	}

	// The in-store key must still be rotated to the new password.
	SetCredential("newpassword2")
	if _, err := km.LoadKeyPEM(inStore); err != nil {
		t.Fatalf("in-store key not re-encrypted under the new password: %v", err)
	}
	SetCredential("oldpassword1")
	if _, err := km.LoadKeyPEM(inStore); err == nil {
		t.Fatal("in-store key still decrypts with the old password")
	}
}

// TestUpgradeLegacyFilesLeavesKeysOutsideTheStoreUntouched asserts the unlock
// migration encrypts legacy files inside the key store only, leaving files that
// are not ours alone.
func TestUpgradeLegacyFilesLeavesKeysOutsideTheStoreUntouched(t *testing.T) {
	d, km := newKeyTestEnv(t, "somepassword1")

	outside := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(outside, plaintextKey, 0o600); err != nil {
		t.Fatalf("write external key: %v", err)
	}
	registerStoredKey(t, d, "system", outside)

	inStorePath := filepath.Join(km.KeysDir(), "legacy")
	if err := os.MkdirAll(km.KeysDir(), 0o700); err != nil {
		t.Fatalf("create keys dir: %v", err)
	}
	if err := os.WriteFile(inStorePath, plaintextKey, 0o600); err != nil {
		t.Fatalf("write in-store legacy key: %v", err)
	}
	registerStoredKey(t, d, "legacy", inStorePath)

	if err := km.UpgradeLegacyFiles(d); err != nil {
		t.Fatalf("upgrade legacy files: %v", err)
	}

	if got, err := os.ReadFile(outside); err != nil || !bytes.Equal(got, plaintextKey) {
		t.Fatalf("key outside the store was modified: got %q, err=%v", got, err)
	}
	got, err := os.ReadFile(inStorePath)
	if err != nil {
		t.Fatalf("read in-store legacy key: %v", err)
	}
	if bytes.HasPrefix(bytes.TrimSpace(got), []byte(pemHeader)) {
		t.Fatal("in-store legacy key was not encrypted by the migration")
	}
}

// TestConcurrentDeletesLeaveOneKey guards the generation lock: deleting two
// keys at once must not leave two records pointing at the same default file.
func TestConcurrentDeletesLeaveOneKey(t *testing.T) {
	d, km := newKeyTestEnv(t, "hunter2hunter2")

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key: %v", err)
	}
	extra := addKey(t, d, km, "extra")

	defaultKey := addKeyByName(t, d, "default")

	handler := NewKeysHandler(d, km)
	delete := func(id string) {
		req := httptest.NewRequest(http.MethodDelete, "/api/keys/"+id, nil)
		req.SetPathValue("id", id)
		handler.HandleDelete(httptest.NewRecorder(), req)
	}

	var wg sync.WaitGroup
	for _, id := range []string{defaultKey.ID, extra.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			delete(id)
		}()
	}
	wg.Wait()

	keys, err := d.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want exactly 1 key after concurrent deletes, got %d", len(keys))
	}
	if _, err := km.LoadKeyPEM(keys[0]); err != nil {
		t.Fatalf("load surviving key: %v", err)
	}
}

// TestDeleteKeyRefusedWhenHostReferencesIt asserts the 409 contract survives the
// file-deletion change and that a referenced key's files are left alone.
func TestDeleteKeyRefusedWhenHostReferencesIt(t *testing.T) {
	d, km := newKeyTestEnv(t, "hunter2hunter2")

	if err := km.EnsureDefaultKey(d); err != nil {
		t.Fatalf("ensure default key: %v", err)
	}
	key := onlyKey(t, d)
	if _, err := d.CreateHost(&db.Host{Name: "h", Host: "127.0.0.1", Port: 22, User: "root", KeyID: key.ID}); err != nil {
		t.Fatalf("create host: %v", err)
	}

	handler := NewKeysHandler(d, km)
	req := httptest.NewRequest(http.MethodDelete, "/api/keys/"+key.ID, nil)
	req.SetPathValue("id", key.ID)
	rec := httptest.NewRecorder()
	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("delete referenced key: want 409, got %d (%s)", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(key.PrivateKeyPath); err != nil {
		t.Fatalf("referenced key file must survive a refused delete: %v", err)
	}
}
