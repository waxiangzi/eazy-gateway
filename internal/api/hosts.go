package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/tun-console/tun-console/internal/db"
)

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
	if req.Host == "" {
		return fmt.Errorf("host is required")
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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

// Delete handles DELETE /api/hosts/{id} — deletes a host config.
// Returns 409 Conflict if any tunnel references this host.
func (h *HostsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check tunnel references"})
		return
	}
	for _, t := range tunnels {
		if t.HostID == id {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "host is referenced by tunnel \"" + t.Name + "\"",
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
