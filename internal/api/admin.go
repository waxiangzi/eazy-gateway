package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/eazy-gateway/eazy-gateway/internal/crypto"
	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// ChangePasswordHandler handles POST /api/admin/change-password. It verifies
// the old password, re-encrypts every SSH private key under the new password
// (direct re-encryption architecture), persists the new admin hash, and
// invalidates all sessions.
func ChangePasswordHandler(d *db.DB, sessions *SessionStore, km *KeyManager, onSuccess func()) http.HandlerFunc {
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
		if len(body.NewPassword) < 8 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
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

		// Re-encrypt all private keys first: decryption still uses the old
		// password because the admin record has not been updated yet.
		if err := km.ReencryptAll(d, body.OldPassword, body.NewPassword); err != nil {
			log.Printf("ERROR: re-encrypt private keys after password change; password NOT changed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to re-encrypt private keys; password not changed"})
			return
		}

		newHash, err := crypto.HashPassword(body.NewPassword)
		if err != nil {
			log.Printf("ERROR: hash password: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		cfg.PasswordHash = newHash
		if err := d.SetAdmin(cfg); err != nil {
			log.Printf("ERROR: update admin config: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Keep the runtime credential in sync so subsequent key reads use
		// the new password without a fresh login.
		SetCredential(body.NewPassword)

		// Invalidate all existing sessions
		sessions.ClearAll()

		if onSuccess != nil {
			onSuccess()
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ResetPasswordHandler handles POST /api/admin/reset-password.
// Generates a new random admin password, re-encrypts every SSH private key
// under it, hashes it, and stores it. Returns the plaintext password in the
// response. It must only be exposed on the local CLI unix socket.
func ResetPasswordHandler(d *db.DB, sessions *SessionStore, km *KeyManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// When the key store is locked (service restarted after a password
		// change, or the password was forgotten) the old password is unknown and
		// stored keys cannot be re-encrypted. Reset the admin password anyway so
		// the console stays reachable, and report that keys must be re-added.
		oldPassword := Credential()
		password, err := ResetAdminPassword(d)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		keysReencrypted := false
		if oldPassword != "" {
			if err := km.ReencryptAll(d, oldPassword, password); err != nil {
				log.Printf("WARN: re-encrypt private keys failed after admin password reset: %v", err)
			} else {
				keysReencrypted = true
			}
		} else {
			log.Printf("WARN: admin password reset while key store was locked; SSH private keys were not re-encrypted")
		}

		SetCredential(password)

		sessions.ClearAll()

		resp := map[string]interface{}{
			"status":   "ok",
			"password": password,
		}
		if !keysReencrypted && storedKeyCount(d) > 0 {
			resp["warning"] = "SSH private keys could not be re-encrypted (key store was locked). The stored keys are unusable: delete the hosts that reference them, then delete the keys, so a new default key can be generated."
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// storedKeyCount reports how many keys are stored, or 0 when the listing fails.
func storedKeyCount(d *db.DB) int {
	keys, err := d.ListKeys()
	if err != nil {
		return 0
	}
	return len(keys)
}
