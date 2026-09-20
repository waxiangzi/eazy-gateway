package api

import (
	"log"
	"net/http"
	"time"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// KeysHandler handles SSH key management endpoints.
type KeysHandler struct {
	db            *db.DB
	adminPassword string
}

// NewKeysHandler creates a KeysHandler with the given database and admin password for encryption.
func NewKeysHandler(d *db.DB, adminPassword string) *KeysHandler {
	return &KeysHandler{
		db:            d,
		adminPassword: adminPassword,
	}
}

// HandleList handles GET /api/keys
func (h *KeysHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	keys, err := h.db.ListKeys()
	if err != nil {
		log.Printf("ERROR: list keys: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list keys"})
		return
	}

	type keyResponse struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		PublicKey string    `json:"publicKey"`
		CreatedAt time.Time `json:"createdAt"`
	}

	result := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		result = append(result, keyResponse{
			ID:        k.ID,
			Name:      k.Name,
			PublicKey: k.PublicKey,
			CreatedAt: k.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleDelete handles DELETE /api/keys/{id} — deletes a key if no host
// references it. Returns 409 Conflict if any host uses this key.
func (h *KeysHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key id is required"})
		return
	}

	// Check if any host references this key
	hosts, err := h.db.ListHosts()
	if err != nil {
		log.Printf("ERROR: list hosts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check host references"})
		return
	}

	var hostName string
	for _, host := range hosts {
		if host.KeyID == id {
			hostName = host.Name
			break
		}
	}
	if hostName != "" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "key is referenced by host \"" + hostName + "\"",
		})
		return
	}

	if err := h.db.DeleteKey(id); err != nil {
		log.Printf("ERROR: delete key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
