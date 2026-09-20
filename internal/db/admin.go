package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// AdminConfig holds the admin authentication data.
type AdminConfig struct {
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// GetAdmin retrieves the admin configuration. Returns nil if no admin has been configured yet.
func (db *DB) GetAdmin() (*AdminConfig, error) {
	var a *AdminConfig
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAdmin)
		data := b.Get([]byte("config"))
		if data == nil {
			return nil
		}
		var ac AdminConfig
		if err := json.Unmarshal(data, &ac); err != nil {
			return fmt.Errorf("unmarshal admin: %w", err)
		}
		a = &ac
		return nil
	})
	return a, err
}

// SetAdmin stores or updates the admin configuration.
func (db *DB) SetAdmin(a *AdminConfig) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAdmin)
		data, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("marshal admin: %w", err)
		}
		return b.Put([]byte("config"), data)
	})
}
