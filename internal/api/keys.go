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

// HandleCreate handles POST /api/keys — accepts { "name": "..." },
// generates an Ed25519 key pair, encrypts the private key with the admin
// password, stores both in DB, returns 201 + key info.
func (h *KeysHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	privPEM, pubKey, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		log.Printf("ERROR: generate key pair: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate key"})
		return
	}

	encrypted, err := crypto.EncryptKey([]byte(privPEM), h.adminPassword)
	if err != nil {
		log.Printf("ERROR: encrypt key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt key"})
		return
	}

	key := &db.Key{
		Name:         body.Name,
		PublicKey:    pubKey,
		EncryptedPEM: string(encrypted),
	}

	id, err := h.db.CreateKey(key)
	if err != nil {
		log.Printf("ERROR: create key: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save key"})
		return
	}

	// Log the private key to stdout on first key generation so it can be saved
	log.Printf("IMPORTANT: Generated SSH key %q (ID: %s)", body.Name, id)
	log.Printf("IMPORTANT: Private key for %q (SAVE THIS — it will not be shown again):\n%s", body.Name, privPEM)

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         id,
		"name":       body.Name,
		"publicKey":  pubKey,
		"privateKey": privPEM,
	})
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
