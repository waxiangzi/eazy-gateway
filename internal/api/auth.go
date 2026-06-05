// Package api provides HTTP handlers and middleware for the tun-console REST API.
package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
)

const (
	cookieName    = "tun-console-session"
	sessionMaxAge = 24 * time.Hour
)

// SessionStore holds active sessions in memory.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token → expiry
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]time.Time),
	}
}

// Create generates a new session token and stores it with a 24h expiry.
func (s *SessionStore) Create() (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = time.Now().Add(sessionMaxAge)
	return token, nil
}

// Get returns true if the token exists and has not expired.
func (s *SessionStore) Get(token string) bool {
	s.mu.RLock()
	expiry, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

// Delete removes a session token from the store.
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// ClearAll invalidates all active sessions.
func (s *SessionStore) ClearAll() {
	s.mu.Lock()
	s.sessions = make(map[string]time.Time)
	s.mu.Unlock()
}

// randomToken generates 32 cryptographically random bytes and returns a base64-encoded string.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generatePassword creates a random 16-character alphanumeric password.
func generatePassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	pw := make([]byte, 16)
	for i := range pw {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		pw[i] = charset[n.Int64()]
	}
	return string(pw), nil
}

// EnsureAdmin checks if an admin config exists in the database. If not, it
// generates a random password, hashes it with bcrypt, and stores it. The
// plaintext password is printed to stdout. Returns the plaintext password on
// first creation, or an empty string if admin already existed.
func EnsureAdmin(d *db.DB) (string, error) {
	cfg, err := d.GetAdmin()
	if err != nil {
		return "", fmt.Errorf("get admin config: %w", err)
	}
	if cfg != nil {
		return "", nil
	}

	password, err := generatePassword()
	if err != nil {
		return "", err
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	if err := d.SetAdmin(&db.AdminConfig{PasswordHash: hash}); err != nil {
		return "", fmt.Errorf("set admin config: %w", err)
	}

	slog.Info("admin password generated", "password", password)
	return password, nil
}

// ResetAdminPassword generates a new random password, hashes it with bcrypt,
// and replaces the existing admin password hash in the database.
// Returns the new plaintext password. Returns an error if no admin exists.
func ResetAdminPassword(d *db.DB) (string, error) {
	cfg, err := d.GetAdmin()
	if err != nil {
		return "", fmt.Errorf("get admin config: %w", err)
	}
	if cfg == nil {
		return "", fmt.Errorf("no admin configured — run the server first to initialize")
	}

	password, err := generatePassword()
	if err != nil {
		return "", err
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	cfg.PasswordHash = hash
	if err := d.SetAdmin(cfg); err != nil {
		return "", fmt.Errorf("set admin config: %w", err)
	}

	return password, nil
}

// LoginHandler handles POST /api/login.
func LoginHandler(d *db.DB, sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if body.Password == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "password required"})
			return
		}

		cfg, err := d.GetAdmin()
		if err != nil {
			log.Printf("ERROR: get admin config: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if cfg == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin not configured"})
			return
		}

		if !crypto.VerifyPassword(body.Password, cfg.PasswordHash) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
			return
		}

		token, err := sessions.Create()
		if err != nil {
			log.Printf("ERROR: create session: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(sessionMaxAge.Seconds()),
		})

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// LogoutHandler handles POST /api/logout.
func LogoutHandler(sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err == nil && cookie.Value != "" {
			sessions.Delete(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// MeHandler handles GET /api/me.
func MeHandler(sessions *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" || !sessions.Get(cookie.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// AuthMiddleware checks the session cookie on all /api/* routes except /api/login.
type AuthMiddleware struct {
	sessions *SessionStore
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(sessions *SessionStore) *AuthMiddleware {
	return &AuthMiddleware{sessions: sessions}
}

// Wrap returns an http.Handler that checks the session cookie before passing
// requests to the next handler. Requests to /api/login and /api/admin/reset-password
// are allowed through without authentication.
func (m *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow login and password reset endpoints without session
		if r.URL.Path == "/api/login" || r.URL.Path == "/api/admin/reset-password" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" || !m.sessions.Get(cookie.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response with the given status code and body.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
