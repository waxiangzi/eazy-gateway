package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Key represents an encrypted SSH private key stored in the database.
type Key struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	EncryptedPEM string    `json:"encryptedPem"`
	CreatedAt    time.Time `json:"createdAt"`
}

// CreateKey stores a new encrypted SSH key and returns its assigned ID.
func (db *DB) CreateKey(k *Key) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", err
	}
	k.ID = id
	k.CreatedAt = time.Now().UTC()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		data, err := json.Marshal(k)
		if err != nil {
			return fmt.Errorf("marshal key: %w", err)
		}
		return b.Put([]byte(id), data)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetKey retrieves an encrypted key by ID. Returns nil if not found.
func (db *DB) GetKey(id string) (*Key, error) {
	var k *Key
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		var key Key
		if err := json.Unmarshal(data, &key); err != nil {
			return fmt.Errorf("unmarshal key: %w", err)
		}
		k = &key
		return nil
	})
	return k, err
}

// DeleteKey removes an encrypted key by ID.
func (db *DB) DeleteKey(id string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketKeys).Delete([]byte(id))
	})
}

// ListKeys returns all stored encrypted keys.
func (db *DB) ListKeys() ([]*Key, error) {
	var keys []*Key
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketKeys)
		return b.ForEach(func(k, v []byte) error {
			var key Key
			if err := json.Unmarshal(v, &key); err != nil {
				return fmt.Errorf("unmarshal key %q: %w", string(k), err)
			}
			keys = append(keys, &key)
			return nil
		})
	})
	return keys, err
}