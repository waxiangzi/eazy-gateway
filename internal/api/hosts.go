package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
	"github.com/eazy-gateway/eazy-gateway/internal/ssh"
)

var hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

func validHost(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return net.ParseIP(s[1:len(s)-1]) != nil
	}
	if net.ParseIP(s) != nil {
		return true
	}
	return hostnameRE.MatchString(s)
}

func hasControlChar(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// HostsHandler handles SSH host management endpoints.
type HostsHandler struct {
	db *db.DB
}

// NewHostsHandler creates a HostsHandler with the given database.
func NewHostsHandler(d *db.DB) *HostsHandler {
	return &HostsHandler{db: d}
}

// hostRequest is the JSON body accepted by Create and Update.
type hostRequest struct {
	Name  string `json:"name"`
	Host  string `json:"host"`
	Port  int    `json:"port"`
	User  string `json:"user"`
	KeyID string `json:"keyId"`
}

func (h *HostsHandler) validate(req *hostRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 128 || hasControlChar(req.Name) {
		return fmt.Errorf("name must not exceed 128 characters or contain control characters")
	}
	if req.Host == "" {
		return fmt.Errorf("host is required")
	}
	if !validHost(req.Host) {
		return fmt.Errorf("host must be a valid IP address or hostname")
	}
	if req.Port <= 0 {
		return fmt.Errorf("port must be greater than 0")
	}
	if req.Port > 65535 {
		return fmt.Errorf("port must be less than or equal to 65535")
	}
	if req.User == "" {
		return fmt.Errorf("user is required")
	}
	if len(req.User) > 128 || hasControlChar(req.User) {
		return fmt.Errorf("user must not exceed 128 characters or contain control characters")
	}
	if req.KeyID == "" {
		return fmt.Errorf("keyId is required")
	}
	key, err := h.db.GetKey(req.KeyID)
	if err != nil {
		return fmt.Errorf("get key: %w", err)
	}
	if key == nil {
		return fmt.Errorf("key %q not found", req.KeyID)
	}
	return nil
}

// List handles GET /api/hosts — returns all host configs.
func (h *HostsHandler) List(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.db.ListHosts()
	if err != nil {
		log.Printf("ERROR: list hosts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if hosts == nil {
		hosts = []*db.Host{}
	}
	writeJSON(w, http.StatusOK, hosts)
}

// Get handles GET /api/hosts/{id} — returns a single host config.
func (h *HostsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	host, err := h.db.GetHost(id)
	if err != nil {
		log.Printf("ERROR: get host %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if host == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	writeJSON(w, http.StatusOK, host)
}

// Create handles POST /api/hosts — creates a new host config.
func (h *HostsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req hostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := h.db.CreateHost(&db.Host{
		Name:  req.Name,
		Host:  req.Host,
		Port:  req.Port,
		User:  req.User,
		KeyID: req.KeyID,
	})
	if err != nil {
		log.Printf("ERROR: create host: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update handles PUT /api/hosts/{id} — updates an existing host config.
func (h *HostsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	existing, err := h.db.GetHost(id)
	if err != nil {
		log.Printf("ERROR: get host %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	var req hostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	host := &db.Host{
		ID:        existing.ID,
		Name:      req.Name,
		Host:      req.Host,
		Port:      req.Port,
		User:      req.User,
		KeyID:     req.KeyID,
		CreatedAt: existing.CreatedAt,
	}

	if err := h.db.UpdateHost(host); err != nil {
		log.Printf("ERROR: update host %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HostsHandler) Test(w http.ResponseWriter, r *http.Request) {
	var req hostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	key, err := h.db.GetKey(req.KeyID)
	if err != nil {
		log.Printf("ERROR: get key %q for host test: %v", req.KeyID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if key == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key not found"})
		return
	}

	keyPEM, err := os.ReadFile(key.PrivateKeyPath)
	if err != nil {
		log.Printf("ERROR: read private key %q: %v", key.PrivateKeyPath, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read key file"})
		return
	}

	cfg := &ssh.Config{
		ID:      "test-" + req.Host,
		SSHHost: req.Host,
		SSHPort: req.Port,
		SSHUser: req.User,
	}

	client, err := ssh.Connect(cfg, keyPEM)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	client.Close()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete handles DELETE /api/hosts/{id} — deletes a host config.
// Returns 409 Conflict if any tunnel references this host.
func (h *HostsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	host, err := h.db.GetHost(id)
	if err != nil {
		log.Printf("ERROR: get host %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if host == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}

	// Check if any tunnel references this host
	tunnels, err := h.db.ListTunnels()
	if err != nil {
		log.Printf("ERROR: list tunnels: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check connection references"})
		return
	}
	for _, t := range tunnels {
		if t.HostID == id {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "host is referenced by connection \"" + t.Name + "\"",
			})
			return
		}
	}

	if err := h.db.DeleteHost(id); err != nil {
		log.Printf("ERROR: delete host %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
