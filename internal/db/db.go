// Package db provides the bbolt-backed persistence layer for tunnels, keys, and admin config.
package db

import (
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	bucketTunnels = []byte("tunnels_v2")
	bucketHosts   = []byte("hosts")
	bucketKeys    = []byte("keys")
	bucketAdmin   = []byte("admin")
)

// DB wraps a bbolt database instance.
type DB struct {
	*bolt.DB
}

// Open opens (or creates) the database at path and initialises the required buckets.
func Open(path string) (*DB, error) {
	bdb, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	db := &DB{bdb}
	if err := db.initBuckets(); err != nil {
		bdb.Close()
		return nil, err
	}
	return db, nil
}

// initBuckets creates the required buckets if they do not already exist.
func (db *DB) initBuckets() error {
	return db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketTunnels, bucketHosts, bucketKeys, bucketAdmin} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %q: %w", string(name), err)
			}
		}
		return nil
	})
}

// Close closes the database.
func (db *DB) Close() error {
	return db.DB.Close()
}
