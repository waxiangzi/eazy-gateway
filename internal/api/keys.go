package api

import (
	"log"
	"net/http"
	"time"

	"github.com/eazy-gateway/eazy-gateway/internal/db"
)

// KeysHandler handles SSH key management endpoints.
type KeysHandler struct {
	db *db.DB
	km *KeyManager
}

// NewKeysHandler creates a KeysHandler with the given database and key manager.
func NewKeysHandler(d *db.DB, km *KeyManager) *KeysHandler {
	return &KeysHandler{db: d, km: km}
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
// references it. Returns 409 Conflict if any host uses this key. The private and
// public key files are removed together with the database record, so the key
// store does not accumulate files that no record points at (which would block
// the default key from being regenerated on the next startup).
func (h *KeysHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key id is required"})
		return
	}

	key, err := h.db.GetKey(id)
	if err != nil {
		log.Printf("ERROR: get key %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if key == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
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

	// Remove the files first: if that fails the record is still there and the
	// operator can retry, whereas the reverse order would orphan the file and
	// break the next startup.
	if err := h.km.RemoveKeyFiles(key); err != nil {
		log.Printf("ERROR: remove key files for %q: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete key files"})
		return
	}

	if err := h.db.DeleteKey(id); err != nil {
		log.Printf("ERROR: delete key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete key"})
		return
	}

	// Hosts require a key, so an empty key store leaves the console unable to
	// create anything until the next restart. Re-create the default keypair at
	// once instead; a deletable key was referenced by no host, so this is safe.
	if keys, err := h.db.ListKeys(); err == nil && len(keys) == 0 {
		if err := h.km.EnsureDefaultKey(h.db); err != nil {
			log.Printf("WARN: regenerate default key after deleting the last key: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
