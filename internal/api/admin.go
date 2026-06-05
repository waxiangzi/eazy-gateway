package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
)

// ChangePasswordHandler handles POST /api/admin/change-password.
// It verifies the old password, updates the admin password hash, and
// invalidates all sessions. SSH private keys live on the filesystem and are
// not affected by a password change.
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

		// Invalidate all existing sessions
		sessions.ClearAll()

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ResetPasswordHandler handles POST /api/admin/reset-password.
// Generates a new random admin password, hashes it, and stores it.
// Returns the plaintext password in the response.
func ResetPasswordHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		password, err := ResetAdminPassword(d)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"password": password,
		})
	}
}
