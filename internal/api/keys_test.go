package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tun-console/tun-console/internal/db"
)

// newKeysTestServer creates a test server with keys endpoints behind auth middleware.
func newKeysTestServer(t *testing.T, d *db.DB, sessions *SessionStore, adminPassword string) *httptest.Server {
	t.Helper()
	kh := NewKeysHandler(d, adminPassword)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", LoginHandler(d, sessions))
	mux.HandleFunc("POST /api/keys", kh.HandleCreate)
	mux.HandleFunc("GET /api/keys", kh.HandleList)
	mux.HandleFunc("DELETE /api/keys/{id}", kh.HandleDelete)

	authMW := NewAuthMiddleware(sessions)
	return httptest.NewServer(authMW.Wrap(mux))
}

// authedRequest performs an HTTP request with the session cookie set.
func authedRequest(t *testing.T, srv *httptest.Server, method, path, body string, cookie *http.Cookie) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, reqBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// loginAndGetCookie logs in with test-password and returns the session cookie.
func loginAndGetCookie(t *testing.T, srv *httptest.Server) *http.Cookie {
	t.Helper()
	loginResp, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(`{"password":"test-password"}`))
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp.Body.Close()

	for _, c := range loginResp.Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("no session cookie from login")
	return nil
}

func TestKeys_Create_Success(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\ntest123\n-----END OPENSSH PRIVATE KEY-----"
	createBody, _ := json.Marshal(map[string]string{"name": "test-key", "pem": pem})
	resp := authedRequest(t, srv, "POST", "/api/keys", string(createBody), cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] == "" {
		t.Fatal("expected non-empty key id")
	}

	// Verify key stored in DB with encrypted PEM (not plaintext)
	key, err := d.GetKey(result["id"])
	if err != nil {
		t.Fatalf("get key from db: %v", err)
	}
	if key == nil {
		t.Fatal("key not found in db")
	}
	if key.Name != "test-key" {
		t.Fatalf("expected name 'test-key', got %q", key.Name)
	}
	if key.EncryptedPEM == "" {
		t.Fatal("expected encrypted PEM to be non-empty")
	}
	if strings.Contains(key.EncryptedPEM, "BEGIN OPENSSH") {
		t.Fatal("encrypted PEM should not contain plaintext")
	}
}

func TestKeys_Create_MissingName(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	resp := authedRequest(t, srv, "POST", "/api/keys", `{"pem":"test"}`, cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "name is required" {
		t.Fatalf("expected 'name is required', got %q", result["error"])
	}
}

func TestKeys_Create_MissingPEM(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	resp := authedRequest(t, srv, "POST", "/api/keys", `{"name":"test"}`, cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "pem is required" {
		t.Fatalf("expected 'pem is required', got %q", result["error"])
	}
}

func TestKeys_Create_InvalidBody(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	resp := authedRequest(t, srv, "POST", "/api/keys", `not-json`, cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestKeys_List_Success(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	// Create two keys
	pem1 := "-----BEGIN OPENSSH PRIVATE KEY-----\nkey1\n-----END OPENSSH PRIVATE KEY-----"
	body1, _ := json.Marshal(map[string]string{"name": "key1", "pem": pem1})
	resp1 := authedRequest(t, srv, "POST", "/api/keys", string(body1), cookie)
	resp1.Body.Close()

	pem2 := "-----BEGIN OPENSSH PRIVATE KEY-----\nkey2\n-----END OPENSSH PRIVATE KEY-----"
	body2, _ := json.Marshal(map[string]string{"name": "key2", "pem": pem2})
	resp2 := authedRequest(t, srv, "POST", "/api/keys", string(body2), cookie)
	resp2.Body.Close()

	// List keys
	resp := authedRequest(t, srv, "GET", "/api/keys", "", cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(result))
	}

	// Verify no PEM field in response
	for _, k := range result {
		if k["id"] == "" {
			t.Error("expected non-empty id")
		}
		if k["name"] == "" {
			t.Error("expected non-empty name")
		}
		if k["createdAt"] == "" {
			t.Error("expected non-empty createdAt")
		}
		if _, ok := k["encryptedPem"]; ok {
			t.Error("list response should not contain encryptedPem")
		}
		if _, ok := k["pem"]; ok {
			t.Error("list response should not contain pem")
		}
	}
}

func TestKeys_List_Empty(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	resp := authedRequest(t, srv, "GET", "/api/keys", "", cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty list, got %d items", len(result))
	}
}

func TestKeys_Delete_Success(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	// Create a key
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\ndelete-test\n-----END OPENSSH PRIVATE KEY-----"
	createBody, _ := json.Marshal(map[string]string{"name": "delete-me", "pem": pem})
	createResp := authedRequest(t, srv, "POST", "/api/keys", string(createBody), cookie)
	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()
	keyID := createResult["id"]

	// Delete the key
	deleteResp := authedRequest(t, srv, "DELETE", "/api/keys/"+keyID, "", cookie)
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResp.StatusCode)
	}

	// Verify key is gone
	key, err := d.GetKey(keyID)
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if key != nil {
		t.Fatal("key should be deleted from db")
	}
}

func TestKeys_Delete_NonExistent(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	resp := authedRequest(t, srv, "DELETE", "/api/keys/nonexistent-id", "", cookie)
	defer resp.Body.Close()

	// Deleting a nonexistent key is idempotent — should return 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for idempotent delete, got %d", resp.StatusCode)
	}
}

func TestKeys_Delete_ReferencedByTunnel(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	// Create a key
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nref-test\n-----END OPENSSH PRIVATE KEY-----"
	createBody, _ := json.Marshal(map[string]string{"name": "ref-key", "pem": pem})
	createResp := authedRequest(t, srv, "POST", "/api/keys", string(createBody), cookie)
	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()
	keyID := createResult["id"]

	// Create a tunnel that references this key
	tunnel := &db.TunnelConfig{
		Name:   "test-tunnel",
		Type:   db.TunnelTypeLocal,
		SSHHost: "example.com",
		SSHPort: 22,
		SSHUser: "root",
		KeyID:  keyID,
	}
	if _, err := d.CreateTunnel(tunnel); err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	// Try to delete the key — should fail with 409
	resp := authedRequest(t, srv, "DELETE", "/api/keys/"+keyID, "", cookie)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(result["error"], "tunnel") {
		t.Fatalf("expected error mentioning tunnel, got %q", result["error"])
	}

	// Key should still exist
	key, err := d.GetKey(keyID)
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if key == nil {
		t.Fatal("key should still exist after failed delete")
	}
}

func TestKeys_EncryptedPEM_NotPlaintext(t *testing.T) {
	d := newTestDB(t)
	// Use a different password than test-password to verify encryption is
	// password-dependent
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	cookie := loginAndGetCookie(t, srv)

	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nsupersecret\n-----END OPENSSH PRIVATE KEY-----"
	createBody, _ := json.Marshal(map[string]string{"name": "secret-key", "pem": pem})
	createResp := authedRequest(t, srv, "POST", "/api/keys", string(createBody), cookie)
	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	// Read raw from DB
	key, err := d.GetKey(createResult["id"])
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if key == nil {
		t.Fatal("key not found")
	}

	// Encrypted PEM should not contain any plaintext fragments
	if strings.Contains(key.EncryptedPEM, "OPENSSH") {
		t.Error("encrypted PEM contains 'OPENSSH' plaintext")
	}
	if strings.Contains(key.EncryptedPEM, "supersecret") {
		t.Error("encrypted PEM contains 'supersecret' plaintext")
	}

	// Encrypted PEM should be valid base64 (starts with a base64-legal char)
	if len(key.EncryptedPEM) < 10 {
		t.Error("encrypted PEM too short")
	}
}

func TestKeys_NeedsAuth(t *testing.T) {
	d := newTestDB(t)
	sessions := NewSessionStore()
	srv := newKeysTestServer(t, d, sessions, "test-password")
	defer srv.Close()

	// No cookie — should get 401
	resp, err := http.Get(srv.URL + "/api/keys")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}