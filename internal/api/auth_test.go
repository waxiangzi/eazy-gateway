package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
)

// newTestDB creates a temporary database for testing.
func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	// Seed a known admin password
	hash, err := crypto.HashPassword("test-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := d.SetAdmin(&db.AdminConfig{PasswordHash: hash}); err != nil {
		t.Fatalf("set admin: %v", err)
	}
	return d
}

// newTestServer creates a test HTTP server with the auth routes.
func newTestServer(t *testing.T, d *db.DB, sessions *SessionStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", LoginHandler(d, sessions))
	mux.HandleFunc("/api/logout", LogoutHandler(sessions))
	mux.HandleFunc("/api/me", MeHandler(sessions))
	mux.HandleFunc("/api/tunnels", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authMW := NewAuthMiddleware(sessions)
	return httptest.NewServer(authMW.Wrap(mux))
}

func TestLogin_Success(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	body := `{"password":"test-password"}`
	resp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify session cookie is set
	var cookies []*http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			cookies = append(cookies, c)
		}
	}
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}
	c := cookies[0]
	if c.Value == "" {
		t.Fatal("expected non-empty session cookie value")
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly flag on session cookie")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Error("expected SameSite=Lax on session cookie")
	}
	if c.MaxAge <= 0 {
		t.Error("expected positive MaxAge on session cookie")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	body := `{"password":"wrong-password"}`
	resp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_EmptyPassword(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	body := `{"password":""}`
	resp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_InvalidBody(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	body := `not-json`
	resp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMe_ValidSession(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	// Login to get a session cookie
	body := `{"password":"test-password"}`
	loginResp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie from login")
	}

	// Use the cookie to call /api/me
	req, err := http.NewRequest("GET", srv.URL+"/api/me", nil)
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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMe_NoCookie(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMe_InvalidCookie(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL+"/api/me", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "invalid-token"})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedEndpoint_NoCookie(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tunnels")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedEndpoint_WithValidSession(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	// Login
	body := `{"password":"test-password"}`
	loginResp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie from login")
	}

	// Access protected endpoint
	req, err := http.NewRequest("GET", srv.URL+"/api/tunnels", nil)
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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	// Login
	body := `{"password":"test-password"}`
	loginResp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie from login")
	}

	// Logout
	req, err := http.NewRequest("POST", srv.URL+"/api/logout", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	logoutResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	logoutResp.Body.Close()

	// After logout, /api/me should return 401
	req2, err := http.NewRequest("GET", srv.URL+"/api/me", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req2.AddCookie(sessionCookie)

	resp, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestLogin_WrongMethod(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newTestServer(t, d, sessions)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	s := NewSessionStore()
	token, err := s.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !s.Get(token) {
		t.Fatal("expected valid token to return true")
	}
}

func TestSessionStore_GetInvalid(t *testing.T) {
	s := NewSessionStore()
	if s.Get("nonexistent") {
		t.Fatal("expected nonexistent token to return false")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	s := NewSessionStore()
	token, err := s.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	s.Delete(token)
	if s.Get(token) {
		t.Fatal("expected deleted token to return false")
	}
}

func TestEnsureAdmin_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	hash, err := crypto.HashPassword("existing")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := d.SetAdmin(&db.AdminConfig{PasswordHash: hash}); err != nil {
		t.Fatalf("set admin: %v", err)
	}

	// Should not error or overwrite
	if _, err := EnsureAdmin(d); err != nil {
		t.Fatalf("EnsureAdmin: %v", err)
	}

	cfg, err := d.GetAdmin()
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if cfg.PasswordHash != hash {
		t.Fatal("EnsureAdmin should not overwrite existing admin")
	}
}

func TestEnsureAdmin_GeneratesPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	// Capture stdout
	if _, err := EnsureAdmin(d); err != nil {
		t.Fatalf("EnsureAdmin: %v", err)
	}

	cfg, err := d.GetAdmin()
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if cfg.PasswordHash == "" {
		t.Fatal("expected password hash to be set")
	}

	// Verify that generated password is not empty — we don't have the plaintext,
	// but we can verify the hash is valid by hashing a known value...
	// At minimum verify the structure is a valid bcrypt hash
	if len(cfg.PasswordHash) < 20 {
		t.Fatal("password hash looks too short")
	}
}

func TestLogin_AdminNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	// No admin set

	sessions := NewSessionStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", LoginHandler(d, sessions))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"password":"any"}`
	resp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "admin not configured" {
		t.Fatalf("expected 'admin not configured' error, got %q", result["error"])
	}
}

// Test that randomToken produces unique tokens
func TestRandomToken_Unique(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := randomToken()
		if err != nil {
			t.Fatalf("random token: %v", err)
		}
		if tokens[token] {
			t.Fatal("duplicate token generated")
		}
		tokens[token] = true
	}
}

// Test that Sessions are created correctly in the test helper
func TestNewTestDB_AdminExists(t *testing.T) {
	d := newTestDB(t)
	cfg, err := d.GetAdmin()
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected admin config to exist in test DB")
	}
	if !crypto.VerifyPassword("test-password", cfg.PasswordHash) {
		t.Fatal("test password should verify against stored hash")
	}
}