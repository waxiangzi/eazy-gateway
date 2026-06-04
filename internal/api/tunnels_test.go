package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	xssh "golang.org/x/crypto/ssh"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
	"github.com/tun-console/tun-console/internal/ssh"
)

// createTestKey inserts a test key into the DB and returns its ID.
func createTestKey(t *testing.T, d *db.DB) string {
	t.Helper()
	id, err := d.CreateKey(&db.Key{
		Name:         "test-key",
		EncryptedPEM: "fake-encrypted-data",
	})
	if err != nil {
		t.Fatalf("create test key: %v", err)
	}
	return id
}

// newTunnelTestServer creates a test HTTP server with tunnel CRUD routes plus
// the start/stop endpoints. The handler is wired with a real engine (shut down
// on cleanup) and the seeded test admin password so the start path can decrypt
// keys.
func newTunnelTestServer(t *testing.T, d *db.DB) *httptest.Server {
	t.Helper()
	srv, _ := newTunnelTestServerWithEngine(t, d)
	return srv
}

// newTunnelTestServerWithEngine is like newTunnelTestServer but also returns
// the engine so tests can assert on tunnel status directly.
func newTunnelTestServerWithEngine(t *testing.T, d *db.DB) (*httptest.Server, *ssh.TunnelEngine) {
	t.Helper()
	engine := ssh.NewTunnelEngine()
	t.Cleanup(engine.Shutdown)
	h := NewTunnelHandler(d, engine, "test-password")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tunnels", h.List)
	mux.HandleFunc("GET /api/tunnels/{id}", h.Get)
	mux.HandleFunc("POST /api/tunnels", h.Create)
	mux.HandleFunc("PUT /api/tunnels/{id}", h.Update)
	mux.HandleFunc("DELETE /api/tunnels/{id}", h.Delete)
	mux.HandleFunc("POST /api/tunnels/{id}/start", h.Start)
	mux.HandleFunc("POST /api/tunnels/{id}/stop", h.Stop)
	return httptest.NewServer(mux), engine
}

// validTunnelBody returns a JSON body for creating a valid tunnel config.
func validTunnelBody(keyID string) string {
	return `{"name":"test-tunnel","type":"local","sshHost":"example.com","sshPort":22,"sshUser":"root","keyId":"` + keyID + `","localAddr":"127.0.0.1:8080","remoteAddr":"localhost:80"}`
}

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestCreateTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(validTunnelBody(keyID)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}

	// Verify it was actually stored
	tc, err := d.GetTunnel(result["id"])
	if err != nil {
		t.Fatalf("get tunnel: %v", err)
	}
	if tc == nil {
		t.Fatal("tunnel not found after create")
	}
	if tc.Name != "test-tunnel" {
		t.Fatalf("expected name 'test-tunnel', got %q", tc.Name)
	}
	if string(tc.Type) != "local" {
		t.Fatalf("expected type 'local', got %q", tc.Type)
	}
	if tc.SSHHost != "example.com" {
		t.Fatalf("expected sshHost 'example.com', got %q", tc.SSHHost)
	}
	if tc.SSHPort != 22 {
		t.Fatalf("expected sshPort 22, got %d", tc.SSHPort)
	}
	if tc.SSHUser != "root" {
		t.Fatalf("expected sshUser 'root', got %q", tc.SSHUser)
	}
	if tc.KeyID != keyID {
		t.Fatalf("expected keyId %q, got %q", keyID, tc.KeyID)
	}
	if tc.LocalAddr != "127.0.0.1:8080" {
		t.Fatalf("expected localAddr '127.0.0.1:8080', got %q", tc.LocalAddr)
	}
}

func TestCreateTunnel_InvalidJSON(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(`not-json`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTunnel_EmptyName(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"","type":"local","sshHost":"h","sshPort":22,"sshUser":"u","keyId":"` + keyID + `"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
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

func TestCreateTunnel_InvalidType(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"invalid","sshHost":"h","sshPort":22,"sshUser":"u","keyId":"` + keyID + `"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "type must be one of: local, remote, dynamic" {
		t.Fatalf("expected type validation error, got %q", result["error"])
	}
}

func TestCreateTunnel_EmptySSHHost(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"local","sshHost":"","sshPort":22,"sshUser":"u","keyId":"` + keyID + `"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTunnel_InvalidSSHPort(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"local","sshHost":"h","sshPort":0,"sshUser":"u","keyId":"` + keyID + `"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTunnel_EmptySSHUser(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"local","sshHost":"h","sshPort":22,"sshUser":"","keyId":"` + keyID + `"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTunnel_NonexistentKeyID(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"local","sshHost":"h","sshPort":22,"sshUser":"u","keyId":"nonexistent-key"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != `key "nonexistent-key" not found` {
		t.Fatalf("expected key not found error, got %q", result["error"])
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestListTunnels_Empty(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tunnels")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var tunnels []*db.TunnelConfig
	if err := json.NewDecoder(resp.Body).Decode(&tunnels); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if tunnels == nil {
		t.Fatal("expected non-nil empty array")
	}
	if len(tunnels) != 0 {
		t.Fatalf("expected 0 tunnels, got %d", len(tunnels))
	}
}

func TestListTunnels_WithData(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Create two tunnels
	for i := 0; i < 2; i++ {
		name := "tunnel-" + string(rune('A'+i))
		body := `{"name":"` + name + `","type":"local","sshHost":"h","sshPort":22,"sshUser":"u","keyId":"` + keyID + `"}`
		resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("create tunnel %d: %v", i, err)
		}
		resp.Body.Close()
	}

	resp, err := http.Get(srv.URL + "/api/tunnels")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var tunnels []*db.TunnelConfig
	if err := json.NewDecoder(resp.Body).Decode(&tunnels); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(tunnels))
	}
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestGetTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Create a tunnel
	createResp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(validTunnelBody(keyID)))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer createResp.Body.Close()

	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	tunnelID := createResult["id"]

	// Fetch it
	resp, err := http.Get(srv.URL + "/api/tunnels/" + tunnelID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var tc db.TunnelConfig
	if err := json.NewDecoder(resp.Body).Decode(&tc); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if tc.ID != tunnelID {
		t.Fatalf("expected id %q, got %q", tunnelID, tc.ID)
	}
	if tc.Name != "test-tunnel" {
		t.Fatalf("expected name 'test-tunnel', got %q", tc.Name)
	}
}

func TestGetTunnel_NotFound(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tunnels/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetTunnel_EmptyID(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tunnels/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Go 1.22+ ServeMux does not match {id} on trailing slash, so the
	// request falls through to a 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Update tests
// ---------------------------------------------------------------------------

func TestUpdateTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Create a tunnel
	createResp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(validTunnelBody(keyID)))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer createResp.Body.Close()

	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	tunnelID := createResult["id"]

	// Update it
	newKeyID := createTestKey(t, d)
	updateBody := `{"name":"updated","type":"remote","sshHost":"updated.com","sshPort":2222,"sshUser":"admin","keyId":"` + newKeyID + `","remoteAddr":"0.0.0.0:9090"}`
	req, err := http.NewRequest("PUT", srv.URL+"/api/tunnels/"+tunnelID,
		strings.NewReader(updateBody))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	updateResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateResp.StatusCode)
	}

	// Verify changes
	tc, err := d.GetTunnel(tunnelID)
	if err != nil {
		t.Fatalf("get tunnel: %v", err)
	}
	if tc == nil {
		t.Fatal("tunnel not found after update")
	}
	if tc.Name != "updated" {
		t.Fatalf("expected name 'updated', got %q", tc.Name)
	}
	if string(tc.Type) != "remote" {
		t.Fatalf("expected type 'remote', got %q", tc.Type)
	}
	if tc.SSHHost != "updated.com" {
		t.Fatalf("expected sshHost 'updated.com', got %q", tc.SSHHost)
	}
	if tc.SSHPort != 2222 {
		t.Fatalf("expected sshPort 2222, got %d", tc.SSHPort)
	}
	if tc.SSHUser != "admin" {
		t.Fatalf("expected sshUser 'admin', got %q", tc.SSHUser)
	}
	if tc.KeyID != newKeyID {
		t.Fatalf("expected keyId %q, got %q", newKeyID, tc.KeyID)
	}
}

func TestUpdateTunnel_NotFound(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"t","type":"local","sshHost":"h","sshPort":22,"sshUser":"u","keyId":"` + keyID + `"}`
	req, err := http.NewRequest("PUT", srv.URL+"/api/tunnels/nonexistent",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateTunnel_InvalidBody(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Create first
	createResp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(validTunnelBody(keyID)))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer createResp.Body.Close()

	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	tunnelID := createResult["id"]

	// Update with invalid body
	req, err := http.NewRequest("PUT", srv.URL+"/api/tunnels/"+tunnelID,
		strings.NewReader("not-json"))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDeleteTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Create a tunnel
	createResp, err := http.Post(srv.URL+"/api/tunnels", "application/json",
		strings.NewReader(validTunnelBody(keyID)))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer createResp.Body.Close()

	var createResult map[string]string
	json.NewDecoder(createResp.Body).Decode(&createResult)
	tunnelID := createResult["id"]

	// Delete it
	req, err := http.NewRequest("DELETE", srv.URL+"/api/tunnels/"+tunnelID, nil)
	if err != nil {
		t.Fatalf("create DELETE request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify it's gone
	tc, err := d.GetTunnel(tunnelID)
	if err != nil {
		t.Fatalf("get tunnel: %v", err)
	}
	if tc != nil {
		t.Fatal("tunnel still exists after delete")
	}
}

func TestDeleteTunnel_NotFound(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	req, err := http.NewRequest("DELETE", srv.URL+"/api/tunnels/nonexistent", nil)
	if err != nil {
		t.Fatalf("create DELETE request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Type-specific tests
// ---------------------------------------------------------------------------

func TestCreateTunnel_RemoteType(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"remote-tunnel","type":"remote","sshHost":"remote.com","sshPort":22,"sshUser":"admin","keyId":"` + keyID + `","remoteAddr":"0.0.0.0:9090"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestCreateTunnel_DynamicType(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	body := `{"name":"dynamic-tunnel","type":"dynamic","sshHost":"socks.com","sshPort":22,"sshUser":"proxy","keyId":"` + keyID + `","dynamicAddr":"127.0.0.1:1080"}`
	resp, err := http.Post(srv.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

// ---------------------------------------------------------------------------
// Helper conversion tests
// ---------------------------------------------------------------------------

func TestToSSHConfig(t *testing.T) {
	tc := &db.TunnelConfig{
		ID:          "abc123",
		Name:        "test",
		Type:        db.TunnelTypeLocal,
		SSHHost:     "host",
		SSHPort:     2222,
		SSHUser:     "user",
		KeyID:       "key1",
		LocalAddr:   "127.0.0.1:8080",
		RemoteAddr:  "localhost:80",
		DynamicAddr: "",
	}
	sc := toSSHConfig(tc)
	if sc.ID != "abc123" {
		t.Fatalf("expected ID 'abc123', got %q", sc.ID)
	}
	if sc.Name != "test" {
		t.Fatalf("expected Name 'test', got %q", sc.Name)
	}
	if sc.Type != "local" {
		t.Fatalf("expected Type 'local', got %q", sc.Type)
	}
	if sc.SSHHost != "host" {
		t.Fatalf("expected SSHHost 'host', got %q", sc.SSHHost)
	}
	if sc.SSHPort != 2222 {
		t.Fatalf("expected SSHPort 2222, got %d", sc.SSHPort)
	}
	if sc.SSHUser != "user" {
		t.Fatalf("expected SSHUser 'user', got %q", sc.SSHUser)
	}
	if sc.LocalAddr != "127.0.0.1:8080" {
		t.Fatalf("expected LocalAddr '127.0.0.1:8080', got %q", sc.LocalAddr)
	}
	if sc.RemoteAddr != "localhost:80" {
		t.Fatalf("expected RemoteAddr 'localhost:80', got %q", sc.RemoteAddr)
	}
	if sc.DynamicAddr != "" {
		t.Fatalf("expected empty DynamicAddr, got %q", sc.DynamicAddr)
	}
}

func TestToDBTunnel(t *testing.T) {
	req := &tunnelRequest{
		Name:        "test",
		Type:        "remote",
		SSHHost:     "h",
		SSHPort:     22,
		SSHUser:     "u",
		KeyID:       "k1",
		LocalAddr:   "",
		RemoteAddr:  "0.0.0.0:9090",
		DynamicAddr: "",
	}
	tc := toDBTunnel("id123", req)
	if tc.ID != "id123" {
		t.Fatalf("expected ID 'id123', got %q", tc.ID)
	}
	if tc.Name != "test" {
		t.Fatalf("expected Name 'test', got %q", tc.Name)
	}
	if string(tc.Type) != "remote" {
		t.Fatalf("expected Type 'remote', got %q", tc.Type)
	}
	if tc.RemoteAddr != "0.0.0.0:9090" {
		t.Fatalf("expected RemoteAddr '0.0.0.0:9090', got %q", tc.RemoteAddr)
	}
}

// ---------------------------------------------------------------------------
// Start / Stop endpoint tests
// ---------------------------------------------------------------------------

// sshTestServer is a minimal in-process SSH server used to exercise the full
// Start path (decrypt key -> engine.Register -> engine.Start -> connect ->
// forwarding). It accepts public-key auth from any key and services
// direct-tcpip channels (needed by local/dynamic forwarders) by echoing,
// which is enough to prove the connection and forwarding came up.
type sshTestServer struct {
	addr     string
	hostKey  xssh.Signer
	listener net.Listener
}

// startSSHTestServer generates a host key, listens on a loopback port, and
// serves SSH connections until the listener is closed (on test cleanup).
func startSSHTestServer(t *testing.T) *sshTestServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := xssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &sshTestServer{addr: ln.Addr().String(), hostKey: signer, listener: ln}
	t.Cleanup(func() { _ = ln.Close() })

	go s.serve()
	return s
}

// hostPort splits the server address into host and integer port.
func (s *sshTestServer) hostPort(t *testing.T) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(s.addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi port: %v", err)
	}
	return host, port
}

func (s *sshTestServer) serve() {
	cfg := &xssh.ServerConfig{
		PublicKeyCallback: func(xssh.ConnMetadata, xssh.PublicKey) (*xssh.Permissions, error) {
			return &xssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(s.hostKey)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn, cfg)
	}
}

func (s *sshTestServer) handleConn(conn net.Conn, cfg *xssh.ServerConfig) {
	sshConn, chans, reqs, err := xssh.NewServerConn(conn, cfg)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sshConn.Close()
	go xssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			_ = newCh.Reject(xssh.UnknownChannelType, "only direct-tcpip")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go xssh.DiscardRequests(chReqs)
		go func(c io.ReadWriteCloser) {
			_, _ = io.Copy(c, c)
			_ = c.Close()
		}(ch)
	}
}

// createStartableTunnel inserts a real encrypted key (encrypted with the test
// admin password) and a tunnel of the given type pointing at the test SSH
// server, returning the tunnel ID.
func createStartableTunnel(t *testing.T, d *db.DB, srv *sshTestServer, typ, localAddr, dynamicAddr string) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	block, err := xssh.MarshalPrivateKey(priv, "test@tun-console")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(block)
	encrypted, err := crypto.EncryptKey(keyPEM, "test-password")
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}
	keyID, err := d.CreateKey(&db.Key{Name: "start-key", EncryptedPEM: string(encrypted)})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	host, port := srv.hostPort(t)
	tunnelID, err := d.CreateTunnel(&db.TunnelConfig{
		Name:        "startable",
		Type:        db.TunnelType(typ),
		SSHHost:     host,
		SSHPort:     port,
		SSHUser:     "tester",
		KeyID:       keyID,
		LocalAddr:   localAddr,
		DynamicAddr: dynamicAddr,
	})
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}
	return tunnelID
}

func TestStartTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	sshSrv := startSSHTestServer(t)
	srv, engine := newTunnelTestServerWithEngine(t, d)
	defer srv.Close()

	tunnelID := createStartableTunnel(t, d, sshSrv, "local", "127.0.0.1:0", "")

	resp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := engine.Status(tunnelID); got != ssh.StatusConnected {
		t.Fatalf("expected status %q, got %q", ssh.StatusConnected, got)
	}
}

func TestStartTunnel_NotFound(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/tunnels/nonexistent/start", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStartTunnel_WrongMethod(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tunnels/anything/start")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestStartTunnel_ConnectFailure(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	// Encrypt a valid key so decryption succeeds, but point the tunnel at a
	// dead port so the SSH dial fails and Start returns 500.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := xssh.MarshalPrivateKey(priv, "test@tun-console")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	encrypted, err := crypto.EncryptKey(pem.EncodeToMemory(block), "test-password")
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}
	keyID, err := d.CreateKey(&db.Key{Name: "k", EncryptedPEM: string(encrypted)})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	tunnelID, err := d.CreateTunnel(&db.TunnelConfig{
		Name:      "deadport",
		Type:      db.TunnelTypeLocal,
		SSHHost:   "127.0.0.1",
		SSHPort:   1,
		SSHUser:   "tester",
		KeyID:     keyID,
		LocalAddr: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	resp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestStopTunnel_Success(t *testing.T) {
	d := newTestDB(t)
	sshSrv := startSSHTestServer(t)
	srv, engine := newTunnelTestServerWithEngine(t, d)
	defer srv.Close()

	tunnelID := createStartableTunnel(t, d, sshSrv, "local", "127.0.0.1:0", "")

	startResp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start request failed: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start: expected 200, got %d", startResp.StatusCode)
	}

	stopResp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("stop request failed: %v", err)
	}
	defer stopResp.Body.Close()
	if stopResp.StatusCode != http.StatusOK {
		t.Fatalf("stop: expected 200, got %d", stopResp.StatusCode)
	}
	if got := engine.Status(tunnelID); got != ssh.StatusDisconnected {
		t.Fatalf("expected status %q after stop, got %q", ssh.StatusDisconnected, got)
	}
}

func TestStopTunnel_NotFound(t *testing.T) {
	d := newTestDB(t)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/tunnels/nonexistent/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStopTunnel_NotRunning(t *testing.T) {
	d := newTestDB(t)
	keyID := createTestKey(t, d)
	srv := newTunnelTestServer(t, d)
	defer srv.Close()

	tunnelID, err := d.CreateTunnel(&db.TunnelConfig{
		Name:    "idle",
		Type:    db.TunnelTypeLocal,
		SSHHost: "127.0.0.1",
		SSHPort: 22,
		SSHUser: "tester",
		KeyID:   keyID,
	})
	if err != nil {
		t.Fatalf("create tunnel: %v", err)
	}

	resp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// The tunnel exists but is not active, so the engine returns an error.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestStartTunnel_DynamicForwarding(t *testing.T) {
	d := newTestDB(t)
	sshSrv := startSSHTestServer(t)
	srv, engine := newTunnelTestServerWithEngine(t, d)
	defer srv.Close()

	// Listen on an ephemeral local SOCKS5 port.
	socksLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve socks port: %v", err)
	}
	socksAddr := socksLn.Addr().String()
	socksLn.Close()

	tunnelID := createStartableTunnel(t, d, sshSrv, "dynamic", "", socksAddr)

	resp, err := http.Post(srv.URL+"/api/tunnels/"+tunnelID+"/start", "application/json", nil)
	if err != nil {
		t.Fatalf("start request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := engine.Status(tunnelID); got != ssh.StatusConnected {
		t.Fatalf("expected status %q, got %q", ssh.StatusConnected, got)
	}

	// Drive the SOCKS5 server through a full CONNECT + echo round-trip to
	// confirm dynamic forwarding actually relays via the SSH connection.
	socksConn := dialSocks5(t, socksAddr)
	defer socksConn.Close()

	payload := []byte("hello-socks")
	if _, err := socksConn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	got := make([]byte, len(payload))
	socksConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(socksConn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected echo %q, got %q", payload, got)
	}
}

// dialSocks5 connects to a SOCKS5 proxy at addr, performs the no-auth
// handshake, and issues a CONNECT to an arbitrary target (serviced by the
// echo test SSH server), returning the established proxy connection.
func dialSocks5(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial socks: %v", err)
	}

	// Method selection: no-auth.
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		t.Fatalf("write greeting: %v", err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, methodReply); err != nil {
		conn.Close()
		t.Fatalf("read method reply: %v", err)
	}
	if methodReply[0] != 0x05 || methodReply[1] != 0x00 {
		conn.Close()
		t.Fatalf("unexpected method reply %v", methodReply)
	}

	// CONNECT to 127.0.0.1:9 (target is irrelevant; the echo server services
	// every direct-tcpip channel).
	req := []byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1}
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, 9)
	req = append(req, port...)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		t.Fatalf("write connect: %v", err)
	}

	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		conn.Close()
		t.Fatalf("read connect reply: %v", err)
	}
	if reply[1] != 0x00 {
		conn.Close()
		t.Fatalf("connect failed with status 0x%02x", reply[1])
	}
	return conn
}