package db

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// GetSetting retrieves a string value by key from the settings bucket.
// Returns an empty string if the key does not exist.
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		data := b.Get([]byte(key))
		if data != nil {
			value = string(data)
		}
		return nil
	})
	return value, err
}

// SetSetting stores a string value under key in the settings bucket.
func (db *DB) SetSetting(key, value string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		if err := b.Put([]byte(key), []byte(value)); err != nil {
			return fmt.Errorf("put setting %q: %w", key, err)
		}
		return nil
	})
}
