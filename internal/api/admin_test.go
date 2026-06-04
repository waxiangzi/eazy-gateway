package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
	"golang.org/x/crypto/ssh"
)

// newTestAdminServer creates a test HTTP server with the admin routes behind auth middleware.
func newTestAdminServer(t *testing.T, d *db.DB, sessions *SessionStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/change-password", ChangePasswordHandler(d, sessions))

	authMW := NewAuthMiddleware(sessions)
	return httptest.NewServer(authMW.Wrap(mux))
}

// seedKey creates a random ed25519 key, encrypts it with the given password, and stores it in DB.
func seedKey(t *testing.T, d *db.DB, password string) *db.Key {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pemBytes, err := ssh.MarshalPrivateKey(priv, "test-key")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	if pemBytes == nil {
		t.Fatalf("marshal private key returned nil")
	}

	encrypted, err := crypto.EncryptKey(pem.EncodeToMemory(pemBytes), password)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	k := &db.Key{
		Name:         "test-key",
		EncryptedPEM: string(encrypted),
	}
	id, err := d.CreateKey(k)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	k.ID = id
	return k
}

func TestChangePassword_Success_NoKeys(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"test-password","newPassword":"new-password-123"}`
	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	cfg, err := d.GetAdmin()
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if !crypto.VerifyPassword("new-password-123", cfg.PasswordHash) {
		t.Fatal("new password should verify against stored hash")
	}
	if crypto.VerifyPassword("test-password", cfg.PasswordHash) {
		t.Fatal("old password should no longer verify")
	}

	if sessions.Get(sessionCookie.Value) {
		t.Fatal("sessions should be cleared after password change")
	}
}

func TestChangePassword_Success_WithKeys(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	// Seed two keys encrypted with old password
	seedKey(t, d, "test-password")
	seedKey(t, d, "test-password")

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"test-password","newPassword":"new-password-456"}`
	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	keys, err := d.ListKeys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	for _, key := range keys {
		if _, err := crypto.DecryptKey([]byte(key.EncryptedPEM), "test-password"); err == nil {
			t.Fatal("key should not decrypt with old password after re-encryption")
		}
		if _, err := crypto.DecryptKey([]byte(key.EncryptedPEM), "new-password-456"); err != nil {
			t.Fatalf("key should decrypt with new password: %v", err)
		}
	}
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"wrong-password","newPassword":"new-password-123"}`
	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "invalid password" {
		t.Fatalf("expected 'invalid password' error, got %q", result["error"])
	}

	cfg, err := d.GetAdmin()
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if !crypto.VerifyPassword("test-password", cfg.PasswordHash) {
		t.Fatal("original password should still work")
	}
}

func TestChangePassword_EmptyNewPassword(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"test-password","newPassword":""}`
	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChangePassword_EmptyOldPassword(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"","newPassword":"new-password-123"}`
	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChangePassword_InvalidBody(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	req, err := http.NewRequest("POST", srv.URL+"/api/admin/change-password", strings.NewReader(`not-json`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChangePassword_WrongMethod(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()

	loginSrv := newTestServer(t, d, sessions)
	defer loginSrv.Close()
	sessionCookie := loginAndGetCookie(t, loginSrv)

	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/api/admin/change-password", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestChangePassword_Unauthenticated(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestAdminServer(t, d, sessions)
	defer srv.Close()

	body := `{"oldPassword":"test-password","newPassword":"new-password-123"}`
	resp, err := http.Post(srv.URL+"/api/admin/change-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestChangePassword_AdminNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	sessions := NewSessionStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/change-password", ChangePasswordHandler(d, sessions))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"oldPassword":"any","newPassword":"new-password-123"}`
	resp, err := http.Post(srv.URL+"/api/admin/change-password", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "admin not configured" {
		t.Fatalf("expected 'admin not configured' error, got %q", result["error"])
	}
}

func TestSessionStore_ClearAll(t *testing.T) {
	s := NewSessionStore()
	tokens := make([]string, 3)
	for i := range tokens {
		token, err := s.Create()
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		tokens[i] = token
	}

	s.ClearAll()

	for _, token := range tokens {
		if s.Get(token) {
			t.Fatal("expected all sessions to be cleared")
		}
	}

	newToken, err := s.Create()
	if err != nil {
		t.Fatalf("create session after clear: %v", err)
	}
	if !s.Get(newToken) {
		t.Fatal("new session should be valid after clear")
	}
}
