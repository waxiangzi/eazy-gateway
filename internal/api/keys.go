package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/tun-console/tun-console/internal/crypto"
	"github.com/tun-console/tun-console/internal/db"
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

// HandleCreate handles POST /api/keys — accepts { "name": "...", "pem": "..." },
// encrypts the PEM with the admin password, stores in DB, returns 201 + key ID.
func (h *KeysHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		PEM  string `json:"pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if body.PEM == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pem is required"})
		return
	}

	encrypted, err := crypto.EncryptKey([]byte(body.PEM), h.adminPassword)
	if err != nil {
		log.Printf("ERROR: encrypt key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt key"})
		return
	}

	key := &db.Key{
		Name:         body.Name,
		EncryptedPEM: string(encrypted),
	}

	id, err := h.db.CreateKey(key)
	if err != nil {
		log.Printf("ERROR: create key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save key"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// HandleList handles GET /api/keys — returns all keys with ID, Name, CreatedAt
// but without the encrypted PEM content.
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
		CreatedAt time.Time `json:"createdAt"`
	}

	result := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		result = append(result, keyResponse{
			ID:        k.ID,
			Name:      k.Name,
			CreatedAt: k.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleDelete handles DELETE /api/keys/{id} — deletes a key if no tunnel
// references it. Returns 409 Conflict if any tunnel uses this key.
func (h *KeysHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key id is required"})
		return
	}

	// Check if any tunnel references this key
	tunnels, err := h.db.ListTunnels()
	if err != nil {
		log.Printf("ERROR: list tunnels: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check tunnel references"})
		return
	}

	for _, t := range tunnels {
		if t.KeyID == id {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "key is referenced by tunnel \"" + t.Name + "\"",
			})
			return
		}
	}

	if err := h.db.DeleteKey(id); err != nil {
		log.Printf("ERROR: delete key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
