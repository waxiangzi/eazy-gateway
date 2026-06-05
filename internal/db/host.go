package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Host holds SSH server connection information.
type Host struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	User      string    `json:"user"`
	KeyID     string    `json:"keyId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateHost stores a new host configuration and returns its assigned ID.
func (db *DB) CreateHost(h *Host) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", err
	}
	h.ID = id
	now := time.Now().UTC()
	h.CreatedAt = now
	h.UpdatedAt = now

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHosts)
		data, err := json.Marshal(h)
		if err != nil {
			return fmt.Errorf("marshal host: %w", err)
		}
		return b.Put([]byte(id), data)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetHost retrieves a host configuration by ID. Returns nil if not found.
func (db *DB) GetHost(id string) (*Host, error) {
	var h *Host
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHosts)
		data := b.Get([]byte(id))
		if data == nil {
			return nil
		}
		var host Host
		if err := json.Unmarshal(data, &host); err != nil {
			return fmt.Errorf("unmarshal host: %w", err)
		}
		h = &host
		return nil
	})
	return h, err
}

// UpdateHost updates an existing host configuration.
func (db *DB) UpdateHost(h *Host) error {
	h.UpdatedAt = time.Now().UTC()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHosts)
		data, err := json.Marshal(h)
		if err != nil {
			return fmt.Errorf("marshal host: %w", err)
		}
		return b.Put([]byte(h.ID), data)
	})
}

// DeleteHost removes a host configuration by ID.
func (db *DB) DeleteHost(id string) error {
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketHosts).Delete([]byte(id))
	})
}

// ListHosts returns all host configurations.
func (db *DB) ListHosts() ([]*Host, error) {
	var hosts []*Host
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHosts)
		return b.ForEach(func(k, v []byte) error {
			var host Host
			if err := json.Unmarshal(v, &host); err != nil {
				return fmt.Errorf("unmarshal host %q: %w", string(k), err)
			}
			hosts = append(hosts, &host)
			return nil
		})
	})
	return hosts, err
}
