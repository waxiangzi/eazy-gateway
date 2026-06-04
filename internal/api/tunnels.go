package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
	"github.com/tun-console/tun-console/internal/ssh"
)

// TunnelHandler holds shared dependencies for tunnel CRUD handlers.
// engine is optional; when set, Delete also stops any running tunnel and the
// Start/Stop endpoints become operational. adminPassword decrypts stored keys
// for the engine at start time; it is required for Start to function.
type TunnelHandler struct {
	db            *db.DB
	engine        *ssh.TunnelEngine
	adminPassword string
}

// NewTunnelHandler creates a TunnelHandler with the given dependencies. The
// admin password is used to decrypt SSH keys when starting a tunnel.
func NewTunnelHandler(d *db.DB, engine *ssh.TunnelEngine, adminPassword string) *TunnelHandler {
	return &TunnelHandler{db: d, engine: engine, adminPassword: adminPassword}
}

// tunnelRequest is the JSON body accepted by Create and Update.
type tunnelRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SSHHost     string `json:"sshHost"`
	SSHPort     int    `json:"sshPort"`
	SSHUser     string `json:"sshUser"`
	KeyID       string `json:"keyId"`
	LocalAddr   string `json:"localAddr,omitempty"`
	RemoteAddr  string `json:"remoteAddr,omitempty"`
	DynamicAddr string `json:"dynamicAddr,omitempty"`
}

// validate checks that all required fields are present and valid.
func (h *TunnelHandler) validate(req *tunnelRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch req.Type {
	case "local", "remote", "dynamic":
	default:
		return fmt.Errorf("type must be one of: local, remote, dynamic")
	}
	if req.SSHHost == "" {
		return fmt.Errorf("sshHost is required")
	}
	if req.SSHPort <= 0 {
		return fmt.Errorf("sshPort must be greater than 0")
	}
	if req.SSHUser == "" {
		return fmt.Errorf("sshUser is required")
	}
	if req.KeyID == "" {
		return fmt.Errorf("keyId is required")
	}
	// Verify key exists
	key, err := h.db.GetKey(req.KeyID)
	if err != nil {
		return fmt.Errorf("get key: %w", err)
	}
	if key == nil {
		return fmt.Errorf("key %q not found", req.KeyID)
	}
	return nil
}

// toDBTunnel converts a tunnelRequest + ID into a db.TunnelConfig.
func toDBTunnel(id string, req *tunnelRequest) *db.TunnelConfig {
	return &db.TunnelConfig{
		ID:          id,
		Name:        req.Name,
		Type:        db.TunnelType(req.Type),
		SSHHost:     req.SSHHost,
		SSHPort:     req.SSHPort,
		SSHUser:     req.SSHUser,
		KeyID:       req.KeyID,
		LocalAddr:   req.LocalAddr,
		RemoteAddr:  req.RemoteAddr,
		DynamicAddr: req.DynamicAddr,
	}
}

// toSSHConfig converts a db.TunnelConfig into an ssh.Config for the engine.
func toSSHConfig(tc *db.TunnelConfig) *ssh.Config {
	return &ssh.Config{
		ID:          tc.ID,
		Name:        tc.Name,
		Type:        string(tc.Type),
		SSHHost:     tc.SSHHost,
		SSHPort:     tc.SSHPort,
		SSHUser:     tc.SSHUser,
		LocalAddr:   tc.LocalAddr,
		RemoteAddr:  tc.RemoteAddr,
		DynamicAddr: tc.DynamicAddr,
	}
}

// List handles GET /api/tunnels — returns all tunnel configs.
func (h *TunnelHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tunnels, err := h.db.ListTunnels()
	if err != nil {
		log.Printf("ERROR: list tunnels: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if tunnels == nil {
		tunnels = []*db.TunnelConfig{}
	}

	writeJSON(w, http.StatusOK, tunnels)
}

// Get handles GET /api/tunnels/{id} — returns a single tunnel config.
func (h *TunnelHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	writeJSON(w, http.StatusOK, tc)
}

// Create handles POST /api/tunnels — creates a new tunnel config.
func (h *TunnelHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req tunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := h.db.CreateTunnel(toDBTunnel("", &req))
	if err != nil {
		log.Printf("ERROR: create tunnel: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update handles PUT /api/tunnels/{id} — updates an existing tunnel config.
func (h *TunnelHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	// Tunnel must exist
	existing, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	var req tunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.validate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	tc := toDBTunnel(id, &req)
	tc.CreatedAt = existing.CreatedAt // preserve original creation time

	if err := h.db.UpdateTunnel(tc); err != nil {
		log.Printf("ERROR: update tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete handles DELETE /api/tunnels/{id} — deletes a tunnel config.
// If the tunnel is currently running and an engine is available, it stops
// the tunnel before deleting the config.
func (h *TunnelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	// Verify tunnel exists
	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	// Stop the tunnel if it is currently running
	if h.engine != nil {
		if status := h.engine.Status(id); status != ssh.StatusDisconnected {
			log.Printf("stopping running tunnel %q before delete", id)
			if err := h.engine.Stop(id); err != nil {
				log.Printf("WARN: stop tunnel %q before delete: %v", id, err)
				// Continue with deletion even if stop fails
			}
			h.engine.Deregister(id)
		}
	}

	if err := h.db.DeleteTunnel(id); err != nil {
		log.Printf("ERROR: delete tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Start handles POST /api/tunnels/{id}/start. It loads the tunnel config and
// its SSH key, decrypts the key with the admin password, registers both with
// the engine, and brings the tunnel online (establishing the configured
// forwarding). Returns 200 on success, 404 if the tunnel is unknown, and 500
// when the engine is unavailable or the connection/forwarding cannot start.
func (h *TunnelHandler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if h.engine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "engine not available"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	key, err := h.db.GetKey(tc.KeyID)
	if err != nil {
		log.Printf("ERROR: get key %q for tunnel %q: %v", tc.KeyID, id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if key == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "tunnel key not found"})
		return
	}

	keyPEM, err := crypto.DecryptKey([]byte(key.EncryptedPEM), h.adminPassword)
	if err != nil {
		log.Printf("ERROR: decrypt key for tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decrypt key"})
		return
	}

	if err := h.engine.Register(toSSHConfig(tc), keyPEM); err != nil {
		log.Printf("ERROR: register tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := h.engine.Start(id); err != nil {
		log.Printf("ERROR: start tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// Stop handles POST /api/tunnels/{id}/stop. It validates the tunnel exists and
// delegates to the engine, which tears down forwarding and the SSH connection.
// Returns 200 on success, 404 if the tunnel is unknown, and 500 when the
// engine is unavailable or the tunnel is not running.
func (h *TunnelHandler) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if h.engine == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "engine not available"})
		return
	}

	tc, err := h.db.GetTunnel(id)
	if err != nil {
		log.Printf("ERROR: get tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if tc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	if err := h.engine.Stop(id); err != nil {
		log.Printf("ERROR: stop tunnel %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
