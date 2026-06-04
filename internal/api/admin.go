package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
)

// ChangePasswordHandler handles POST /api/admin/change-password.
// It verifies the old password, re-encrypts all SSH keys with the new password,
// updates the admin password hash, and invalidates all sessions.
func ChangePasswordHandler(d *db.DB, sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			OldPassword string `json:"oldPassword"`
			NewPassword string `json:"newPassword"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if body.OldPassword == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "old password required"})
			return
		}
		if body.NewPassword == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password required"})
			return
		}

		cfg, err := d.GetAdmin()
		if err != nil {
			log.Printf("ERROR: get admin config: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if cfg == nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin not configured"})
			return
		}

		// Verify old password against bcrypt hash
		if !crypto.VerifyPassword(body.OldPassword, cfg.PasswordHash) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid password"})
			return
		}

		// Retrieve all keys for re-encryption
		keys, err := d.ListKeys()
		if err != nil {
			log.Printf("ERROR: list keys: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if len(keys) > 0 {
			log.Printf("Re-encrypting %d SSH keys with new password (this may take time)", len(keys))
		}

		// Decrypt all keys with old password and re-encrypt with new password.
		// All decryption/re-encryption happens in memory first to avoid partial writes.
		type reencrypted struct {
			key *db.Key
			pem string
		}
		reencryptedKeys := make([]reencrypted, 0, len(keys))

		for _, key := range keys {
			decryptedPEM, err := crypto.DecryptKey([]byte(key.EncryptedPEM), body.OldPassword)
			if err != nil {
				log.Printf("ERROR: decrypt key %s: %v", key.ID, err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to re-encrypt keys"})
				return
			}

			encryptedPEM, err := crypto.EncryptKey(decryptedPEM, body.NewPassword)
			if err != nil {
				log.Printf("ERROR: encrypt key %s: %v", key.ID, err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to re-encrypt keys"})
				return
			}

			reencryptedKeys = append(reencryptedKeys, reencrypted{
				key: key,
				pem: string(encryptedPEM),
			})
		}

		// Hash the new password
		newHash, err := crypto.HashPassword(body.NewPassword)
		if err != nil {
			log.Printf("ERROR: hash password: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Atomically update all keys and admin config in a single DB transaction.
		// This ensures consistency: if the transaction fails, nothing is written.
		updatedKeys := make([]*db.Key, len(reencryptedKeys))
		for i, rk := range reencryptedKeys {
			rk.key.EncryptedPEM = rk.pem
			updatedKeys[i] = rk.key
		}

		cfg.PasswordHash = newHash
		if err := d.ReencryptKeysAndSetAdmin(updatedKeys, cfg); err != nil {
			log.Printf("ERROR: atomic update failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Invalidate all existing sessions
		sessions.ClearAll()

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
